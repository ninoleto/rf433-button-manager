package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreReopenAndRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manager.db")
	s, c, e := OpenStore(path)
	if e != nil {
		t.Fatal(e)
	}
	c.Settings.ReceiverSerial = "portable-serial"
	if e = s.Save(c); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 510; i++ {
		if _, e = s.Record(Event{Kind: "test", Message: "record"}); e != nil {
			t.Fatal(e)
		}
	}
	s.Close()
	s, c, e = OpenStore(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if c.Settings.ReceiverSerial != "portable-serial" {
		t.Fatal("settings lost")
	}
	events, e := s.Events(1000)
	if e != nil || len(events) != 500 {
		t.Fatal("history unbounded", len(events), e)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0600 {
		t.Fatal("unsafe DB permissions")
	}
}
func TestCorruptDatabaseNeverReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manager.db")
	raw := []byte("not a database")
	os.WriteFile(path, raw, 0600)
	if _, _, e := OpenStore(path); e == nil {
		t.Fatal("corrupt DB accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(raw) {
		t.Fatal("corrupt DB was overwritten")
	}
}
