package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

func recordingApp(t *testing.T) (*App, *clock, string, uint64) {
	t.Helper()
	a, c := testApp(t)
	r := remote(t, a)
	id := r.Buttons[0].ID
	learn(t, a, c, id, signalA)
	a.captureEnabled = true // No analysis worker or live receiver in these tests.
	_, gen := a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(gen)
	a.SetReceiverFor(gen, receiver.Status{State: "connected", Message: "receiver is acquiring samples", Serial: "synthetic", USBPort: "fake:1"})
	c.advance(2 * time.Second)
	return a, c, id, gen
}

func TestRecordingPreservesReceiverDebounceAndActionDelivery(t *testing.T) {
	a, c, id, _ := recordingApp(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	configure(t, a, id, server.URL+"/PRIVATE-DESTINATION")
	_, gen := a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(gen)
	a.SetReceiverFor(gen, receiver.Status{State: "connected", Message: "receiver is acquiring samples", Serial: "synthetic", USBPort: "fake:1"})
	c.advance(2 * time.Second)
	plan, generation := a.ReceiverPlan("rtl_433")
	before, epoch, mute := a.Backup(), a.epoch, a.muteUntil
	if _, err := a.StartRecording(id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartRecording(id); err == nil {
		t.Fatal("overlapping recording replaced evidence")
	}
	for i := 0; i < 2; i++ {
		a.Receive(signalA)
		a.wg.Wait()
		c.advance(100 * time.Millisecond)
		a.Receive(signalA)
		if i == 0 {
			c.advance(2 * time.Second)
		}
	}
	status, err := a.StopRecording()
	if err != nil || status.Active || status.Presses != 2 || status.Repeats != 2 || calls.Load() != 2 {
		t.Fatalf("recording changed press/action behavior: %+v, calls=%d, err=%v", status, calls.Load(), err)
	}
	// Starting another recording must not clear the held-button quiet period.
	if _, err := a.StartRecording(id); err != nil {
		t.Fatal(err)
	}
	a.Receive(signalA)
	a.wg.Wait()
	if calls.Load() != 2 || a.recording.Presses != 0 || a.recording.Repeats != 1 {
		t.Fatal("recording reset the existing debounce")
	}
	a.StopRecording()
	afterPlan, afterGeneration := a.ReceiverPlan("rtl_433")
	after := a.Backup()
	// Normal reception updates LastReceived; no configuration change is allowed.
	findButton(&after, id).LastReceived = findButton(&before, id).LastReceived
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(plan, afterPlan) || generation != afterGeneration || a.epoch != epoch || !a.muteUntil.Equal(mute) || a.captureSuppressed || a.learning != nil {
		t.Fatal("recording altered receiver, configuration, suppression or dispatch gates")
	}
	if !plan.MonitorPulses || plan.CapturePulses {
		t.Fatal("normal pulse stream requires an inspection restart")
	}
}

func TestRecordingExportsOriginalTrafficAndSafeOutcomes(t *testing.T) {
	a, c, id, gen := recordingApp(t)
	// Put secrets in every non-RF configuration location; none may be exported.
	a.mu.Lock()
	b := findButton(&a.config, id)
	b.Webhook = Webhook{URL: "https://PRIVATE-URL.test/secret", Payload: "PRIVATE-BODY", Headers: map[string]string{"Authorization": "PRIVATE-HEADER"}}
	a.config.Settings.AllowedOrigins = []string{"https://PRIVATE-ORIGIN.test"}
	a.mu.Unlock()
	a.ReceivePulse(gen, receiver.PulsePacket{Progress: true, Stream: receiver.PulseStream{Headers: 7}})
	if _, err := a.StartRecording(id); err != nil {
		t.Fatal(err)
	}
	p := receiver.PulsePacket{OOK: true, Pulses: 2, Data: []byte(";ook 2 pulses\n100 300\n300 10000\n;end\n"), Stream: receiver.PulseStream{Headers: 7, Completed: 7}}
	a.ReceivePulse(gen, p) // Header was already present before the recording.
	p.Stream.Headers++
	a.ReceivePulse(gen-1, p)
	a.ReceivePulse(gen, p)
	p.OOK = false
	p.Stream.Headers++
	p.Data = []byte(";fsk 2 pulses\n100 300\n300 10000\n;end\n")
	a.ReceivePulse(gen, p)
	c.advance(time.Second)
	a.Receive(signalB) // Background report must be distinguishable from a match.
	a.Receive(signalA)
	a.mu.Lock()
	a.eventLocked("webhook_error", "HTTP 403 PRIVATE-ERROR https://PRIVATE-URL.test", id, nil)
	a.mu.Unlock()
	a.StopRecording()
	out, err := a.ExportRecording()
	if err != nil || out.PacketCount != 2 || out.DecodedCount != 2 || out.Presses != 1 || out.Packets[1].Modulation != "FSK" {
		t.Fatalf("missing original RF evidence: %+v, %v", out, err)
	}
	reasons := map[string]bool{}
	foundRaw, foundStatus := false, false
	for _, event := range out.Events {
		reasons[event.Reason] = true
		if event.Kind == "decoded" && string(event.Data) == string(signalA) {
			foundRaw = true
		}
		if event.Kind == "webhook_error" && event.HTTPStatus == 403 {
			foundStatus = true
		}
	}
	if !foundRaw || !foundStatus || !reasons["no_saved_mapping"] || !reasons["recognized"] || !reasons["action_disabled"] {
		t.Fatalf("missing decode, matching or delivery evidence: %+v", out.Events)
	}
	encoded, _ := json.Marshal(out)
	if strings.Contains(string(encoded), "PRIVATE-") {
		t.Fatal("recording leaked application secrets")
	}
	state, _ := json.Marshal(a.State())
	var fields map[string]json.RawMessage
	json.Unmarshal(state, &fields)
	if strings.Contains(string(fields["reception_recording"]), "decoded_data") || strings.Contains(string(fields["reception_recording"]), "packets") {
		t.Fatal("ordinary polling exposes the recording payload")
	}
}

func TestRecordingBoundsAndLifecycle(t *testing.T) {
	for _, kind := range []string{"quiet_timeout", "events", "packets", "bytes", "large_record", "receiver", "configuration", "inspection", "stream"} {
		t.Run(kind, func(t *testing.T) {
			a, c, id, gen := recordingApp(t)
			if _, err := a.StartRecording(id); err != nil {
				t.Fatal(err)
			}
			want := "data_limit"
			switch kind {
			case "quiet_timeout":
				c.advance(61 * time.Second)
				want = "time_limit"
			case "events":
				for i := 0; i < recordingEventLimit+1; i++ {
					a.Receive(signalB)
				}
			case "packets", "bytes":
				data := ";ook 1 pulses\n100 10000\n;end\n"
				if kind == "bytes" {
					data = strings.Repeat("100 300\n", 10000)
				}
				for i := 0; i < recordingPacketLimit+1; i++ {
					a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Pulses: 1, Data: []byte(data), Stream: receiver.PulseStream{Headers: i + 1}})
				}
			case "large_record":
				a.Receive([]byte(`{"model":"` + strings.Repeat("x", 9000) + `"}`))
				want = "record_too_large"
			case "receiver":
				a.SetReceiverFor(gen, receiver.Status{State: "disconnected", Message: "synthetic unplug"})
				want = "receiver_unavailable"
			case "configuration":
				if err := a.UpdateSettings(a.Settings()); err != nil {
					t.Fatal(err)
				}
				want = "configuration_changed"
			case "inspection":
				if _, err := a.StartInspection(); err != nil {
					t.Fatal(err)
				}
				want = "inspection_or_learning_started"
			case "stream":
				a.ReceivePulse(gen, receiver.PulsePacket{Final: true, Stream: receiver.PulseStream{Closed: true}})
				want = "receiver_stream_closed"
			}
			out, err := a.ExportRecording()
			if err != nil || out.Active || out.StopReason != want || out.Bytes > receiver.MaxCaptureStreamBytes || len(out.Packets) > recordingPacketLimit || len(out.Events) > recordingEventLimit {
				t.Fatalf("unbounded or incorrectly finalized recording: status=%+v err=%v", out.RecordingStatus, err)
			}
			before := clone(out)
			a.Receive(signalB)
			a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Data: []byte("ignored"), Stream: receiver.PulseStream{Headers: 9999}})
			after, _ := a.ExportRecording()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("finished evidence mutated by later traffic")
			}
		})
	}
}

