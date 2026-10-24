//go:build linux

package receiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDiscoverUSBTopologyAndSerial(t *testing.T) {
	root := t.TempDir()
	add := func(name, vid, pid, serial, num string) {
		t.Helper()
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for k, v := range map[string]string{"idVendor": vid, "idProduct": pid, "serial": serial, "busnum": "2", "devnum": num} {
			if err := os.WriteFile(filepath.Join(dir, k), []byte(v+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("2-1", "0bda", "2838", "00000001", "8")
	add("2-2", "1d6b", "0003", "hub", "9")
	ds, err := discoverUSB(root)
	if err != nil || len(ds) != 1 || ds[0].serial != "00000001" {
		t.Fatalf("%v %v", ds, err)
	}
	old := ds[0].incarnation
	if err := os.WriteFile(filepath.Join(root, "2-1", "devnum"), []byte("10"), 0600); err != nil {
		t.Fatal(err)
	}
	ds, _ = discoverUSB(root)
	if ds[0].incarnation == old {
		t.Fatal("missed same-port re-enumeration")
	}
	add("2-3", "0ccd", "00b3", "another-brand", "11")
	ds, _ = discoverUSB(root)
	if len(ds) != 2 {
		t.Fatal("failed to count non-Realtek librtlsdr device")
	}
	if _, err = selectDevice(ds, ""); err == nil {
		t.Fatal("unsafe numeric selection")
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-rtl_433")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNativeProcessOutputAndShutdown(t *testing.T) {
	p, err := startProcess(context.Background(), writeScript(t, "printf '%s\\n' '"+signalJSON+"'\nwhile :; do sleep 1; done\n"), nil, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.stop)
	if rec := receive(t, p.output()); string(rec.data) != signalJSON {
		t.Fatal(string(rec.data))
	}
	start := time.Now()
	p.stop()
	if time.Since(start) > time.Second {
		t.Fatal("slow shutdown")
	}
	// Second stop must be harmless.
	p.stop()
}

func TestNativeProcessOversizedOutputFailsBoundedly(t *testing.T) {
	script := writeScript(t, "head -c 300000 /dev/zero | tr '\\000' x\nprintf '\\n'\nwhile :; do sleep 1; done\n")
	p, err := startProcess(context.Background(), script, nil, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer p.stop()
	if rec := receive(t, p.output()); rec.err == nil {
		t.Fatal("accepted oversized line")
	}
}

func TestNativeProcessDrainsStderr(t *testing.T) {
	script := writeScript(t, "i=0; while [ \"$i\" -lt 5000 ]; do echo 'diagnostic noise' >&2; i=$((i+1)); done\nprintf '%s\\n' '"+signalJSON+"'\n")
	p, err := startProcess(context.Background(), script, nil, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer p.stop()
	if rec := receive(t, p.output()); string(rec.data) != signalJSON {
		t.Fatal(string(rec.data))
	}
	if err = receive(t, p.exited()); err != nil {
		t.Fatal(err)
	}
}

func TestNativeProcessForcesTermination(t *testing.T) {
	script := writeScript(t, "trap '' TERM\nprintf '%s\\n' '"+signalJSON+"'\nwhile :; do sleep 1; done\n")
	p, err := startProcess(context.Background(), script, nil, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer p.stop()
	receive(t, p.output())
	pid := p.(*nativeProcess).cmd.Process.Pid
	p.stop()
	if err = syscall.Kill(pid, 0); err == nil {
		t.Fatalf("process %d survived", pid)
	}
}

func TestNativeProcessUsesArgvWithoutShell(t *testing.T) {
	script := writeScript(t, "printf '%s\\n' \"$1\"\n")
	arg := fmt.Sprintf("$(touch %s); echo bad", filepath.Join(t.TempDir(), "bad"))
	p, err := startProcess(context.Background(), script, []string{arg}, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer p.stop()
	if got := string(receive(t, p.output()).data); got != arg || !strings.Contains(got, "$(") {
		t.Fatal(got)
	}
}
