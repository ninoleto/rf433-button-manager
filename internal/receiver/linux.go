//go:build linux

package receiver

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const platformSupported = true

func defaultRuntime() runtime {
	return runtime{discover: func() ([]device, error) { return discoverUSB("/sys/bus/usb/devices") }, start: startProcess,
		poll: time.Second, initialBackoff: time.Second, maxBackoff: 30 * time.Second,
		stableAfter: time.Minute, heartbeatTimeout: 45 * time.Second, stopTimeout: 2 * time.Second}
}

// USB IDs recognized by upstream librtlsdr (osmocom/rtl-sdr, src/librtlsdr.c).
// Include the complete upstream table, so singleton/index fallback cannot silently
// pick another supported receiver merely because its vendor is not Realtek.
var rtlUSBIDs = map[string]bool{
	"0bda:2832": true, "0bda:2838": true, "0413:6680": true, "0413:6f0f": true, "0458:707f": true,
	"0ccd:00a9": true, "0ccd:00b3": true, "0ccd:00b4": true, "0ccd:00b5": true, "0ccd:00b7": true,
	"0ccd:00b8": true, "0ccd:00b9": true, "0ccd:00c0": true, "0ccd:00c6": true, "0ccd:00d3": true,
	"0ccd:00d7": true, "0ccd:00e0": true, "1554:5020": true, "15f4:0131": true, "15f4:0133": true,
	"185b:0620": true, "185b:0650": true, "185b:0680": true, "1b80:d393": true, "1b80:d394": true,
	"1b80:d395": true, "1b80:d397": true, "1b80:d398": true, "1b80:d39d": true, "1b80:d3a4": true,
	"1b80:d3a8": true, "1b80:d3af": true, "1b80:d3b0": true, "1d19:1101": true, "1d19:1102": true,
	"1d19:1103": true, "1d19:1104": true, "1f4d:a803": true, "1f4d:b803": true, "1f4d:c803": true,
	"1f4d:d286": true, "1f4d:d803": true,
}

func discoverUSB(root string) ([]device, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var devices []device
	read := func(dir, key string) string {
		b, _ := os.ReadFile(filepath.Join(dir, key))
		return strings.TrimSpace(string(b))
	}
	for _, ent := range entries {
		dir := filepath.Join(root, ent.Name())
		vendor, product := read(dir, "idVendor"), read(dir, "idProduct")
		if !rtlUSBIDs[strings.ToLower(vendor+":"+product)] {
			continue
		}
		// Re-enumeration on the same physical port changes devnum; both values are
		// ephemeral identity only. Nothing under /dev/bus/usb is stored or opened.
		bus, number := read(dir, "busnum"), read(dir, "devnum")
		if bus == "" || number == "" {
			continue
		} // transient incomplete enumeration
		devices = append(devices, device{serial: read(dir, "serial"), incarnation: dir + ":" + bus + ":" + number})
	}
	return devices, nil
}

type nativeProcess struct {
	cmd            *exec.Cmd
	records        chan record
	wait           chan error
	done           chan struct{}
	readers        sync.WaitGroup
	cancel         context.CancelFunc
	stdout, stderr *os.File
	stopOnce       sync.Once
	stopTimeout    time.Duration
	diagnosticMu   sync.Mutex
	diagnostic     string
}

func startProcess(parent context.Context, binary string, args []string, stopTimeout time.Duration) (process, error) {
	ctx, cancel := context.WithCancel(parent)
	outR, outW, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		cancel()
		outR.Close()
		outW.Close()
		return nil, err
	}
	cmd := exec.Command(binary, args...)
	// Dedicated group allows graceful shutdown of all descendants; Pdeathsig
	// covers abrupt manager death. systemd KillMode=control-group is an extra guard.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Stdout, cmd.Stderr = outW, errW
	if err = cmd.Start(); err != nil {
		cancel()
		outR.Close()
		outW.Close()
		errR.Close()
		errW.Close()
		return nil, err
	}
	outW.Close()
	errW.Close()
	p := &nativeProcess{cmd: cmd, records: make(chan record, 64), wait: make(chan error, 1), done: make(chan struct{}), cancel: cancel, stdout: outR, stderr: errR, stopTimeout: stopTimeout}
	p.readers.Add(2)
	scan := func(file *os.File, name string, deliver bool) {
		defer p.readers.Done()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 256*1024)
		for scanner.Scan() {
			if !deliver {
				line := strings.TrimSpace(strings.Map(func(r rune) rune {
					if r < 32 || r == 127 {
						return -1
					}
					return r
				}, scanner.Text()))
				if len(line) > 512 {
					line = line[:512]
				}
				if line != "" {
					p.diagnosticMu.Lock()
					p.diagnostic = line
					p.diagnosticMu.Unlock()
				}
				continue
			} // Drain stderr; never route logs into RF learning.
			r := record{data: append([]byte(nil), scanner.Bytes()...)}
			select {
			case p.records <- r:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			select {
			case p.records <- record{err: lineError(name)}:
			case <-ctx.Done():
			}
		}
	}
	go scan(outR, "stdout", true)
	go scan(errR, "stderr", false)
	go func() { p.wait <- cmd.Wait(); close(p.done) }()
	return p, nil
}

func (p *nativeProcess) output() <-chan record { return p.records }
func (p *nativeProcess) exited() <-chan error  { return p.wait }
func (p *nativeProcess) lastDiagnostic() string {
	p.diagnosticMu.Lock()
	defer p.diagnosticMu.Unlock()
	return p.diagnostic
}
func (p *nativeProcess) stop() {
	p.stopOnce.Do(func() {
		p.cancel()
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(p.stopTimeout)
		defer timer.Stop()
		select {
		case <-p.done:
		case <-timer.C:
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
			<-p.done
		}
		// Kill remaining descendants even if the direct child exited before they did.
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		p.stdout.Close()
		p.stderr.Close()
		p.readers.Wait()
	})
}
