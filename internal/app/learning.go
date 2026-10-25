package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

const learningPressWindow = 20 * time.Second

// One worker, at most two analysis starts per second, for the fixed 20s window.
const maxWindowAnalyses = 40
const analysisInterval = 500 * time.Millisecond

func waitingMessage(l *Learning) string {
	if l.WindowAnalyses >= maxWindowAnalyses {
		return "Pulse analysis limit reached for this window. Still listening for decoded button events; cancel or wait for timeout to retry discovery."
	}
	return "Unrecognized signal ignored; waiting for your button"
}

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
	a.lastDecodedAt = ""
	a.receiver = receiver.Status{State: "recovering", Message: "Starting reception; waiting for sample acquisition"}
	a.receiverMode = "normal"
	if l := a.learning; l != nil && l.Automatic && l.CaptureGeneration == generation {
		a.receiverMode = "learning_capture"
		l.CaptureReceiver = a.receiver
		l.CaptureReceiverAt = a.now().UTC().Format(time.RFC3339Nano)
	}
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
		l.CaptureReceiver = s
		l.CaptureReceiverAt = a.now().UTC().Format(time.RFC3339Nano)
		if s.State == "connected" && l.Stage == "preparing" {
			l.Stage = "waiting"
			l.armedAt = a.now()
			l.WindowExpiresAt = l.armedAt.Add(learningPressWindow).UTC().Format(time.RFC3339Nano)
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
	if a.closed {
		return
	}
	a.recordCapturePacketLocked(generation, packet)
	if l == nil || !l.Automatic || generation != l.CaptureGeneration {
		// The supervisor finishes the old stream after restoration was requested.
		// Preserve evidence on that session's notice, never accept a late candidate.
		if notice := a.terminalCapture; notice != nil && generation == notice.CaptureGeneration {
			a.observeCaptureLocked(notice, packet)
			if packet.Final {
				a.eventLocked("learning_diagnostics", "Capture stream closed; failure evidence retained", notice.ButtonID, captureEvidence(notice))
			}
		}
		return
	}
	a.observeCaptureLocked(l, packet)
	if l.Diagnostics.Stream.Bytes > receiver.MaxCaptureStreamBytes || l.Diagnostics.ObservedPackets > 128 {
		l.Diagnostics.FailureCode = "stream_limit"
		l.Diagnostics.FailureReason = "Capture stream exceeded its bounded byte/packet budget"
		a.finishLearningLocked("capture_limit", "Too much RF traffic during learning. No decoder or mapping changed.")
		return
	}
	if packet.Progress {
		return
	}
	if packet.Error != "" || packet.ErrorCode != "" {
		stage := "malformed_capture"
		if packet.ErrorCode == "empty_packet" {
			stage = "no_capture"
		}
		if strings.Contains(packet.ErrorCode, "limit") {
			stage = "capture_limit"
		}
		if packet.ErrorCode == "stream_read_error" {
			stage = "capture_error"
		}
		l.Diagnostics.FailureCode = packet.ErrorCode
		l.Diagnostics.FailureReason = packet.Error
		a.finishLearningLocked(stage, "Pulse input was rejected before analysis: "+packet.Error+". No remote compatibility conclusion was made.")
		return
	}
	if len(packet.Data) == 0 {
		l.Diagnostics.FailureCode = "empty_packet"
		l.Diagnostics.FailureReason = "Packet callback contained no pulse recording"
		a.finishLearningLocked("no_capture", "No pulse recording was available for analysis. No decoder or mapping changed.")
		return
	}
	if packet.Cleanup {
		return
	}
	if l.decodedSource {
		return
	}
	if !packet.OOK {
		if l.Stage == "waiting" {
			l.Message = waitingMessage(l)
		}
		return
	}
	// Skip very short bursts in generic discovery without consuming analysis
	// work or resetting the release timer. Decoded events use a separate path.
	if packet.Pulses > 0 && packet.Pulses < 12 {
		l.Diagnostics.RejectedPackets++
		l.Diagnostics.RejectionCode = "short_burst"
		l.Diagnostics.RejectionReason = "Pulse burst too short for generic code discovery; still listening for decoded events"
		return
	}
	now := a.now()
	l.lastRF = now
	if (l.Stage != "waiting" && l.Stage != "receiving") || now.Before(l.armedAt) {
		l.Diagnostics.DiscardedPackets++
		return
	}
	if l.WindowAnalyses >= maxWindowAnalyses {
		l.Diagnostics.DiscardedPackets++
		l.Message = waitingMessage(l)
		return
	}
	if len(l.packets) >= receiver.MaxCapturePackets || l.bytes+len(packet.Data) > receiver.MaxCaptureBytes {
		l.Diagnostics.FailureCode = "capture_budget"
		l.Diagnostics.FailureReason = "One press exceeded the 32 packet / 256 KiB limit"
		a.finishLearningLocked("capture_limit", "Too much OOK data in one press. No decoder or mapping changed.")
		return
	}
	if l.firstPulse.IsZero() {
		l.firstPulse = now
	}
	l.Stage = "receiving"
	l.Message = "OOK transmission captured. Release the button; checking the captured signal after RF activity stops."
	packet.Data = append([]byte(nil), packet.Data...)
	l.packets = append(l.packets, packet)
	l.bytes += len(packet.Data)
	l.Diagnostics.Packets++
	l.Diagnostics.AcceptedBytes += len(packet.Data)
}

