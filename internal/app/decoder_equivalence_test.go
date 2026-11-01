package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Artificial identifiers; no operator recording or radio identity is included.
// Reversing all 24 bits of 0x123451 gives 0x8a2c48.
var equivalentNative = []byte(`{"model":"Akhan-100F14","id":74565,"data":"0x1 (Lock)"}`)
var equivalentSmoke = []byte(`{"model":"Smoke-GS558","id":20834,"unit":8,"learn":0,"code":"8a2c48"}`)

func TestDecodedRemotePairPreservesAllPayloadBits(t *testing.T) {
	native, alternate, fromAlternate := decodedRemotePair(equivalentNative)
	want := `{"command":"0x1 (Lock)","id":74565,"model":"Akhan-100F14"}`
	if string(native) != want || fromAlternate {
		t.Fatal("native identity changed")
	}
	secondNative, secondAlternate, fromAlternate := decodedRemotePair(equivalentSmoke)
	if string(secondNative) != string(native) || string(secondAlternate) != string(alternate) || !fromAlternate {
		t.Fatal("equivalent 24-bit payloads did not agree")
	}
}

func TestDecodedRemotePairSupportsOnlyTheFourKnownCommands(t *testing.T) {
	for _, tc := range []struct {
		command string
		code    string
	}{
		{"0x1 (Lock)", "8a2c48"},
		{"0x2 (Unlock)", "4a2c48"},
		{"0x4 (Mute)", "2a2c48"},
		{"0x8 (Alarm)", "1a2c48"},
	} {
		native := changedEvent(t, equivalentNative, "data", tc.command)
		smoke := changedEvent(t, equivalentSmoke, "code", tc.code)
		one, _, _ := decodedRemotePair(native)
		two, _, alternate := decodedRemotePair(smoke)
		if one == nil || string(one) != string(two) || !alternate {
			t.Fatal("known command lost or confused")
		}
	}
}

func changedEvent(t *testing.T, raw []byte, key string, value any) []byte {
	t.Helper()
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	event[key] = value
	result, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestDecodedRemotePairRejectsMalformedOrUnsupportedEvents(t *testing.T) {
	cases := []struct {
		name  string
		raw   []byte
		key   string
		value any
	}{
		{"different model", equivalentSmoke, "model", "ExampleSensor"},
		{"short code", equivalentSmoke, "code", "8a2c4"},
		{"long code", equivalentSmoke, "code", "8a2c4800"},
		{"invalid hex", equivalentSmoke, "code", "8a2c4g"},
		{"padded code", equivalentSmoke, "code", "8a2c48 "},
		{"numeric code", equivalentSmoke, "code", 9055304},
		{"wrong id", equivalentSmoke, "id", 20833},
		{"wrong unit", equivalentSmoke, "unit", 9},
		{"pairing traffic", equivalentSmoke, "learn", 1},
		{"null learn", equivalentSmoke, "learn", nil},
		{"null unit", equivalentSmoke, "unit", nil},
		{"boolean learn", equivalentSmoke, "learn", false},
		{"string id", equivalentSmoke, "id", "20834"},
		{"fractional id", equivalentSmoke, "id", 20834.5},
		{"unexpected identity", equivalentSmoke, "command", "other"},
		{"too wide id", equivalentNative, "id", 0x100000},
		{"negative id", equivalentNative, "id", -1},
		{"null id", equivalentNative, "id", nil},
		{"unsupported command", equivalentNative, "data", "0x3 (Other)"},
		{"numeric command", equivalentNative, "data", 1},
		{"conflicting command", equivalentNative, "command", "0x2 (Unlock)"},
		{"extra identity", equivalentNative, "channel", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if native, _, _ := decodedRemotePair(changedEvent(t, tc.raw, tc.key, tc.value)); native != nil {
				t.Fatal("unsafe equivalence accepted")
			}
		})
	}
	for _, key := range []string{"id", "unit", "learn", "code"} {
		var event map[string]json.RawMessage
		_ = json.Unmarshal(equivalentSmoke, &event)
		delete(event, key)
		raw, _ := json.Marshal(event)
		if native, _, _ := decodedRemotePair(raw); native != nil {
			t.Fatal("incomplete alternate event accepted")
		}
	}
}

