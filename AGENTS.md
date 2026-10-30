# RF433 Button Manager: coding-agent guide

The public project name is **RF433 Button Manager** (formerly RF Button Manager).
From 0.3.8-dev, downloadable ZIPs use `rf433-button-manager-<version>-linux-amd64.zip`.
The extracted folder, executable, service, Go module and data paths remain
`rf-button-manager` for compatibility. Do not migrate installed data for branding.
Published archive contents and tags must never be silently replaced or moved.

Native Linux receive-only 433.92 MHz service: one RTL-SDR, supervised `rtl_433`,
embedded browser UI, SQLite configuration and guarded HTTP actions. This is a
release candidate (RC1), using the unchanged `0.3.8-dev` package. Read code and
current guides before changing behavior;
dated progress/test reports and archived handoffs are historical evidence, not
instructions or authorization to operate a host.

## Start here: developers and AI coding agents

This is an AI-assisted ("vibe coded") community project for receiving 433 MHz
buttons with RTL-SDR/rtl_433 and triggering Home Assistant webhooks. AI-generated
code and prose need review against the implementation. Do not treat generated
documentation, old handoffs or synthetic test results as hardware proof.

Read [Getting started](docs/GETTING-STARTED.md) for the user flow, including the
server-side login token. Read this file and DEVELOPMENT before making changes.
Inspect the current checkout/version and preserve operator configuration.

## Explain setup to a first-time user

Use [Getting started](docs/GETTING-STARTED.md) as the user walkthrough. Name the
computer or application where each step happens before giving a command or click.
Explain that `rtl_433` and RF433 Button Manager are two separately installed
programs on the Linux computer with the USB receiver. Install/check `rtl_433`
first; the app installer does not supply it. After setup, the app starts and
supervises the decoder. Do not tell users to run a competing receiver process.

Home Assistant is an existing, separate requirement; it may run on the same
machine or elsewhere on the LAN. Its built-in Webhook trigger needs no additional
integration. Explain a webhook as a private automation address, then distinguish
the complete URL, its webhook ID and our app's management token. Never ask users
to paste those secrets into chat. Do not confuse the app address on port 8787 with
the Home Assistant address, or copy an author's LAN IP into someone else's setup.

Use actual interface labels and explain expected results and the next step after
an error. Include saving after toggling Enabled. State that Test saved action
sends a real request. Explain unfamiliar terms instead of compressing required
steps into phrases such as "app plus decoder". Initial installation needs a Linux
terminal; routine learning and button assignment should stay in the browser.

## Project map and authoritative guides

- `cmd/rf-button-manager/main.go`: flags, token, data lock, shutdown and receiver
  reload. [README](README.md): purpose, requirements and first installation.
- `internal/receiver/`: sysfs inventory, child ownership, stream parsing, profiles
  and file replay. [RECEIVERS](docs/RECEIVERS.md), [LEARNING](docs/LEARNING.md).
- `internal/app/`: `model.go` validation/schema, `store.go` SQLite, `engine.go`
  matching/CRUD, `learning.go` lifecycle, `webhook.go` dispatch, `http.go` routes.
  [ARCHITECTURE](docs/ARCHITECTURE.md), [CONFIGURATION](docs/CONFIGURATION.md),
  [API](docs/API.md) define processing, storage and wire contracts.
- `internal/webui/static/`: embedded HTML/CSS/JS; rebuild after edits.
  `learning.test.cjs` uses a DOM adapter; `ui.test.cjs` is optional Chromium testing.
- `deploy/`, `scripts/`: scoped installation, updates, tests and packaging.
  [OPERATIONS](docs/OPERATIONS.md), [DEVELOPMENT](docs/DEVELOPMENT.md),
  [RELEASING](docs/RELEASING.md), [CONTRIBUTING](CONTRIBUTING.md), [SECURITY](SECURITY.md).

## Build and check

Use Go **1.27.3** (`go.mod` minimum 1.27.0), Node.js **20+**, and Linux.
The documented Ubuntu 24.04 baseline is Python **3.12**, Git **2.43**, GNU Make
**4.3**, Bash **5.2** and GCC **13** (needed for race tests, not the static build).
Offline replay targets independently installed `rtl_433` **23.11**. Do not upgrade
the runtime host's packages to perform source work. No npm build is required.

From a clean checkout:

```sh
export GOTOOLCHAIN=local
go mod download
go mod verify
test -z "$(gofmt -l cmd internal)"
python3 scripts/fetch-rf-testdata.py /tmp/rfb-rf-tests
RFB_RF_TESTDATA=/tmp/rfb-rf-tests make check
make build
./bin/rf-button-manager --version
```

