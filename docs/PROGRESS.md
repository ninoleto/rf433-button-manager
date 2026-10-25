# Project progress

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
