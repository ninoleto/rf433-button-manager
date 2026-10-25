//go:build linux

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

// Runs the production analysis, three-arm learning, store reload, live-event
// normalization and webhook dispatch with private saved recordings. It opens
// no receiver and uses only an isolated loopback HTTP server.
func TestPrivateTwoSwitchLearningToWebhookOptional(t *testing.T) {
	dir := os.Getenv("RFB_PRIVATE_CAPTURE_DIR")
	if dir == "" {
		t.Skip("private operator recordings not supplied")
	}
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "regression.json"))
	if err != nil {
		t.Fatal(err)
	}
	var groups []struct {
		Name, Code string
		Files      []string
	}
	if err = json.Unmarshal(data, &groups); err != nil || len(groups) < 2 {
		t.Fatal("private capture manifest", err)
	}
	groups = groups[:2]
	path := filepath.Join(t.TempDir(), "manager.db")
	store, cfg, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(store, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{}
	c.n.Store(time.Now().Add(time.Minute).UnixNano())
	a.now = c.now
	a.captureEnabled, a.rtlBinary, a.analyzeCapture = true, binary, receiver.AnalyzeCapture
	t.Cleanup(func() { a.Close(); store.Close() })
	r := remote(t, a)
	for which, group := range groups {
		generation := startCapture(t, a, r.Buttons[which].ID)
		for press := 0; press < 3; press++ {
			raw, err := os.ReadFile(filepath.Join(dir, group.Files[press]))
			if err != nil {
				t.Fatal(err)
			}
			pulses := 0
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(line, ";ook ") {
					_, _ = fmt.Sscanf(line, ";ook %d pulses", &pulses)
					break
				}
			}
			c.advance(1100 * time.Millisecond)
			a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: pulses, Data: raw})
			c.advance(1200 * time.Millisecond)
			a.learningTick()
			if a.learning == nil || a.learning.Presses != press+1 {
				t.Fatalf("%s press %d not learned: %+v %+v", group.Name, press+1, a.learning, a.learningNotice)
			}
			if !strings.Contains(a.learning.Candidate, group.Code) {
				t.Fatal("wrong learning identity")
			}
			if press < 2 {
				if _, err = a.ArmLearning(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err = a.ConfirmLearning(); err != nil {
			t.Fatal(err)
		}
		_, generation = a.ReceiverPlan(binary)
		a.ReceiverPlanApplied(generation)
		c.advance(2 * time.Second)
	}
	backup := a.Backup()
	if len(backup.DecoderProfiles) != 2 || backup.Remotes[0].Buttons[0].Enabled || backup.Remotes[0].Buttons[1].Enabled {
		t.Fatal("profiles or disabled-action state not preserved")
	}
	// Reopen the actual saved database; persisted profiles must validate normally.
	otherStore, otherConfig, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(otherStore, otherConfig, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close(); otherStore.Close() })
	b.now = c.now
	var calls [2]atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/first" {
			calls[0].Add(1)
		} else if req.URL.Path == "/second" {
			calls[1].Add(1)
		} else {
			t.Error("unexpected webhook destination")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	configure(t, b, r.Buttons[0].ID, server.URL+"/first")
	configure(t, b, r.Buttons[1].ID, server.URL+"/second")
	plan, _ := b.ReceiverPlan(binary)
	if len(plan.Profiles) != 2 {
		t.Fatal("normal reception lost a learned decoder")
	}
	for which, group := range groups {
		args := []string{"-c", "/dev/null", "-r", filepath.Join(dir, group.Files[1]), "-s", "250000", "-R", "0", "-F", "json"}
		for _, profile := range plan.Profiles {
			args = append(args, "-X", profile.FlexDecoder)
		}
		cmd := exec.CommandContext(context.Background(), binary, args...)
		output, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		var events [][]byte
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "{") {
				events = append(events, []byte(line))
			}
		}
		if len(events) == 0 {
			t.Fatal("normal reception replay produced no button event")
		}
		c.advance(2 * time.Second)
		for repeat := 0; repeat < 4; repeat++ {
			for _, event := range events {
				b.Receive(event)
			}
			c.advance(100 * time.Millisecond)
		}
		b.wg.Wait()
		if calls[which].Load() != 1 || calls[1-which].Load() != int32(which) {
			t.Fatalf("wrong webhook or repeated dispatch: %d / %d", calls[0].Load(), calls[1].Load())
		}
	}
	t.Log("two recorded switches learned through three explicit arms, survived database reload, and each dispatched exactly one correct local webhook despite repeated RF messages")
}

func TestPrivateInspectionDiscoversCodeWithoutCreatingRemoteOptional(t *testing.T) {
	dir := os.Getenv("RFB_PRIVATE_CAPTURE_DIR")
	if dir == "" {
		t.Skip("private operator recordings not supplied")
	}
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "regression.json"))
	if err != nil {
		t.Fatal(err)
	}
	var groups []struct {
		Name, Code string
		Files      []string
	}
	if err = json.Unmarshal(data, &groups); err != nil || len(groups) == 0 || len(groups[0].Files) < 3 {
		t.Fatal("private capture manifest", err)
	}
	a, c := testApp(t)
	a.captureEnabled, a.rtlBinary, a.analyzeCapture = true, binary, receiver.AnalyzeCapture
	gen := startInspectionCapture(t, a)
	for press := 0; press < 3; press++ {
		raw, err := os.ReadFile(filepath.Join(dir, groups[0].Files[press]))
		if err != nil {
			t.Fatal(err)
		}
		pulses := 0
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, ";ook ") {
				_, _ = fmt.Sscanf(line, ";ook %d pulses", &pulses)
				break
			}
		}
		c.advance(1100 * time.Millisecond)
		a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Pulses: pulses, Data: raw})
		c.advance(1200 * time.Millisecond)
		a.learningTick()
		if a.learning == nil || a.learning.Presses != press+1 {
			t.Fatal("recorded inspection failed")
		}
		if press < 2 {
			if _, err = a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
		}
	}
	report, err := a.InspectionResult()
	if err != nil || !report.RepeatedIdentifier || report.Profile == nil || !strings.Contains(report.Candidate, groups[0].Code) {
		t.Fatal("inspection did not report recorded code/profile", err)
	}
	if err = a.ConfirmLearning(); err == nil {
		t.Fatal("inspection persisted a mapping")
	}
	if len(a.Backup().Remotes) != 0 || len(a.Backup().DecoderProfiles) != 0 {
		t.Fatal("inspection changed configuration")
	}
	t.Log("private recordings produced a code and candidate decoder across three arms without a remote, mapping or webhook")
}
