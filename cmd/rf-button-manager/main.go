// RF433 Button Manager is a native Linux receive-only automation service.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/ninoleto/rf-button-manager/internal/app"
	"github.com/ninoleto/rf-button-manager/internal/receiver"
	"github.com/ninoleto/rf-button-manager/internal/webui"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func tokenFile(dir string) (string, error) {
	path := filepath.Join(dir, "management.token")
	raw, e := os.ReadFile(path)
	if e == nil {
		s := strings.TrimSpace(string(raw))
		if len(s) != 64 {
			return "", errors.New("invalid management.token; expected 64 hex characters")
		}
		if _, e = hex.DecodeString(s); e != nil {
			return "", e
		}
		if e = os.Chmod(path, 0600); e != nil {
			return "", e
		}
		return s, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	b := make([]byte, 32)
	if _, e = rand.Read(b); e != nil {
		return "", e
	}
	s := hex.EncodeToString(b)
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	if _, e = f.WriteString(s + "\n"); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	return s, nil
}
func validListen(address string) error {
	host, _, e := net.SplitHostPort(address)
	if e != nil {
		return e
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || (!ip.IsLoopback() && !ip.IsPrivate() && !tailscaleIP(ip)) {
		return errors.New("listen must use an explicit loopback, private LAN or Tailscale IP; wildcard/public binds are rejected")
	}
	return nil
}
func tailscaleIP(ip net.IP) bool { _, n, _ := net.ParseCIDR("100.64.0.0/10"); return n.Contains(ip) }
func main() {
	if e := run(); e != nil {
		log.Fatal(e)
	}
}
func run() error {
	listen := flag.String("listen", "127.0.0.1:8787", "Explicit private IP and port")
	dir := flag.String("data-dir", "./data", "Private state directory")
	rtl := flag.String("rtl433", "/usr/bin/rtl_433", "rtl_433 executable")
	demo := flag.Bool("demo", false, "Synthetic RF events; no hardware or webhook execution")
	publicOrigin := flag.String("public-origin", "", "Exact HTTPS browser origin for a trusted local reverse proxy")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *version {
		fmt.Println(app.Version)
		return nil
	}
	if *publicOrigin != "" {
		u, e := url.Parse(*publicOrigin)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("public-origin must be https://hostname[:port] without path")
		}
	}
	if e := validListen(*listen); e != nil {
		return e
	}
	if e := os.MkdirAll(*dir, 0700); e != nil {
		return e
	}
	if e := os.Chmod(*dir, 0700); e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(*dir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return errors.New("another RF433 Button Manager owns this data directory")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	token, e := tokenFile(*dir)
	if e != nil {
		return e
	}
	store, config, e := app.OpenStore(filepath.Join(*dir, "manager.db"))
	if e != nil {
		return e
	}
	defer store.Close()
	manager, e := app.New(store, config, *demo)
	if e != nil {
		return e
	}
	defer manager.Close()
	if !*demo {
		manager.EnableCaptureLearning(*rtl, "")
	}
	static, e := fs.Sub(webui.FS, "static")
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	receiverDone := make(chan struct{})
	go func() { defer close(receiverDone); runReceiver(ctx, manager, *rtl, *demo) }()
	defer func() { stop(); <-receiverDone }()
	server := &http.Server{Addr: *listen, Handler: manager.Handler(token, static, *publicOrigin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	ln, e := net.Listen("tcp", *listen)
	if e != nil {
		return e
	}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(ln) }()
	log.Printf("RF433 Button Manager %s listening on http://%s; management token: %s", app.Version, *listen, filepath.Join(*dir, "management.token"))
	select {
	case e = <-finished:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		e = server.Shutdown(shutdown)
		if e != nil {
			_ = server.Close()
		}
		return e
	}
}
func runReceiver(ctx context.Context, a *app.App, binary string, demo bool) {
	if demo {
		a.SetReceiver(receiver.Status{State: "demo", Message: "Simulated input; hardware and webhooks disabled"})
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.Receive([]byte(`{"model":"Demo-Switch433","codes":["{24}a1b201"],"description":"Synthetic demonstration only"}`))
			}
		}
	}
	runReceiverLoop(ctx, a, binary, receiver.Run)
}

type receiverRunner func(context.Context, receiver.Config, func([]byte), func(receiver.Status), ...func(receiver.PulsePacket)) error

func runReceiverLoop(ctx context.Context, a *app.App, binary string, run receiverRunner) {
	for {
		if ctx.Err() != nil {
			return
		}
		plan, generation := a.ReceiverPlan(binary)
		a.ReceiverPlanApplied(generation)
		child, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- run(child, plan, a.Receive, func(s receiver.Status) { a.SetReceiverFor(generation, s) }, func(p receiver.PulsePacket) { a.ReceivePulse(generation, p) })
		}()
		select {
		case <-ctx.Done():
			cancel()
			<-done
			return
		case <-a.SettingsChanges():
			cancel()
			<-done
		case err := <-done:
			cancel()
			if err != nil {
				a.SetReceiver(receiver.Status{State: "error", Message: "Receiver configuration rejected; review receiver settings and diagnostics"})
			}
			select {
			case <-ctx.Done():
				return
			case <-a.SettingsChanges():
			case <-time.After(30 * time.Second):
			}
		}
	}
}
