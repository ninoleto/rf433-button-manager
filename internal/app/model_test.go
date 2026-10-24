package app

import (
	"strings"
	"testing"
)

func TestSignatureNormalizesFlexCodesAndExcludesMetadata(t *testing.T) {
	one := []byte(`{"model":"Switch433","codes":["{24}A1B201"," {24}a1b201 "],"time":"old","rssi":-20,"count":5}`)
	two := []byte(`{"count":50,"rssi":-70,"codes":["{24}a1b201"],"model":"Switch433","time":"new"}`)
	a, err := Signature(one, defaultFields)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Signature(two, defaultFields)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a != `{"codes":["{24}a1b201"],"model":"Switch433"}` {
		t.Fatalf("unstable identity: %s / %s", a, b)
	}
}

func TestSignatureRejectsUnsafeOrAmbiguousFrames(t *testing.T) {
	frames := map[string]string{
		"multiple flex rows": `{"model":"Switch433","codes":["{24}a1b201","{24}a1b202"]}`,
		"no flex rows":       `{"model":"Switch433","codes":[]}`,
		"empty flex code":    `{"model":"Switch433","codes":[" "]}`,
		"model only":         `{"model":"Switch433"}`,
		"device without key": `{"model":"Switch433","id":17}`,
		"key without device": `{"model":"Switch433","button":1}`,
		"empty code":         `{"model":"Switch433","code":""}`,
		"boolean code":       `{"model":"Switch433","code":false}`,
		"empty ID":           `{"model":"Switch433","id":" ","button":1}`,
		"boolean ID":         `{"model":"Switch433","id":false,"button":1}`,
		"nested identity":    `{"model":"Switch433","code":{"value":1}}`,
		"empty model":        `{"model":" ","code":1}`,
		"trailing JSON":      `{"model":"Switch433","code":1}{"extra":true}`,
	}
	for name, raw := range frames {
		t.Run(name, func(t *testing.T) {
			if got, err := Signature([]byte(raw), defaultFields); err == nil {
				t.Fatalf("unsafe identity accepted: %s", got)
			}
		})
	}
	if _, err := Signature([]byte(`{"model":"Switch433","code":0}`), defaultFields); err != nil {
		t.Fatalf("zero can be a valid code: %v", err)
	}
	if _, err := Signature([]byte(`{"model":"Switch433","id":17,"state":false}`), defaultFields); err != nil {
		t.Fatalf("false can be a valid OFF state: %v", err)
	}
	if _, err := Signature([]byte(`{"model":"Switch433","code":"`+strings.Repeat("a", 9000)+`"}`), defaultFields); err == nil {
		t.Fatal("oversized identity would be impossible to restore safely")
	}
}

func TestIdentityFieldsRejectVolatileAndDuplicateKeys(t *testing.T) {
	for _, fields := range [][]string{{"rssi"}, {"model"}, {"codes", "codes"}, {"time"}, {"repeats"}, {"temperature_C"}, {"bad.key"}, {}} {
		if err := validateFields(fields); err == nil {
			t.Fatalf("invalid identity fields accepted: %v", fields)
		}
	}
}

func TestSignatureOverlapPreservesLargeDeviceIDs(t *testing.T) {
	a := `{"button":1,"id":9007199254740992,"model":"Remote"}`
	b := `{"button":1,"id":9007199254740993,"model":"Remote"}`
	if signaturesOverlap(a, b) {
		t.Fatal("different integer IDs were rounded into the same mapping")
	}
	if !signaturesOverlap(a, a) {
		t.Fatal("identical mappings must overlap")
	}
	superset := `{"button":1,"channel":2,"id":9007199254740992,"model":"Remote"}`
	if !signaturesOverlap(a, superset) || !signaturesOverlap(superset, a) {
		t.Fatal("subset and superset mappings must overlap symmetrically")
	}
	if signaturesOverlap(a, `{"button":2,"id":9007199254740992,"model":"Remote"}`) {
		t.Fatal("different buttons on one remote must be distinct")
	}
}

func modelTestConfig() Config {
	c := DefaultConfig()
	b := newButton("A ON")
	b.IdentityFields = []string{"id", "button"}
	b.Signature = `{"button":1,"id":17,"model":"Remote"}`
	c.Remotes = []Remote{{ID: newID(), Name: "Remote", Columns: 2, Buttons: []Button{b}}}
	return c
}

func TestConfigRejectsInvalidRestoredMappings(t *testing.T) {
	base := modelTestConfig()
	if err := validateConfig(base); err != nil {
		t.Fatal(err)
	}
	for _, signature := range []string{
		`{"model":"Remote"}`,
		`{"model":{},"id":17,"button":1}`,
		`{"button":1,"id":17,"model":"Remote","rssi":-1}`,
		`{"id":17,"button":1,"model":"Remote"}`,
		`{"button":1,"id":"","model":"Remote"}`,
		`{"code":1,"model":"Remote"}`,
		`null`,
	} {
		c := clone(base)
		c.Remotes[0].Buttons[0].Signature = signature
		if err := validateConfig(c); err == nil {
			t.Fatalf("invalid restored signature accepted: %s", signature)
		}
	}
	duplicate := clone(base.Remotes[0].Buttons[0])
	duplicate.ID = newID()
	duplicate.IdentityFields = []string{"id", "button", "channel"}
	duplicate.Signature = `{"button":1,"channel":2,"id":17,"model":"Remote"}`
	base.Remotes[0].Buttons = append(base.Remotes[0].Buttons, duplicate)
	if err := validateConfig(base); err == nil {
		t.Fatal("overlapping mappings with different selected fields accepted")
	}
}

func TestConfigAllowsDistinctLargeIntegerMappings(t *testing.T) {
	c := modelTestConfig()
	c.Remotes[0].Buttons[0].Signature = `{"button":1,"id":9007199254740992,"model":"Remote"}`
	b := clone(c.Remotes[0].Buttons[0])
	b.ID = newID()
	b.Signature = `{"button":1,"id":9007199254740993,"model":"Remote"}`
	c.Remotes[0].Buttons = append(c.Remotes[0].Buttons, b)
	if err := validateConfig(c); err != nil {
		t.Fatalf("distinct 64-bit IDs rejected: %v", err)
	}
}

func TestWebhookRejectsCaseInsensitiveDuplicateHeaders(t *testing.T) {
	w := newButton("Test").Webhook
	w.Headers = map[string]string{"Authorization": "one", "authorization": "two"}
	if err := validateWebhook(w, DefaultConfig().Settings); err == nil {
		t.Fatal("ambiguous authorization headers accepted")
	}
	w.Headers = map[string]string{"Authorization": "one", "Content-Type": "application/json"}
	if err := validateWebhook(w, DefaultConfig().Settings); err != nil {
		t.Fatalf("distinct valid headers rejected: %v", err)
	}
}

func TestConfigFitsRestoreRequestLimit(t *testing.T) {
	c := modelTestConfig()
	c.Remotes[0].Buttons = nil
	for i := 0; i < 40; i++ {
		b := newButton("Large payload")
		b.Webhook.Payload = `"` + strings.Repeat("x", 65000) + `"`
		c.Remotes[0].Buttons = append(c.Remotes[0].Buttons, b)
	}
	if err := validateConfig(c); err == nil || !strings.Contains(err.Error(), "2 MiB") {
		t.Fatalf("unrestorable config accepted: %v", err)
	}
}
