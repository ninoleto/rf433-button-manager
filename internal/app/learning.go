package app

import (
	"errors"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

// EnableCaptureLearning installs one bounded worker. The production supervisor
// uses ReceiverPlan/ReceiverPlanApplied to serialize changes of the live child.
func (a *App) EnableCaptureLearning(binary, tempDir string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.captureEnabled || a.closed || a.demo {
		return
	}
	a.captureEnabled = true
	a.rtlBinary = binary
	a.captureTempDir = tempDir
	a.analyzeCapture = receiver.AnalyzeCapture
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.learningTick()
			}
		}
	}()
}

func (a *App) receiverChangedLocked() {
	a.receiverGeneration++
	select {
	case a.settingsChanged <- struct{}{}:
	default:
	}
}

func (a *App) ReceiverPlan(binary string) (receiver.Config, uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.config.Settings
	return receiver.Config{Binary: binary, Serial: s.ReceiverSerial, FlexDecoder: s.FlexDecoder,
		Protocols: clone(s.Protocols), Profiles: clone(a.config.DecoderProfiles),
		CapturePulses: a.learning != nil && a.learning.Automatic}, a.receiverGeneration
}

// Called only after the previous receiver.Run has returned, including child
// process and pipe cleanup. A stale acknowledgement must never unpause actions.
func (a *App) ReceiverPlanApplied(generation uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.receiverGeneration || a.closed {
		return
	}
	a.receiverApplied = generation
	a.receiver = receiver.Status{State: "recovering", Message: "Starting reception; waiting for sample acquisition"}
	a.releaseCaptureSuppressionLocked()
}

func (a *App) releaseCaptureSuppressionLocked() {
	if a.learning == nil && !a.analysisActive && a.receiverApplied == a.receiverGeneration {
		a.captureSuppressed = false
		a.muteUntil = a.now().Add(time.Second)
	}
}

func (a *App) SetReceiverFor(generation uint64, s receiver.Status) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || generation != a.receiverGeneration {
		return
	}
	a.setReceiverLocked(s)
	if l := a.learning; l != nil && l.Automatic {
		if s.State == "connected" && l.Stage == "preparing" {
			l.Stage = "waiting"
			l.armedAt = a.now()
			l.Message = "Receiver ready. Press the physical button once, then release it."
		} else if s.State != "connected" {
			l.Diagnostics.Detail = s.Message
			if l.Stage != "preparing" {
				a.finishLearningLocked("receiver_unavailable", "Receiver became unavailable during learning. No mapping changed. Wait for receiver recovery, then retry.")
			}
		}
	}
}

// No files or extra USB processes are created here. The one live rtl_433 child
// supplies bounded pulse dumps; replay uses a private temporary file later.
func (a *App) ReceivePulse(generation uint64, packet receiver.PulsePacket) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	l := a.learning
	if a.closed || l == nil || !l.Automatic || generation != a.receiverGeneration {
		return
	}
	now := a.now()
	previous := l.lastRF
	l.lastRF = now
	if l.Stage != "waiting" && l.Stage != "receiving" {
		return
	}
	if now.Before(l.armedAt) {
		return
	}
	if l.Stage == "waiting" && !previous.IsZero() && now.Sub(previous) < time.Second {
		l.Message = "Transmission is still active. Release the button and allow one second of silence before pressing again."
		return
	}
	if packet.Error != "" || !packet.OOK || len(packet.Data) == 0 {
		a.finishLearningLocked("unsupported", "Transmission detected, but it is not a valid supported OOK capture. Retry with one brief press. No mapping changed.")
		return
	}
	if len(l.packets) >= receiver.MaxCapturePackets || l.bytes+len(packet.Data) > receiver.MaxCaptureBytes {
		a.finishLearningLocked("capture_limit", "Too much RF data in one press. Release the button, keep other remotes idle, and retry. No mapping changed.")
		return
	}
	if l.firstPulse.IsZero() {
		l.firstPulse = now
	}
	l.Stage = "receiving"
	l.Message = "Transmission detected. Release the button; checking the captured signal after RF activity stops."
	packet.Data = append([]byte(nil), packet.Data...)
	l.packets = append(l.packets, packet)
	l.bytes += len(packet.Data)
	l.Diagnostics.Packets++
	l.Diagnostics.Pulses += packet.Pulses
	l.Diagnostics.Bytes += len(packet.Data)
}

func (a *App) finishLearningLocked(stage, message string) {
	l := a.learning
	if l == nil {
		return
	}
	if l.cancel != nil {
		l.cancel()
	}
	l.Stage = stage
	l.Message = message
	l.packets = nil
	a.learningNotice = clone(l)
	a.learning = nil
	a.epoch++
	a.muteUntil = a.now().Add(time.Second)
	if l.Automatic {
		a.receiverChangedLocked()
	}
	a.eventLocked("learning", message, l.ButtonID, nil)
}

