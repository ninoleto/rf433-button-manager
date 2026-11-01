# Receiver audit before 0.4.0-rc1

Audit date: 1 November 2026. Application source reviewed:
`918208c3d96c4c84c69c40e308325f9d2fd756e0` (`0.4.0-test.3`).
This is an audit and a proposed experiment, not a new receiver implementation,
package, hardware acceptance result or publication approval.

## Decision

There is a worthwhile sensitivity experiment before publication: an opt-in
adaptive pulse-detection threshold using upstream rtl_433's `-Y autolevel`.
Start with this setting alone. Preserve the existing reception mode and button
mappings. Do not change gain, sample rate, pulse estimator and matching rules at
the same time.

Offline tests demonstrate that this can recover synthetic low-amplitude signals
which the current defaults miss. They do not demonstrate improved kitchen range.
The optional `-Y magest` estimator helped some additional synthetic cases but
also changed the decoder reports for a public recording. It needs a separate
comparison and should not be silently made the default.

## Findings

### 1. Adaptive sensitivity is not enabled

`internal/receiver/receiver.go:arguments` pins 433.92 MHz and uses
`-c /dev/null`, so user/system rtl_433 configuration files do not supply receiver
options. It does not set `-Y autolevel`, `-Y magest`, gain, sample rate or PPM.
The reviewed 23.11 source defaults to 250,000 complex samples/second, automatic
tuner gain, the amplitude estimator and a minimum detection level of about
-12.14 dB. Automatic tuner gain and automatic pulse-detection threshold are
different settings; one does not imply the other.

Upstream's analysis guide recommends trying adaptive threshold and magnitude
estimation for sensitivity. The exact options were verified in the locally
available upstream 23.11 source at commit `59133f4`. In that version, autolevel
lowers the minimum threshold when the estimated noise floor permits it. It does
not remove interference or reconstruct a signal buried in noise.

### 2. Current diagnostics cannot establish why every physical press is missed

Normal reception does not request `-M level` or `-M noise`. Learned-profile
normalization also intentionally reduces an event to its model and exact code;
simply adding metadata flags would not preserve profile reception measurements
in the current event monitor.

Add diagnostic measurements separately from the matching identity: last target
signal level, estimated noise, SNR where available, decode/match time and action
outcome. Never use those changing measurements as button identity fields. Values
from `-M level` are relative to the receiver's current gain, not calibrated dBm
or a universal percentage. Do not show another transmitter's level as the
selected button's signal strength.

The current capture export contains pulse records and decoded JSON, not raw
I/Q samples. It cannot be replayed through different RF detection thresholds to
recover transmissions that never became pulse records. A bounded, explicit raw
I/Q diagnostic capture would help if the next physical comparison is inconclusive;
it is not implemented or a requirement for ordinary operation.

### 3. No new normal-matching regression reproduced in the audited cases

Reviewed the receive arguments, supervisor, configured/learned normalization,
bounded decoder equivalence, duplicate mapping checks, debounce and dispatch
gates. The receive arguments themselves are unchanged from 0.3.8-rc.1. The 0.4
matching changes address different decoder representations of the same payload.

The normal receive path dispatches the first eligible match without deliberately
waiting for a second press. The default 700 ms interval suppresses repeated
bursts until there is a quiet interval; it is not a 700 ms first-press delay.
Very rapid intentional repeats can be suppressed. Lowering this interval is not
a remedy for weak reception and can produce duplicate light toggles.

Race-enabled targeted tests passed: 44 top-level tests plus 55 subtests, no
failures or skips. Coverage included acquisition heartbeat, shutdown, exclusive
child ownership, device selection/re-enumeration, exact payload equivalence,
changed-bit rejection, duplicate mappings and action suppression. Selected
private recording regressions produced the expected isolated HTTP requests:

- Three recorded bursts: three requests, including the alternate-only scenario.
- Saved Smoke mapping: three requests for all recorded reports, simulated
  custom-only reports, and simulated Smoke-only reports.
- Earlier weak recording: nine requests from nine recorded burst groups.

