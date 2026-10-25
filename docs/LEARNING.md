# Browser learning and measured decoder profiles

Normal operation is through the browser. Select a button, choose **Learn**, wait
for the receiver-ready instruction, briefly press/release the physical button,
repeat when prompted, and **Confirm and save**. No terminal history, decoder string,
configuration file or receiver command is needed. Existing remotes, mappings and
advanced decoder settings are preserved. New mappings stay disabled.

## What is supported

The app accepts button events already decoded by the live rtl_433 process. It
recognizes command/cmd/action/data and numbered button fields, alongside existing
code/button/state identities. RSSI, timestamps and sensor readings are excluded.
Each press must match the first candidate, with explicit arming between presses.
If no live decoder identifies a button, the app replays captured pulses through
existing decoders and saved profiles before asking rtl_433 for timing candidates.

When those decoders cannot identify a capture, automatic profile discovery supports
**fixed-code OOK PWM or PPM, 16–64 bits**, at the application's fixed 433.92 MHz frequency.
It requires at least two matching RF frames across the captured recording
(including separate packet/JSON records) and the same identity across three
separately armed physical presses. A continuous hold cannot supply three presses.
Keep other remotes idle: the application cannot know which physical transmitter
you intended, and the final review remains necessary.

Analyzer suggestions are candidates only. The app never installs stock timings
for “Switch433,” chooses the first arbitrary code, or claims universal learning.
FSK and other modulation discovery, rolling/encrypted codes, captures without
stable repeated frames, and ambiguous interpretations are unsupported. Three
matching observations do not prove that an unfamiliar protocol can never change
its code. Physical evidence for a particular remote is tracked separately.

## A ON: the operator flow

1. Select **A ON** in the existing six-button remote. Save any pending editor edits
   and click **Learn signal**. All webhook actions pause, including explicit tests.
2. Wait for **Press the physical button once**. Briefly press physical **A ON** and
   release it completely. A known decoder can advance directly to **1 of 3
   matching presses**. An unknown signal first shows capture and analysis progress.
3. After release and at least one second without another matching frame, select **Ready for next press**. Press
   and release the same physical button once. Expect **2 of 3**.
4. Arm again and make the third separate press. Expect **Ready to save / 3 of 3**.
5. Review the candidate and choose **Confirm and save**. Decoder profile and button
   mapping are saved together. The action stays disabled. Later buttons on this
   remote preferentially reuse the profile and still need three separate presses.

Cancel preserves the previous mapping. Failed attempts retain a visible explanation,
**Retry learning**, and **Diagnostics and receiver status**. Retry becomes available
after normal reception is restored. It starts a new attempt with zero captures.
The browser may be closed; a server timer still expires and cleans up learning.

The channel can contain unrelated traffic before you press anything. FSK packets
are excluded. A complete OOK recording that yields no validated decoder, no stable
identity, or an identity different from the first verified press is recorded and
skipped. The dialog says **Unrecognized signal ignored; waiting for your button**.
It stays in the same armed window and retains any previously verified candidate.
This does not identify the ignored transmitter as the intended remote, or establish
that the intended remote uses changing, rolling or encrypted codes.

Each armed window is **20 seconds total, including capture and analysis**. Ignored
traffic cannot restart that deadline or the 180-second session deadline. At most
40 recordings are analyzed per window, with one worker and at least 500 ms between
analysis starts. Every analysis has a ten-second limit and is cancelled at the
window deadline. FSK and bursts shorter than 16 pulses do not consume analysis
attempts. Unsupported captures return to waiting. Even if the analysis limit is
reached, live decoded button events can still be accepted before the deadline.
Ordinary packets never arm the next window;
only **Ready for next press** does so after a verified capture.

| Visible state | Meaning / next step |
| --- | --- |
| Preparing receiver | The manager is changing its own receiver process to capture mode. Wait. |
| Receiver unavailable | USB/sample acquisition failed or was lost. Inspect receiver status; retry after recovery. |
| Waiting for transmission | Samples are available; briefly press/release the intended button. |
| Transmission detected | Pulses arrived; release and allow RF silence before analysis. |
| Checking captured signal | rtl_433 is replaying a private recording; do not press again yet. |
| No pulse capture | No usable pulse recording reached analysis. Decoded JSON or a sample heartbeat alone is not capture proof. |
| Malformed pulse capture | A header, timing row, count or packet boundary was invalid. Diagnostics retain the specific parser rejection; no compatibility conclusion is made. |
| Unrecognized signal ignored; waiting for your button | FSK was excluded or a complete OOK recording failed replay/identity validation. The same window remains open; no press was counted. |
| Analysis budget reached | The pulse-analysis safety limit was reached. Live decoded events can still be accepted; cancel/retry if necessary. |
| Press window timed out | No matching press was verified within the original 20-second window. Diagnostics retain ignored-capture evidence; compatibility remains unverified. |
| Capture stream failed | Reading stdout failed or a line exceeded its limit. Inspect the retained stream error. |
| Inconsistent capture (diagnostic) | Ambiguous frames or another identity were ignored; the previously verified candidate is retained. Keep other remotes idle. |
| Capture limit / session timeout | Prolonged RF activity, the overall session deadline, or a resource limit prevented completion. |
| Ready to save | Three separate captures agree. Review and confirm; actions stay disabled. |

