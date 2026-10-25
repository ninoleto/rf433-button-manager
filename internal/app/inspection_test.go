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

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func startInspectionCapture(t *testing.T, a *App) uint64 {
	t.Helper()
	l, err := a.StartInspection()
	if err != nil || !l.InspectOnly || l.ButtonID != "" {
		t.Fatal("inspection requires a remote or was not marked read-only", err)
	}
	plan, gen := a.ReceiverPlan("rtl_433")
	if !plan.CapturePulses {
		t.Fatal("inspection omitted pulse capture")
	}
	a.ReceiverPlanApplied(gen)
	a.SetReceiverFor(gen, receiver.Status{State: "connected", Message: "samples"})
	return gen
}

func TestInspectionWithoutRemoteCannotPersistMapping(t *testing.T) {
	a, c, r := captureApp(t)
	if err := a.DeleteRemote(r.ID); err != nil {
		t.Fatal(err)
	}
	before := a.Backup()
	gen := startInspectionCapture(t, a)
	if _, err := a.StartInspection(); err == nil {
		t.Fatal("overlapping capture allowed")
	}
	for press := 0; press < 3; press++ {
		capturePress(t, a, c, gen)
		report, err := a.InspectionResult()
		if err != nil || report.MatchingPresses != press+1 || report.Candidate == "" {
			t.Fatal("missing observed code", err)
		}
		if report.RepeatedIdentifier != (press == 2) {
			t.Fatal("claimed repeated identity before three arms")
		}
		if err := a.ConfirmLearning(); err == nil {
			t.Fatal("inspection was able to save")
		}
		if press < 2 {
			if _, err := a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !reflect.DeepEqual(a.Backup(), before) {
		t.Fatal("inspection altered configuration")
	}
	if _, err := a.ExportRFCapture(); err != nil {
		t.Fatal("missing raw diagnostic capture", err)
	}
	if err := a.CancelLearning(); err != nil {
		t.Fatal(err)
	}
	if !a.captureSuppressed {
		t.Fatal("actions resumed before receiver restoration")
	}
	plan, restore := a.ReceiverPlan("rtl_433")
	if plan.CapturePulses {
		t.Fatal("inspection receiver was not restored")
	}
	a.ReceiverPlanApplied(restore - 1)
	if !a.captureSuppressed {
		t.Fatal("stale receiver confirmation unpaused actions")
	}
	a.ReceiverPlanApplied(restore)
	if a.captureSuppressed || !reflect.DeepEqual(a.Backup(), before) {
		t.Fatal("restoration changed mappings")
	}
}

func TestInspectionPausesActionsAndExportsOnlyAuthenticatedRFData(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	configure(t, a, r.Buttons[0].ID, server.URL+"/PRIVATE-WEBHOOK")
	before := a.Backup()
	a.captureEnabled = true
	startInspectionCapture(t, a)
	for press := 0; press < 3; press++ {
		c.advance(2 * time.Second)
		a.Receive(signalA)
		if press < 2 {
			c.advance(time.Second)
			if _, err := a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := a.TestButton(r.Buttons[0].ID); err == nil {
		t.Fatal("manual webhook permitted during inspection")
	}
	a.wg.Wait()
	if calls.Load() != 0 || !reflect.DeepEqual(a.Backup(), before) {
		t.Fatal("inspection dispatched or edited an action")
	}
	h := a.Handler("PRIVATE-TOKEN", fstest.MapFS{})
	for _, auth := range []bool{false, true} {
		req := httptest.NewRequest("GET", "/api/inspect/result", nil)
		if auth {
			req.Header.Set("Authorization", "Bearer PRIVATE-TOKEN")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		want := 401
		if auth {
			want = 200
		}
		if w.Code != want || strings.Contains(w.Body.String(), "PRIVATE-") {
			t.Fatal("inspection export authentication/privacy", w.Code)
		}
		if auth {
			var report InspectionReport
			if json.Unmarshal(w.Body.Bytes(), &report) != nil || !report.RepeatedIdentifier || report.Candidate == "" {
				t.Fatal("missing report evidence")
			}
		}
	}
}

func TestInspectionFailureRetainsUnidentifiedTrafficWithoutCompatibilityClaim(t *testing.T) {
	a, c, _ := captureApp(t)
	before := a.Backup()
	gen := startInspectionCapture(t, a)
	a.ReceivePulse(gen, syntheticFSK(t))
	c.advance(21 * time.Second)
	report, err := a.InspectionResult()
	if err != nil || report.Stage != "press_timeout" || report.RepeatedIdentifier || report.Candidate != "" || report.Diagnostics.FSKPackets != 1 {
		t.Fatal("unidentified traffic misrepresented", err)
	}
	if !strings.Contains(report.Limitations, "No decode does not identify rolling codes") {
		t.Fatal("failure was overinterpreted")
	}
	if !reflect.DeepEqual(a.Backup(), before) {
		t.Fatal("failed inspection modified configuration")
	}
	if _, err := a.ExportRFCapture(); err != nil {
		t.Fatal("failed signal capture unavailable", err)
	}
}

func TestInspectionStartEndpointNeedsAuthenticationAndStrictBody(t *testing.T) {
	a, _ := testApp(t)
	h := a.Handler("fixture", fstest.MapFS{})
	for _, tc := range []struct {
		body, token, origin string
		want                int
	}{
		{`{}`, "", "", 401}, {`{}`, "fixture", "https://other.invalid", 403},
		{`{"button_id":"invented"}`, "fixture", "", 400}, {`{}`, "fixture", "", 200},
	} {
		req := httptest.NewRequest("POST", "/api/inspect/start", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
}
