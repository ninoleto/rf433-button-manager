# Developer guide

## Requirements

Runtime: Linux x86_64 for the supplied binary, systemd/udev for the supplied
installer, an independently installed `/usr/bin/rtl_433` (initial integration
target 23.11), a compatible RTL-SDR, antenna and supported 433.92 MHz remote, USB
permissions, and a browser for management. A verified decoder is required for
learning that remote. No Go, Node, Docker, MQTT or cloud account is needed to
install the packaged binary. Keep state on reliable internal storage with space
for SQLite/WAL and retained backups. No minimum RAM/CPU specification or power-loss
reliability has been established by hardware testing.

Source development: **Go 1.27.3**, Git, GNU Make and Bash on Linux; a C compiler
for `-race` tests; Python 3 for license collection and examples. The pure-Go SQLite
release build uses `CGO_ENABLED=0` and needs no SQLite development package.
Internet access is needed for initial toolchain/module downloads; a verified
populated cache permits later offline builds. Allow several GB for toolchain,
modules, build cache and race instrumentation. Node/Playwright/Chromium are only
needed for the optional browser suite, not frontend compilation.

## Recover a source package's history

A package can contain source files but no `.git`, plus
`rf-button-manager.git.bundle`. Do not clone over an extracted directory or use
`reset --hard` to attach history. From the extracted directory:

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
./bin/rf-button-manager --version
sha256sum bin/rf-button-manager
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
package. `make check` runs tests, vet and deployment/history shell checks. The
Go tests use temporary databases and fake receiver/process fixtures. Webhook
tests use local ephemeral HTTP servers and require loopback socket permissions.
They do not contact Home Assistant or acquire a real receiver. Never point a test
at production state or enable real actions to make a software test pass.

For an interactive preview, use a fresh isolated directory and non-production port:

```sh
rfb_demo_dir=$(mktemp -d)
./bin/rf-button-manager --demo --data-dir "$rfb_demo_dir" --listen 127.0.0.1:18788
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
  RFB_UI_BINARY="$PWD/bin/rf-button-manager" \
  node internal/webui/ui.test.cjs
```

The version above is a pinned optional harness dependency, independent of the
application runtime. Check the harness against the chosen browser version when
upgrading it. `RFB_UI_BROWSER` can select an existing compatible Chromium binary;
`RFB_UI_PORT` overrides 18788. Browser libraries must already be available; any
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
   compatibility for saved schema-1 configurations rather than interpreting an
   absent field as a valid frequency.
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
