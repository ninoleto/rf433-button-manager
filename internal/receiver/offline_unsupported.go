//go:build !linux

package receiver

import (
	"context"
	"errors"
	"strconv"
)

func intString(n int) string { return strconv.Itoa(n) }
func offlineCommand(context.Context, string, []string) (string, string, error) {
	return "", "", errors.New("capture analysis requires Linux")
}
