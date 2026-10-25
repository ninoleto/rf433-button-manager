//go:build linux

package receiver

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

func intString(n int) string { return strconv.Itoa(n) }

// File input is mandatory at the caller; this child never owns a USB receiver.
func offlineCommand(parent context.Context, binary string, args []string) (string, string, error) {
	if binary == "" {
		binary = "rtl_433"
	}
	hasFile := false
	for i, arg := range args {
		if arg == "-d" {
			return "", "", errors.New("offline analysis cannot select hardware")
		}
		if arg == "-r" && i+1 < len(args) && args[i+1] != "" && args[i+1] != "-" {
			hasFile = true
		}
	}
	if !hasFile {
		return "", "", errors.New("offline analysis requires a recorded file")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	p, err := startProcessDetailed(ctx, binary, args, 100*time.Millisecond, true)
	if err != nil {
		return "", "", err
	}
	defer p.stop()
	var output, diagnostic strings.Builder
	records, exited := p.output(), p.exited()
	var exitError error
	for records != nil || exited != nil {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case err := <-exited:
			exitError = err
			exited = nil
		case record, ok := <-records:
			if !ok {
				records = nil
				continue
			}
			if record.err != nil {
				return "", "", record.err
			}
			target := &output
			limit := 512 * 1024
			if record.stderr {
				target = &diagnostic
				limit = 128 * 1024
			}
			if target.Len()+len(record.data)+1 > limit {
				return "", "", errors.New("analysis output limit exceeded")
			}
			target.Write(record.data)
			target.WriteByte('\n')
		}
	}
	return output.String(), diagnostic.String(), exitError
}
