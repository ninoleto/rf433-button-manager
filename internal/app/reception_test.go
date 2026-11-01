package app

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

func TestAdaptiveSettingPersistsAndBothReceivePlansUseIt(t *testing.T) {
	a, clk := testApp(t)
	r := remote(t, a)
	learn(t, a, clk, r.Buttons[0].ID, signalA)
	before := a.Backup()
	if a.Settings().AdaptiveSensitivity {
		t.Fatal("legacy/default settings must keep Standard")
	}
	s := a.Settings()
	s.AdaptiveSensitivity = true
	if err := a.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	plan, generation := a.ReceiverPlan("fixture-rtl")
	if !plan.AdaptiveSensitivity || plan.CapturePulses || generation == 0 {
		t.Fatal("normal receive plan lost sensitivity")
	}
	a.EnableCaptureLearning("fixture-rtl", t.TempDir())
	if _, err := a.StartInspection(); err != nil {
		t.Fatal(err)
	}
	capture, _ := a.ReceiverPlan("fixture-rtl")
	if !capture.AdaptiveSensitivity || !capture.CapturePulses || !a.rfCapture.AdaptiveSensitivity {
		t.Fatal("capture and normal reception disagree")
	}
	// Only Settings changes; existing learned mappings remain exact.
	if !reflect.DeepEqual(before.Remotes, a.Backup().Remotes) {
		t.Fatal("sensitivity changed saved mappings")
	}
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	copyStore, _, err := OpenStore(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyStore.Save(a.Backup()); err != nil {
		t.Fatal(err)
	}
	copyStore.Close()
	reopened, stored, err := OpenStore(copyPath)
	if err != nil || !stored.Settings.AdaptiveSensitivity {
		t.Fatal("sensitivity was not persisted", err)
	}
	defer reopened.Close()
}

func TestReceptionMetricsStaySeparateFromMatchingAndResetDoesNotRetrigger(t *testing.T) {
	a, clk, id, p, calls := setupProfileEquivalentAction(t, true)
	raw := equivalentPWMFrame(t, p, 0x12345c)
	raw = changedEvent(t, raw, "rssi", -24.5)
	raw = changedEvent(t, raw, "snr", 14.0)
	raw = changedEvent(t, raw, "noise", -38.5)
	before := findButton(&a.config, id).Signature
	a.Receive(raw)
	a.wg.Wait()
	d := a.reception.Buttons[id]
	if calls.Load() != 1 || d.Presses != 1 || d.Reports != 1 || *d.Levels.RSSI != -24.5 || d.Action == "" {
		t.Fatalf("lost profile metrics or action: %+v", d)
	}
	if findButton(&a.config, id).Signature != before {
		t.Fatal("measurement changed mapping")
	}
	// Other transmitters cannot replace the selected button's measurement.
	a.Receive(changedEvent(t, raw, "codes", []string{"{25}1111118"}))
	if d.Reports != 1 || *d.Levels.RSSI != -24.5 {
		t.Fatal("unmatched RF changed target diagnostics")
	}
	clk.advance(100 * time.Millisecond)
	a.Receive(changedEvent(t, raw, "rssi", -26.0))
	a.wg.Wait()
	if d.Presses != 1 || d.Repeats != 1 || *d.Levels.RSSI != -26.0 || calls.Load() != 1 {
		t.Fatal("repeat was not suppressed")
	}
	if err := a.ResetReception(id); err != nil {
		t.Fatal(err)
	}
	clk.advance(100 * time.Millisecond)
	a.Receive(raw)
	a.wg.Wait()
	d = a.reception.Buttons[id]
	if d.Presses != 0 || d.Repeats != 1 || calls.Load() != 1 {
		t.Fatal("reset cleared debounce or sent an action")
	}
	clk.advance(time.Second)
	a.Receive(equivalentPWMFrame(t, p, 0x12345c))
	a.wg.Wait()
	if d.Levels != nil || d.Presses != 1 || calls.Load() != 2 {
		t.Fatal("missing levels must be unknown, not a previous reading")
	}
	var state struct{ Reception ReceptionDiagnostics }
	data, _ := json.Marshal(a.State())
	if json.Unmarshal(data, &state) != nil || state.Reception.Buttons[id].Presses != 1 {
		t.Fatal("diagnostics missing from state")
	}
	backup, _ := json.Marshal(a.Backup())
	var object map[string]any
	_ = json.Unmarshal(backup, &object)
	if object["reception"] != nil {
		t.Fatal("ephemeral metrics entered backup")
	}
}

func TestReceptionCountsDisabledButtonsButNotLearningAndClearsOnReceiverChange(t *testing.T) {
	a, clk := testApp(t)
	r := remote(t, a)
	id := r.Buttons[0].ID
	learn(t, a, clk, id, signalA)
	a.Receive(signalA)
	if a.reception.Buttons[id].Presses != 1 || findButton(&a.config, id).Enabled {
		t.Fatal("disabled button should still count")
	}
	if _, err := a.StartLearning(id, nil); err != nil {
		t.Fatal(err)
	}
	clk.advance(time.Second)
	a.Receive(signalA)
	if a.reception.Buttons[id].Presses != 1 || a.ResetReception(id) == nil {
		t.Fatal("learning affected normal counts or accepted reset")
	}
	a.SetReceiver(receiver.Status{State: "connected", Serial: "new-fixture", USBPort: "fixture-port"})
	if len(a.reception.Buttons) != 0 {
		t.Fatal("stale metrics survived receiver change")
	}
}

func TestReceptionResetUsesManagementAuthentication(t *testing.T) {
	a, _ := testApp(t)
	r := remote(t, a)
	h := a.Handler("fixture-token", fstest.MapFS{}, "")
	for _, authorized := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/api/buttons/"+r.Buttons[0].ID+"/reception/reset", nil)
		if authorized {
			req.Header.Set("Authorization", "Bearer fixture-token")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		want := 401
		if authorized {
			want = 200
		}
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
