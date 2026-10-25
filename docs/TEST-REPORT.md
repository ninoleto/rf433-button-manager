# Verification report — 2026-10-25

## 0.3.1-dev: RF evidence export (diagnostic update)

The operator deployed 0.3.0 successfully but learning still timed out. Their
reported attempt contained 46 complete packets: 44 FSK excluded from discovery,
and two OOK recordings with zero supported analyzer candidates. This confirms
the exclusion/analysis path, not which packets came from the intended button.
Neither physical learning nor the intended webhook has passed.

0.3.1 retains the most recent attempt's valid OOK **and FSK** pulse records and
actual decoded RF events in bounded RAM. An authenticated browser download exposes
only that capture, never the database, application token, settings or webhooks.
Retry/restart replaces or clears it. Tests cover FSK retention, timeout retention,
128-packet/2-MiB bounds, stale-generation rejection, authentication, isolated copies,
and omission of private configuration. The browser test downloads a synthetic
capture through the actual control. No signal-compatibility correction is claimed.


## 0.3.0-dev: direct rtl_433 learning and PWM/PPM discovery

Validation environment: isolated Linux x86_64 build machine, Go 1.27.3,
rtl_433 **23.11**, file replay only. The test decoder was compiled without USB
backends; it is not shipped as a replacement for the server's rtl_433.

Passed in this environment:

- Full Go race suite, including HTTP tests, and `go vet`.
- Four frontend controller tests against the actual JavaScript and HTML IDs.
- Ten real Chromium headless-browser checks: token login, remote editing, unsaved
  edits, three-press demo learning, escaped labels, mocked capture/diagnostic states,
  settings/backup/restore, duplicate-remote reset, desktop/mobile themes and logout.
  Browser checks use a real isolated demo service, not USB or physical RF.
- Deployment/history tests and four isolated prebuilt-update tests: apply dispatch,
  real temporary-file replacement/backup, wrong-live-version rollback, and checksum refusal.
- Four real upstream IQ recordings decoded by rtl_433: EV1527 (Akhan event),
  PT2262 (Generic-Remote event), Akhan, and HT680. Each follows three explicitly
  armed presses, disabled confirmation, persisted configuration loaded into a new
  app instance, then one localhost test webhook despite repeated RF frames.
- Automatic synthetic PWM discovery and measured PPM discovery; PPM repeats in one
  packet and in six separate packets. Saved profile replay, single-frame runtime
  normalization, and legacy `repeats>=2` profile compatibility.
- Short background bursts do not consume analysis attempts. Unrecognized traffic
  cannot extend the press deadline. A live decoded button interrupts background
  replay, and the cancelled replay cannot overwrite the accepted identity.

Reproduce recording tests without touching a receiver:

```sh
python3 scripts/fetch-rf-testdata.py /tmp/rfb-rf-tests
RFB_RF_TESTDATA=/tmp/rfb-rf-tests make check
```

The fetch script pins `merbanan/rtl_433_tests` commit
`1aa8c29d6b9383739d1aeed495dbc5069b704392`, source paths and SHA-256 hashes.
Recordings remain upstream; they are not embedded in this project's release.
Their repeated replay simulates separate presses for software integration; it is
not evidence of three new physical presses, radio range, USB reliability, or the
operator's new Amazon switch. Local test webhooks never contact Home Assistant.

ZORIN deployment, real operator learning, intended webhook execution and physical
USB recovery remain pending for this version. No GitHub CI result is claimed.
The sections below preserve evidence for earlier versions; their older limits do
not describe the current learning contract in [LEARNING.md](LEARNING.md).


## Background capture correction: 0.2.2-dev

Base: installed/home source `9c700312b10be4506a1d1550c0b5c2282af0333d`.
The premature terminal failure was reproduced before the change with four packet
records, three exclusions, one accepted capture, zero analyzer candidates, two
replays and zero presses. The same regression now stays waiting and keeps its
original deadlines. See [BACKGROUND-LEARNING.md](BACKGROUND-LEARNING.md).

Executed in the development checkout:

- **65 top-level Go tests passed with `-race -count=1`**, excluding the same eight
  socket-dependent tests listed in the 0.2.1 section below. A current loopback socket
  creation probe returned `EPERM`; no new full-host-suite pass is claimed.
