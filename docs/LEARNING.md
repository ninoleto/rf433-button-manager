# Browser learning and measured decoder profiles

Normal operation is through the browser. Select a button, choose **Learn**, wait
for the receiver-ready instruction, briefly press/release the physical button,
repeat when prompted, and **Confirm and save**. No terminal history, decoder string,
configuration file or receiver command is needed. Existing remotes, mappings and
advanced decoder settings are preserved. New mappings stay disabled.

## What is supported

The application first reuses a profile associated with the remote, then tries
existing configured/default rtl_433 decoders and other saved profiles. A decoded
identity must satisfy the existing stable model/device/button rules.

When those decoders cannot identify a capture, automatic profile discovery supports
**fixed-code OOK/PWM, 16–64 bits**, at the application's fixed 433.92 MHz frequency.
It requires repeated RF rows inside a capture and the same identity across three
separately armed physical presses. A continuous hold cannot supply three presses.
Keep other remotes idle: the application cannot know which physical transmitter
you intended, and the final review remains necessary.

Analyzer suggestions are candidates only. The app never installs stock timings
for “Switch433,” chooses the first arbitrary code, or claims universal learning.
FSK/PPM and other modulation discovery, rolling/encrypted codes, captures without
stable repeated frames, and ambiguous interpretations are unsupported. Three
matching observations do not prove that an unfamiliar protocol can never change
its code. Physical evidence for a particular remote is tracked separately.

## A ON: the operator flow

1. Select **A ON** in the existing six-button remote. Save any pending editor edits
   and click **Learn signal**. All webhook actions pause, including explicit tests.
2. Wait for **Press the physical button once**. Briefly press physical **A ON** and
   release it completely. The dialog shows **Transmission detected**, then
   **Checking the captured signal**, then **1 of 3 matching presses**.
3. After release and one second of silence, select **Ready for next press**. Press
   and release the same physical button once. Expect **2 of 3**.
4. Arm again and make the third separate press. Expect **Ready to save / 3 of 3**.
5. Review the candidate and choose **Confirm and save**. Decoder profile and button
   mapping are saved together. The action stays disabled. Later buttons on this
   remote preferentially reuse the profile and still need three separate presses.

Cancel preserves the previous mapping. Failed attempts retain a visible explanation,
**Retry learning**, and **Diagnostics and receiver status**. Retry becomes available
after normal reception is restored. It starts a new attempt with zero captures.
The browser may be closed; a server timer still expires and cleans up learning.

| Visible state | Meaning / next step |
| --- | --- |
| Preparing receiver | The manager is changing its own receiver process to capture mode. Wait. |
| Receiver unavailable | USB/sample acquisition failed or was lost. Inspect receiver status; retry after recovery. |
| Waiting for transmission | Samples are available; briefly press/release the intended button. |
| Transmission detected | Pulses arrived; release and allow RF silence before analysis. |
| Checking captured signal | rtl_433 is replaying a private recording; do not press again yet. |
| Detected but unsupported | No supported, replay-validated interpretation exists. No mapping/profile changed. |
| Inconsistent captures | Different identities or ambiguous frames appeared. Keep other remotes idle and retry the same button. |
| Capture limit / timeout | No transmission, prolonged RF activity, or a resource limit prevented completion. Retry a brief press. |
| Ready to save | Three separate captures agree. Review and confirm; actions stay disabled. |

Diagnostics show receiver status, packets/pulses/bytes, candidate/replay counts,
decoder source and failure detail. Raw IQ, tokens and webhook configuration are
not included. Decoded identifiers can still identify a remote: review before sharing.

## Capture, analysis and validation

```mermaid
flowchart TD
    Learn[Browser: Learn] --> Pause[Wait for in-flight actions; suppress new actions]
    Pause --> Live[Stop and wait for old live child; start same receiver with pulse output]
    Live --> Capture[Brief press; bounded OOK pulses; one second of silence]
    Capture --> Existing[Replay through existing decoders and profiles]
    Existing -->|No valid identity| Analyze[rtl_433 file analysis: measured PWM candidates]
    Analyze --> Proof[Replay candidate; repeated consistent fixed code; exact bit length]
    Existing -->|Valid identity| Match[Compare with previous separate presses]
    Proof --> Match
    Match -->|Fewer than three| Arm[Browser: Ready for next press]
    Arm --> Capture
    Match -->|Three match| Confirm[Browser: Confirm and save]
    Confirm --> Save[Atomic profile + mapping save; new action disabled]
    Save --> Restore[Stop capture child; finish replay cleanup; restore normal reception]
    Capture -->|Cancel, failure or timeout| Restore
```

