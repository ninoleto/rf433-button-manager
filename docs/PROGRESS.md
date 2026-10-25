# Project progress

## 0.3.6-dev public-release preparation

The supplied ZIP and all internal checksums were verified. Its complete 20-commit
bundle was cloned with `--branch main` into a fresh development checkout at
`f9b496f3387cd357c1270c28566c97d26cb48dd8`. A separate clean clone reproduced the
supplied binary byte-for-byte. Existing checkouts and the running application were
preserved; no receiver access, service operation or production webhook call was made.

Reviewed the source and existing guides; added root AGENTS.md and corrected
toolchain requirements, source recovery, schema compatibility, capture-export
security, analysis limits and release/tag instructions. Retired the obsolete
installation task handoff. The operator tested three assigned home buttons and
their webhooks and accepted the latest interface within available testing.
Physical multi-tuner, USB/hub recovery, soak and broader compatibility remain
unverified; this is development-release evidence, not production acceptance.

The public repository `ninoleto/rf-button-manager` was created using existing
authentication verified as `ninoleto`. GitHub rejected the atomic history/tag push
because that OAuth credential lacks `workflow` scope. Neither main nor the tag
was published, no CI run exists, and no release asset was uploaded. A credential
permitted to publish the included workflow is required to finish. See
[TEST-REPORT.md](TEST-REPORT.md) for exact local checks and the release gate.

The dated entries below are historical reports for their stated versions, not
current instructions or fresh verification. Current operation/development
instructions are linked from [AGENTS.md](../AGENTS.md).

## 2026-10-25: receiver choice and inspection framing (0.3.6-dev)

The operator confirms all three existing button assignments were physically
working earlier today. Preserve that acceptance record and all saved mappings.
A subsequent inspection exposed selection of a once-per-record fragment even
though a different full frame repeated within each transmission. Corrected new
learning/inspection and saved-profile replay validation without changing dispatch
of existing mappings. A partial existing mapping is not silently rewritten.

Added authenticated read-only tuner inventory, a GUI device dropdown with refresh,
active name/serial/USB-port display, decoded RF activity and historical event
source attribution. One selected tuner is used at a time. Missing selections and
duplicate factory serials never cause silent fallback; physical multi-tuner
validation remains pending. No EEPROM, USB rule or unrelated service changes.


## 2026-10-25: signal inspection and compatibility reporting (0.3.5-dev)

All three currently available home buttons and webhook handling are now reported
working by the operator. The future Amazon remote and other commercial models
remain untested. Added an explicit compatibility matrix and community report form.

**Inspect signal** uses the existing bounded receive/learning machinery without
requiring or saving a remote, mapping or webhook. It displays the candidate code,
compares three separately armed presses and exports RF-only evidence. Actions
pause during inspection/restoration. Unknown traffic is not called encrypted or
rolling merely because it was not decoded. Tests pass, including real offline
recording discovery. Live inspection acceptance and publication remain pending.

## 2026-10-25: simplified webhook paste/save (0.3.4-dev)

The operator reports learning works after 0.3.3. The next reported obstacle was
the destination allowlist when pasting a complete Home Assistant URL. The button
editor now accepts that URL directly and offers one **Save & allow destination**
action for new addresses. Its optional ID helper accepts complete URLs too.
Button data and its exact destination approval persist atomically; failed saves
change neither. Saving sends no test request and does not restart reception.

Source/API/controller regressions pass. The prebuilt update preserves learned
signals, configuration and authentication. Installation and browser acceptance
of this editor change remain operator steps; no production webhook was invoked.

## 2026-10-25: two-switch learning repair (0.3.3-dev)

Direct operator tests established live decoding for two separate switches using
measured rtl_433 PWM settings. The six-button Amazon remote has not arrived and
is not part of this evidence. The installed app could not learn these signals.

The update adds bounded measured sync-PWM discovery when the analyzer says no
clue, permits validated 12–64 bit profiles, selects repeated complete rows before
excluding edge fragments, and normalizes explicit exact-width custom decoders.
Existing profiles, mappings, token and receiver settings remain compatible.

Private recordings now pass the production learning path, three software-armed
observations, saved database reload, two simultaneous decoder profiles and correct
isolated local webhook routing with repeated packets. This is offline evidence;
0.3.3 installation, physical GUI learning and real webhook effects on ZORIN are
pending. Source does not contain private codes, recordings or preset timings.

