package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func readJSON(w http.ResponseWriter, r *http.Request, out any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(out); e != nil {
		return errors.New("invalid JSON body or unsupported fields")
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func (a *App) Handler(token string, static fs.FS, publicOrigins ...string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "running", "version": Version})
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.State()) })
	handle := func(pattern string, fn func(http.ResponseWriter, *http.Request) (any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			v, e := fn(w, r)
			if e != nil {
				writeJSON(w, 400, map[string]string{"error": e.Error()})
				return
			}
			if v == nil {
				v = map[string]bool{"ok": true}
			}
			writeJSON(w, 200, v)
		})
	}
	handle("GET /api/receivers", func(w http.ResponseWriter, r *http.Request) (any, error) {
		devices, err := a.discoverReceivers()
		return struct {
			Devices any `json:"devices"`
		}{devices}, err
	})
	handle("POST /api/remotes", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var b struct {
			Name        string `json:"name"`
			Columns     int    `json:"columns"`
			ButtonCount int    `json:"button_count"`
		}
		if e := readJSON(w, r, &b); e != nil {
			return nil, e
		}
		return a.CreateRemote(b.Name, b.Columns, b.ButtonCount)
	})
	handle("PUT /api/remotes/{id}", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var b struct {
			Name    string `json:"name"`
			Columns int    `json:"columns"`
		}
		if e := readJSON(w, r, &b); e != nil {
			return nil, e
		}
		return a.EditRemote(r.PathValue("id"), b.Name, b.Columns)
	})
	handle("DELETE /api/remotes/{id}", func(w http.ResponseWriter, r *http.Request) (any, error) {
		return nil, a.DeleteRemote(r.PathValue("id"))
	})
	handle("POST /api/remotes/{id}/duplicate", func(w http.ResponseWriter, r *http.Request) (any, error) { return a.DuplicateRemote(r.PathValue("id")) })
	handle("POST /api/remotes/{id}/buttons", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var b struct {
			Name string `json:"name"`
		}
		if e := readJSON(w, r, &b); e != nil {
			return nil, e
		}
		return a.CreateButton(r.PathValue("id"), b.Name)
	})
	handle("PUT /api/buttons/{id}", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var b struct {
			Button
			AllowWebhookOrigin string `json:"allow_webhook_origin"`
		}
		if e := readJSON(w, r, &b); e != nil {
			return nil, e
		}
		return a.editButton(r.PathValue("id"), b.Button, b.AllowWebhookOrigin)
	})
	handle("DELETE /api/buttons/{id}", func(w http.ResponseWriter, r *http.Request) (any, error) {
		return nil, a.DeleteButton(r.PathValue("id"))
	})
	handle("POST /api/buttons/{id}/clear", func(w http.ResponseWriter, r *http.Request) (any, error) {
		return nil, a.ClearButton(r.PathValue("id"))
	})
	handle("POST /api/buttons/{id}/test", func(w http.ResponseWriter, r *http.Request) (any, error) { return nil, a.TestButton(r.PathValue("id")) })
	handle("POST /api/learn/start", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var b struct {
			ButtonID string   `json:"button_id"`
			Fields   []string `json:"identity_fields"`
		}
		if e := readJSON(w, r, &b); e != nil {
			return nil, e
		}
		return a.StartLearning(b.ButtonID, b.Fields)
	})
	handle("POST /api/inspect/start", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var body struct{}
		if err := readJSON(w, r, &body); err != nil {
			return nil, err
		}
		return a.StartInspection()
	})
	handle("GET /api/inspect/result", func(w http.ResponseWriter, r *http.Request) (any, error) {
		result, err := a.InspectionResult()
		if err == nil {
			w.Header().Set("Content-Disposition", `attachment; filename="rf-button-manager-inspection.json"`)
		}
		return result, err
	})
	handle("POST /api/learn/arm", func(w http.ResponseWriter, r *http.Request) (any, error) { return a.ArmLearning() })
	handle("POST /api/learn/confirm", func(w http.ResponseWriter, r *http.Request) (any, error) { return nil, a.ConfirmLearning() })
	handle("POST /api/learn/cancel", func(w http.ResponseWriter, r *http.Request) (any, error) { return nil, a.CancelLearning() })
	handle("GET /api/learn/capture", func(w http.ResponseWriter, r *http.Request) (any, error) {
		capture, err := a.ExportRFCapture()
		if err == nil {
			w.Header().Set("Content-Disposition", `attachment; filename="rf-button-manager-capture.json"`)
		}
		return capture, err
	})
	handle("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var s Settings
		if e := readJSON(w, r, &s); e != nil {
			return nil, e
		}
		e := a.UpdateSettings(s)
		return s, e
	})
	mux.HandleFunc("GET /api/backup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="rf-button-manager-backup.json"`)
		writeJSON(w, 200, a.Backup())
	})
	handle("POST /api/restore", func(w http.ResponseWriter, r *http.Request) (any, error) {
		var c Config
		if e := readJSON(w, r, &c); e != nil {
			return nil, e
		}
		return nil, a.Restore(c)
	})
	mux.Handle("/", http.FileServer(http.FS(static)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				writeJSON(w, 401, map[string]string{"error": "Management token required"})
				return
			}
			if o := r.Header.Get("Origin"); o != "" {
				u, e := url.Parse(o)
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				trusted := false
				for _, allowed := range publicOrigins {
					if allowed != "" && o == allowed {
						trusted = true
					}
				}
				if !trusted && (e != nil || u.Scheme != scheme || u.Host != r.Host || u.Path != "") {
					writeJSON(w, 403, map[string]string{"error": "Cross-origin request rejected"})
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeJSON(w, 403, map[string]string{"error": "Cross-site request rejected"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
