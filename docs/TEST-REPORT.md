# Verification reports

## 0.3.7-dev branding verification

This update changes the product name shown in the browser and CLI to **RF433
Button Manager**, with version 0.3.7-dev. It does not change RF processing,
webhook dispatch, schema 2, persistence, selectors or installation identifiers.
The installer ownership marker deliberately retains its original text so older
managed installations remain recognized by update/uninstall tools.

- Go 1.27.3: dependency download/verification and formatting passed.
- Full `make check` passed in this environment: Go race tests, vet, all 12 Node
  controller tests, shell/deployment/history checks and five updater simulations.
  Four existing pinned public RF recordings and rtl_433 23.11 were available for
  offline tests. Optional private-recording tests were not supplied for this rename.
- `make build` passed; the executable reports 0.3.7-dev. A temporary demo instance
  served the new title, header, footer, login and learning text over HTTP. Demo
  mode used no receiver and sent no webhooks. No full browser rendering test was run.
- New source changes were reviewed for secrets and private artifacts; only public
  branding, version, documentation and the existing controller assertion changed.
- Clean bundle rebuild and exact-commit CI are publication gates; final results,
  asset checksums and the CI link are recorded in the 0.3.7-dev release notes.

No host installation or new physical RF, webhook, multi-tuner, USB/hub recovery
or soak test was performed for this branding change. Prior operator acceptance
remains evidence only for the hardware and version actually tested.

## 0.3.6-dev publication verification — 2026-10-25 UTC

This is a fresh publication review; version-specific records below describe
earlier runs and must not be read as newly executed checks.

### Provenance and public-data review

- Original ZIP SHA-256 verified:
  `4a55d123df03763ac79f0a6795e3118d65e0d3ac899126a22d8796f466b7df13`.
  Every entry in its `SHA256SUMS` passed after fresh extraction.
- `git clone --branch main` restored a complete 20-commit bundle at
  `f9b496f3387cd357c1270c28566c97d26cb48dd8`; bundle verification and
  `git fsck --full` passed. All 100 packaged tracked files match that commit.
- A separate clean bundle clone built with Go 1.27.3 and `make build` reproduced
  the supplied executable byte-for-byte: SHA-256
  `f7c4fad9f61a21918bfc7adef756c8a5a421fd8760b8669b7425df98aaafd66f`.
  It reports `0.3.6-dev`; the build left tracked files unchanged.
- Gitleaks 8.30.1 reviewed the full imported history, package, commit metadata,
  executable printable strings and all 287 distinct historical blobs. One finding
  in the filename-free blob scan was the `modernc.org/token` dependency checksum
  in `go.sum`, reviewed as a false positive. No credential was found by these scans.
- Reviewed every historical path and both image blobs. No database, token,
  production configuration, private RF recording or production screenshot is
  included. The two pulse fixtures are synthetic; both screenshots visibly show
  demo mode. Test URLs use loopback or invented/example destinations. Such review
  reduces disclosure risk; automated scanning is not proof that all secrets are detectable.

### Checks performed and environment limits

Linux amd64; Go 1.27.3, Node.js 22.23.3, Python 3.12.3, Git 2.43.0,
Make 4.3, Bash 5.2, GCC 13.3 and rtl_433 23.11. Verified cached modules were used;
no runtime host package, service or receiver configuration was changed.

- `go mod download`, `go mod verify` and `gofmt` verification passed.
- Full `make check` was attempted. CLI and receiver race-test packages passed,
  but the application package stopped at
  `TestButtonDestinationApprovalIsAuthenticatedAtomicAndDoesNotSend` because
  `httptest` could not open a loopback socket (`operation not permitted`).
  This is a blocked full-suite result, not a pass.
- A separate `go test -race -count=1 -timeout=2m -v -skip REGEX ./internal/app`
  passed 46 top-level tests. The regex excluded the 14 socket-dependent cases
  listed below; one additional optional private-recording test skipped because
  its input was deliberately not supplied.
- `make vet test-ui test-deploy build` passed: static analysis, JS syntax/controller
  suite, shell syntax and deployment/history checks, five simulated updater tests,
  static build and `--version`. These tests use temporary state/fake processes.
- `scripts/fetch-rf-testdata.py` verified all four pinned public IQ files. Explicit
  `rtl_433 -c /dev/null -r FILE -F json` replay produced 5 EV1527, 14 PT2262,
  4 Akhan and 9 HT680 decoded records. Only counts were retained in the review log.
  This separate replay is not the socket-dependent end-to-end test or new RF reception.
- Documentation links, shell/Python/JSON example syntax and `git diff --check`
  were checked. Optional Chromium rendering and private-recording regressions
  were not rerun; no physical test was performed.

The exact excluded application tests were:

```text
TestButtonDestinationApprovalIsAuthenticatedAtomicAndDoesNotSend
TestCaptureRealReplayPersistenceAndProfileReuse
TestDebounceHeldButtonAndRearm
TestDifferentButtonsDispatchConcurrently
TestExactConfigured12BitFlexLearnsFragmentsAndRoutesOnce
TestInspectionPausesActionsAndExportsOnlyAuthenticatedRFData
TestLearningSuppressesAllActionsAndTests
TestLearningWaitsForInFlightRequest
TestPrivateTwoSwitchLearningToWebhookOptional
TestRecordedRemoteEndToEnd
TestStorageFailureDisablesActions
TestWebhookDoesNotFollowRedirectOrRetry
TestWebhookPayloadAndHeaders
TestWebhookTimeoutIsUnknownAndNotRetried
```

### Publication and operator evidence

Existing GitHub connector and CLI credentials identify `ninoleto`. The repository
was absent from the authenticated owner's inventory and was created **public** at
<https://github.com/ninoleto/rf-button-manager>. Native Git/SSH network access was
unavailable in this execution environment. The approved HTTPS transport reached
GitHub, which rejected the atomic main/tag push because the OAuth credential lacks
`workflow` scope for `.github/workflows/ci.yml`. Remote discovery confirmed no main
or release tag; the Actions API returned zero runs. No release was created or ZIP
uploaded. The complete history and documentation remain prepared locally.

Minimum next step: in a normal authenticated host terminal, run
`gh auth refresh --hostname github.com --scopes workflow` for the existing account,
then resume publication using [RELEASING.md](RELEASING.md). Recheck remote refs,
preserve any work that appeared meanwhile, push the reviewed main and original-commit
tag, and require successful CI for the exact packaged commit before uploading.
Also verify CI for the final documentation commit. Do not remove CI, rewrite the
imported history or move the tag to get around the missing permission.

The operator reports three tested home button assignments and webhooks and accepts
the latest interface within available testing. Physical multi-tuner, USB/hub
recovery, soak and broader compatibility remain unverified. No universal Amazon
button support or production readiness is claimed. The running installation and
unrelated services were left untouched throughout this publication review.

## 0.3.6-dev: receiver selector, source indicator and saved-fragment guard

- Full race-enabled Go suite, vet, twelve frontend controller tests,
  deployment/history checks and five updater tests passed in isolated Linux.
- Synthetic USB inventory/supervision tests cover two distinct serials, selection,
  disappearance without fallback, duplicate serials, automatic-mode ambiguity and
  moving the pinned receiver to another port. No real multi-tuner test was run.
- Backend checks preserve three learned/enabled assignments and webhook settings
  across selection, verify persisted serial, reject changes during learning and
  enforce authentication/origin checks for device inventory. Activity/source tests
  distinguish new events from old receiver history.
- All existing private RF replay cases and pinned public remote fixtures pass.
  Four additional private captures reproduce the saved-fragment defect: preferred
  profile replay now selects the complete frame and explicitly selected conflicting
  partial profiles fail validation. Private recordings and identifiers are omitted
  from source/history/releases. Synthetic regressions cover both profile orders,
  genuine shorter frames, custom flex and unchanged existing-mapping dispatch.
- UI tests verify selection/refresh, missing/duplicate labels, activity state,
  blocked editing during capture and failed scans preserving saved selection.
  These run actual HTML/JS with a DOM adapter. Full Chromium layout testing was
  unavailable in this environment; live browser and physical acceptance of this
  update remain pending.
- Operator reports three working home buttons on the earlier installed release.
  This update preserves that work; it is not a claim of universal button support.


## 0.3.5-dev: signal inspector and explicit compatibility scope

- Operator reports all three available home buttons and webhook handling work
  after 0.3.4. This is operator evidence for those devices; exact product model
  names were not supplied. It is not universal Amazon compatibility.
- Added inspection without a remote: current rtl_433 receive/capture pipeline,
  visible candidate identity and decoder, three separately armed observations,
  RF-only report download and existing bounded raw pulse export. Inspection
  cannot confirm/save a mapping. Reports explicitly mark demo mode.
- Four new backend tests cover no-remote inspection, three-press reporting,
  no configuration change, rejection of mapping save, suppression of enabled
  actions/manual tests, authenticated exports without application credentials,
  strict API bodies, failure evidence and receiver-restoration generations.
- Private-recording regression discovers the first recorded switch from measured
  pulses through the real rtl_433 analysis path with no remote configured. Three
  software-armed observations yield its expected identifier and candidate profile;
  no profile or mapping persists. This does not constitute three new physical presses.
- Full race suite, vet, nine frontend controller tests, deployment/history and
  five updater tests passed. Existing private/public recording regressions passed.
  Full Chromium layout testing was not available in this environment.
- Documentation separates native decoder support, bounded generic OOK discovery,
  unrecognized FSK, changing/encrypted identities and out-of-band technologies.
  Added a community compatibility-report template. GitHub publication and live
  operator acceptance of the inspector remain pending.

## 0.3.4-dev: paste a complete webhook URL and save

- Operator reports the 0.3.3 learning update works; no additional physical webhook
  or USB recovery claim is made from that brief report.
