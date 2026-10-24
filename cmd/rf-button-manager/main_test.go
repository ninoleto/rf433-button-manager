package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateListenOnly(t *testing.T) {
	for _, s := range []string{"0.0.0.0:8787", ":8787", "[::]:8787", "8.8.8.8:8787", "example.com:8787"} {
		if validListen(s) == nil {
			t.Error("allowed", s)
		}
	}
	for _, s := range []string{"127.0.0.1:8787", "[::1]:8787", "192.168.1.5:8787", "100.100.1.1:8787"} {
		if e := validListen(s); e != nil {
			t.Error(e)
		}
	}
}
func TestManagementTokenPersistsPrivately(t *testing.T) {
	dir := t.TempDir()
	one, e := tokenFile(dir)
	if e != nil {
		t.Fatal(e)
	}
	two, e := tokenFile(dir)
	if e != nil || one != two || len(one) != 64 {
		t.Fatal("token not stable")
	}
	st, _ := os.Stat(filepath.Join(dir, "management.token"))
	if st.Mode().Perm() != 0600 {
		t.Fatal("unsafe token mode")
	}
	os.WriteFile(filepath.Join(dir, "management.token"), []byte("invalid"), 0600)
	if _, e = tokenFile(dir); e == nil {
		t.Fatal("corrupt token silently replaced")
	}
}
