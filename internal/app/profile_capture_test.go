package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Private input stays outside source/package/history. These are recorded live
// rtl_433 JSON messages, not new physical presses or proof of receive range.
// The export omits profile configuration: its model label alone is substituted
// with a registered synthetic 25-bit PWM fixture. Code bits, fragments, ordering
// and timestamps are unchanged. Filtered scenarios explicitly simulate missing
// output from one decoder. HTTP requests go only to an isolated local endpoint.
func TestPrivateSavedSmokeProfileCapture(t *testing.T) {
	path := os.Getenv("RFB_SMOKE_PROFILE_CAPTURE")
	if path == "" {
		t.Skip("private saved-Smoke recording not supplied")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("capture unavailable")
	}
	var capture struct {
		Events []json.RawMessage `json:"decoded_events"`
	}
	if json.Unmarshal(data, &capture) != nil {
		t.Fatal("invalid capture")
	}
	var target json.RawMessage
	for _, raw := range capture.Events {
		var e struct{ Model string }
		_ = json.Unmarshal(raw, &e)
		if e.Model == "Smoke-GS558" {
			if target != nil {
				one, _ := Signature(target, defaultFields)
				two, _ := Signature(raw, defaultFields)
				if one != two {
					t.Fatal("recording contains multiple alternate identities")
				}
			}
			target = raw
		}
	}
	if target == nil {
		t.Fatal("no reference candidate")
	}
	for _, mode := range []string{"all_recorded_events", "only_custom_reports_simulated", "only_smoke_reports_simulated"} {
		t.Run(mode, func(t *testing.T) {
			a, clk := testApp(t)
			r := remote(t, a)
			p := equivalentPWMProfile(t)
			if err := a.mutate(func(c *Config) error { c.DecoderProfiles = append(c.DecoderProfiles, p); return nil }); err != nil {
				t.Fatal(err)
			}
			learn(t, a, clk, r.Buttons[0].ID, target)
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(204) }))
			defer srv.Close()
			configure(t, a, r.Buttons[0].ID, srv.URL+"/synthetic-action")
			start := clk.now().Add(time.Second)
			var first time.Time
			frames := 0
			for _, raw := range capture.Events {
				var e struct {
					Model string
					Time  string
				}
				_ = json.Unmarshal(raw, &e)
				custom := strings.HasPrefix(e.Model, "RFB-PWM-")
				if mode == "only_custom_reports_simulated" && !custom {
					continue
				}
				if mode == "only_smoke_reports_simulated" && e.Model != "Smoke-GS558" {
					continue
				}
				at, err := time.Parse("2006-01-02 15:04:05", e.Time)
				if err != nil {
					t.Fatal("invalid capture time")
				}
				if first.IsZero() {
					first = at
				}
				clk.n.Store(start.Add(at.Sub(first)).UnixNano())
				if custom {
					raw = changedEvent(t, raw, "model", p.Model)
				}
				a.Receive(raw)
				a.wg.Wait()
				frames++
			}
			if calls.Load() != 3 {
				t.Fatalf("expected three separate recorded bursts, got %d isolated requests", calls.Load())
			}
			t.Logf("%d recorded messages -> %d isolated requests", frames, calls.Load())
		})
	}
}
