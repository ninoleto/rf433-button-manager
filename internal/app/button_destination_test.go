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
)

func TestButtonDestinationApprovalIsAuthenticatedAtomicAndDoesNotSend(t *testing.T) {
	a, clk := testApp(t)
	r := remote(t, a)
	learn(t, a, clk, r.Buttons[0].ID, signalA)
	before := a.Backup()
	b := before.Remotes[0].Buttons[0]
	var sent atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sent.Add(1) }))
	defer destination.Close()
	b.Webhook.URL = destination.URL + "/api/webhook/synthetic-only?value=one%2Ftwo"
	handler := a.Handler("fixture-token", fstest.MapFS{})
	put := func(button Button, approval, token, browserOrigin string) int {
		t.Helper()
		body, err := json.Marshal(struct {
			Button
			AllowOrigin string `json:"allow_webhook_origin,omitempty"`
		}{button, approval})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/api/buttons/"+button.ID, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if browserOrigin != "" {
			req.Header.Set("Origin", browserOrigin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}
	for _, tc := range []struct {
		name, approval, token, browserOrigin string
		want                                 int
	}{
		{"unapproved", "", "fixture-token", "", 400},
		{"different origin", "http://different.invalid", "fixture-token", "", 400},
		{"anonymous", destination.URL, "", "", 401},
		{"cross origin", destination.URL, "fixture-token", "https://other.invalid", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := put(b, tc.approval, tc.token, tc.browserOrigin); got != tc.want {
				t.Fatalf("status %d, want %d", got, tc.want)
			}
			if !reflect.DeepEqual(a.Backup(), before) {
				t.Fatal("failed request changed configuration")
			}
		})
	}
	invalid := b
	invalid.Webhook.Payload = "not JSON"
	if put(invalid, destination.URL, "fixture-token", "") != 400 || !reflect.DeepEqual(a.Backup(), before) {
		t.Fatal("invalid button widened the destination allowlist")
	}
	if got := put(b, destination.URL, "fixture-token", ""); got != 200 {
		t.Fatalf("save status %d", got)
	}
	after := a.Backup()
	want := clone(before)
	want.Remotes[0].Buttons[0] = b
	want.Settings.AllowedOrigins = append(want.Settings.AllowedOrigins, destination.URL)
	if !reflect.DeepEqual(after, want) {
		t.Fatal("save changed unrelated fields or lost mapping")
	}
	if got := put(b, destination.URL, "fixture-token", ""); got != 200 || !reflect.DeepEqual(a.Backup(), want) {
		t.Fatal("repeated save duplicated origin")
	}
	var raw []byte
	if err := a.store.db.QueryRow("SELECT value FROM configuration WHERE id=1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved Config
	if err := json.Unmarshal(raw, &saved); err != nil || !reflect.DeepEqual(saved, want) {
		t.Fatal("button and approval did not persist together", err)
	}
	select {
	case <-a.SettingsChanges():
		t.Fatal("destination save restarted reception")
	default:
	}
	if sent.Load() != 0 {
		t.Fatal("saving invoked a webhook")
	}
}
