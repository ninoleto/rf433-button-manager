package management

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Network struct {
	Address   string `json:"address"`
	HTTP      bool   `json:"http"`
	HTTPPort  int    `json:"http_port"`
	HTTPS     bool   `json:"https"`
	HTTPSPort int    `json:"https_port"`
}
type Pending struct {
	ID        string    `json:"id"`
	Config    Network   `json:"config"`
	ExpiresAt time.Time `json:"expires_at"`
}
type listener struct {
	server *http.Server
	socket net.Listener
}
type Server struct {
	mu            sync.Mutex
	dir           string
	token         *Token
	config        Network
	pending       *Pending
	timer         *time.Timer
	listeners     map[string]listener
	handler       http.Handler
	cert          certificateFiles
	pair          tls.Certificate
	closed        bool
	Errors        chan error
	confirmWindow time.Duration
}

func PrivateIP(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	_, tail, _ := net.ParseCIDR("100.64.0.0/10")
	return ip.IsLoopback() || ip.IsPrivate() || tail.Contains(ip)
}
func (c Network) Validate() error {
	if !PrivateIP(c.Address) {
		return errors.New("choose a specific loopback, private LAN or Tailscale IP address")
	}
	if !c.HTTP && !c.HTTPS {
		return errors.New("enable HTTP or HTTPS")
	}
	if c.HTTPPort < 1024 || c.HTTPPort > 65535 || c.HTTPSPort < 1024 || c.HTTPSPort > 65535 {
		return errors.New("ports must be between 1024 and 65535")
	}
	if c.HTTP && c.HTTPS && c.HTTPPort == c.HTTPSPort {
		return errors.New("HTTP and HTTPS require different ports")
	}
	return nil
}
func (c Network) URLs() []string {
	var urls []string
	if c.HTTP {
		urls = append(urls, "http://"+net.JoinHostPort(c.Address, strconv.Itoa(c.HTTPPort)))
	}
	if c.HTTPS {
		urls = append(urls, "https://"+net.JoinHostPort(c.Address, strconv.Itoa(c.HTTPSPort)))
	}
	return urls
}
func NewServer(dir, initial string, token *Token) (*Server, error) {
	host, port, err := net.SplitHostPort(initial)
	if err != nil {
		return nil, err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	s := &Server{dir: dir, token: token, config: Network{Address: host, HTTP: true, HTTPPort: p, HTTPSPort: 8788}, listeners: map[string]listener{}, Errors: make(chan error, 1), confirmWindow: 3 * time.Minute}
	path := filepath.Join(dir, "network.json")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return nil, errors.New("network.json must be a regular file")
	}
	if raw, e := os.ReadFile(path); e == nil {
		if json.Unmarshal(raw, &s.config) != nil {
			return nil, errors.New("invalid network.json; preserve it and use the documented recovery procedure")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if err = s.config.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Server) Start(handler http.Handler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
	if s.config.HTTPS {
		if err := s.certificateLocked(false, s.config.Address); err != nil {
			return err
		}
	}
	if err := s.openLocked(s.config); err != nil {
		s.retainLocked(Network{})
		return err
	}
	return nil
}
func (s *Server) certificateLocked(renew bool, hosts ...string) error {
	files, pair, err := loadCertificate(s.dir, hosts, renew)
	if err != nil {
		return err
	}
	s.cert = files
	s.pair = pair
	return nil
}
func (s *Server) openLocked(c Network) error {
	for _, url := range c.URLs() {
		if _, ok := s.listeners[url]; ok {
			continue
		}
		scheme, address, _ := strings.Cut(url, "://")
		ln, err := net.Listen("tcp", address)
		if err != nil {
			return errors.New("cannot open the selected address/port; check that the address belongs to this computer and the port is free")
		}
		srv := &http.Server{Handler: s.handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
		s.listeners[url] = listener{srv, ln}
		if scheme == "https" {
			ln = tls.NewListener(ln, &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				s.mu.Lock()
				defer s.mu.Unlock()
				pair := s.pair
				return &pair, nil
			}})
		}
		go func() {
			if e := srv.Serve(ln); e != nil && !errors.Is(e, http.ErrServerClosed) && !errors.Is(e, net.ErrClosed) {
				select {
				case s.Errors <- errors.New("management listener stopped unexpectedly"):
				default:
				}
			}
		}()
	}
	return nil
}
func (s *Server) retainLocked(config Network) {
	keep := map[string]bool{}
	for _, u := range config.URLs() {
		keep[u] = true
	}
	for u, l := range s.listeners {
		if !keep[u] {
			delete(s.listeners, u)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = l.server.Shutdown(ctx)
				_ = l.server.Close()
				_ = l.socket.Close()
			}()
		}
	}
}
func (s *Server) Stage(c Network) (*Pending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("service stopping")
	}
	if s.pending != nil {
		return nil, errors.New("confirm or cancel the pending network change first")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c == s.config {
		return nil, errors.New("network settings are unchanged")
	}
	if c.HTTPS {
		if err := s.certificateLocked(false, s.config.Address, c.Address); err != nil {
			return nil, err
		}
	}
	if err := s.openLocked(c); err != nil {
		s.retainLocked(s.config)
		return nil, err
	}
	id, err := secret()
	if err != nil {
		s.retainLocked(s.config)
		return nil, err
	}
	s.pending = &Pending{ID: id, Config: c, ExpiresAt: time.Now().Add(s.confirmWindow)}
	s.timer = time.AfterFunc(s.confirmWindow, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.pending != nil && s.pending.ID == id {
			s.pending = nil
			s.retainLocked(s.config)
		}
	})
	copy := *s.pending
	return &copy, nil
}
func (s *Server) confirm(id, host string, secure bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil || s.pending.ID != id || time.Now().After(s.pending.ExpiresAt) {
		return errors.New("network change expired; previous settings retained")
	}
	c := s.pending.Config
	port := c.HTTPPort
	if c.HTTPS {
		port = c.HTTPSPort
	}
	if host != net.JoinHostPort(c.Address, strconv.Itoa(port)) || secure != c.HTTPS {
		return errors.New("open the proposed connection and confirm there; HTTPS must be tested when enabled")
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = AtomicPrivate(filepath.Join(s.dir, "network.json"), raw); err != nil {
		return errors.New("network settings could not be saved; previous settings retained")
	}
	s.config = c
	s.pending = nil
	s.timer.Stop()
	s.retainLocked(c)
	return nil
}
func (s *Server) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.pending = nil
	s.retainLocked(s.config)
}
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
	}
	s.pending = nil
	listeners := s.listeners
	s.listeners = map[string]listener{}
	s.mu.Unlock()
	for _, l := range listeners {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = l.server.Shutdown(ctx)
		cancel()
		_ = l.server.Close()
		_ = l.socket.Close()
	}
}
func (s *Server) URLs() []string { s.mu.Lock(); defer s.mu.Unlock(); return s.config.URLs() }
func interfaces() []map[string]string {
	rows := []map[string]string{}
	list, _ := net.Interfaces()
	for _, i := range list {
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, _ := i.Addrs()
		for _, a := range addresses {
			host, _, e := net.ParseCIDR(a.String())
			if e == nil && PrivateIP(host.String()) {
				rows = append(rows, map[string]string{"address": host.String(), "interface": i.Name})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["address"] < rows[j]["address"] })
	return rows
}
func response(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func body(w http.ResponseWriter, r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		return errors.New("invalid JSON body or unsupported fields")
	}
	return nil
}
func (s *Server) status() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending *Pending
	if s.pending != nil {
		copy := *s.pending
		pending = &copy
	}
	cert := map[string]string{}
	if block, _ := pem.Decode(s.cert.Cert); block != nil {
		if c, e := x509.ParseCertificate(block.Bytes); e == nil {
			cert["expires_at"] = c.NotAfter.Format(time.RFC3339)
		}
	}
	if block, _ := pem.Decode(s.cert.CA); block != nil {
		sum := sha256.Sum256(block.Bytes)
		cert["ca_sha256"] = hex.EncodeToString(sum[:])
	}
	return map[string]any{"config": s.config, "urls": s.config.URLs(), "pending": pending, "interfaces": interfaces(), "certificate": cert}
}

