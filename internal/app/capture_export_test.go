package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func TestRFExportKeepsRejectedFSKWithoutApplicationSecrets(t *testing.T) {
	a, c, r := captureApp(t)
	// These values must never be part of the export, even though the app owns them.
	a.config.Settings.ReceiverSerial = "PRIVATE-USB-SERIAL"
	a.config.Remotes[0].Buttons[0].Webhook.URL = "http://private.invalid/PRIVATE-WEBHOOK"
	gen := startCapture(t, a, r.Buttons[0].ID)
	fsk := syntheticFSK(t)
	a.ReceivePulse(gen, fsk)
	ook := receiver.PulsePacket{OOK: true, Pulses: 144, Data: captureFixture(t)}
	a.ReceivePulse(gen, ook)
	a.Receive([]byte(`{"model":"Weather","id":42,"temperature_C":20}`))
	c.advance(21 * time.Second)
	a.State() // Expire and retain the same attempt's data.
	exported, err := a.ExportRFCapture()
	if err != nil || len(exported.Packets) != 2 || len(exported.Events) != 1 {
		t.Fatalf("RF evidence missing: %+v %v", exported, err)
	}
	if exported.Packets[0].Data != string(fsk.Data) || exported.Packets[0].Modulation != "FSK" || exported.Packets[1].Data != string(ook.Data) {
		t.Fatal("rejected packet changed or disappeared")
	}
	raw, _ := json.Marshal(exported)
	if strings.Contains(string(raw), "PRIVATE-") || strings.Contains(string(raw), "webhook") {
		t.Fatal("private configuration entered RF export")
	}
	state, _ := json.Marshal(a.State())
	if strings.Contains(string(state), "pulse_data") {
		t.Fatal("RF recordings leaked into routine polling")
	}
	exported.Packets[0].Data = "changed by caller"
	unchanged, _ := a.ExportRFCapture()
	if unchanged.Packets[0].Data != string(fsk.Data) {
		t.Fatal("caller mutated retained capture")
	}
	h := a.Handler("PRIVATE-TOKEN", fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("app")}})
	for _, authenticated := range []bool{false, true} {
		req := httptest.NewRequest("GET", "/api/learn/capture", nil)
		if authenticated {
			req.Header.Set("Authorization", "Bearer PRIVATE-TOKEN")
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		want := 401
		if authenticated {
			want = 200
		}
		if res.Code != want || strings.Contains(res.Body.String(), "PRIVATE-") {
			t.Fatalf("export authentication/privacy failed: status %d", res.Code)
		}
	}
}

func TestRFExportBoundsResetAndStalePackets(t *testing.T) {
	a, _, r := captureApp(t)
	gen := startCapture(t, a, r.Buttons[0].ID)
	packet := syntheticFSK(t)
	// Exercise retention bound independently of the learning session's stop limit.
	for i := 0; i < 200; i++ {
		a.recordCapturePacketLocked(gen, packet)
	}
	exported, err := a.ExportRFCapture()
	if err != nil || !exported.Limited || len(exported.Packets) != 128 || exported.Bytes > receiver.MaxCaptureStreamBytes {
		t.Fatal("RF retention is not bounded")
	}
	a.CancelLearning()
	_, normal := a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(normal)
	startCapture(t, a, r.Buttons[0].ID)
	a.ReceivePulse(gen, packet) // Old generation must not enter the new export.
	if _, err := a.ExportRFCapture(); err == nil {
		t.Fatal("retry retained stale data")
	}
}
