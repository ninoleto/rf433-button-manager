# Capture failure investigation: 0.2.0-dev / 413ac5b

The reported **“Transmission detected but unsupported”** with zero packets,
pulses, bytes, analyzer candidates and replays does **not** establish that the
intended remote is unsupported. The failure classification and evidence accounting
were defective. This report separates the demonstrated software failure from
unknown physical evidence.

## What the old code actually did

1. The live child emitted JSON and pulse text on stdout. `pulseAssembler.feed`
   recognized `;ook` and `;fsk` headers and returned a packet at `;end`.
2. `App.ReceivePulse` tested `packet.Error != "" || !packet.OOK || len(Data) == 0`
   **before** incrementing packet, pulse or byte counters. Any of these conditions
   immediately ended learning as `unsupported`, with a generic explanation.
3. Consequently, a rejected first packet produced exactly the reported zero
   counters and zero replays. A completely empty stream would instead wait and
   time out. Analyzer failure after an accepted capture would have nonzero
   accepted-capture counters. The reported path therefore reached the application
   with a rejected packet; it was not evidence of successful analysis.
4. Parser failures discarded the recording and retained only a generic error.
   The application did not retain even that error. It is impossible to recover
   whether the historical packet was FSK, had a count mismatch, or failed another
   parser condition from those diagnostics alone.
5. `finishLearningLocked` copied diagnostics into the failure notice. It did not
   zero them during cleanup: the misleading zeros were already present.
   Separately, the receiver supervisor reset its assembler and discarded queued
   output when stopping. An incomplete packet could disappear without a reason.
6. The failed dialog displayed top-level `state.receiver`, which follows the
   replacement normal receiver. After restoration, **connected and acquiring
   samples** could describe normal reception rather than the failed capture.
   There was no capture-generation/status snapshot to prove otherwise.

## Actual rtl_433 23.11 compatibility

The installed executable was exercised using generated unsigned 8-bit IQ input,
with explicit file input throughout. No USB receiver was opened. The synthetic
waveform contains six copies of an arbitrary 24-bit sequence with known OOK or
FSK modulation. Its timings are test data, **not the intended remote's decoder**.

The tests use the same sample-processing callback as live acquisition. This is
different from `.ook` file replay, which bypasses IQ pulse detection. The normal
default protocol set is retained: `-R 0` disables FSK demodulation in this version
and would conceal the compatibility issue.

Demonstrated results:

- OOK IQ produced a complete `;ook 144 pulses` packet and decoded JSON on the same
  stdout stream. JSON appeared before the packet in this run.
- FSK IQ produced a complete `;fsk 144 pulses` packet on **`-w OOK:-`**. The output
  option's name does not mean that it contains only OOK.
- Both streams included `;pulse data`, `;version 1`, `;timescale 1us`, timestamp
  and frequency metadata, integer pulse/gap pairs, and a final `;end` boundary.
- Each complete packet arrived while the IQ input pipe was still open, before
  EOF. No EOF-only stdout buffering failure was observed. This does not establish
  real USB latency or every driver/device behavior.
- Injecting a valid FSK packet into the old application reproduced the reported
  immediate `unsupported` result with all five counters zero. The regression
  first failed on the old code and passes after the correction.

Upstream source independently explains these observations: the 23.11
[sample callback](https://github.com/merbanan/rtl_433/blob/23.11/src/rtl_433.c)
dumps both OOK and FSK to a `PULSE_OOK` output; that file also disables stdout
buffering with `setbuf(stdout, NULL)`. The
[pulse writer](https://github.com/merbanan/rtl_433/blob/23.11/src/pulse_data.c)
emits the modulation-specific header, numeric rows and `;end`.

**This reproduces a software failure, not the physical remote's modulation.**
FSK may be unrelated nearby traffic. The original packet/rejection was not saved,
so the historical trigger cannot be narrowed to FSK with certainty.

## Correction in 0.2.1-dev

- Count observed input before deciding whether it is eligible for replay. Keep
  observed, rejected, malformed, empty, modulation, accepted and discarded counts
  separate. Track JSON, stdout bytes/lines, packet boundaries and incomplete rows.
- Continue waiting for OOK when a valid FSK packet arrives. FSK-only timeout says
  that the intended remote was not identified. No FSK decoder is invented.
- Reject malformed input with its specific bounded reason: invalid timing row,
  count mismatch, missing end boundary, unsupported explicit units/version, read
  failure or resource limit. Never echo raw rejected text or repair it by guessing.
- Distinguish no capture, malformed capture, unsupported modulation and a complete
  capture with no replay-validated decoder in the GUI.
- Snapshot receiver status and generation for the attempt, separately from current
  normal reception. Freeze the terminal failure reason while adding late evidence.
- Stop/join the old child, drain its already-read bounded queue for diagnostics,
  and report incomplete packets before starting another receiver. Cleanup data
  cannot become a new candidate. Persist bounded diagnostic metadata in history,
  even after cancel/dismiss; do not persist rejected RF recordings.
- Preserve three separate matching presses, strict replay proof, mapping overlap
  protection, existing settings/profiles and disabled newly learned actions.
  Webhooks remain suppressed through live-child and analysis cleanup.

## Regression coverage and remaining evidence

`internal/receiver/iq_test.go` covers actual installed rtl_433 sample processing,
mixed JSON and pulse output, FSK on the OOK output, and output before EOF.
`pulses_test.go` covers metadata, CRLF/blank lines, packet boundaries, partial EOF,
invalid timings/counts/units and resource limits. `capture_lifecycle_test.go`
proves queued output and partial packets survive shutdown as diagnostics.
`internal/app/capture_diagnostics_test.go` covers the reproduced FSK abort, timeout
classification, cleanup, cancellation, store reopen, receiver provenance and
unchanged configuration. Existing tests cover inconsistent identities, three-press
proof, profile persistence/reuse, and exclusive receiver transitions. The frontend
controller test checks distinct failure titles and capture/restored status text.

See [TEST-REPORT.md](TEST-REPORT.md) for executed checks and environment limitations.
No new physical press, webhook, USB reconnect, port/hub recovery or reboot test
was performed in this investigation. Install/verify the correction before any
agreed further operator test. All RF operator interaction remains in the GUI.