// Handler is mounted ONLY beneath App's authenticated, same-origin /api/ wrapper.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/manage/network", func(w http.ResponseWriter, r *http.Request) { response(w, 200, s.status()) })
	mux.HandleFunc("POST /api/manage/network/stage", func(w http.ResponseWriter, r *http.Request) {
		var c Network
		err := body(w, r, &c)
		var p *Pending
		if err == nil {
			p, err = s.Stage(c)
		}
		if err != nil {
			response(w, 400, map[string]string{"error": err.Error()})
			return
		}
		response(w, 200, p)
	})
	mux.HandleFunc("POST /api/manage/network/confirm", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ID string `json:"id"`
		}
		err := body(w, r, &b)
		if err == nil {
			err = s.confirm(b.ID, r.Host, r.TLS != nil)
		}
		if err != nil {
			response(w, 400, map[string]string{"error": err.Error()})
			return
		}
		response(w, 200, s.status())
	})
	mux.HandleFunc("POST /api/manage/network/cancel", func(w http.ResponseWriter, r *http.Request) { s.cancel(); response(w, 200, s.status()) })
	mux.HandleFunc("POST /api/manage/token/rotate", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Confirm bool `json:"confirm"`
		}
		if body(w, r, &b) != nil || !b.Confirm {
			response(w, 400, map[string]string{"error": "explicit token replacement confirmation required"})
			return
		}
		next, err := s.token.Rotate(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil {
			response(w, 400, map[string]string{"error": err.Error()})
			return
		}
		response(w, 200, map[string]string{"token": next})
	})
	mux.HandleFunc("POST /api/manage/certificate/prepare", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		hosts := []string{s.config.Address}
		if s.pending != nil {
			hosts = append(hosts, s.pending.Config.Address)
		}
		err := s.certificateLocked(false, hosts...)
		s.mu.Unlock()
		if err != nil {
			response(w, 400, map[string]string{"error": "certificate could not be prepared; inspect private TLS storage locally"})
			return
		}
		response(w, 200, s.status())
	})
	mux.HandleFunc("POST /api/manage/certificate/renew", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		var err error
		if s.pending != nil {
			err = errors.New("finish the pending network change first")
		} else {
			err = s.certificateLocked(true, s.config.Address)
		}
		s.mu.Unlock()
		if err != nil {
			response(w, 400, map[string]string{"error": "certificate renewal failed; finish pending network changes and check storage"})
			return
		}
		response(w, 200, s.status())
	})
	mux.HandleFunc("GET /api/manage/certificate", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		cert := append([]byte(nil), s.cert.CA...)
		s.mu.Unlock()
		if len(cert) == 0 {
			response(w, 404, map[string]string{"error": "prepare the certificate first"})
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Content-Disposition", `attachment; filename="rf433-button-manager-ca.crt"`)
		_, _ = w.Write(cert)
	})
	return mux
}