func setupEquivalentAction(t *testing.T) (*App, *clock, string, *atomic.Int32) {
	t.Helper()
	a, clock := testApp(t)
	r := remote(t, a)
	id := r.Buttons[0].ID
	learn(t, a, clock, id, equivalentNative)
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	configure(t, a, id, server.URL+"/synthetic-action")
	return a, clock, id, calls
}

func TestEquivalentSmokeOnlyDispatchAndSharedDebounce(t *testing.T) {
	a, clock, id, calls := setupEquivalentAction(t)
	before := a.Backup()
	// No native event is fed for this press. It must use the normal dispatch path
	// immediately, without waiting for another frame or another physical press.
	a.Receive(equivalentSmoke)
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("alternate-only press did not dispatch one action")
	}
	for i := 0; i < 15; i++ {
		clock.advance(100 * time.Millisecond)
		if i%2 == 0 {
			a.Receive(equivalentNative)
		} else {
			a.Receive(equivalentSmoke)
		}
		a.wg.Wait() // A completed request must not hide a debounce defect.
		if calls.Load() != 1 {
			t.Fatal("two decoder names or a held burst produced extra actions")
		}
	}
	clock.advance(701 * time.Millisecond)
	a.Receive(equivalentNative)
	a.wg.Wait()
	clock.advance(50 * time.Millisecond)
	a.Receive(equivalentSmoke)
	a.wg.Wait()
	if calls.Load() != 2 {
		t.Fatal("second press was lost or sent twice")
	}
	after := a.Backup()
	previous, current := findButton(&before, id), findButton(&after, id)
	previous.LastReceived, current.LastReceived = "", ""
	oldJSON, _ := json.Marshal(before)
	newJSON, _ := json.Marshal(after)
	if string(oldJSON) != string(newJSON) {
		t.Fatal("equivalence rewrote saved mappings, flags or configuration")
	}
	found := false
	for _, event := range a.events {
		found = found || (event.Kind == "matched" && strings.Contains(event.Message, "decoder equivalent"))
	}
	if !found {
		t.Fatal("alternate matching is invisible in diagnostics")
	}
}

func TestEquivalentMatchRequiresEveryPayloadBit(t *testing.T) {
	a, clock, _, calls := setupEquivalentAction(t)
	for bit := 0; bit < 24; bit++ {
		encoded := uint32(0x8a2c48) ^ (1 << bit)
		raw, _ := json.Marshal(map[string]any{"model": "Smoke-GS558", "id": (encoded >> 5) & 0x7fff, "unit": encoded & 31, "learn": 0, "code": fmt.Sprintf("%06x", encoded)})
		clock.advance(time.Second)
		a.Receive(raw)
		a.wg.Wait()
	}
	for _, model := range []string{"Nexus-TH", "Toyota", "UnknownRemote"} {
		clock.advance(time.Second)
		a.Receive(changedEvent(t, equivalentSmoke, "model", model))
		a.wg.Wait()
	}
	if calls.Load() != 0 {
		t.Fatal("different payload or unrelated decoder triggered the action")
	}
}

func TestExistingEquivalentCollisionsRemainReadableButCannotDispatch(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("other_disabled_%v", disabled), func(t *testing.T) {
			a, clock, _, calls := setupEquivalentAction(t)
			backup := a.Backup()
			other := backup.Remotes[0].Buttons[1]
			other.Signature, _ = Signature(equivalentSmoke, other.IdentityFields)
			other.Webhook = backup.Remotes[0].Buttons[0].Webhook
			other.Enabled = !disabled
			backup.Remotes[0].Buttons[1] = other
			// This was a legal schema-2 configuration. Preserve that compatibility.
			if err := validateConfig(backup); err != nil {
				t.Fatal("existing configuration became unreadable", err)
			}
			if err := a.mutate(func(c *Config) error { *c = backup; return nil }); err != nil {
				t.Fatal(err)
			}
			for _, raw := range [][]byte{equivalentNative, equivalentSmoke} {
				clock.advance(time.Second)
				a.Receive(raw)
				a.wg.Wait()
			}
			if calls.Load() != 0 {
				t.Fatal("colliding mappings dispatched an action")
			}
			found := false
			for _, event := range a.events {
				found = found || event.Kind == "rf_ambiguous"
			}
			if !found {
				t.Fatal("mapping collision was not explained")
			}
			if err := a.ClearButton(other.ID); err != nil {
				t.Fatal(err)
			}
			clock.advance(2 * time.Second)
			a.Receive(equivalentSmoke)
			a.wg.Wait()
			if calls.Load() != 1 {
				t.Fatal("clearing duplicate did not resolve the conflict")
			}
		})
	}
}