These are recording replays, not new physical presses. The saved-Smoke regression
substitutes the exported profile model label with a registered synthetic profile
because the export omits its timing configuration. It preserves recorded code
bits, fragments, order and timestamps; it does not prove the user's actual
profile timing behavior under new RF conditions. Full unfiltered recordings
already produced three actions before the test.3 correction, so this audit does
not attribute every physical miss to that corrected matching gap.

### 4. The supplied host evidence does not establish a USB fault

The operator's USB snapshot showed the receiver at 480 Mb/s, runtime active and
`power/control=on`, which disallows autosuspend. A stored autosuspend delay does
not override that setting. The service was active with no systemd restarts;
that counter alone does not count rtl_433 child restarts.

The excerpt showed DVB attach/deinitialize cycles, without receiver USB-core
reset/disconnect lines in that excerpt. The operator confirmed using Inspect
signal and moving USB ports beforehand. Those observations do not justify USB
driver blacklisting, hub resets or system-wide power changes. Stream Deck events
are not receiver evidence.

The operator estimates roughly 95% success from the kitchen, improving when the
front of the remote is not covered. This is an informal report without a counted
denominator. It suggests grip, orientation or the radio path deserves testing;
it does not prove a defective remote or exclude receiver settings as a factor.

## Offline sensitivity experiment

Used rtl_433 23.11 with file input only. A deterministic synthetic CU8 recording
contains three transmissions starting at 4, 7 and 10 seconds; each contains six
identical 24-bit PWM frames. Each file is 12 seconds at 250 kS/s. Carrier offset:
20 kHz. Seed: 4330400. Independent Gaussian noise is added to I and Q before
rounding/clipping around 127.5 to eight-bit samples. The synthetic code is
`a9b2c5`, unrelated to the operator's devices.

The same file was replayed in each mode, with unchanged decoder criteria:

```text
-c /dev/null -r FILE_433.92M_250k.cu8 -s 250000 -R 0
-X n=AuditSynthetic,m=OOK_PWM,s=400,l=1200,r=20000,g=7000,t=160,bits=24,repeats>=2
-F json -F log:/dev/stderr -M level -M time:rel
```

The adaptive modes append `-Y autolevel`, or `-Y autolevel -Y magest`.
Counts below are distinct synthetic transmission windows with the exact expected
code, not a count of repeated decoder messages. Amplitude and noise are synthetic
ADC units, not measured antenna power. The strong baseline decoded all three
windows before the weaker comparisons were accepted as meaningful.

| Signal amplitude | Noise sigma | Current defaults | Autolevel | Autolevel + magest |
| --- | --- | --- | --- | --- |
| 80 | 2 | 3/3 | 3/3 | 3/3 |
| 40 | 2 | 3/3 | 3/3 | 3/3 |
| 20 | 2 | 0/3 | 3/3 | 3/3 |
| 10 | 2 | 0/3 | 3/3 | 3/3 |
| 5 | 2 | 0/3 | 0/3 | 2/3 |
| 80 | 8 | 3/3 | 3/3 | 3/3 |
| 40 | 8 | 3/3 | 3/3 | 3/3 |
| 20 | 8 | 0/3 | 2/3 | 3/3 |
| 10 | 8 | 0/3 | 0/3 | 0/3 |
| 5 | 8 | 0/3 | 0/3 | 0/3 |

Zero-signal controls at both noise levels produced no custom decoded events.
Separate 12-second zero-signal files at noise sigma 2, 8 and 16 produced no events
with all default built-in decoders in any of the three modes. These short,
Gaussian-noise controls do not model a household's other transmitters and do not
establish a false-trigger rate.

Four SHA-256-pinned public IQ fixtures from `scripts/fetch-rf-testdata.py` were
also replayed. Autolevel alone preserved the complete decoded payload sequence
for all four (metadata excluded). Autolevel plus magest preserved three sequences;
the Akhan fixture still produced three Akhan reports but no longer its additional
Smoke report. A saved Smoke-only mapping without a registered equivalent PWM
profile could therefore behave differently. No automatic estimator change is
recommended on this evidence.