The live process uses `-w OOK:-` alongside JSON output. The parser separates actual
`;ook ... ;end` pulse records from decoded events. Learning does not count live
JSON as capture proof. A quiet interval closes each press recording. The application
replays it with `rtl_433 -r PRIVATE_FILE`, never another USB selector.

When needed, `-A -R 0` produces timing candidates. The parser accepts only measured
numeric `m=OOK_PWM,s,l,r,g,t,y` values within bounds. Probe replay requires 16–64
bits and at least two repeated rows. Multiple distinct eligible codes are rejected.
An accepted candidate is narrowed to its exact measured bit count, replayed again,
and then used unchanged for the second and third presses. A profile is not persisted
until final confirmation and overlap validation succeed.

Profile IDs/models derive from a hash of canonical timing and framing, not an RF
button code. Different buttons can share a profile. Runtime normalization retains
the exact model and code; shorter delimiter rows can be excluded only after the
profile bit count has been fixed. Existing custom decoder behavior is preserved.

## Ownership, limits and cleanup

There is at most **one live USB-owning rtl_433 child** and **one file-replay child**.
The receiver supervisor waits for process-group and pipe cleanup before starting
a replacement live child. Each plan carries a generation; stale acknowledgements
cannot clear suppression. File replay uses explicit argv, private mode-0600 data
inside a mode-0700 scratch directory, and no shell. The supplied systemd unit's
private `/tmp` is on internal storage; captures are not written to the backup disk.

| Bound | Limit |
| --- | --- |
| Overall automatic session | 180 seconds |
| Receiver preparation | 30 seconds |
| Waiting per armed press | 20 seconds |
| Continuous RF activity per capture | 6 seconds |
| Quiet interval between capture and analysis / rearming | 1 second |
| One press recording | 32 packets, 256 KiB total; 4,096 pulse pairs per packet |
| Offline analysis | 10 seconds total, 2 seconds per child |
| Child output | 512 KiB stdout, 128 KiB stderr; bounded individual lines |
| Distinct analyzer candidates | 8 |
| Saved profiles | 16 |

The replay process group is terminated/reaped on cancellation or timeout. Scratch
files are removed after analysis, including failure. On abrupt process death,
parent-death signaling and systemd control-group cleanup stop children; the service's
private temporary directory is discarded on stop. Learning state does not survive
a restart, and no provisional mapping is recovered or silently accepted.

Cancel, timeout, receiver loss and failure request restoration of the normal
receiver plan. All webhook execution remains suppressed until the old capture
child and the replay worker have finished cleanup. Existing enabled actions are
preserved and may resume after restoration; newly learned actions stay disabled.

## Storage and upgrade compatibility

Schema 2 adds top-level `decoder_profiles` and button `decoder_profile_id`. The
existing settings object remains compatible with its full-object PUT API. Profiles
are immutable through ordinary settings/button edits, persisted atomically with
confirmed mappings, and included in authenticated logical backups. Restore validates
profiles and references and disables restored actions. No direct SQLite editing is
part of the workflow.

Schema-1 configurations/backups migrate without losing existing data. An older
0.1 executable rejects schema 2. Preserve a stopped pre-upgrade state snapshot for
rollback; see [OPERATIONS.md](OPERATIONS.md). Do not downgrade only the executable
after migration and expect the old application to accept new data.

## Evidence and extension points

`internal/receiver/analysis_test.go` replays an explicitly synthetic pulse fixture
through an installed rtl_433 without USB. Application tests cover lifecycle,
timeouts, inconsistent captures, late results, collisions, persistence and profile
reuse. The supervisor test holds cleanup open to prove a replacement live process
cannot start early. These are software tests, not physical button or USB tests.

Receiver compatibility, actual A ON learning, other buttons, physical webhook
effects and USB port/hub recovery remain separate operator-assisted tests. A
supported software path does not establish success for the user's actual remote.
For extensions, read [DEVELOPMENT.md](DEVELOPMENT.md).

Upstream references: [rtl_433 operation](https://triq.org/rtl_433/OPERATION.html),
[pulse analysis](https://triq.org/rtl_433/ANALYZE.html), and the
[23.11 source](https://github.com/merbanan/rtl_433/tree/23.11).
