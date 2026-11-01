package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/bits"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

// Synthetic timing/identity only; no operator recording is embedded here.
func equivalentPWMProfile(t *testing.T) receiver.DecoderProfile {
	t.Helper()
	key := "m=OOK_PWM,s=300,l=900,r=10000,g=2000,t=0,y=0,bits=25,repeats>=1"
	hash := sha256.Sum256([]byte(key))
	id := hex.EncodeToString(hash[:16])
	p := receiver.DecoderProfile{ID: id, Model: "RFB-PWM-" + id, Name: "Synthetic 25-bit PWM", Bits: 25,
		ValidatedPresses: 3, CreatedAt: "2026-01-01T00:00:00Z"}
	p.FlexDecoder = "n=" + p.Model + "," + key
	if err := p.Validate(true); err != nil {
		t.Fatal(err)
	}
	return p
}

func equivalentPWMFrame(t *testing.T, profile receiver.DecoderProfile, payload uint32) []byte {
	t.Helper()
	frame := ((payload ^ 0xffffff) << 4) | 8 // full 24 bits, one trailing short mark, three zero padding bits
	raw, _ := json.Marshal(map[string]any{"model": profile.Model, "codes": []string{fmt.Sprintf("{25}%07x", frame)}})
	return raw
}

func equivalentSmokeFrame(payload uint32) []byte {
	encoded := bits.Reverse32(payload) >> 8
	raw, _ := json.Marshal(map[string]any{"model": "Smoke-GS558", "code": fmt.Sprintf("%06x", encoded),
		"id": (encoded >> 5) & 0x7fff, "unit": encoded & 31, "learn": 0})
	return raw
}

func setupProfileEquivalentAction(t *testing.T, smokeMapping bool) (*App, *clock, string, receiver.DecoderProfile, *atomic.Int32) {
	t.Helper()
	a, clk := testApp(t)
	r := remote(t, a)
	p := equivalentPWMProfile(t)
	if err := a.mutate(func(c *Config) error { c.DecoderProfiles = append(c.DecoderProfiles, p); return nil }); err != nil {
		t.Fatal(err)
	}
	raw := equivalentPWMFrame(t, p, 0x12345c)
	if smokeMapping {
		raw = equivalentSmokeFrame(0x12345c)
	}
	id := r.Buttons[0].ID
	learn(t, a, clk, id, raw)
	calls := new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(204) }))
	t.Cleanup(srv.Close)
	configure(t, a, id, srv.URL+"/synthetic-action")
	return a, clk, id, p, calls
}

func TestSavedSmokeReceivesEquivalentLearnedPWMWithoutAkhanCommand(t *testing.T) {
	a, clk, _, p, calls := setupProfileEquivalentAction(t, true)
	before := a.Backup()
	pwm, smoke := equivalentPWMFrame(t, p, 0x12345c), equivalentSmokeFrame(0x12345c)
	a.Receive(pwm)
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("valid custom-only report did not trigger saved Smoke mapping")
	}
	for i := 0; i < 20; i++ {
		clk.advance(100 * time.Millisecond)
		if i%2 == 0 {
			a.Receive(smoke)
		} else {
			a.Receive(pwm)
		}
		a.wg.Wait()
		if calls.Load() != 1 {
			t.Fatal("mixed decoder reports or a hold produced duplicate actions")
		}
	}
	clk.advance(701 * time.Millisecond)
	a.Receive(pwm)
	a.wg.Wait()
	if calls.Load() != 2 {
		t.Fatal("next separate press did not trigger immediately")
	}
	after := a.Backup()
	for i := range before.Remotes {
		for j := range before.Remotes[i].Buttons {
			before.Remotes[i].Buttons[j].LastReceived = ""
			after.Remotes[i].Buttons[j].LastReceived = ""
		}
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("fallback changed saved mappings or configuration")
	}
}

func TestSavedProfileReceivesEquivalentSmokeWithoutAkhanCommand(t *testing.T) {
	a, _, _, _, calls := setupProfileEquivalentAction(t, false)
	a.Receive(equivalentSmokeFrame(0x12345c))
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("alternate-only report did not trigger existing profile mapping")
	}
}