Diagnostics separately show the capture receiver's last observed status and the
current reception status/mode. After failure, current **connected** can describe
restored normal reception. It does not prove the failed capture succeeded.
Observed packets/pulses/bytes are counted before rejection, separately from packets
accepted for replay. Stdout bytes/lines, JSON records, headers, end boundaries,
incomplete rows, modulation and rejection counts explain where input stopped.
The original failure reason remains fixed while late cleanup evidence is added.
Bounded metadata is saved in event history, including when the dialog is dismissed.
Each unsuccessful analysis has its own `learning_capture` event with the attempt
number, press window, rejection code/reason, packet/byte counts and replay/candidate
counts. The dialog also retains the eight most recent analyzed rejections. Packet
exclusions (such as FSK) and unsuccessful analyses are separate counters.
Raw IQ, tokens and webhook configuration are not included. Decoded identifiers in
other state fields can still identify a remote: review before sharing.

## Capture, analysis and validation

```mermaid
flowchart TD
    Learn[Browser: Learn] --> Pause[Wait for in-flight actions; suppress new actions]
    Pause --> Live[Stop and wait for old live child; start same receiver with pulse output]
    Live --> Capture[Brief press; bounded OOK pulses]
    Capture --> Existing[Replay through existing decoders and profiles]
    Existing -->|No valid identity| Analyze[rtl_433 file analysis: measured PWM or PPM candidates]
    Analyze --> Proof[Replay candidate; repeated consistent fixed code; exact bit length]
    Existing -->|Valid identity| Match[Compare with previous separate presses]
    Proof --> Match
    Existing -->|Unrecognized capture; time/work budget remains| Capture
    Match -->|Does not match verified identity; window remains| Capture
    Match -->|Fewer than three| Arm[Browser: Ready for next press]
    Arm --> Capture
    Match -->|Three match| Confirm[Browser: Confirm and save]
    Confirm --> Save[Atomic profile + mapping save; new action disabled]
    Save --> Restore[Stop capture child; finish replay cleanup; restore normal reception]
    Capture -->|Cancel, failure or timeout| Restore
```

The live process uses `-w OOK:-` alongside JSON output. In rtl_433 23.11 this output
includes **both OOK and FSK** packets. The parser separates complete JSON objects,
stream metadata, and `;ook N pulses` / `;fsk N pulses` packets ending in `;end`.
Version 1, an explicit 1us timescale, matching row counts and bounded integer timing
pairs are required; malformed input is never repaired by guessing units or timings.
Valid FSK traffic is counted and excluded while OOK learning continues to wait.
Live JSON with a supported button identity follows the direct learning path;
it is not evidence of pulse capture. For discovery, complete packets are collected
until 350 ms of quiet or 1.2 seconds of collection, whichever occurs first. The
application replays this bounded recording with `rtl_433 -r PRIVATE_FILE`, never
another USB selector. See [CAPTURE-INVESTIGATION.md](CAPTURE-INVESTIGATION.md) for
the 0.2.0 diagnostic failure and the actual 23.11 IQ-stream regression evidence.

When needed, `-A -R 0` produces timing candidates. The parser accepts only measured
numeric `m=OOK_PWM` or `m=OOK_PPM` plus `s,l,r,g,t,y` values within bounds. Probe replay requires 16–64
bits and at least two matching frames across the replay output. Multiple distinct eligible codes are rejected.
An accepted candidate is narrowed to its exact measured bit count, replayed again,
and then used unchanged for the second and third presses. The saved flex decoder
uses `repeats>=1`, allowing normal operation when a press yields one JSON row;
learning still verifies repeated frames across each recording. Legacy `repeats>=2`
profiles remain valid and keep their existing identity. A profile is not persisted
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
| Armed press window, including analysis | 20 seconds; rejected captures never extend it |
| Continuous RF activity per capture | 6 seconds |
| Capture collection / rearming | 350 ms quiet or 1.2 s collection; at least 1 s release before rearming |
| One press recording | 32 packets, 256 KiB total; 4,096 pulse pairs per packet |
| Session stream | 2 MiB / 128 observed packet records, checked at progress callbacks |
| Offline analysis | 10 seconds total, 2 seconds per child |
| Analysis attempts | 40 per armed window; one worker, starts at least 500 ms apart |
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
The stopped child's already-read queue is drained for diagnostics. Partial packets
produce a specific missing-boundary rejection. Data read during shutdown is never
eligible as a new candidate. Counts describe records read by the supervisor, not
unread bytes still in OS or device buffers when a child is terminated.
An unrecognized/mismatched recording only returns to waiting; it does not restore
the receiver or unpause actions. Malformed stream input, receiver loss, process/
storage failures and hard resource limits remain errors, not candidate evidence.
Offline process failures are distinguished from successful replays with no decode,
including replay through a previously selected profile.

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