func TestRecordingEndpointsRequireTokenAndSameOrigin(t *testing.T) {
	a, _, id, _ := recordingApp(t)
	h := a.Handler("PRIVATE-TOKEN", fstest.MapFS{})
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/buttons/" + id + "/reception/record"},
		{"POST", "/api/reception/recording/stop"},
		{"GET", "/api/reception/recording"},
		{"POST", "/api/inspect/finish"},
	} {
		for _, mode := range []string{"no_token", "cross_origin"} {
			req := httptest.NewRequest(route.method, route.path, nil)
			want := 401
			if mode == "cross_origin" {
				req.Header.Set("Authorization", "Bearer PRIVATE-TOKEN")
				req.Header.Set("Origin", "https://untrusted.example.test")
				want = 403
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != want || a.recording != nil {
				t.Fatalf("unauthorized operation %s: %d", route.path, w.Code)
			}
		}
	}
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/buttons/" + id + "/reception/record"},
		{"POST", "/api/reception/recording/stop"},
		{"GET", "/api/reception/recording"},
	} {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("Authorization", "Bearer PRIVATE-TOKEN")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "PRIVATE-") {
			t.Fatalf("authorized recording operation %s: %d %s", route.path, w.Code, w.Body.String())
		}
		if route.method == "GET" && !strings.Contains(w.Header().Get("Content-Disposition"), ".json") {
			t.Fatal("recording download did not attach a JSON file")
		}
	}
}

func TestDemoRecordingAfterSettingsSaveDoesNotWaitForARealReceiver(t *testing.T) {
	a, _, id, _ := recordingApp(t)
	a.demo = true
	a.captureEnabled = false
	if err := a.UpdateSettings(a.Settings()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartRecording(id); err != nil {
		t.Fatal("demo blocked waiting for a receiver plan that is never run", err)
	}
	a.Receive(signalA)
	out, err := a.ExportRecording()
	if err != nil || !out.Demo || out.DecodedCount != 1 || out.PacketCount != 0 {
		t.Fatal("demo recording invented physical RF evidence", err)
	}
}