## 2026-10-25: recorded PCM discovery regression fixed (0.3.2-dev)

An operator-exported 0.3.1 recording contains 52 packets (50 FSK, two OOK),
no decoded button events, and no export truncation. One OOK packet contains two
identical complete PCM rows surrounded by incomplete fragments. Stock default
23.11/25.12 decoders and their analyzer did not identify it; explicit upstream PCM
replay did. The old learner's PWM/PPM-only candidate path rejected it. The new
bounded measured-PCM fallback reproduces a consistent code using unmodified
rtl_433 23.11, without changing the recording or hardcoding a remote's timings.

The same private recording fails the regression on 0.3.1 and passes on this code.
It remains outside Git/history/releases. A separate synthetic PCM fixture covers
three software-armed observations, save, database reopen, later matching and one
isolated local webhook despite repeated frames. This is **not** proof that the
recorded packet belongs to the intended button, or that three new physical presses
work. The GUI/operator check and physical webhook/USB matrix remain pending.


## 2026-10-25: physical learning still fails; retain actual RF evidence (0.3.1-dev)

0.3.0 is confirmed deployed and receiving on ZORIN. The intended button has not
learned. The newest attempt excluded 44 FSK packets and replayed two OOK captures
without a supported candidate. No claim is made that the intended remote is FSK
or unsupported. 0.3.1 adds a browser RF-capture export so those recordings can be
replayed directly, instead of proposing another decoder change from counters.

The linked Android implementation was inspected: it uses upstream rtl_433 25.12,
default protocol registration and a JSON output bridge. ZORIN uses 23.11. That
version difference and our limited discovery path require evidence-based comparison;
upgrading or widening modulation alone has not been proven to fix this remote.


## 2026-10-25: rtl_433-first browser learning (0.3.0-dev)

Restored the supplied development bundle at `6f43a1f5ba5d9f8b109a38235e0265eac3ab0fa1`.
This source baseline was newer than the last confirmed running ZORIN binary
(0.2.1-dev). No further local-server deployment is assumed.

Learning now accepts upstream decoded button events, including cmd/data and
numbered-button identities. A decoded press takes priority over background replay.
Generic automatic discovery supports PWM and PPM, aggregates repetition across
packet records, and preserves existing schema-2 profiles. Each button still needs
three separate presses and explicit confirmation; actions remain disabled.
Short noise is filtered without spending replay work. Analysis stays bounded and
does not require the entire RF channel to be silent before each initial capture.

The release includes a static prebuilt binary, full source/history and an update
helper that checks the LIVE version and rolls back failed installation. It needs
no user-side toolchain. See TEST-REPORT.md for recording and synthetic test evidence.
Physical acceptance and public GitHub publication remain pending.


## 2026-10-25: ignore background captures within a fixed learning window (0.2.2-dev)

The operator's no-button attempt on installed `9c700312` reached replay with one
OOK capture and three excluded FSK packets, then ended at 0/3. The application
incorrectly promoted that single recording's `unsupported` result into a terminal
session failure. A regression reproduced the same counts and failed before the fix.
See [BACKGROUND-LEARNING.md](BACKGROUND-LEARNING.md) for the trace and correction.

Unrecognized/mismatched captures now retain rejection metadata and return to the
same waiting window. There are four analysis attempts per window, and the unchanged
180-second session deadline plus the fixed 20-second window deadline cannot be
extended by RF traffic. No automatic rearming or confirmation was introduced.
Process/receiver/stream failures remain terminal; decoder validation is unchanged.

65 top-level Go tests pass with the race detector, excluding the eight existing
HTTP-listener tests blocked by this session's socket restriction. Four frontend
controller scenarios, vet and deployment/history checks pass. Actual installed
rtl_433 file replay verifies background rejection followed by synthetic profile
learning. These are software results, not compatibility evidence for the remote.

The original home checkout/history/configuration/token remain preserved. A separate
application-only update is prepared because the home/system filesystem is read-only
and elevation is unavailable. The actual update bundle is to be checked through the
updater's fresh clone with explicit `--branch main`; final packaging evidence is
kept beside that bundle. Full host checks are required before deployment. No new
physical test, public push or GitHub CI result is claimed.

