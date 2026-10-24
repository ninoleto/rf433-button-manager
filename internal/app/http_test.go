package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

func TestHTTPAuthenticationOriginAndStrictJSON(t *testing.T) {
	a, _ := testApp(t)
	h := a.Handler("secret", fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("app")}}, "https://rf.example.test")
	cases := []struct {
		name, method, path, token, origin, body string
		want                                    int
	}{{"anonymous state", "GET", "/api/state", "", "", "", 401}, {"wrong token", "GET", "/api/state", "wrong", "", "", 401}, {"health", "GET", "/healthz", "", "", "", 200}, {"state", "GET", "/api/state", "secret", "", "", 200}, {"cross origin", "POST", "/api/remotes", "secret", "https://evil.invalid", `{"name":"R"}`, 403}, {"trusted proxy", "POST", "/api/remotes", "secret", "https://rf.example.test", `{"name":"R"}`, 200}, {"same origin", "POST", "/api/remotes", "secret", "http://example.com", `{"name":"R"}`, 200}, {"unknown fields", "POST", "/api/remotes", "secret", "", `{"name":"R","shell":"bad"}`, 400}, {"multiple JSON", "POST", "/api/remotes", "secret", "", `{"name":"R"} {}`, 400}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing cache guard")
			}
		})
	}
}
func TestWebhookDoesNotFollowRedirectOrRetry(t *testing.T) {
	var source, target atomic.Int32
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { target.Add(1) }))
	defer dest.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source.Add(1)
		w.Header().Set("Location", dest.URL)
		w.WriteHeader(302)
	}))
	defer server.Close()
	kind, _ := sendWebhook(context.Background(), Webhook{URL: server.URL, Method: "POST", TimeoutMS: 1000})
	if kind != "webhook_error" || source.Load() != 1 || target.Load() != 0 {
		t.Fatal("redirect followed or repeated")
	}
}
func TestWebhookTimeoutIsUnknownAndNotRetried(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); <-r.Context().Done() }))
	defer server.Close()
	kind, msg := sendWebhook(context.Background(), Webhook{URL: server.URL, Method: "POST", TimeoutMS: 100})
	if kind != "webhook_unknown" || calls.Load() != 1 || !strings.Contains(msg, "Not retried") {
		t.Fatalf("unexpected result %s %s calls=%d", kind, msg, calls.Load())
	}
}
func TestWebhookPayloadAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(raw) != `{"a":1}` || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("request changed")
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	kind, msg := sendWebhook(context.Background(), Webhook{URL: server.URL, Method: "POST", Payload: `{"a":1}`, Headers: map[string]string{"Authorization": "Bearer secret"}, TimeoutMS: 1000})
	if kind != "webhook_ok" || strings.Contains(msg, "secret") {
		t.Fatal(kind, msg)
	}
}
func TestUnsafeDestinationsAndHeaders(t *testing.T) {
	s := DefaultConfig().Settings
	for _, url := range []string{"file:///etc/passwd", "http://user:pass@127.0.0.1:8123/a", "http://127.0.0.1:8123/a#secret", "http://evil.example/action"} {
		if e := validateWebhook(Webhook{URL: url, Method: "POST", TimeoutMS: 1000}, s); e == nil {
			t.Error("accepted", url)
		}
	}
	for _, ip := range []string{"0.0.0.0", "169.254.169.254", "224.0.0.1", "fe80::1", "::"} {
		if safeIP(net.ParseIP(ip)) {
			t.Error("allowed prohibited address", ip)
		}
	}
	for _, ip := range []string{"127.0.0.1", "192.168.1.10", "::1"} {
		if !safeIP(net.ParseIP(ip)) {
			t.Error("blocked intended local destination", ip)
		}
	}
}