func (a *App) observeCaptureLocked(l *Learning, p receiver.PulsePacket) {
	d := &l.Diagnostics
	if p.Stream.Bytes >= d.Stream.Bytes {
		d.Stream = p.Stream
	}
	if p.Progress {
		return
	}
	d.ObservedPackets++
	if p.Cleanup {
		d.DiscardedPackets++
	}
	d.Pulses += p.Pulses
	n := p.ObservedBytes
	if n == 0 {
		n = len(p.Data)
	}
	d.Bytes += n
	if p.OOK {
		d.OOKPackets++
	} else if p.Error == "" && len(p.Data) > 0 {
		d.FSKPackets++
	}
	code, reason := p.ErrorCode, p.Error
	if p.Error != "" || p.ErrorCode != "" {
		if p.ErrorCode == "empty_packet" {
			d.EmptyPackets++
		} else {
			d.MalformedPackets++
		}
	} else if len(p.Data) == 0 {
		d.EmptyPackets++
		code = "empty_packet"
		reason = "Packet callback contained no pulse recording"
	} else if !p.OOK {
		code = "unsupported_modulation"
		reason = "rtl_433 marked this packet FSK; it was excluded from OOK learning, not identified as the intended remote"
	}
	if code != "" || reason != "" {
		d.RejectedPackets++
		d.RejectionCode = code
		d.RejectionReason = reason
	}
}

// Persist only bounded diagnostics, never pulse content or webhook configuration.
func captureEvidence(l *Learning) json.RawMessage {
	diagnostics := l.Diagnostics
	// Rejections have individual history events; don't duplicate the bounded ring
	// into every lifecycle event and exceed the history payload limit.
	diagnostics.RecentRejections = nil
	raw, _ := json.Marshal(struct {
		SessionID   string                       `json:"session_id"`
		Generation  uint64                       `json:"capture_generation"`
		Receiver    receiver.Status              `json:"capture_receiver"`
		ReceiverAt  string                       `json:"capture_receiver_at"`
		FinishedAt  string                       `json:"finished_at"`
		Diagnostics receiver.LearningDiagnostics `json:"diagnostics"`
	}{l.SessionID, l.CaptureGeneration, l.CaptureReceiver, l.CaptureReceiverAt, l.FinishedAt, diagnostics})
	return raw
}

func (a *App) rejectCaptureLocked(l *Learning, rejection receiver.CaptureRejection) {
	rejection.Time = a.now().UTC().Format(time.RFC3339Nano)
	if len(rejection.Code) > 64 {
		rejection.Code = rejection.Code[:64]
	}
	if len(rejection.Reason) > 512 {
		rejection.Reason = rejection.Reason[:512]
	}
	l.Diagnostics.RejectedCaptures++
	l.Diagnostics.RejectionCode = rejection.Code
	l.Diagnostics.RejectionReason = rejection.Reason
	l.Diagnostics.RecentRejections = append(l.Diagnostics.RecentRejections, rejection)
	if len(l.Diagnostics.RecentRejections) > 8 {
		l.Diagnostics.RecentRejections = l.Diagnostics.RecentRejections[len(l.Diagnostics.RecentRejections)-8:]
	}
	raw, _ := json.Marshal(struct {
		SessionID string                    `json:"session_id"`
		Rejection receiver.CaptureRejection `json:"rejection"`
	}{l.SessionID, rejection})
	a.eventLocked("learning_capture", "Capture rejected: "+rejection.Code+"; no press confirmed", l.ButtonID, raw)
}