- Background FSK and undecodable OOK expire with zero presses; repeated background
  traffic cannot exceed four analysis attempts in an armed window. Deadline and
  verified candidate/profile remain unchanged after rejection.
- Matching traffic cannot automatically open subsequent windows or save a mapping.
  Mismatched traffic can be followed by the intended matching capture in the same
  manually armed window. Three windows and explicit confirmation remain necessary.
- Actual installed rtl_433 23.11 file replay rejects a synthetic single-pulse OOK
  recording with zero candidates/two replays, then learns an existing synthetic PWM
  fixture through profile discovery and three separately armed matching captures.
  Additional background input fails the selected profile without ending learning.
  The resulting mapping is saved disabled. No USB was opened.
- Late successful analysis after the press deadline cannot increment presses.
  Actual offline process exits remain terminal with default, selected, preferred
  and fallback profiles. Existing strict-decoder, cancellation, receiver ownership,
  persistence and overlap tests continue to pass.
- **Four frontend-controller tests** pass, including rejection evidence, visible
  fixed-window time, analysis budget exhaustion and no automatic API rearming.
  These use the actual controller with a DOM/API adapter, not a real browser.
- Vet, JavaScript/shell syntax, installer safeguards and history restoration pass.

Packaging must verify the actual distributed bundle with `git clone --branch main`,
check the selected commit, run the available checks and build the executable from
that fresh checkout. The one-time helper's verification-only mode makes this path
testable without home/system writes. Its normal deployment path still requires full
`make check` on the host before any installation. The bundle/build hashes and final
fresh-clone verification are recorded in the adjacent private update manifest/report.

Live health still reports 0.2.1-dev. This session has not deployed the correction.
Physical compatibility, successful GUI learning of the actual remote, webhook effects,
USB recovery, public publication and GitHub CI all remain unverified for this update.
Earlier sections document their respective historical versions.

## Capture diagnostic correction: 0.2.1-dev

Prepared from the installed/home source base `413ac5b`. The operator's failed
physical A ON attempt on 0.2.0-dev is recorded as a failure with insufficient
diagnostics, not as proof of unsupported RF. The demonstrated accounting/status
defects and the limit of retrospective attribution are documented in
[CAPTURE-INVESTIGATION.md](CAPTURE-INVESTIGATION.md).

Executed software checks for the correction:

- **58 top-level Go tests pass with `-race -count=1`**, excluding the same eight
  loopback-socket-dependent tests listed below. New regression coverage includes
  FSK exclusion without aborting OOK learning, empty/JSON-only/malformed/partial
  capture classification, cleanup evidence, cancellation, event persistence after
  store reopen, receiver-status provenance and unmodified configuration.
- Installed **rtl_433 23.11** processed generated unsigned-8-bit IQ input with
  default protocol detection and `-w OOK:- -F json`. OOK produced 144 pulse rows
  alongside decoded JSON; FSK produced a valid 144-row FSK packet on the same
  output type. Both packets completed **before input EOF**. No USB was opened.
- Parser regressions cover interleaved JSON/metadata, blank lines and CRLF, row
  counts, explicit units/version, invalid and excessive durations, packet size,
  repeated headers, missing end boundaries and incomplete flushes.
- A fake reader/child lifecycle regression proves already-queued complete and
  incomplete packets are counted during shutdown and are ineligible as new
  learning candidates. The existing supervisor test still proves exclusive live
  receiver ownership across transitions.
- **Three frontend-controller scenarios pass** with an in-memory DOM/API adapter,
  including distinct failure titles and capture status separate from restored
  normal reception. These are not real browser-layout or physical RF tests.
- Vet, formatting, JavaScript/shell syntax and deployment/history tests pass.
  A static Linux amd64 executable builds with `make build` and reports 0.2.1-dev.

A separate clean `--no-local` clone of
`a7dc500a6f4077c8c180b1e39cf0c0ecdfc3b822` passed the documented `make build` with
Go 1.27.3 and left its tracked worktree unchanged. The static 0.2.1-dev executable
matched the development build byte-for-byte, SHA-256
`7bd7cd0e2f39a7c4592afaee6cfd85d6e61b828a7c7a593f16ab69217b2fb513`.
The update's 52 local documentation links resolved; 70 source files were checked
for private runtime artifacts and high-confidence credential patterns, with none
found by those checks.