## Proposed physical comparison

1. Expose **Standard / Adaptive sensitivity** in the app, defaulting existing
   installations to Standard. Pass the same selection to normal reception and
   learning. Keep one supervised receiver child, fixed identity checks, existing
   mappings and ordinary action suppression. Add measurements separately.
2. Test the actual kitchen location with a counted run, initially 30 deliberate
   presses in each mode, spaced two seconds apart after allowing reception to
   settle. Keep location, orientation, grip and battery unchanged. Compare
   recognized presses and duplicate/wrong matches, not total ambient RF events.
   Keep configured actions disabled for the RF comparison. Repeat the comparison
   in reverse order if the apparent difference is small. This is a practical
   comparison, not statistical proof of a reliability target.
3. Separately try moving the receiver/antenna away from the computer and metal
   using a short USB extension, and changing antenna orientation. The upstream
   guide specifically recommends separating the receiver from the computer to
   reduce noise. Change one variable at a time. Do not infer Stream Deck cable
   behavior applies to this receiver.
4. Check all nine saved button assignments before keeping a new mode, then do
   explicitly authorized end-to-end action checks and idle observation. If the
   misses remain, collect a bounded raw IQ sample of counted failed presses
   before considering gain, sample-rate or frequency changes.

No maximum-gain preset, relaxed bit matching, partial-code acceptance, webhook
retry, extra repeat requirement or forced decoder upgrade is proposed. RF data
that is absent or obscured by noise cannot be guaranteed recoverable in software.

## Before packaging the candidate

- Implement and physically compare any selected reception change before calling
  it an improvement on the operator's equipment. Retain the working option.
- Complete the operator's pending network UI correction: the upper **Server
  address** picker is the sole address control. Remove the duplicate editable IP
  field, rename **Advanced: address and ports** to **Advanced: ports**, and update
  help and controller/browser checks. This correction is not implemented by the
  audit.
- Record the latest operator results as reported observations, retaining the
  unverified multi-tuner, USB/hub recovery, formal soak and wider-hardware limits.
- The requested candidate name is **0.4.0-rc1**. A release candidate is not proof
  of universal hardware compatibility. Run final package/build checks and CI for
  the exact packaged commit after any edits.
- Verify the replacement asset and public download before removing the old
  release page/assets as requested. Preserve tags, history and rollback material.

No application logic, live host settings, webhooks, release or GitHub refs were
changed during this audit.

## Implementation follow-up: private test.4

After this audit, the maintainer authorized another private build. Test.4 removes
the duplicate address field, keeps Standard as the default, adds optional
`-Y autolevel` and provides separate per-button counters and signal/noise readings.
It does not add `magest`, new gain/sample-rate settings, fuzzy matching or a delay
before dispatch. The audit measurements above remain observations of their stated
inputs, not proof of improved reception in the apartment. The fresh physical
comparison is documented in [TESTING-0.4.md](TESTING-0.4.md); software verification
is recorded separately in TEST-REPORT.md. Publication and replacement of the old
release are deferred until the maintainer reviews the new physical results.

## Sources checked

- [Upstream analysis and sensitivity guidance](https://triq.org/rtl_433/ANALYZE.html).
- [Upstream operation: gain, frequency and sample rate](https://triq.org/rtl_433/OPERATION.html).
- [rtl_433 23.11 source defaults](https://github.com/merbanan/rtl_433/blob/23.11/src/r_api.c)
  and [sample processing / option parsing](https://github.com/merbanan/rtl_433/blob/23.11/src/rtl_433.c),
  inspected from the local upstream checkout at `59133f4`.
- [Upstream maintainer on signal-level units](https://github.com/merbanan/rtl_433/discussions/2076).
- [Kernel USB runtime power controls](https://docs.kernel.org/driver-api/usb/power-management.html).

This report omits private RF identifiers, webhook URLs, management tokens and
production logs. Synthetic inputs and public fixture results do not substitute
for a new physical test on Zorin.
