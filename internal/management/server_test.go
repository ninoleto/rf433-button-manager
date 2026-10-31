package management

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return p
}
func testServer(t *testing.T) (*Server, *Token) {
	t.Helper()
	dir := t.TempDir()
	token, e := OpenToken(dir)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewServer(dir, net.JoinHostPort("127.0.0.1", strconv.Itoa(freePort(t))), token)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Start(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ready")) })); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s, token
}
func TestTokenRotationPersistsAndRevokesOld(t *testing.T) {
	dir := t.TempDir()
	token, e := OpenToken(dir)
	if e != nil {
		t.Fatal(e)
	}
	old := token.value
	next, e := token.Rotate(old)
	if e != nil {
		t.Fatal(e)
	}
	if token.Valid(old) || !token.Valid(next) || next == old {
		t.Fatal("rotation did not revoke old credential")
	}
	if _, e = token.Rotate(old); e == nil {
		t.Fatal("stale authenticated request rotated again")
	}
	reopen, e := OpenToken(dir)
	if e != nil || !reopen.Valid(next) {
		t.Fatal("rotation did not persist")
	}
	info, _ := os.Stat(filepath.Join(dir, "management.token"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("token permissions")
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				token.Valid(next)
			}
		})
	}
	_, _ = token.Rotate(next)
	wg.Wait()
}
func TestTokenWriteFailureKeepsOldCredential(t *testing.T) {
	s, token := testServer(t)
	old := token.value
	if e := os.Rename(filepath.Join(s.dir, "management.token"), filepath.Join(s.dir, "saved")); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(token.path, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := token.Rotate(old); e == nil || !token.Valid(old) {
		t.Fatal("failed write replaced current credential")
	}
}
func TestHTTPSStageConfirmAndRestart(t *testing.T) {
	s, token := testServer(t)
	old := s.config
	c := old
	c.HTTP = false
	c.HTTPS = true
	c.HTTPSPort = freePort(t)
	p, e := s.Stage(c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.dir, "network.json")); !os.IsNotExist(e) {
		t.Fatal("unconfirmed network settings were persisted")
	}
	if e = s.confirm(p.ID, net.JoinHostPort(old.Address, strconv.Itoa(old.HTTPPort)), false); e == nil {
		t.Fatal("confirmed HTTPS without using it")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(s.cert.CA) {
		t.Fatal("CA certificate missing")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}, Timeout: time.Second}
	defer client.CloseIdleConnections()
	resp, e := client.Get(c.URLs()[0])
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(raw) != "ready" {
		t.Fatal("TLS handler failed")
	}
	if e = s.confirm(p.ID, net.JoinHostPort(c.Address, strconv.Itoa(c.HTTPSPort)), true); e != nil {
		t.Fatal(e)
	}
	restarted, e := NewServer(s.dir, old.URLs()[0][7:], token)
	if e != nil || restarted.config != c {
		t.Fatal("confirmed network settings not retained", e)
	}
	ca := string(s.cert.CA)
	if e = s.certificateLocked(true, c.Address); e != nil || string(s.cert.CA) != ca {
		t.Fatal("leaf renewal changed trust authority", e)
	}
	block, _ := pem.Decode(s.cert.Cert)
	cert, _ := x509.ParseCertificate(block.Bytes)
	if cert.VerifyHostname(c.Address) != nil || cert.VerifyHostname("192.0.2.1") == nil {
		t.Fatal("IP SAN validation failed")
	}
}
func TestNetworkTimeoutAndBusyPortPreserveOldListener(t *testing.T) {
	s, _ := testServer(t)
	old := s.config
	s.confirmWindow = 20 * time.Millisecond
	c := old
	c.HTTPPort = freePort(t)
	if _, e := s.Stage(c); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		done := s.pending == nil
		s.mu.Unlock()
		if done {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	s.mu.Lock()
	pending := s.pending
	config := s.config
	s.mu.Unlock()
	if pending != nil || config != old {
		t.Fatal("timeout did not restore original settings")
	}
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	c.HTTPPort = ln.Addr().(*net.TCPAddr).Port
	if _, e = s.Stage(c); e == nil {
		t.Fatal("occupied listener accepted")
	}
	resp, e := http.Get(old.URLs()[0])
	if e != nil {
		t.Fatal("old listener lost", e)
	}
	resp.Body.Close()
}
func TestNetworkValidationAndCertificatePrivacy(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "8.8.8.8", "example.org", "224.0.0.1"} {
		if (Network{Address: host, HTTP: true, HTTPPort: 8787, HTTPSPort: 8788}).Validate() == nil {
			t.Fatal("unsafe bind accepted", host)
		}
	}
	s, _ := testServer(t)
	if e := s.certificateLocked(false, "127.0.0.1"); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(s.cert.CA), "PRIVATE KEY") {
		t.Fatal("certificate includes key")
	}
	info, _ := os.Stat(filepath.Join(s.dir, "tls.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("TLS private storage permissions")
	}
}

func TestCertificateDownloadPreparationRetainsPendingAddress(t *testing.T) {
	s, _ := testServer(t)
	c := s.config
	c.Address, c.HTTPS, c.HTTPSPort = "127.0.0.2", true, freePort(t)
	if _, err := s.Stage(c); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/manage/certificate/prepare", nil))
	if w.Code != 200 {
		t.Fatal("preparing download failed")
	}
	block, _ := pem.Decode(s.cert.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || cert.VerifyHostname(c.Address) != nil || cert.VerifyHostname(s.config.Address) != nil {
		t.Fatal("download preparation removed a required IP SAN")
	}
}

func TestManagementTokenPersistsPrivately(t *testing.T) {
	dir := t.TempDir()
	one, e := OpenToken(dir)
	if e != nil {
		t.Fatal(e)
	}
	two, e := OpenToken(dir)
	if e != nil || one.value != two.value || len(one.value) != 64 {
		t.Fatal("token not stable")
	}
	st, _ := os.Stat(filepath.Join(dir, "management.token"))
	if st.Mode().Perm() != 0600 {
		t.Fatal("unsafe token mode")
	}
	os.WriteFile(filepath.Join(dir, "management.token"), []byte("invalid"), 0600)
	if _, e = OpenToken(dir); e == nil {
		t.Fatal("corrupt token silently replaced")
	}
}
