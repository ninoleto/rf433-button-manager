package app

import (
	"errors"
	"time"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

// Ephemeral observations: never part of settings, backup, signal identity or
// SQLite configuration. Counts are recognized groups, not a physical-press total.
type ButtonReception struct {
	Since         string                 `json:"since"`
	Reports       uint64                 `json:"reports"`
	Presses       uint64                 `json:"presses"`
	Repeats       uint64                 `json:"repeats"`
	LastMatchedAt string                 `json:"last_matched_at,omitempty"`
	Levels        *receiver.SignalLevels `json:"levels,omitempty"`
	Action        string                 `json:"action,omitempty"`
	ActionAt      string                 `json:"action_at,omitempty"`
}

type ReceptionDiagnostics struct {
	Since   string                      `json:"since"`
	Buttons map[string]*ButtonReception `json:"buttons"`
}

func (a *App) resetReceptionLocked() {
	a.reception = ReceptionDiagnostics{Since: a.now().UTC().Format(time.RFC3339Nano), Buttons: map[string]*ButtonReception{}}
}

func (a *App) pruneReceptionLocked(next *Config) {
	for id := range a.reception.Buttons {
		before, after := findButton(&a.config, id), findButton(next, id)
		if before == nil || after == nil || before.Signature != after.Signature {
			delete(a.reception.Buttons, id)
		}
	}
}

func (a *App) buttonReceptionLocked(id string) *ButtonReception {
	if a.reception.Buttons == nil {
		a.resetReceptionLocked()
	}
	// Bound memory even after many create/delete cycles.
	for key := range a.reception.Buttons {
		if findButton(&a.config, key) == nil {
			delete(a.reception.Buttons, key)
		}
	}
	if a.reception.Buttons[id] == nil {
		a.reception.Buttons[id] = &ButtonReception{Since: a.reception.Since}
	}
	return a.reception.Buttons[id]
}

// ResetReception clears only diagnostic counters. It cannot affect debounce,
// mappings, action enablement, receiver ownership or pending requests.
func (a *App) ResetReception(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if findButton(&a.config, id) == nil {
		return errors.New("button not found")
	}
	if a.closed || a.learning != nil || a.captureSuppressed {
		return errors.New("wait for normal reception before resetting counts")
	}
	*a.buttonReceptionLocked(id) = ButtonReception{Since: a.now().UTC().Format(time.RFC3339Nano)}
	return nil
}
