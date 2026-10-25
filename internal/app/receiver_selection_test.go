package app

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func TestReceiverSelectionPreservesThreeMappingsAndPersists(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	for i, code := range []string{"abc101", "abc102", "abc103"} {
		learn(t, a, c, r.Buttons[i].ID, []byte(`{"model":"Synthetic","codes":["{24}`+code+`"]}`))
		configure(t, a, r.Buttons[i].ID, "http://127.0.0.1:8123/api/webhook/synthetic")
	}
	devices := []receiver.DeviceInfo{{Serial: "desk", Selectable: true}, {Serial: "roof", Selectable: true}}
	a.discoverReceivers = func() ([]receiver.DeviceInfo, error) { return devices, nil }
	before := a.Backup()
	settings := a.Settings()
	settings.ReceiverSerial = "roof"
	if err := a.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	after := a.Backup()
	if !reflect.DeepEqual(before.Remotes, after.Remotes) || !reflect.DeepEqual(before.DecoderProfiles, after.DecoderProfiles) {
		t.Fatal("receiver choice altered mappings/actions")
	}
	var raw string
	if err := a.store.db.QueryRow("SELECT value FROM configuration WHERE id=1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved Config
	if json.Unmarshal([]byte(raw), &saved) != nil || saved.Settings.ReceiverSerial != "roof" {
		t.Fatal("selection not persisted")
	}
	plan, _ := a.ReceiverPlan("rtl_433")
	if plan.Serial != "roof" {
		t.Fatal("wrong receiver plan")
	}
	// Removing that receiver never silently saves the other one. Existing saved
	// serial remains editable for unrelated settings; runtime waits for its return.
	devices = devices[:1]
	if err := a.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings.ReceiverSerial = "missing"
	if a.UpdateSettings(settings) == nil || a.Settings().ReceiverSerial != "roof" {
		t.Fatal("missing receiver accepted")
	}
	devices = []receiver.DeviceInfo{{Serial: "desk", Reason: "Duplicate serial"}, {Serial: "desk", Reason: "Duplicate serial"}}
	settings.ReceiverSerial = "desk"
	if a.UpdateSettings(settings) == nil {
		t.Fatal("duplicate selected")
	}
	settings.ReceiverSerial = ""
	if a.UpdateSettings(settings) == nil {
		t.Fatal("ambiguous automatic selected")
	}
	devices = []receiver.DeviceInfo{{Serial: "desk", Selectable: true}}
	if _, err := a.StartLearning(r.Buttons[0].ID, nil); err != nil {
		t.Fatal(err)
	}
	settings.ReceiverSerial = "desk"
	if a.UpdateSettings(settings) == nil {
		t.Fatal("receiver switched during learning")
	}
	if !reflect.DeepEqual(before.Remotes, a.Backup().Remotes) {
		t.Fatal("failed receiver choices changed assignments")
	}
}

func TestReceiverInventoryAPIIsAuthenticatedReadOnly(t *testing.T) {
	a, _ := testApp(t)
	calls := 0
	a.discoverReceivers = func() ([]receiver.DeviceInfo, error) {
		calls++
		return []receiver.DeviceInfo{{Name: "Test tuner", Serial: "test-serial", USBPort: "1-2", Selectable: true}}, nil
	}
	h := a.Handler("secret", fstest.MapFS{})
	for _, tc := range []struct {
		token, origin string
		code          int
	}{{"", "", 401}, {"wrong", "", 401}, {"secret", "https://elsewhere.invalid", 403}, {"secret", "", 200}} {
		req := httptest.NewRequest("GET", "/api/receivers", nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		if w.Code == 200 && !strings.Contains(w.Body.String(), "Test tuner") {
			t.Fatal(w.Body)
		}
	}
	if calls != 1 {
		t.Fatal("unauthorized inventory read")
	}
}

func TestDecodedActivityAndEventSourceFollowReceiver(t *testing.T) {
	a, c := testApp(t)
	a.SetReceiver(receiver.Status{State: "connected", Name: "Desk tuner", Serial: "desk", USBPort: "1-2"})
	a.Receive(signalA)
	if a.lastDecodedAt == "" || a.events[0].Receiver == nil || a.events[0].Receiver.Serial != "desk" {
		t.Fatal("missing RF attribution")
	}
	a.SetReceiver(receiver.Status{State: "disconnected", Serial: "desk"})
	if a.lastDecodedAt != "" {
		t.Fatal("stale activity after disconnect")
	}
	c.advance(time.Second)
	a.SetReceiver(receiver.Status{State: "connected", Name: "Roof tuner", Serial: "roof", USBPort: "2-3"})
	a.Receive(signalB)
	if a.events[0].Receiver.Serial != "roof" || a.events[0].Receiver.USBPort != "2-3" {
		t.Fatal("wrong RF source")
	}
	if a.events[3].Receiver.Serial != "desk" {
		t.Fatal("historical RF source relabeled")
	}
}
