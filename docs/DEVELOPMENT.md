# Developer guide

## Requirements

Runtime: Linux x86_64 for the supplied binary, systemd/udev for the supplied
installer, an independently installed `/usr/bin/rtl_433` (initial integration
target 23.11), a compatible RTL-SDR, antenna and supported 433.92 MHz remote, USB
permissions, and a browser for management. The browser can derive supported fixed-code OOK PWM/PPM/PCM profiles internally;
other protocols need a working existing decoder or an explicit developer extension. No Go, Node, Docker, MQTT or cloud account is needed to
install the packaged binary. Keep state on reliable internal storage with space
for SQLite/WAL and retained backups. No minimum RAM/CPU specification or power-loss
reliability has been established by hardware testing.

Source development: **Go 1.27.3**, Git, GNU Make and Bash on Linux; a C compiler
for `-race` tests; Python 3.12 for checks, packaging and license collection. The pure-Go SQLite
release build uses `CGO_ENABLED=0` and needs no SQLite development package.
Internet access is needed for initial toolchain/module downloads; a verified
populated cache permits later offline builds. Allow several GB for toolchain,
modules, build cache and race instrumentation. Node.js 20+ is required by
`make check` for syntax/controller tests; Playwright/Chromium are only needed for
the optional browser suite. Neither is a frontend compilation step. Ubuntu 24.04
provides the reference Git 2.43, Make 4.3, Bash 5.2 and GCC 13 environment. Python
3.12 supplies the tar extraction filter used by the release packager.

## Recover a source package's history

A package can contain source files but no `.git`, plus
`rf433-button-manager.git.bundle`. Prefer a new checkout, preserving the extracted
package and any older development work:

```sh
git clone --branch main /path/to/rf433-button-manager/rf433-button-manager.git.bundle ./rf433-button-manager-dev
cd rf433-button-manager-dev
git bundle verify /path/to/rf433-button-manager/rf433-button-manager.git.bundle
git log --oneline --all --decorate
git status --short
```

For every package, compare `git rev-parse HEAD` with its `release.json` commit,
and the built `--version` with its manifest version. The corresponding release
tag must identify that same source; `main` may contain later work.

Historical reference: for the 0.3.6-dev ZIP, HEAD must be
`f9b496f3387cd357c1270c28566c97d26cb48dd8`; compare with the package's
`release.json`. The bundle contains 20 commits through that version. The GitHub
tag `v0.3.6-dev` must identify that same source; `main` may contain later documentation.

Alternatively, to attach history to an extracted source tree without checking out
over local edits, run from that extracted directory:

```sh
bash scripts/restore-history.sh
git log --oneline --all --decorate
git status --short
```

The helper verifies the self-contained bundle, creates repository metadata,
imports branches under `refs/remotes/bundle/` and tags, attaches `main`, and reads
its tree into the index **without checking out files**. Modified and untracked
workspace files remain intact. It refuses an existing `.git` instead of changing
someone's branch/index. If a repository already exists, inspect its status and
history, then import the bundle into a new remote namespace manually; integrate
only after reviewing divergence. Bundles and packaged binary/checksum/notices
files are intentionally ignored in the source repository.

## Reproducible clean build

