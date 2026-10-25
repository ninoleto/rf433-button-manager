package app

import (
	"encoding/json"
	"errors"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

// An explicit allowlist of RF-only report fields, never a configuration export.
type InspectionReport struct {
	Format             string                       `json:"format"`
	Version            string                       `json:"version"`
	Demo               bool                         `json:"demo"`
	FrequencyHz        int                          `json:"frequency_hz"`
	SampleRate         int                          `json:"sample_rate"`
	Stage              string                       `json:"stage"`
	MatchingPresses    int                          `json:"matching_presses"`
	RequiredPresses    int                          `json:"required_presses"`
	RepeatedIdentifier bool                         `json:"repeated_identifier_observed"`
	Candidate          string                       `json:"candidate"`
	Raw                json.RawMessage              `json:"decoded_data,omitempty"`
	Profile            *receiver.DecoderProfile     `json:"candidate_profile,omitempty"`
	Diagnostics        receiver.LearningDiagnostics `json:"diagnostics"`
	Limitations        string                       `json:"limitations"`
}

func (a *App) InspectionResult() (*InspectionReport, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	l := a.learning
	if l == nil {
		l = a.learningNotice
	}
	if l == nil || !l.InspectOnly {
		return nil, errors.New("start Inspect signal before downloading an inspection report")
	}
	return &InspectionReport{
		Format: "rf-button-manager-inspection-v1", Version: Version, Demo: a.demo,
		FrequencyHz: 433920000, SampleRate: 250000, Stage: l.Stage,
		MatchingPresses: l.Presses, RequiredPresses: l.Required,
		RepeatedIdentifier: l.Candidate != "" && l.Presses >= l.Required,
		Candidate:          l.Candidate, Raw: append(json.RawMessage(nil), l.Raw...),
		Profile: clone(l.Profile), Diagnostics: clone(l.Diagnostics),
		Limitations: "433.92 MHz only. Matching observations do not prove a fixed or unencrypted protocol. No decode does not identify rolling codes, encryption, the intended transmitter, or hardware failure. No mapping or webhook is saved by inspection.",
	}, nil
}