The environment still prohibits loopback listening sockets, home-checkout writes
and sudo escalation. The host update helper must run full `make check` before
deployment; no new full-host-suite pass is claimed here. The excluded tests are
`TestDebounceHeldButtonAndRearm`, `TestLearningSuppressesAllActionsAndTests`,
`TestDifferentButtonsDispatchConcurrently`, `TestLearningWaitsForInFlightRequest`,
`TestStorageFailureDisablesActions`, `TestWebhookDoesNotFollowRedirectOrRetry`,
`TestWebhookTimeoutIsUnknownAndNotRetried`, and `TestWebhookPayloadAndHeaders`.

The live `/healthz` still reports 0.2.0-dev. Application deployment/restart checks,
real browser rendering, corrected physical learning, physical webhooks and USB
recovery remain pending. No decoder timing, button identity or physical result was
invented. No public repository push or GitHub CI result is claimed for this update.
Earlier sections are historical reports for their stated versions.

## Browser learning update: 0.2.0-dev

The new code was prepared from the existing imported history at `08b7fc4`.
The installed baseline was operator-verified at 0.1.0-dev: application/authentication/
configuration persistence passed; receiver samples were acquired after one initial
receiver reconnect. Physical A ON decoding remained unsuccessful. This update has
not yet been deployed or tested with that physical remote.

Verified software checks for the update:

- 49 top-level Go tests with `-race -count=1`, excluding exactly eight existing
  tests that require a local HTTP listening socket. New cases include cancellation
  during analysis, late results, timeouts without browser polling, receiver loss,
  prolonged transmissions, inconsistent captures, duplicate mappings, existing
  configuration preservation, schema migration and profile persistence/reuse.
- Real installed rtl_433 23.11 replay of an explicitly synthetic OOK/PWM fixture;
  measured candidate discovery, exact-profile replay and simultaneous JSON/pulse
  output parsing. All invocations use file input, never USB.
- Rejection of unproven analyzer suggestions, unsafe candidates, mixed codes,
  malformed/oversized captures, excessive output and unbounded analysis lifetime.
- A supervisor test deliberately holds old-child cleanup open and verifies that
  the replacement live receiver cannot start until cleanup finishes.
- Two frontend-controller scenarios using the actual app.js with an in-memory
  DOM/API adapter: capture progress, diagnostic details, failure retention, cleanup
  gating, retry and confirmation. These do not establish browser rendering.
- `go vet`, module checksum verification, existing deployment/history tests,
  JavaScript/shell syntax and local documentation-link checks.

The full Go suite was attempted and blocked by `socket: operation not permitted`
in an existing `httptest.NewServer` test. The eight excluded tests cover HTTP
webhook behavior and existing suppression/concurrency checks; they must run in the
normal host before deployment. Playwright/browser rendering and all physical
learning/webhook/USB recovery tests remain pending. No new physical PASS is claimed.

Clean-build evidence: a separate `--no-local` clone of `65f9f02265fdb486dcff2f5808f4410c7533ccfd`
built successfully with the documented `make build` command and Go 1.27.3. The
static Linux amd64 executable reports `0.2.0-dev`; its SHA-256 is
`88e5cce534d4f6cbbe3ec718107ba4754a5597dbc4ff009dc3631275668ee4b8`. It matches the development-checkout build exactly,
and the clean clone's tracked worktree remained unchanged. The inherited history
and outgoing source were checked for private runtime artifacts and high-confidence
credential patterns; none were found by those checks. Publication remains pending.

The one-time host update helper runs full `make check` before deployment and takes
a stopped state snapshot because schema 2 requires matching old data for a 0.1
rollback. The current session cannot write the home checkout, use sudo, or start
local test servers; these are environment limitations, not completed checks.

## Continuation evidence (separate from original package report below)

The original package's report below describes its earlier isolated run. It is not
a new successful full-suite or hardware result from the continuation environment.

Verified in the continuation:

- Original ZIP SHA-256: `fa8d873a3c54ea31e676411e76ebf72716cad372f7df8b143c2353acedcb9fec`.
- Supplied executable checksum/version and source agreement with bundled main.
- Official temporary Go 1.27.3 toolchain checksum and all pinned module checksums.
- Static amd64 build reproduced the supplied executable SHA-256:
  `dddc9ce0479a46193d6c258d7354221f9284cc87ba105de9f681e09a6cf1f144`.
  A second build from a clean `--no-local` clone of commit
  `266a97fa04b729ef0d6fe2ecb8871ec022576f4d` produced the same hash. Dependency
  download/verification, formatting, vet, deployment checks, build and version
  commands passed there, and the tracked worktree remained clean.
- 2 CLI and 16 receiver race-enabled tests, plus 16 application tests that do not
  create network listeners: 34 top-level tests passed. Full-suite HTTP-server
  checks are blocked by `socket: operation not permitted`, not reported passed.
- `go vet ./...`, deployment dry-run/argument tests, all three known competing
  decoder guards, bundle restoration preserving edits/untracked files/tags,
  shell syntax, JavaScript syntax and whitespace checks passed.
- 37 local Markdown links and 26 shell/Python/JSON code snippets checked for
  target existence/syntax. API examples were reviewed against handler/model code;
  the existing in-process authentication/strict-JSON tests passed without sockets.
- Original historical blobs and outgoing source reviewed for private artifacts
  and high-confidence credential patterns; none found by those checks. Existing
  screenshots were visually reviewed and contain explicitly synthetic demo data.

Pending: full `make check` in a normal host execution environment; rerun of the
optional browser suite; host service installation/LAN access/auth/persistence;
working decoder verification; all physical learning/webhook/USB/reboot tests;
public repository creation/push and GitHub CI. A staged development checkout is
not the installed service. No physical test row changes to PASS based on this work.

## Original package verification record

Build: RF Button Manager 0.1.0-dev, Linux amd64, Go 1.27.3.
Environment: isolated development workspace, not ZORIN.

## Passed

- 42 top-level Go tests (including subtests), `go test -race -count=1 -timeout=2m ./...`.
- `go vet ./...`, `go mod verify`, and shell syntax checks.
- Deployment installer/uninstaller dry-run and invalid-argument tests.
- Static Linux amd64 executable build, version check and ELF static-link inspection.
- Native executable smoke check: HTTP access, configuration surviving process
  restart, second-instance refusal, missing-receiver status and graceful shutdown.
- Nine grouped real Chromium checks against the actual isolated Go demo service:
  login, six-button remote/action editing, unsaved-input preservation, three-press
  learning, HTML-safe labels, backup/restore, duplicate remote, desktop/mobile themes,
  and lock/token removal. No JavaScript/CSP errors reported.
- systemd unit syntax verification with a temporary stub executable; this does
  not test service installation, hardening compatibility or USB access on ZORIN.

Go tests exercise learning arm/quiet intervals, mismatches, duplicate detection,
timeout/cancel preservation, global action suppression, trailing burst suppression,
held-button debounce, simultaneous distinct-button webhooks, learning waiting for
in-flight requests, SQLite persistence and bounded history, corruption refusal,
storage failure stopping actions, authenticated API, cross-origin rejection, trusted
HTTPS proxy origin, redirects, HTTP timeout/no retry, payload/headers, URL validation,
prohibited network addresses, 64-bit IDs, restore canonicalization and limits.

Receiver tests use fake process/sysfs inputs: disconnected/reconnected/moved receiver,
serial ambiguity, crash backoff, RF silence with sample heartbeat, stalled acquisition,
cancellation, child kill escalation, saturated stderr, oversized lines and safe argv.

## Not performed; required before production release

- Actual ZORIN installation or systemd startup.
- Actual RTL2838/R820T acquisition or Switch433 decoder verification.
- Physical learning and Home Assistant webhook/automation execution.
- Physical USB same/different port, different hub, prolonged disconnect and reboot.
- Sustained server soak test and power-loss validation.
- GitHub-hosted CI run; workflow is prepared but repository publication is pending.

See HARDWARE-TESTS.md. Synthetic test evidence must not be recorded as physical
receiver success. Busy limits (eight global requests, one per button) intentionally
reject excess presses with an event; there is no deferred action queue or replay.