func (a *App) learningTick() {
	a.mu.Lock()
	a.expireLocked()
	l := a.learning
	if a.closed || l == nil || !l.Automatic || a.analysisActive {
		a.mu.Unlock()
		return
	}
	now := a.now()
	switch l.Stage {
	case "preparing":
		l.Message = "Receiver unavailable or preparing: " + a.receiver.Message
		if now.Sub(l.armedAt) > 30*time.Second {
			a.finishLearningLocked("receiver_unavailable", "Receiver did not become ready. Check receiver status in Diagnostics and retry after recovery. No mapping changed.")
		}
		a.mu.Unlock()
		return
	case "waiting":
		if now.Sub(l.armedAt) > 20*time.Second {
			a.finishLearningLocked("timeout", "No transmission detected. Keep the remote near the receiver, check its battery, and retry. No mapping changed.")
		}
		a.mu.Unlock()
		return
	case "receiving":
		if l.lastRF.Sub(l.firstPulse) > 6*time.Second {
			a.finishLearningLocked("capture_limit", "RF activity did not stop. Release the button and keep other remotes idle, then retry. No mapping changed.")
			a.mu.Unlock()
			return
		}
		if now.Sub(l.lastRF) < time.Second {
			a.mu.Unlock()
			return
		}
	default:
		a.mu.Unlock()
		return
	}
	capture, err := receiver.ValidatePulseCapture(l.packets)
	if err != nil {
		a.finishLearningLocked("unsupported", "Transmission detected, but the captured pulse data could not be validated. Retry learning.")
		a.mu.Unlock()
		return
	}
	l.packets = nil
	l.Stage = "analyzing"
	l.Message = "Analyzing the captured transmission and replaying decoder candidates. No decoder has been saved yet."
	s := a.config.Settings
	fields := append([]string{}, l.Fields...)
	request := receiver.AnalysisRequest{Binary: a.rtlBinary, TempDir: a.captureTempDir, Capture: capture,
		Config:   receiver.Config{FlexDecoder: s.FlexDecoder, Protocols: clone(s.Protocols), Profiles: clone(a.config.DecoderProfiles)},
		Selected: clone(l.Profile), Signature: func(raw []byte) (string, error) { return Signature(raw, fields) }}
	for _, remote := range a.config.Remotes {
		contains := false
		for _, b := range remote.Buttons {
			if b.ID == l.ButtonID {
				contains = true
			}
		}
		if contains {
			for _, b := range remote.Buttons {
				if b.DecoderProfileID != "" {
					request.PreferredIDs = append(request.PreferredIDs, b.DecoderProfileID)
				}
			}
		}
	}
	id, ctx, analyze := l.SessionID, l.ctx, a.analyzeCapture
	a.analysisActive = true
	a.mu.Unlock()
	result, err := analyze(ctx, request)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.analysisActive = false
	a.releaseCaptureSuppressionLocked()
	a.expireLocked()
	l = a.learning
	if a.closed || l == nil || l.SessionID != id || ctx.Err() != nil {
		return
	}
	l.Diagnostics.Replays += result.Diagnostics.Replays
	l.Diagnostics.Candidates += result.Diagnostics.Candidates
	l.Diagnostics.Source = result.Diagnostics.Source
	if err != nil {
		var failure *receiver.LearningError
		if errors.As(err, &failure) {
			a.finishLearningLocked(failure.Code, failure.Message)
		} else {
			a.finishLearningLocked("analysis_error", "Capture analysis failed. Retry learning; no mapping changed.")
		}
		return
	}
	if result.Profile != nil {
		l.Fields = []string{"codes"}
	}
	signature, err := Signature(result.Raw, l.Fields)
	if err != nil {
		a.finishLearningLocked("unsupported", "Transmission detected, but no stable button identity was decoded. No mapping changed.")
		return
	}
	if l.Candidate != "" && l.Candidate != signature {
		a.finishLearningLocked("inconsistent", "Separate presses produced different signals. Use the same button and keep other remotes idle. Changing, rolling or encrypted codes are not supported; no mapping changed.")
		return
	}
	l.Candidate = signature
	l.Raw = append([]byte(nil), result.Raw...)
	l.Profile = result.Profile
	l.Presses++
	l.Stage = "captured"
	l.Message = "Press verified. Release the physical button, then select Ready for next press."
	if l.Presses == l.Required {
		l.Stage = "ready"
		l.Message = "Three separate presses decoded consistently. Confirm this is the intended button and save. Its action will remain disabled."
	}
	a.eventLocked("learning", "Captured press verified by rtl_433 replay", l.ButtonID, nil)
}
