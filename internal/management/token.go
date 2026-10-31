// Package management owns credentials and web listeners independently of RF reception.
package management

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Token struct {
	mu          sync.RWMutex
	path, value string
}

func secret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// AtomicPrivate writes a complete private file before publishing its name.
func AtomicPrivate(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("refusing non-regular management file")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".management-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	if d, e := os.Open(filepath.Dir(path)); e == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func OpenToken(dir string) (*Token, error) {
	t := &Token{path: filepath.Join(dir, "management.token")}
	info, err := os.Lstat(t.path)
	if err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("management.token must be a regular file")
	}
	raw, err := os.ReadFile(t.path)
	if os.IsNotExist(err) {
		t.value, err = secret()
		if err == nil {
			err = AtomicPrivate(t.path, []byte(t.value+"\n"))
		}
	} else if err == nil {
		t.value = strings.TrimSpace(string(raw))
		decoded, e := hex.DecodeString(t.value)
		if e != nil || len(decoded) != 32 {
			return nil, errors.New("invalid management.token; expected 64 hex characters")
		}
		err = os.Chmod(t.path, 0600)
	}
	return t, err
}

func (t *Token) Valid(provided string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(provided) == 64 && subtle.ConstantTimeCompare([]byte(provided), []byte(t.value)) == 1
}

// Rotate rejects a stale authenticated request as well as revoking all old tabs.
func (t *Token) Rotate(current string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(current) != 64 || subtle.ConstantTimeCompare([]byte(current), []byte(t.value)) != 1 {
		return "", errors.New("current management token required")
	}
	next, err := secret()
	if err != nil {
		return "", err
	}
	if err = AtomicPrivate(t.path, []byte(next+"\n")); err != nil {
		return "", errors.New("token could not be saved; current token remains valid")
	}
	t.value = next
	return next, nil
}