func TestLearningRejectsNewEquivalentMappingCollision(t *testing.T) {
	for _, order := range [][]([]byte){{equivalentNative, equivalentSmoke}, {equivalentSmoke, equivalentNative}} {
		a, clock := testApp(t)
		r := remote(t, a)
		learn(t, a, clock, r.Buttons[0].ID, order[0])
		if _, err := a.StartLearning(r.Buttons[1].ID, nil); err != nil {
			t.Fatal(err)
		}
		for press := 0; press < 3; press++ {
			clock.advance(time.Second)
			a.Receive(order[1])
			if press < 2 {
				if _, err := a.ArmLearning(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := a.ConfirmLearning(); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Fatal("equivalent mapping was saved as a second button")
		}
		if a.Backup().Remotes[0].Buttons[1].Signature != "" {
			t.Fatal("failed confirmation changed the mapping")
		}
	}
}

func TestEquivalentFallbackRespectsActionGates(t *testing.T) {
	for _, gate := range []string{"disabled", "storage", "capture cleanup", "learning", "muted", "closed"} {
		t.Run(gate, func(t *testing.T) {
			a, clock, id, calls := setupEquivalentAction(t)
			switch gate {
			case "disabled":
				a.config.Remotes[0].Buttons[0].Enabled = false
			case "storage":
				a.storageErr = true
			case "capture cleanup":
				a.captureSuppressed = true
			case "learning":
				if _, err := a.StartLearning(id, nil); err != nil {
					t.Fatal(err)
				}
			case "muted":
				a.muteUntil = clock.now().Add(time.Minute)
			case "closed":
				a.Close()
			}
			a.Receive(equivalentSmoke)
			a.wg.Wait()
			if calls.Load() != 0 {
				t.Fatal("equivalent match bypassed an action gate")
			}
		})
	}
}

func TestEquivalenceDoesNotBroadenSmokeOnlyMappings(t *testing.T) {
	a, clock := testApp(t)
	r := remote(t, a)
	learn(t, a, clock, r.Buttons[0].ID, equivalentSmoke)
	a.Receive(equivalentNative)
	if a.Backup().Remotes[0].Buttons[0].LastReceived != "" {
		t.Fatal("Akhan event broadened an existing Smoke-only mapping")
	}
	a.Receive(equivalentSmoke)
	if a.Backup().Remotes[0].Buttons[0].LastReceived == "" {
		t.Fatal("direct Smoke matching stopped working")
	}
}

func TestEquivalentFallbackCannotDiscardExtraSavedIdentityFields(t *testing.T) {
	a, clock := testApp(t)
	r := remote(t, a)
	id := r.Buttons[0].ID
	button := r.Buttons[0]
	button.IdentityFields = []string{"id", "command", "channel"}
	if _, err := a.EditButton(id, button); err != nil {
		t.Fatal(err)
	}
	withChannel := changedEvent(t, equivalentNative, "channel", 3)
	learn(t, a, clock, id, withChannel)
	a.Receive(equivalentSmoke)
	if a.Backup().Remotes[0].Buttons[0].LastReceived != "" {
		t.Fatal("fallback discarded a required identity field")
	}
	a.Receive(withChannel)
	if a.Backup().Remotes[0].Buttons[0].LastReceived == "" {
		t.Fatal("strict direct match stopped working")
	}
}