func TestProfileEquivalentRequiresEveryBitAndRegisteredProfile(t *testing.T) {
	a, clk, _, p, calls := setupProfileEquivalentAction(t, true)
	frame := uint32(((0x12345c ^ 0xffffff) << 4) | 8)
	for bit := 0; bit < 28; bit++ {
		raw, _ := json.Marshal(map[string]any{"model": p.Model, "codes": []string{fmt.Sprintf("{25}%07x", frame^(1<<bit))}})
		clk.advance(time.Second)
		a.Receive(raw)
		a.wg.Wait()
	}
	for _, model := range []string{"RFB-PWM-unregistered", "GenericRemote", "Smoke-GS558"} {
		clk.advance(time.Second)
		a.Receive(changedEvent(t, equivalentPWMFrame(t, p, 0x12345c), "model", model))
		a.wg.Wait()
	}
	for _, codes := range [][]string{{"{24}edcba3"}, {"{26}edcba38"}, {"{25}edcba38", "{25}edcba28"}, {"{25}edcba380"}, {"{25}zzzzzzz"}} {
		clk.advance(time.Second)
		a.Receive(changedEvent(t, equivalentPWMFrame(t, p, 0x12345c), "codes", codes))
		a.wg.Wait()
	}
	if calls.Load() != 0 {
		t.Fatal("a changed bit, malformed frame or unregistered model triggered an action")
	}
	a.config.DecoderProfiles = nil
	clk.advance(time.Second)
	a.Receive(equivalentPWMFrame(t, p, 0x12345c))
	a.wg.Wait()
	if calls.Load() != 0 {
		t.Fatal("profile prefix alone enabled equivalence")
	}
}

func TestProfileEquivalentDoesNotIgnoreExtraIdentityOrPairingFields(t *testing.T) {
	a, clk, id, p, calls := setupProfileEquivalentAction(t, true)
	b := findButton(&a.config, id)
	withChannel := changedEvent(t, equivalentSmokeFrame(0x12345c), "channel", 7)
	var err error
	b.Signature, err = Signature(withChannel, b.IdentityFields)
	if err != nil {
		t.Fatal(err)
	}
	a.Receive(equivalentPWMFrame(t, p, 0x12345c))
	a.wg.Wait()
	if calls.Load() != 0 {
		t.Fatal("fallback discarded a required identity field")
	}
	clk.advance(time.Second)
	a.Receive(withChannel)
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("direct matching regressed")
	}

	other, otherClock, _, _, otherCalls := setupProfileEquivalentAction(t, false)
	for _, field := range []string{"learn", "unit", "id"} {
		otherClock.advance(time.Second)
		other.Receive(changedEvent(t, equivalentSmokeFrame(0x12345c), field, 1))
		other.wg.Wait()
	}
	other.Receive(changedEvent(t, equivalentSmokeFrame(0x12345c), "channel", 7))
	other.wg.Wait()
	if otherCalls.Load() != 0 {
		t.Fatal("pairing, inconsistent or extra identity fields were discarded")
	}
}

func TestProfileEquivalenceRequiresValidatedCanonicalPWMProfile(t *testing.T) {
	good := equivalentPWMProfile(t)
	for _, damage := range []string{"unverified", "timestamp", "hash", "width", "modulation"} {
		t.Run(damage, func(t *testing.T) {
			p := good
			switch damage {
			case "unverified":
				p.ValidatedPresses = 2
			case "timestamp":
				p.CreatedAt = ""
			case "hash":
				p.FlexDecoder = strings.Replace(p.FlexDecoder, "s=300", "s=301", 1)
			case "width":
				p.Bits = 24
			case "modulation":
				p.FlexDecoder = strings.Replace(p.FlexDecoder, "OOK_PWM", "OOK_PPM", 1)
			}
			if damage == "width" || damage == "modulation" {
				_, key, _ := strings.Cut(p.FlexDecoder, ",")
				key = strings.Replace(key, "bits=25", fmt.Sprintf("bits=%d", p.Bits), 1)
				hash := sha256.Sum256([]byte(key))
				p.ID = hex.EncodeToString(hash[:16])
				p.Model = "RFB-PWM-" + p.ID
				if damage == "modulation" {
					p.Model = "RFB-PPM-" + p.ID
				}
				p.FlexDecoder = "n=" + p.Model + "," + key
				if err := p.Validate(true); err != nil {
					t.Fatal("test must use an otherwise valid profile", err)
				}
			}
			for _, raw := range [][]byte{equivalentSmokeFrame(0x12345c), equivalentPWMFrame(t, p, 0x12345c)} {
				if len(profileEquivalentEvents(raw, []receiver.DecoderProfile{p})) != 0 {
					t.Fatal("invalid profile enabled cross-decoder matching")
				}
			}
		})
	}
}