## 2026-10-25: failed capture evidence and 23.11 stream compatibility (0.2.1-dev)

The operator installed `0.2.0-dev` / `413ac5b` and reported a failed A ON browser
attempt: unsupported with zero capture/analyzer/replay counters. Investigation
demonstrated rejection before accounting and a receiver-status display drawn from
restored reception. Valid FSK on the actual rtl_433 23.11 `-w OOK:-` output reproduces
that failure, but the lost historical rejection means it cannot be attributed to
the intended remote. See [CAPTURE-INVESTIGATION.md](CAPTURE-INVESTIGATION.md).

Prepared 0.2.1-dev with separate failure categories, strict stream parsing, observed
versus accepted counts, capture receiver provenance, queued/partial cleanup evidence
and persistent bounded diagnostic history. FSK does not abort waiting for OOK.
Replay validation and three-press confirmation remain required; no decoder timing
was inferred for the operator's remote. Existing history is preserved from 413ac5b.

Actual 23.11 synthetic IQ tests prove mixed JSON/OOK and FSK packet handling before
EOF, without USB. Parser, application lifecycle/persistence and frontend regression
tests pass; detailed verification and deployment limits are in TEST-REPORT.md.
The live health endpoint still reports 0.2.0-dev. This session cannot write the home
checkout or elevate privileges: the corrected update is prepared separately for
the already-authorized application-only host helper. No new physical test or public
publication is claimed. Earlier entries describe their respective historical builds.

## 2026-10-25: browser capture learning (0.2.0-dev)

The operator verified the preceding 0.1 installation, authentication, configuration
persistence and receiver sample acquisition after an initial receiver-only reconnect.
A ON remained at 0/3 because the old learner accepted only already-decoded JSON.
Those observations do not establish physical button decoding or USB recovery.

Implemented a browser-only capture/analyze/replay workflow, measured fixed-code
OOK/PWM profiles, three separate matching presses, atomic confirmation, profile
reuse, progress/failure explanations and retry/diagnostics. Existing remotes and
legacy settings are retained. Actions remain suppressed through receiver/replay
cleanup; new mappings stay disabled. No verified legacy Switch433 definition was
located and no stock/guessed timing was installed. See [LEARNING.md](LEARNING.md).

Software evidence: 49 top-level Go tests pass with the race detector when the eight
socket-dependent tests are excluded; real installed rtl_433 23.11 file replay,
combined JSON/pulse output, candidate proof, lifecycle, persistence and exclusive
receiver transitions are included. Two frontend-controller scenarios pass with
an in-memory DOM/API adapter. Vet, module checksums, shell/deployment/history tests
and JavaScript syntax pass. These are software tests and synthetic RF evidence.

A separate clean clone of `65f9f02265fdb486dcff2f5808f4410c7533ccfd` reproduced the
static development executable byte-for-byte; see TEST-REPORT for its SHA-256.

The full suite reaches an existing HTTP test and is blocked by denied loopback
sockets. Browser rendering tests and physical learning remain pending. The existing
home checkout and system service are read-only from this agent session; the update
is staged with preserved Git ancestry, not claimed deployed. A reviewed one-time
host helper is kept outside public source to run the full checks, import the new
history, snapshot application data, replace only its binary and verify service
restart/persistence. It preserves authentication/listener and performs no RF or
Home Assistant action test. Source publication and GitHub CI remain pending the
agreed testing gates. Earlier entries below are historical evidence for earlier builds.

2026-10-25: Requirements read; upstream rtl_433, Home Assistant and SQLite researched.
Architecture and API contract established. Implementation started in isolated workspace.
No server connection, service modification or physical receiver test performed.

## Implemented and verified in this workspace

- Native Linux executable built, statically linked, version 0.1.0-dev.
- SQLite persistence, bounded history, remote/button CRUD, three-press learning,
  conservative identity normalization, overlap rejection, learning suppression,
  sliding debounce, authenticated backup/restore and disabled restored actions.
- HTTP(S) origin allowlist, no redirects or automatic retries, per-button in-flight
  guard and eight global action slots, timeouts with unknown-outcome reporting.
- Serial-based USB discovery, ambiguity rejection, process supervision, sample
  heartbeat, capped backoff, clean cancellation and parent-death handling.