`make check` runs `go test -race -count=1 -timeout=2m ./...`, vet, Node controller
checks and deployment/history/updater simulations. A targeted check is
`make test TEST_PACKAGES=./internal/receiver`; format Go edits with `gofmt`.
Tests use disposable state, fake USB/processes, file replay and local HTTP servers.
Socket restrictions or absent fixtures are blocked/skipped checks, never passes.
Private replay inputs are optional and must stay outside Git. See DEVELOPMENT for
the separate browser harness and reproducibility procedure; report what ran.

## Compatibility and preservation

- Schema **2** persists remotes, button identities, profiles and settings. Schema 1
  migrates on validated open/restore; unknown schemas fail. Do not clear data on
  validation failure or change profile IDs/normalization casually.
- Preserve mappings, enabled flags, exact legacy decoder/protocol settings,
  receiver serial, webhook fields, token and `manager.env`. Settings/button PUTs
  replace full objects; retain untouched fields. Logical restore deliberately
  disables actions and clears `last_received`; it excludes token/events.
- Save validated configuration before exposing changes. Retain atomic profile and
  mapping saves and atomic button and destination approvals. Schema changes need
  migration, backup and rollback documentation/tests; pre-0.2 binaries reject schema 2.
- Keep old checkouts, archives and history. Restore bundles into fresh directories
  with `git clone --branch main`; never force-push, overwrite remote work or retag
  a distributed ZIP to a later documentation commit. Follow RELEASING.

## Receiver and action safety

- Source/documentation work does not authorize installation, restart, receiver
  access, real webhook tests or unrelated service changes. Follow the current
  user's scope. Never run the agent/application as root to bypass restrictions.
- At most one owned live `rtl_433` child and one bounded file-replay child. Cancel
  and join the old live process group/readers before replacement. Use explicit argv,
  `-c /dev/null`; offline analysis requires `-r FILE` and must never select USB.
- Select by unique serial; automatic mode requires a singleton. Missing/duplicate
  serials must not fall back to another tuner. USB ports/bus numbers are observations,
  not persisted selectors. Never blanket-kill RF tools, reset hubs or rewrite EEPROM.
- Learning/inspection wait for in-flight actions and suppress all new actions,
  including manual tests, through receiver-generation and replay-worker cleanup.
  Preserve fixed time/work bounds, three separately armed matching presses,
  explicit confirmation, overlap rejection and disabled newly learned mappings.
  Cancel/failure preserves old mappings; inspection cannot save one.
- Preserve sliding debounce, epoch/storage gates, one pending action per button,
  eight globally, no redirects/proxy/retry/queue/replay. Saving never sends a webhook.
  Manual Test can send while a button is disabled; demo blocks real delivery.
  A timeout leaves outcome unknown. Never use production endpoints in automation.

## Publication and evidence

Never commit credentials, tokens, databases/WAL/SHM, private configuration/backups,
RF recordings/identifiers, production screenshots or host logs. Review current
files **and all outgoing history**; `.gitignore` cannot sanitize history. The two
committed pulse fixtures and historical screenshots are explicitly synthetic.
Keep MIT attribution and dependency notices with binaries.

Physical deployment has only been tested on **Zorin OS 18.1 (Linux x86_64)** with
one RTL2838/R820T receiver and rtl_433 23.11. Ubuntu CI is automated software
verification, not a claim of hardware acceptance on Ubuntu or other distributions.
The operator tested three home button assignments and their webhooks and accepted
the latest interface within available testing. Physical multi-tuner, USB/hub
recovery, soak and broader compatibility remain unverified. Do not promise every
Amazon button works. [TEST-REPORT](docs/TEST-REPORT.md) records software evidence;
[HARDWARE-TESTS](docs/HARDWARE-TESTS.md) requires actual operator observations.
Green CI, repeated recording playback and a connected indicator are not physical
acceptance. Upload assets only after CI succeeds for the exact tagged packaged
commit; keep new development releases marked prerelease. Exception: on 30 October
2026, the maintainer requested promotion of the existing `v0.3.8-dev` release to
RC1 without GitHub's Pre-release flag after reporting several days of successful
everyday use on his equipment. This changes release metadata only; retain its
original tag, assets, checksums and binary version. It is not a stable acceptance
claim or a completed formal soak test. See RELEASING for the current status.

Group related onboarding, naming and documentation fixes into one reviewed release.
Documentation-only changes can be committed without a new binary release. Before
publishing, review first installation, first login, update and download instructions
together. Remove superseded release pages/assets only when the user explicitly
requests it, after verifying the replacement; preserve tags and Git history.