func (a *App) continueLearningLocked(l *Learning) {
	l.Stage = "waiting"
	l.Message = waitingMessage(l)
	l.packets = nil
	l.bytes = 0
	l.firstPulse = time.Time{}
	// Do not change armedAt, expires, lastRF, the verified candidate or its profile.
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
	l.FinishedAt = a.now().UTC().Format(time.RFC3339Nano)
	if stage != "saved" && stage != "cancelled" {
		if l.Diagnostics.FailureCode == "" {
			l.Diagnostics.FailureCode = stage
		}
		if l.Diagnostics.FailureReason == "" {
			l.Diagnostics.FailureReason = message
		}
	}
	l.packets = nil
	a.learningNotice = clone(l)
	if l.Automatic {
		a.terminalCapture = a.learningNotice
	}
	a.learning = nil
	a.epoch++
	a.muteUntil = a.now().Add(time.Second)
	if l.Automatic {
		a.receiverChangedLocked()
	}
	a.eventLocked("learning", message, l.ButtonID, captureEvidence(l))
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
		a.mu.Unlock()
		return
	case "receiving":
		if l.lastRF.Sub(l.firstPulse) > 6*time.Second {
			a.finishLearningLocked("capture_limit", "RF activity did not stop. Release the button and keep other remotes idle, then retry. No mapping changed.")
			a.mu.Unlock()
			return
		}
		// Complete pulse packets already have rtl_433 boundaries. Do not wait
		// indefinitely for an entirely quiet RF channel.
		if now.Sub(l.lastRF) < 350*time.Millisecond && now.Sub(l.firstPulse) < 1200*time.Millisecond {
			a.mu.Unlock()
			return
		}
	default:
		a.mu.Unlock()
		return
	}
	if !l.lastAnalysis.IsZero() && now.Sub(l.lastAnalysis) < analysisInterval {
		a.mu.Unlock()
		return
	}
	l.lastAnalysis = now
	capture, err := receiver.ValidatePulseCapture(l.packets)
	if err != nil {
		l.Diagnostics.FailureCode = "capture_assembly"
		l.Diagnostics.FailureReason = err.Error()
		a.finishLearningLocked("malformed_capture", "The captured pulse recording failed validation before analysis. See Diagnostics; no decoder was tested.")
		a.mu.Unlock()
		return
	}
	l.WindowAnalyses++
	l.Diagnostics.AnalysisAttempts++
	rejection := receiver.CaptureRejection{Attempt: l.Diagnostics.AnalysisAttempts, Press: l.Presses + 1, Packets: len(l.packets), Bytes: l.bytes}
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
	id, analyze := l.SessionID, a.analyzeCapture
	ctx, cancel := context.WithTimeout(l.ctx, min(10*time.Second, l.armedAt.Add(learningPressWindow).Sub(now)))
	l.analysisCancel = cancel
	a.analysisActive = true
	a.mu.Unlock()
	result, err := analyze(ctx, request)
	contextErr := ctx.Err()
	cancel()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.analysisActive = false
	a.releaseCaptureSuppressionLocked()
	a.expireLocked()
	l = a.learning
	rejection.Replays = result.Diagnostics.Replays
	rejection.Candidates = result.Diagnostics.Candidates
	if err != nil {
		rejection.Code, rejection.Reason = "analysis_error", "Capture analysis failed"
		var failure *receiver.LearningError
		if errors.As(err, &failure) {
			rejection.Code, rejection.Reason = failure.Code, failure.Message
		}
	}
	if a.closed || l == nil || l.SessionID != id {
		if notice := a.terminalCapture; notice != nil && notice.SessionID == id {
			notice.Diagnostics.Replays += result.Diagnostics.Replays
			notice.Diagnostics.Candidates += result.Diagnostics.Candidates
			if err != nil {
				a.rejectCaptureLocked(notice, rejection)
			}
			a.eventLocked("learning_diagnostics", "Analysis cleanup completed; attempted replay counts retained", notice.ButtonID, captureEvidence(notice))
		}
		return
	}
	l.Diagnostics.Replays += result.Diagnostics.Replays
	l.Diagnostics.Candidates += result.Diagnostics.Candidates
	l.analysisCancel = nil
	if l.decodedSource {
		return // A live decoder already verified this press; discard background result.
	}
	l.Diagnostics.Source = result.Diagnostics.Source
	if contextErr != nil {
		if rejection.Code == "" {
			rejection.Code, rejection.Reason = "analysis_error", "Capture analysis was cancelled or exceeded its deadline"
		}
		a.rejectCaptureLocked(l, rejection)
		a.finishLearningLocked("analysis_error", "Capture analysis was cancelled or exceeded its deadline. No mapping changed.")
		return
	}
	if err != nil {
		a.rejectCaptureLocked(l, rejection)
		var failure *receiver.LearningError
		if errors.As(err, &failure) {
			if failure.Code == "unsupported" || failure.Code == "inconsistent" {
				a.continueLearningLocked(l)
			} else {
				a.finishLearningLocked(failure.Code, failure.Message)
			}
		} else {
			a.finishLearningLocked("analysis_error", "Capture analysis failed. Retry learning; no mapping changed.")
		}
		return
	}
	fields = l.Fields
	if result.Profile != nil {
		fields = []string{"codes"}
	}
	signature, err := Signature(result.Raw, fields)
	if err != nil {
		rejection.Code, rejection.Reason = "unsupported", "No stable button identity was decoded from this capture"
		a.rejectCaptureLocked(l, rejection)
		a.continueLearningLocked(l)
		return
	}
	if l.Candidate != "" && l.Candidate != signature {
		rejection.Code, rejection.Reason = "inconsistent", "This capture did not match the verified button identity; it may be unrelated traffic"
		a.rejectCaptureLocked(l, rejection)
		a.continueLearningLocked(l)
		return
	}
	l.Fields = append([]string(nil), fields...)
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