Use the exact pinned toolchain. `go.mod` requires Go 1.27.0 or newer, but the
documented release and CI toolchain is 1.27.3. Get its archive and SHA-256 from
the official [Go downloads](https://go.dev/dl/) and verify before use. A private
temporary toolchain works; installing it system-wide is unnecessary. Do not change
the runtime server's packages merely to install a supplied executable.

From a Git checkout with desired changes committed, create a separate clean clone:

```sh
rfb_build_root=$(mktemp -d)
git clone --no-local "$PWD" "$rfb_build_root/source"
cd "$rfb_build_root/source"
git status --porcelain
go version
export GOTOOLCHAIN=local
go mod download
go mod verify
test -z "$(gofmt -l cmd internal)"
make check
make build
./bin/rf433-button-manager --version
sha256sum bin/rf433-button-manager
```

The status must be empty before building. Keep the command output and commit SHA
with your validation record. `make build` fixes `GOOS=linux`, `GOARCH=amd64`,
`CGO_ENABLED=0`, uses `-mod=readonly -trimpath -buildvcs=false`, strips debug data
and clears the build ID. No timestamp or local path is inserted. Compare SHA-256
from two independent clean clones using the same commit, exact toolchain,
dependency checksums and build command. Reproducibility is an observed equality,
not a guarantee across arbitrary toolchains or environments.

`go.mod` and `go.sum` are versioned inputs. Do not run `go get -u` or `go mod tidy`
as an installation step. Intentional dependency changes need their own review,
tests, license review and newly recorded binary hashes. If module network access
is restricted, prepopulate a verified cache using an approved download path; do
not weaken checksum validation. A socket-denied test environment must be reported
as blocked, not counted as passing networking tests.

## Fast edit/test cycle

```sh
gofmt -w cmd internal
make test TEST_PACKAGES=./internal/receiver
make vet
make test-deploy
make build
```

`make test` uses `-race -count=1 -timeout=2m`; override `TEST_PACKAGES` to target a
package. `make check` runs tests, vet, Node controller tests and deployment/history/updater checks. The
Go tests use temporary databases and fake receiver/process fixtures. Webhook
tests use local ephemeral HTTP servers and require loopback socket permissions.
They do not contact Home Assistant or acquire a real receiver. Never point a test
at production state or enable real actions to make a software test pass.

For an interactive preview, use a fresh isolated directory and non-production port:

```sh
rfb_demo_dir=$(mktemp -d)
./bin/rf433-button-manager --demo --data-dir "$rfb_demo_dir" --listen 127.0.0.1:18788
```

Read its token privately from `$rfb_demo_dir/management.token`. Demo emits one
synthetic `Demo-Switch433` signal every five seconds and blocks real webhook
execution. It demonstrates GUI/learning behavior, not RF decoding or hardware.

### Optional browser integration

Use an isolated npm directory so Node dependencies do not enter the source tree:

```sh
rfb_ui_tools=$(mktemp -d)
npm install --prefix "$rfb_ui_tools" --save-exact playwright@1.56.1
"$rfb_ui_tools/node_modules/.bin/playwright" install chromium
CODEX_PRIMARY_RUNTIME_NODE_MODULES="$rfb_ui_tools/node_modules" \
  RFB_UI_BINARY="$PWD/bin/rf433-button-manager" \
  node internal/webui/ui.test.cjs
```

The version above is a pinned optional harness dependency, independent of the
application runtime. Check the harness against the chosen browser version when
upgrading it. `RFB_UI_BROWSER` can select an existing compatible Chromium binary;
`RFB_UI_PORT` overrides 18788; the network-cancel scenario also temporarily uses
18789 on loopback. Run it where these test ports are free. The harness uses a
touch-capable Chromium context and desktop/tablet/phone viewports; this is
emulation, not a physical Android/iOS test. Browser libraries must already be available; any
host package installation is a separate operation. The harness uses disposable
`.qa` state, synthetic signals and screenshots; inspect artifacts before sharing.
It is not part of the default CI workflow. Record whether you actually ran it.

## Extension example: support a new decoder JSON shape

Suppose a verified decoder produces synthetic-shaped data like:

```json
{"model":"ExampleRemote","address":42,"action":"on","rssi":-31}
```

No parser change is needed: choose `address,action` as that button's identity
fields, because these already satisfy device/command requirements. Prove it with
a sanitized regression test in `internal/app/model_test.go`:

```go
func TestExampleAddressAction(t *testing.T) {
    got, err := Signature([]byte(`{"model":"ExampleRemote","address":42,"action":"on","rssi":-31}`), []string{"address", "action"})
    if err != nil {
        t.Fatal(err)
    }
    want := `{"action":"on","address":42,"model":"ExampleRemote"}`
    if got != want {
        t.Fatalf("got %s; want %s", got, want)
    }
}
```

Also test repeated frames, the OFF command, a different device, large integer
IDs, changing metadata and ambiguous inputs. For a genuinely new field role,
change `Signature`'s device/action classification deliberately, with negative
tests. Do not accept arbitrary model-only or volatile identities. Existing
`codes` normalization intentionally rejects multiple distinct codes. Real raw
samples must be sanitized before adding fixtures; report their provenance and
which physical behavior was actually observed.

## Extension example: add a receive-frequency setting

Frequency is currently fixed in `receiver.arguments`, not an existing setting.
To make it configurable:

1. Add a typed field to `app.Settings` and a defined default. Plan backward
   compatibility for current schema-2 configurations and schema-1 imports rather
   than interpreting an absent field as a valid frequency.
2. Validate range/units in `validateSettings`; reject malformed values before
   persistence. Update backup/restore and migration tests.
3. Pass the value through `main.runReceiver` into `receiver.Config`. Render it
   as one safe argv element; never interpolate a shell command.
4. Add a labeled GUI control, full-object API examples, argument tests and a
   configuration-persistence test. Verify it triggers exactly one child reload.
5. Test actual acquisition on supported hardware separately. Update release notes
   and any schema compatibility/rollback requirements.

This is a design example, not a claim that the current binary supports other
frequencies. The same path applies to other receiver settings.

## Extension example: UI-only changes

Edit `internal/webui/static/` and rebuild to refresh embedded assets. Keep untrusted
labels and RF JSON in `textContent`, preserve CSP compatibility, accessible labels,
focus and mobile layout, and preserve unsaved edits across one-second state polls.
Use the demo browser harness for changed behavior; no frontend bundle/CDN is needed.

Adding a new action type is a broader design change: update models, validation,
API/UI, backup schema and tests together. Preserve learning suppression, bounded
concurrency, no stale replay, redacted diagnostics and explicit outcome ambiguity.
Do not silently introduce retry semantics for physical actions.

## Build and release references

[ARCHITECTURE.md](ARCHITECTURE.md) maps code and concurrency;
[API.md](API.md) defines the current wire contract;
[CONTRIBUTING.md](../CONTRIBUTING.md) covers forks and review;
[RELEASING.md](RELEASING.md) covers public-history review and packaging;
[TEST-REPORT.md](TEST-REPORT.md) and [HARDWARE-TESTS.md](HARDWARE-TESTS.md) distinguish
executed software checks from outstanding physical gates.

## Extending browser capture learning

Read [LEARNING.md](LEARNING.md) first. Production capture uses the same live child
as normal reception and one offline worker; do not add a second USB reader. To add
a modulation, implement strict candidate parsing/framing and conservative identity
normalization in `internal/receiver`, add explicit persisted profile validation,
and supply synthetic plus operator-approved private replay tests. Do not broaden
the candidate parser to accept arbitrary analyzer output or shell commands.

Cover cancellation during analysis, independent timeouts, mixed captures, stale
receiver acknowledgements, duplicate mapping rejection, atomic profile/mapping
persistence and restoration before action dispatch. Add UI progress/failure states
and documentation for the precise supported range. Run `go test -race ./...` with
rtl_433 available; replay tests use file input and must never touch USB. The fixture
`internal/receiver/testdata/synthetic-pwm.ook` is artificial software-test data,
not evidence for a physical remote or a default decoder configuration.

`make test-ui` runs the real frontend controller against an in-memory DOM/API
adapter using Node.js 20+ and no external modules. It covers capture progress,
failure persistence, retry gating and confirmation. It does not establish real
browser layout; the separate Playwright suite remains the rendering check.

## Reproducing remote compatibility checks

Install upstream rtl_433 for file replay. The supported reference for these checks
is 23.11. `python3 scripts/fetch-rf-testdata.py /tmp/rfb-rf-tests` fetches four
checksum-pinned public IQ recordings; `RFB_RF_TESTDATA=/tmp/rfb-rf-tests make check`
runs the normal suite plus recorded-remote integration. No USB device is opened.
See TEST-REPORT.md for provenance, tested flow and the physical-test distinction.

To add an already-decoded remote format, add a narrowly justified identity alias
in `internal/app/decoded_learning.go`, plus positive and different-button tests.
Never identify a button by RSSI, timestamp or sensor measurements. For an unknown
pulse format, extend the measured profile parser and replay tests; do not insert
unverified timing constants into runtime configuration. `generic_learning_test.go`
shows separate-packet PPM repetition tests. Keep timeout, ambiguity, cancellation,
overlap and webhook-suppression tests passing when extending learning.

The package updater is deliberately independent of Git checkout state and Go.
`python3 scripts/test-update-prebuilt.py` tests dispatch and rollback with temporary
files and simulated systemd/API responses. It must fail if only the source or
package binary is updated while the HTTP service still reports an older version.


## 0.4 management, migration and private RF checks

`make check` includes the management unit tests and isolated Python updater/name
migration simulations. After `make build`, run `python3 scripts/test-management-runtime.py`
to check the actual demo executable's HTTP/HTTPS, trust, confirmation, rotation and
restart persistence. It opens only disposable loopback sockets; it uses no USB or
real webhooks. Use `RFB_MANAGEMENT_BINARY` to select a clean-build executable.

`RFB_WEAK_CAPTURE=/private/other-room-export.json go test -race -count=1 ./internal/app -run TestPrivateWeakCaptureBuiltinToWebhook -v`
is an optional, specific operator regression for the nine-burst recording. It
performs file-only rtl_433 replay, three-arm learning and local webhook counting.
The recording is not distributed; absence is a skip, not a pass. Test logs report
counts without identifiers. Never commit the private capture or create a fixture
by copying its identifiers.

For frontend changes run both Node controller tests and the optional Playwright
`internal/webui/ui.test.cjs` suite, then inspect real screen sizes and coarse-pointer
use. A DOM adapter cannot establish rendered layout. See TESTING-0.4.md for blocked
and pending acceptance checks.

For the post-test.1 exact decoder-equivalence regression, set
`RFB_EQUIVALENT_CAPTURE=/private/recording.json` and run
`go test -race -count=1 ./internal/app -run TestPrivateEquivalentCaptureToWebhook -v`.
This optional test replays raw OOK packets with `rtl_433 -r` and checks isolated
HTTP request counts. It then deliberately removes the native messages to test
the alternate-only dispatch case. That second scenario is a simulation, not a
claim that the unmodified recording contains an alternate-only physical failure.
The public synthetic tests cover each changed payload bit, unsupported shapes,
collision refusal, shared sliding debounce, additional selected identity fields
and action gates. Keep source decoder references pinned to rtl_433 23.11 when
extending this relation; JSON does not prove the original packet's full framing.

For the saved-Smoke/profile regression, set
`RFB_SMOKE_PROFILE_CAPTURE=/private/recording.json` and run
`go test -race -count=1 ./internal/app -run TestPrivateSavedSmokeProfileCapture -v`.
This specific three-burst recording test feeds recorded live JSON into the real
normalization/matching path. The export omits profile timing configuration, so its
custom model label is substituted with a registered synthetic 25-bit PWM profile;
payload bits, fragment rows, order and timestamps remain unchanged. Separate
filtered scenarios simulate missing custom or Smoke reports. They do not prove
the exact deployed profile's timing configuration or physical receive range.