- Responsive embedded interface, dark/light themes, token login and live monitor.
- MIT license, dependency notices, systemd and dry-run install/upgrade/uninstall,
  read-only server inspection, troubleshooting, CI and hardware test matrix.
- Final `make check`: uncached race-enabled Go tests, go vet and deployment
  dry-run/argument tests passed. Linux amd64 static build succeeded.
- Nine grouped Chromium integration checks passed against the actual isolated
  demo service; no JavaScript/CSP errors. Screenshots are synthetic demo only.
- Review corrections: exact trusted HTTPS proxy origin, distinct-button webhook
  concurrency, inherited identity defaults, 64-bit-safe mapping comparisons,
  canonical restore validation, case-insensitive header uniqueness and size caps.

No test in this workspace establishes operation with the user's RF hardware or
Home Assistant. The exact final Switch433 decoder was not present in the earlier
analyzer capture; no guessed decoder has been enabled.

## Acceptance gates

1. Architecture: documented components, security boundaries, risks, deployment model.
2. Minimum service: build executable, embedded GUI, receiver supervisor, live history.
3. Learning: 3 separately armed matching presses, explicit confirmation, duplicates,
   timeout/cancel, stable normalization, global action suppression, persistence tests.
4. Webhooks: allowlisted HTTP(S), timeout, no redirects/retries, bounded concurrency,
   debounce, test action, redacted logs and outcome-unknown handling tests.
5. Reliability: synthetic USB disappearance/reappearance, ambiguity, child crash,
   backoff, shutdown, no restart on radio silence; systemd service and installer.
6. UI: remote/button CRUD, editor, learning, history, settings, backup/restore, themes;
   desktop/mobile browser verification.
7. Release: unit/integration/race tests, build, docs/license/CI, source bundle;
   ZORIN inspection + approved deployment + actual hardware matrix before stable release.

## Outstanding external gates

- Exact final Switch433 decoder and real JSON samples from ZORIN.
- Full host inspection and completed deployment. Application-owned installation
  and configuration were explicitly authorized in the 2026-10-25 continuation;
  receiver handover, physical tests and unrelated changes remain separate.
- New GitHub repository creation/push (connected tool currently has no repository-create operation).
- Actual receiver, port/hub, reboot and Home Assistant tests.

Continuation instructions: [ZORIN-CODEX-TASK.md](ZORIN-CODEX-TASK.md). Existing
scope authorization supersedes its older request to reconfirm installation.

## 2026-10-25 continuation: documentation and reproducibility

- Existing extracted source and binary verified; original archive retained.
- Original bundle restored in a temporary development checkout: five commits,
  ending at `2643e783e9b95ad2ebb6635e70d55fe440800f60`. Extracted source matched.
- Existing home workspace is read-only in this agent environment. No host install,
  system configuration, receiver handover or webhook call has been performed.
- Read-only visible evidence: Zorin OS 18.1, Linux x86_64, rtl_433 23.11, internal
  disk space available, one visible Realtek RTL2838 device. Full host ownership,
  LAN binding and physical validation remain pending operator-run host checks.
- Architecture/API/operations expanded; configuration, security, contribution,
  developer extension and release guides added. Source build/install requirements
  and physical versus simulated evidence are explicitly separated.
- Safe bundle-history restoration added and tested with edited/untracked files
  and tags; existing repositories are refused rather than reset. Installer now
  refuses known foreign rtl_433, rtl_tcp and rtl_test processes, without stopping
  them. Read-only inspection reports blocked host visibility explicitly.
- Checksum-verified temporary Go 1.27.3 toolchain and pinned modules used for
  development only. Static build reproduced the supplied executable SHA-256.
  A separate clean clone of `266a97fa04b729ef0d6fe2ecb8871ec022576f4d` reproduced
  that checksum too, with no tracked-file changes from the build.
- CLI/receiver race tests and 16 non-socket application tests passed (34 total).
  Full `make check` is blocked by denied loopback sockets in eight HTTP-server
  tests. Vet, deployment/history checks and JS syntax checks passed. See TEST-REPORT.
- Authenticated GitHub identity verified as `ninoleto`; repository lookup returned
  404. No repository creation, push or GitHub-hosted CI run is claimed. Publication
  waits for the authorized installation/testing gates and full host verification.

Local continuation helpers and pending-work details are kept outside public source;
never commit host reports, runtime files, private configuration or captures.