- The reported screenshot error was an unapproved destination origin, not an
  invalid Home Assistant ID. The URL field now shows its destination and offers
  **Save & allow destination** when needed. The optional ID helper accepts either
  a complete URL or an ID with a base URL; complete URLs retain their path/query.
- Authenticated API regression verifies explicit matching approval, anonymous and
  cross-origin rejection, rollback on invalid input, duplicate-free saves,
  database persistence and unchanged learned mappings/settings. No webhook is sent
  and no receiver restart is requested by this save operation.
- Seven frontend controller tests include direct full-URL paste, single-request
  save/approval, full URL in the helper despite a stale base, ID-only compatibility,
  explicit port handling and rejected credential/fragment/non-HTTP URLs.
- Full Go race suite (including saved private/public RF replay regressions), vet,
  seven frontend controller tests, deployment/history and five updater checks
  passed. No new receiver code or dependency versions are included.
- No production automation was called. UI controller and API tests are isolated;
  live operator acceptance of the new editor remains pending.

## 0.3.3-dev: measured PWM learning and partial-row correction

Validation: isolated Linux x86_64, Go 1.27.3 and unmodified upstream rtl_433
23.11 compiled without USB backends. No production service, USB receiver or real
webhook destination was accessed from this environment.

- Verified archive/source baseline 0.3.2. User terminal output independently proves
  live decoding of a first switch and a separate front-door switch using measured
  PWM settings. This does not prove the GUI update is installed or accepted.
- Added automatic measured sync-PWM candidates without per-device timing presets,
  12-bit profile support and exact-width normalization for explicitly bounded
  custom decoders. Existing profile hashes/schema remain compatible.
- Private replay regression: 23 cases (five first-switch analyzer cases, thirteen
  front-door analyzer cases, five original app pulse-export cases). Every case
  passed automatic discovery and reuse of its group's first learned profile.
  Analyzer cases use the encoded pulse patterns embedded in the supplied logs;
  these are quantized representations, not original IQ. One case combines two
  successive packets from one analyzer burst; it is not an independent press.
- Full production app regression using those private recordings: three separately
  armed software observations per switch, explicit save, disabled actions,
  database reopen, two concurrent profiles, and correct local HTTP routing with
  repeated events 100 ms apart. No real Home Assistant URL was used.
- Synthetic regression covers 12/16-bit discovery, jitter, another full code,
  incomplete rows, conflicting full codes, competing repeated widths, strict
  FSK exclusion and preservation of existing PCM/PPM behavior.
- Full Go race suite and vet, four frontend controller checks, deployment/history
  checks, five updater tests, and four checksum-pinned upstream IQ examples
  (EV1527, PT2262, Akhan, HT680) passed. Module verification passed.
- Optional full Chromium harness was not run: the browser download returned an
  invalid/truncated archive. The browser controller checks above did run.
- Private RF files and identifiers are excluded from Git, history and packages.
  Existing third-party notices are reused: go.mod, go.sum and Go 1.27.3 are unchanged.

Operator acceptance still required: install this version, learn each physical
switch in the GUI, verify saved matching and deliberately test its real webhook.
Physical USB recovery and the not-yet-arrived Amazon remote remain untested.

Private regressions can be repeated with `RFB_PRIVATE_CAPTURE_DIR` pointing to a
local directory containing `regression.json`: an array of objects with `Name`,
`Code`, and `Files` (relative `.ook` recording paths). The app integration test
uses the first two groups and the first three files in each. Never add this private
directory or manifest to a public commit or release.


## 0.3.2-dev: actual recording reproduces a missing PCM learning path

- Private operator export: 52 complete packets, two OOK and 50 FSK; no decoded
  events. One OOK packet yields two identical complete rows with upstream PCM.
- Regression run against 0.3.1 commit b8c88a6: zero accepted candidates (expected
  failure). The same unchanged recording passes on this version with rtl_433
  23.11, including exact-profile replay. The raw recording and RF identifiers are
  excluded from this repository and all packages.
- Synthetic NRZ fixture uses independently chosen data. Tests cover timing jitter,
  incomplete fragments, strict same-length identity, conflicting repeated lengths,
  irregular timing, different complete frame widths, FSK exclusion and bounded
  short-traffic rejection.
- Application integration: three separately armed **software** observations,
  duplicate-mapping rejection, atomic disabled save, database reopen into a new
  application, normal upstream decode matching and exactly one local test webhook.
- Full race-enabled Go suite, vet, Node controller checks, deployment/history and
  prebuilt-updater tests pass. Four checksum-pinned public IQ recordings also pass.
- Ten headless Chromium integration checks pass, including learning controls and
  capture export; RF states in browser tests are mocked.
- No live ZORIN deployment, new physical press, actual Home Assistant webhook or
  USB recovery test is claimed by this development run.

The original pulse export is not raw IQ. Unmodified stock text-file replay cannot
correctly validate exported FSK packets; the source loader drops the modulation
marker. An isolated diagnostic loader that preserved it also produced no built-in
FSK decode. That observation does not identify those transmissions as noise or
as the intended remote. No modified rtl_433 binary is distributed.


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
