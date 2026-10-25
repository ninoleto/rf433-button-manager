# Background traffic must not end a learning session

## Report and demonstrated cause

Installed 0.2.1-dev, commit `9c700312b10be4506a1d1550c0b5c2282af0333d`, ended an
A ON learning session before the operator pressed a button. Its evidence showed
four packet records (one OOK, three FSK), one accepted recording, three excluded
packets, no malformed/empty input, no analyzer candidates and two offline replays.
It ended as `unsupported` at 0/3, without a decoded identity or validated profile.

The three FSK packets were correctly excluded. The OOK recording reached
`AnalyzeCapture`: the existing/default decoder replay yielded no identity, then
the analyzer replay yielded no supported candidate. That returned a typed
`LearningError{Code: "unsupported"}`. `learningTick` treated **every typed analysis
error as terminal** and called `finishLearningLocked`. It confused one recording's
failure with failure of the entire operator session.

`TestBackgroundCaptureDoesNotTerminateLearning` first failed against the installed
source with the same four/three/one packet counts, two replays, zero candidates,
and terminal `unsupported` at 0/3. After correction it remains `waiting`, records
the rejected analysis and preserves the original deadlines. A separate integration
test uses the installed rtl_433 with an explicitly synthetic one-pulse OOK recording:
two successful process runs yield zero candidates; that recording is ignored, then
a synthetic PWM recording passes normal discovery/replay and three separately
armed matching windows. No USB or physical transmitter is involved.

## Correction in 0.2.2-dev

- Unrecognized and inconsistent analysis results are recorded and skipped. Failed
  identity extraction and a mismatch with the previously verified identity also
  return to waiting. A selected profile/candidate is retained, never replaced by
  the rejected recording. No rejected capture counts as a press.
- The GUI says **Unrecognized signal ignored; waiting for your button**, shows
  both window and session time remaining, and retains per-capture rejection detail.
  It does not infer rolling/encrypted behavior from an unidentified transmission.
- The original 20-second armed deadline now covers waiting, receiving and analysis.
  The 180-second session deadline is unchanged. Neither moves after a rejection.
- At most four recordings are analyzed per window. Once that budget is exhausted,
  the GUI explains that no further signal can be accepted in this window; reception
  and bounded accounting continue until cancel or timeout. Only the operator can
  retry or arm a subsequent verified press. Existing byte/packet/process limits
  remain in force, and each analysis context is clipped to the window's deadline.
- Timeout produces `press_timeout` / `press_window_timeout`, with the saved capture
  evidence and zero new presses. Overall session timeout, receiver loss, malformed
  stream input, decoder-process errors and hard resource limits remain meaningful
  terminal failures. Process errors during profile replay are no longer mistaken
  for an unrelated signal.
- The single live receiver is not replaced after an ordinary capture rejection.
  All webhook actions remain suppressed. Strict decoder proof, three separately
  armed matching presses, explicit confirmation and disabled new mappings remain.

Each rejected analysis is a bounded `learning_capture` history event. The session
also shows its eight most recent rejections with attempt/window numbers, reason,
packet/byte counts and candidate/replay counts. FSK exclusions remain distinct
packet counters. The existing bounded event retention applies; recordings are not
stored in event history. Configuration/schema/authentication are unchanged.

## Verification boundaries and updating

Regression cases cover the reported early abort, FSK/OOK-only expiry with zero
presses, bounded analysis under repeated traffic, selected-profile reuse after
background input, mismatched identity preservation, no automatic rearming,
deadline cancellation/late success, and actual offline process failures. Existing
tests cover cancellation, exclusive receiver transitions, storage and configuration
persistence, overlap protection, strict replay validation and action suppression.

The update bundle contains `main`; every fresh verification clone and the one-time
updater must use **`git clone --branch main`** explicitly. A bundle made from only
the branch ref need not supply a usable default HEAD. Verify the actual distributed
bundle, its checked-out commit and clean build before applying it. The private host
helper runs full software checks, preserves a stopped application snapshot, replaces
only this executable and verifies health/authentication/configuration/ownership.

Physical remote compatibility remains unverified. The operator's no-button attempt
is not RF support evidence. No new physical learning, webhook, USB port/hub recovery
or reboot test is claimed. Further RF operator testing belongs in the installed GUI.
Executed software evidence is recorded in [TEST-REPORT.md](TEST-REPORT.md).
