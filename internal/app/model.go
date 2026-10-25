package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"context"
	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

type Webhook struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Payload   string            `json:"payload"`
	Headers   map[string]string `json:"headers"`
	TimeoutMS int               `json:"timeout_ms"`
}
type Button struct {
	DecoderProfileID string   `json:"decoder_profile_id,omitempty"`
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Enabled          bool     `json:"enabled"`
	DebounceMS       int      `json:"debounce_ms"`
	Signature        string   `json:"signature"`
	IdentityFields   []string `json:"identity_fields"`
	Webhook          Webhook  `json:"webhook"`
	LastReceived     string   `json:"last_received"`
}
type Remote struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Columns int      `json:"columns"`
	Buttons []Button `json:"buttons"`
}
type Settings struct {
	ReceiverSerial string   `json:"receiver_serial"`
	FlexDecoder    string   `json:"flex_decoder"`
	Protocols      []int    `json:"protocols"`
	IdentityFields []string `json:"identity_fields"`
	AllowedOrigins []string `json:"allowed_origins"`
}
type Config struct {
	DecoderProfiles []receiver.DecoderProfile `json:"decoder_profiles"`
	SchemaVersion   int                       `json:"schema_version"`
	Remotes         []Remote                  `json:"remotes"`
	Settings        Settings                  `json:"settings"`
}
type Event struct {
	ID       int64           `json:"id"`
	Time     string          `json:"time"`
	Kind     string          `json:"kind"`
	Message  string          `json:"message"`
	ButtonID string          `json:"button_id,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}
type Learning struct {
	CaptureGeneration   uint64                       `json:"capture_generation"`
	CaptureReceiver     receiver.Status              `json:"capture_receiver"`
	CaptureReceiverAt   string                       `json:"capture_receiver_at"`
	FinishedAt          string                       `json:"finished_at,omitempty"`
	SessionID           string                       `json:"session_id"`
	Automatic           bool                         `json:"automatic"`
	Message             string                       `json:"message"`
	Diagnostics         receiver.LearningDiagnostics `json:"diagnostics"`
	Profile             *receiver.DecoderProfile     `json:"profile,omitempty"`
	ButtonID            string                       `json:"button_id"`
	Stage               string                       `json:"stage"`
	Candidate           string                       `json:"candidate"`
	Raw                 json.RawMessage              `json:"raw"`
	Presses             int                          `json:"presses"`
	Required            int                          `json:"required"`
	ExpiresAt           string                       `json:"expires_at"`
	WindowExpiresAt     string                       `json:"window_expires_at,omitempty"`
	WindowAnalyses      int                          `json:"window_analyses"`
	WindowAnalysisLimit int                          `json:"window_analysis_limit"`
	Fields              []string                     `json:"identity_fields"`
	expires             time.Time
	armedAt             time.Time
	lastRF              time.Time
	firstPulse          time.Time
	packets             []receiver.PulsePacket
	bytes               int
	ctx                 context.Context
	cancel              context.CancelFunc
}

var defaultFields = []string{"id", "channel", "command", "state", "button", "code", "codes"}

func DefaultConfig() Config {
	return Config{SchemaVersion: 2, DecoderProfiles: []receiver.DecoderProfile{}, Remotes: []Remote{}, Settings: Settings{Protocols: []int{}, IdentityFields: append([]string{}, defaultFields...), AllowedOrigins: []string{"http://127.0.0.1:8123"}}}
}

func migrateConfig(c *Config) {
	if c.SchemaVersion == 1 {
		c.SchemaVersion = 2
	}
	if c.DecoderProfiles == nil {
		c.DecoderProfiles = []receiver.DecoderProfile{}
	}
}
func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func newButton(name string) Button {
	return Button{ID: newID(), Name: name, DebounceMS: 700, IdentityFields: append([]string{}, defaultFields...), Webhook: Webhook{Method: "POST", Headers: map[string]string{}, TimeoutMS: 5000}}
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func validateName(s string) error {
	if strings.TrimSpace(s) == "" || len(s) > 100 {
		return errors.New("name must contain 1–100 characters")
	}
	return nil
}

var fieldName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)
var volatile = map[string]bool{"model": true, "time": true, "timestamp": true, "rssi": true, "snr": true, "noise": true, "freq": true, "freq1": true, "freq2": true, "mod": true, "count": true, "repeats": true, "temperature_C": true, "humidity": true, "battery_ok": true, "mic": true}

func validateFields(fields []string) error {
	if len(fields) == 0 || len(fields) > 16 {
		return errors.New("choose 1–16 stable identity fields")
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if !fieldName.MatchString(f) || volatile[f] || seen[f] {
			return fmt.Errorf("invalid, duplicate or volatile identity field: %s", f)
		}
		seen[f] = true
	}
	return nil
}
func origin(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u == nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("destination must be an HTTP(S) URL without credentials or fragment")
	}
	if strings.ContainsAny(raw, "\r\n\t") {
		return "", errors.New("invalid URL")
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}
func validateSettings(s Settings) error {
	if len(s.ReceiverSerial) > 128 || strings.ContainsAny(s.ReceiverSerial, "\r\n\x00") {
		return errors.New("invalid receiver serial")
	}
	if len(s.FlexDecoder) > 4096 || strings.ContainsAny(s.FlexDecoder, "\r\n\x00") {
		return errors.New("invalid flex decoder")
	}
	if len(s.Protocols) > 32 {
		return errors.New("too many protocols")
	}
	for _, p := range s.Protocols {
		if p < 1 || p > 10000 {
			return errors.New("protocol IDs must be positive")
		}
	}
	if e := validateFields(s.IdentityFields); e != nil {
		return e
	}
	if len(s.AllowedOrigins) > 64 {
		return errors.New("too many destination origins")
	}
	for _, o := range s.AllowedOrigins {
		canonical, e := origin(o)
		if e != nil {
			return e
		}
		u, _ := url.Parse(o)
		if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || canonical != o {
			return errors.New("allowed origins must be lowercase scheme://host:port without path, query or trailing slash")
		}
	}
	return nil
}
func validateWebhook(w Webhook, s Settings) error {
	if w.Method != "GET" && w.Method != "POST" {
		return errors.New("webhook method must be GET or POST")
	}
	if w.TimeoutMS < 100 || w.TimeoutMS > 30000 {
		return errors.New("HTTP timeout must be 100–30000 ms")
	}
	if len(w.Payload) > 65536 || (w.Payload != "" && !json.Valid([]byte(w.Payload))) {
		return errors.New("payload must be valid JSON, at most 64 KiB")
	}
	if w.Method == "GET" && w.Payload != "" {
		return errors.New("GET requests cannot have a payload")
	}
	if len(w.Headers) > 32 {
		return errors.New("too many headers")
	}
	headerNames := make(map[string]bool, len(w.Headers))
	for k, v := range w.Headers {
		if len(k) > 128 || len(v) > 4096 || !headerName.MatchString(k) || strings.ContainsAny(v, "\r\n\x00") {
			return errors.New("invalid HTTP header")
		}
		canonical := http.CanonicalHeaderKey(k)
		if headerNames[canonical] {
			return errors.New("duplicate HTTP header name (header names are case-insensitive)")
		}
		headerNames[canonical] = true
		switch canonical {
		case "Host", "Content-Length", "Transfer-Encoding", "Connection", "Proxy-Authorization", "Proxy-Connection", "Upgrade", "Trailer":
			return errors.New("reserved HTTP header")
		}
	}
	if w.URL == "" {
		return nil
	}
	if len(w.URL) > 4096 {
		return errors.New("URL too long")
	}
	o, e := origin(w.URL)
	if e != nil {
		return e
	}
	for _, allowed := range s.AllowedOrigins {
		if o == allowed {
			return nil
		}
	}
	return errors.New("destination origin is not allowed; add it in receiver/settings first")
}

var headerName = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

func validateButton(b Button, s Settings) error {
	if e := validateName(b.Name); e != nil {
		return e
	}
	if b.DebounceMS < 100 || b.DebounceMS > 60000 {
		return errors.New("debounce must be 100–60000 ms")
	}
	if e := validateFields(b.IdentityFields); e != nil {
		return e
	}
	return validateWebhook(b.Webhook, s)
}
func validateConfig(c Config) error {
	// A backup is emitted as compact JSON plus one newline. Keep every accepted
	// configuration within the restore endpoint's 2 MiB request limit.
	encoded, err := json.Marshal(c)
	if err != nil || len(encoded) >= 2<<20 {
		return errors.New("configuration exceeds the 2 MiB backup/restore limit")
	}
	if c.SchemaVersion != 2 {
		return errors.New("unsupported backup schema")
	}
	if e := validateSettings(c.Settings); e != nil {
		return e
	}
	if len(c.Remotes) > 100 {
		return errors.New("maximum 100 remotes")
	}
	if len(c.DecoderProfiles) > 16 {
		return errors.New("maximum 16 learned decoder profiles")
	}
	profiles := map[string]receiver.DecoderProfile{}
	for _, p := range c.DecoderProfiles {
		if err := p.Validate(true); err != nil {
			return err
		}
		if _, exists := profiles[p.ID]; exists {
			return errors.New("duplicate decoder profile")
		}
		profiles[p.ID] = p
	}
	ids := map[string]bool{}
	// Parse each signature once. Comparing raw canonical values preserves large
	// integer device identifiers instead of rounding them through float64.
	signaturesByModel := make(map[string][]map[string]json.RawMessage)
	for _, r := range c.Remotes {
		if !validID.MatchString(r.ID) || ids[r.ID] {
			return errors.New("invalid or duplicate remote id")
		}
		ids[r.ID] = true
		if e := validateName(r.Name); e != nil {
			return e
		}
		if r.Columns < 1 || r.Columns > 6 || len(r.Buttons) > 64 {
			return errors.New("invalid layout")
		}
		for _, b := range r.Buttons {
			if !validID.MatchString(b.ID) || ids[b.ID] {
				return errors.New("invalid or duplicate button id")
			}
			ids[b.ID] = true
			if e := validateButton(b, c.Settings); e != nil {
				return e
			}
			if b.DecoderProfileID != "" {
				p, ok := profiles[b.DecoderProfileID]
				var signal struct {
					Model string `json:"model"`
				}
				if !ok || json.Unmarshal([]byte(b.Signature), &signal) != nil || signal.Model != p.Model {
					return errors.New("mapping references an absent or different decoder profile")
				}
			}
			if b.Signature != "" {
				canonical, err := Signature([]byte(b.Signature), b.IdentityFields)
				if err != nil || len(b.Signature) > 8192 || canonical != b.Signature {
					return errors.New("mapping must be a canonical signal using its own identity fields")
				}
				var values map[string]json.RawMessage
				if err := json.Unmarshal([]byte(b.Signature), &values); err != nil {
					return errors.New("invalid mapping")
				}
				model := string(values["model"])
				for _, previous := range signaturesByModel[model] {
					if signatureValuesOverlap(values, previous) {
						return errors.New("signal overlaps an existing button mapping")
					}
				}
				signaturesByModel[model] = append(signaturesByModel[model], values)
			}
		}
	}
	return nil
}

// Inputs must be canonical signatures, as checked by validateConfig or produced
// by Signature. Conflicting shared fields distinguish two mappings; otherwise
// one future event could satisfy both, so they are conservatively overlapping.
func signatureValuesOverlap(a, b map[string]json.RawMessage) bool {
	if len(a["model"]) == 0 || string(a["model"]) != string(b["model"]) {
		return false
	}
	for key, value := range a {
		if other, shared := b[key]; shared && string(value) != string(other) {
			return false
		}
	}
	return true
}

var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Signature uses selected scalar identity fields; RF metadata never participates.
// A flex event with several distinct codes is ambiguous and cannot be learned.
func Signature(raw []byte, fields []string) (string, error) {
	if len(raw) > 65536 {
		return "", errors.New("RF frame too large")
	}
	if !json.Valid(raw) {
		return "", errors.New("invalid RF JSON")
	}
	if e := validateFields(fields); e != nil {
		return "", e
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var m map[string]any
	if e := dec.Decode(&m); e != nil || m == nil {
		return "", errors.New("invalid RF JSON")
	}
	model, ok := m["model"].(string)
	if !ok || strings.TrimSpace(model) == "" || len(model) > 128 {
		return "", errors.New("RF event has no model")
	}
	out := map[string]any{"model": model}
	action, device := false, false
	for _, k := range fields {
		v, exists := m[k]
		if !exists {
			continue
		}
		if k == "codes" {
			arr, ok := v.([]any)
			if !ok {
				return "", errors.New("codes must be an array")
			}
			codes := map[string]bool{}
			for _, x := range arr {
				str, ok := x.(string)
				if !ok || len(str) > 2048 {
					return "", errors.New("invalid flex code")
				}
				codes[strings.ToLower(strings.TrimSpace(str))] = true
			}
			if len(codes) != 1 {
				return "", errors.New("multiple distinct flex codes; adjust decoder before learning")
			}
			for code := range codes {
				if code == "" {
					return "", errors.New("empty flex code")
				}
				v = []string{code}
			}
		}
		switch v.(type) {
		case string, json.Number, bool, []string:
		default:
			return "", errors.New("identity fields must be scalar (except codes)")
		}
		if value, ok := v.(string); ok && strings.TrimSpace(value) == "" {
			return "", errors.New("identity fields cannot be empty")
		}
		if _, boolean := v.(bool); boolean && (k == "code" || k == "id" || k == "address" || k == "device_id") {
			return "", errors.New("device identifiers and codes cannot be boolean values")
		}
		out[k] = v
		switch k {
		case "code", "codes":
			action = true
			device = true
		case "id", "address", "device_id":
			device = true
		case "command", "state", "button", "action":
			action = true
		}
	}
	if len(out) < 2 || !device || !action {
		return "", errors.New("need a code or both device ID and button/command/state; model alone is unsafe")
	}
	b, e := json.Marshal(out)
	if len(b) > 8192 {
		return "", errors.New("signal identity exceeds 8 KiB; choose narrower identity fields")
	}
	return string(b), e
}