func TestProfileEquivalentCollisionExplainedWithoutRewritingOldConfig(t *testing.T) {
	a, clk, _, p, calls := setupProfileEquivalentAction(t, true)
	other := &a.config.Remotes[0].Buttons[1]
	other.Signature, _ = Signature(equivalentPWMFrame(t, p, 0x12345c), other.IdentityFields)
	other.DecoderProfileID = p.ID
	// Deliberately disabled: ambiguity must not silently change meaning on enable.
	if err := validateConfig(a.config); err != nil {
		t.Fatal("legacy configuration no longer loads", err)
	}
	for _, raw := range [][]byte{equivalentPWMFrame(t, p, 0x12345c), equivalentSmokeFrame(0x12345c)} {
		clk.advance(time.Second)
		a.Receive(raw)
		a.wg.Wait()
	}
	if calls.Load() != 0 {
		t.Fatal("ambiguous payload triggered an action")
	}
	found := false
	for _, event := range a.events {
		found = found || event.Kind == "rf_ambiguous"
	}
	if !found {
		t.Fatal("collision missing from activity")
	}
}

func TestLearningRejectsEquivalentProfileSmokeDuplicate(t *testing.T) {
	for _, smokeFirst := range []bool{false, true} {
		a, clk, _, p, _ := setupProfileEquivalentAction(t, smokeFirst)
		id := a.config.Remotes[0].Buttons[1].ID
		raw := equivalentSmokeFrame(0x12345c)
		if smokeFirst {
			raw = equivalentPWMFrame(t, p, 0x12345c)
		}
		if _, err := a.StartLearning(id, nil); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			clk.advance(time.Second)
			a.Receive(raw)
			if i < 2 {
				if _, err := a.ArmLearning(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := a.ConfirmLearning(); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Fatal("duplicate payload was learned as another button")
		}
	}
}

func TestProfileEquivalenceKeepsAllSixRemoteCommandsDistinct(t *testing.T) {
	a, clk, _, p, calls := setupProfileEquivalentAction(t, true)
	// All nibble values are compared in full. They are not relabelled as one of
	// Akhan's four known commands; six-key remotes need additional values.
	for command := uint32(0); command < 16; command++ {
		if command == 12 {
			continue
		}
		clk.advance(time.Second)
		a.Receive(equivalentPWMFrame(t, p, 0x123450|command))
		a.wg.Wait()
	}
	if calls.Load() != 0 {
		t.Fatal("another command activated the selected button")
	}
	clk.advance(time.Second)
	a.Receive(equivalentPWMFrame(t, p, 0x12345c))
	a.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("selected command did not activate")
	}
}

func TestProfileEquivalenceRespectsActionGates(t *testing.T) {
	for _, gate := range []string{"disabled", "storage", "capture cleanup", "learning", "muted", "closed"} {
		t.Run(gate, func(t *testing.T) {
			a, clk, id, p, calls := setupProfileEquivalentAction(t, true)
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
				a.muteUntil = clk.now().Add(time.Minute)
			case "closed":
				a.Close()
			}
			a.Receive(equivalentPWMFrame(t, p, 0x12345c))
			a.wg.Wait()
			if calls.Load() != 0 {
				t.Fatal("profile fallback bypassed action gate")
			}
		})
	}
}

func TestNewCandidateProfileCannotIntroduceDuplicateMapping(t *testing.T) {
	a, clk := testApp(t)
	r := remote(t, a)
	learn(t, a, clk, r.Buttons[0].ID, equivalentSmokeFrame(0x12345c))
	p := equivalentPWMProfile(t)
	if _, err := a.StartLearning(r.Buttons[1].ID, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		clk.advance(time.Second)
		a.Receive(equivalentPWMFrame(t, p, 0x12345c))
		if i < 2 {
			if _, err := a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
		}
	}
	a.learning.Profile = &p
	if err := a.ConfirmLearning(); err == nil {
		t.Fatal("new profile bypassed equivalence overlap check")
	}
	if len(a.config.DecoderProfiles) != 0 || a.config.Remotes[0].Buttons[1].Signature != "" {
		t.Fatal("failed confirmation modified configuration")
	}
}

func TestProfileEquivalenceSurvivesStoreReopenWithoutMappingMigration(t *testing.T) {
	a, clk, _, p, calls := setupProfileEquivalentAction(t, true)
	before := a.Backup()
	path := filepath.Join(t.TempDir(), "restart.db")
	s, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(before); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, loaded, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !reflect.DeepEqual(loaded, before) {
		t.Fatal("mapping or settings changed on reload")
	}
	restarted, err := New(s, loaded, false)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.now = clk.now
	restarted.Receive(equivalentPWMFrame(t, p, 0x12345c))
	restarted.wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("equivalent matching failed after store reopen")
	}
}
