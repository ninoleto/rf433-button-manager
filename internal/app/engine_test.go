package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct{ n atomic.Int64 }

func (c *clock) now() time.Time          { return time.Unix(0, c.n.Load()) }
func (c *clock) advance(d time.Duration) { c.n.Add(int64(d)) }
func testApp(t *testing.T) (*App, *clock) {
	t.Helper()
	s, c, e := OpenStore(filepath.Join(t.TempDir(), "manager.db"))
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s, c, false)
	if e != nil {
		t.Fatal(e)
	}
	clk := &clock{}
	clk.n.Store(time.Now().Add(time.Minute).UnixNano())
	a.now = clk.now
	t.Cleanup(func() { a.Close(); s.Close() })
	return a, clk
}

var signalA = []byte(`{"model":"Switch433","codes":["{24}abc001","{24}abc001"],"time":"ignored","rssi":-12}`)
var signalB = []byte(`{"model":"Switch433","codes":["{24}abc002"]}`)

func remote(t *testing.T, a *App) Remote {
	t.Helper()
	r, e := a.CreateRemote("Living room", 2, 6)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func learn(t *testing.T, a *App, c *clock, id string, raw []byte) {
	t.Helper()
	if _, e := a.StartLearning(id, nil); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		c.advance(time.Second)
		a.Receive(raw)
		if i < 2 {
			if _, e := a.ArmLearning(); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := a.ConfirmLearning(); e != nil {
		t.Fatal(e)
	}
	c.advance(2 * time.Second)
}
func configure(t *testing.T, a *App, id, url string) {
	t.Helper()
	s := a.Settings()
	o, _ := origin(url)
	found := false
	for _, x := range s.AllowedOrigins {
		if x == o {
			found = true
		}
	}
	if !found {
		s.AllowedOrigins = append(s.AllowedOrigins, o)
		if e := a.UpdateSettings(s); e != nil {
			t.Fatal(e)
		}
	}
	c := a.Backup()
	b := *findButton(&c, id)
	b.Enabled = true
	b.Webhook.URL = url
	if _, e := a.EditButton(id, b); e != nil {
		t.Fatal(e)
	}
}
func TestLearningRequiresSeparateArmedPresses(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	id := r.Buttons[0].ID
	if _, e := a.StartLearning(id, nil); e != nil {
		t.Fatal(e)
	}
	a.Receive(signalA)
	for i := 0; i < 30; i++ {
		c.advance(20 * time.Millisecond)
		a.Receive(signalA)
	}
	if a.learning.Presses != 1 {
		t.Fatal("burst counted as presses")
	}
	if e := a.ConfirmLearning(); e == nil {
		t.Fatal("confirmed one press")
	}
	if _, e := a.ArmLearning(); e != nil {
		t.Fatal(e)
	}
	c.advance(100 * time.Millisecond)
	a.Receive(signalA)
	if a.learning.Presses != 1 {
		t.Fatal("held press counted after arming")
	}
	c.advance(time.Second)
	a.Receive(signalB)
	if a.learning.Presses != 1 {
		t.Fatal("unrelated code accepted")
	}
	a.Receive(signalA)
	if a.learning.Presses != 2 {
		t.Fatal("second separate press not counted")
	}
	a.ArmLearning()
	c.advance(time.Second)
	a.Receive(signalA)
	if e := a.ConfirmLearning(); e != nil {
		t.Fatal(e)
	}
	backup := a.Backup()
	b := findButton(&backup, id)
	if b.Signature == "" || b.Enabled {
		t.Fatal("mapping not saved disabled")
	}
}
func TestLearningDuplicateCancelTimeout(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	if _, e := a.StartLearning(r.Buttons[1].ID, nil); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		c.advance(time.Second)
		a.Receive(signalA)
		if i < 2 {
			a.ArmLearning()
		}
	}
	if e := a.ConfirmLearning(); e == nil {
		t.Fatal("duplicate allowed")
	}
	a.CancelLearning()
	backup := a.Backup()
	if findButton(&backup, r.Buttons[1].ID).Signature != "" {
		t.Fatal("duplicate persisted")
	}
	old := findButton(&backup, r.Buttons[0].ID).Signature
	a.StartLearning(r.Buttons[0].ID, nil)
	c.advance(91 * time.Second)
	a.State()
	if a.learning != nil {
		t.Fatal("session did not expire")
	}
	backup = a.Backup()
	if findButton(&backup, r.Buttons[0].ID).Signature != old {
		t.Fatal("timeout destroyed old mapping")
	}
}
func TestDebounceHeldButtonAndRearm(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	id := r.Buttons[0].ID
	learn(t, a, c, id, signalA)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	configure(t, a, id, server.URL)
	a.Receive(signalA)
	a.wg.Wait()
	for i := 0; i < 30; i++ {
		c.advance(100 * time.Millisecond)
		a.Receive(signalA)
		a.wg.Wait()
	}
	if calls.Load() != 1 {
		t.Fatalf("held button sent %d actions", calls.Load())
	}
	c.advance(701 * time.Millisecond)
	a.Receive(signalA)
	a.wg.Wait()
	if calls.Load() != 2 {
		t.Fatal("new press not accepted")
	}
}
func TestLearningSuppressesAllActionsAndTests(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	configure(t, a, r.Buttons[0].ID, server.URL)
	a.StartLearning(r.Buttons[1].ID, nil)
	a.Receive(signalA)
	a.wg.Wait()
	if e := a.TestButton(r.Buttons[0].ID); e == nil {
		t.Fatal("test allowed during learn")
	}
	if calls.Load() != 0 {
		t.Fatal("webhook fired during learning")
	}
	a.CancelLearning()
	a.Receive(signalA)
	a.wg.Wait()
	if calls.Load() != 0 {
		t.Fatal("trailing burst after cancel fired")
	}
}
func TestDifferentButtonsDispatchConcurrently(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	learn(t, a, c, r.Buttons[1].ID, signalB)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release }))
	defer server.Close()
	configure(t, a, r.Buttons[0].ID, server.URL)
	configure(t, a, r.Buttons[1].ID, server.URL)
	a.Receive(signalA)
	a.Receive(signalB)
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("different button blocked by active webhook")
		}
	}
	close(release)
	a.wg.Wait()
}
func TestLearningWaitsForInFlightRequest(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-release }))
	defer server.Close()
	configure(t, a, r.Buttons[0].ID, server.URL)
	a.Receive(signalA)
	<-entered
	done := make(chan error, 1)
	go func() { _, e := a.StartLearning(r.Buttons[1].ID, nil); done <- e }()
	select {
	case <-done:
		close(release)
		t.Fatal("learning started before in-flight action finished")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("learning did not start")
	}
	a.wg.Wait()
}
func TestPersistRestoreAndDuplicate(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	backup := a.Backup()
	duplicate, e := a.DuplicateRemote(r.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range duplicate.Buttons {
		if b.Enabled || b.Signature != "" {
			t.Fatal("duplicate retained action mapping")
		}
	}
	backup.Remotes[0].Buttons[0].Enabled = true
	if e = a.Restore(backup); e != nil {
		t.Fatal(e)
	}
	if a.Backup().Remotes[0].Buttons[0].Enabled {
		t.Fatal("restore enabled actions")
	}
	stored := Config{}
	var raw string
	if e = a.store.db.QueryRow("SELECT value FROM configuration WHERE id=1").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal([]byte(raw), &stored); e != nil {
		t.Fatal(e)
	}
	if stored.Remotes[0].Buttons[0].Signature != backup.Remotes[0].Buttons[0].Signature {
		t.Fatal("mapping not persisted")
	}
	before, _ := json.Marshal(a.Backup())
	bad := clone(backup)
	bad.SchemaVersion = 99
	if e = a.Restore(bad); e == nil {
		t.Fatal("bad restore accepted")
	}
	after, _ := json.Marshal(a.Backup())
	if string(before) != string(after) {
		t.Fatal("failed restore mutated state")
	}
}
func TestStorageFailureDisablesActions(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalA)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("webhook on failed storage") }))
	defer server.Close()
	configure(t, a, r.Buttons[0].ID, server.URL)
	a.store.Close()
	a.Receive(signalA)
	a.wg.Wait()
	if !a.storageErr {
		t.Fatal("storage error not latched")
	}
	if e := a.TestButton(r.Buttons[0].ID); e == nil {
		t.Fatal("test permitted after storage failure")
	}
}
func TestGlobalIdentityFieldsApplyToNewButtons(t *testing.T) {
	a, _ := testApp(t)
	s := a.Settings()
	s.IdentityFields = []string{"code"}
	if e := a.UpdateSettings(s); e != nil {
		t.Fatal(e)
	}
	r := remote(t, a)
	if strings.Join(r.Buttons[0].IdentityFields, ",") != "code" {
		t.Fatal("new remote did not inherit settings")
	}
	b, e := a.CreateButton(r.ID, "New")
	if e != nil || strings.Join(b.IdentityFields, ",") != "code" {
		t.Fatal("new button did not inherit settings")
	}
}
