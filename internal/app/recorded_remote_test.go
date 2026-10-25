package app

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Optional public IQ corpus (scripts/fetch-rf-testdata.py). This uses real
// upstream recordings with rtl_433 file input; it never opens a USB receiver.
func TestRecordedRemoteEndToEnd(t *testing.T) {
	dir := os.Getenv("RFB_RF_TESTDATA")
	if dir == "" {
		t.Skip("set RFB_RF_TESTDATA to the checksum-verified upstream recordings")
	}
	rtl, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ev1527", "pt2262", "akhan", "ht680"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			raw, err := exec.CommandContext(ctx, rtl, "-c", "/dev/null", "-r", filepath.Join(dir, name+"_433.92M_250k.cu8"), "-F", "json").Output()
			if err != nil {
				t.Fatal(err)
			}
			var events [][]byte
			scan := bufio.NewScanner(strings.NewReader(string(raw)))
			for scan.Scan() {
				if strings.HasPrefix(scan.Text(), "{") {
					events = append(events, append([]byte(nil), scan.Bytes()...))
				}
			}
			if len(events) == 0 {
				t.Fatal("upstream recording produced no decoder events")
			}
			a, c, r := captureApp(t)
			startCapture(t, a, r.Buttons[0].ID)
			for press := 0; press < 3; press++ {
				if press > 0 {
					c.advance(1100 * time.Millisecond)
					if _, err = a.ArmLearning(); err != nil {
						t.Fatal(err)
					}
				}
				for _, event := range events {
					a.Receive(event)
				}
				if a.learning == nil || a.learning.Presses != press+1 {
					t.Fatalf("recording not learned: %+v", a.learning)
				}
			}
			if err = a.ConfirmLearning(); err != nil {
				t.Fatal(err)
			}
			config := a.Backup()
			// Load the persisted configuration into another application instance.
			store, loaded, err := func() (*Store, Config, error) {
				path := filepath.Join(t.TempDir(), "persist.db")
				s, _, e := OpenStore(path)
				if e != nil {
					return nil, Config{}, e
				}
				if e = s.Save(config); e != nil {
					return nil, Config{}, e
				}
				s.Close()
				s, loaded, e := OpenStore(path)
				if e == nil {
					t.Cleanup(func() { s.Close() })
				}
				return s, loaded, e
			}()
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Remotes[0].Buttons[0].Signature != config.Remotes[0].Buttons[0].Signature || loaded.Remotes[0].Buttons[0].Enabled {
				t.Fatal("mapping did not persist disabled")
			}
			resumed, err := New(store, loaded, false)
			if err != nil {
				t.Fatal(err)
			}
			defer resumed.Close()
			resumed.now = c.now
			resumed.muteUntil = c.now()
			a = resumed
			hits := make(chan struct{}, 10)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Error("wrong method")
				}
				hits <- struct{}{}
				w.WriteHeader(204)
			}))
			defer server.Close()
			s := a.Settings()
			s.AllowedOrigins = []string{server.URL}
			if err = a.UpdateSettings(s); err != nil {
				t.Fatal(err)
			}
			b := a.Backup().Remotes[0].Buttons[0]
			b.Webhook.URL = server.URL
			b.Enabled = true
			if _, err = a.EditButton(b.ID, b); err != nil {
				t.Fatal(err)
			}
			c.advance(2 * time.Second)
			for _, event := range events {
				a.Receive(event)
			}
			select {
			case <-hits:
			case <-time.After(2 * time.Second):
				t.Fatal("matching decoded press did not trigger local webhook")
			}
			select {
			case <-hits:
				t.Fatal("repeated RF frames triggered duplicate webhook")
			default:
			}
		})
	}
}
