# RF Button Manager

Receive 433.92 MHz remote-control buttons with a native Linux service, learn decoded
signals in a browser, and assign each button an HTTP webhook. Reception and actions
continue when the browser is closed. No MQTT broker, Docker container or cloud
account is required. This application receives signals; it does not transmit RF or
reprogram remotes.

**Development build.** Automated software tests do not establish physical USB
reliability. The ZORIN receiver, its exact `Switch433` decoder and Home Assistant
integration must pass the [hardware acceptance matrix](docs/HARDWARE-TESTS.md)
before this build can be called a production release.

![Running application in explicitly labeled demo mode](docs/screenshots/dashboard.png)

[Mobile interface preview](docs/screenshots/mobile.png). These screenshots use
synthetic RF data and demonstrate the actual application, not physical receiver tests.

## What it provides

- A six-button remote layout, plus additional remotes and custom button counts.
- Learn/relearn with three separately armed matching presses and final confirmation.
- Global webhook suppression during learning, duplicate-mapping checks and debounce.
- Per-button GET/POST webhooks, headers, JSON payload, timeout and explicit test action.
- Receiver state and recent signal/action events, light/dark themes and mobile layout.
- Persistent SQLite configuration and authenticated configuration backup/restore.
- Native supervision of `rtl_433`, USB rediscovery and bounded restart backoff.
- Loopback binding and a management token by default.

Learning requires a working `rtl_433` decoder for the remote. Unsupported protocols,
rolling codes and unstable frames cannot be learned reliably by simply pressing a
button. The existing `Switch433` flex specification must be copied from the verified
ZORIN setup; analyzer guesses are not treated as a working configuration.

## Requirements and supplied binary

The supplied package targets Linux x86_64. Native installation needs systemd,
udev, the existing `rtl_433` executable (initial target 23.11), and a supported
RTL-SDR with a verified decoder for the intended remote. Runtime state belongs on
reliable internal storage. A browser is the management client; no Node server,
Go compiler, Docker container or MQTT broker is required to install the binary.

After verifying the outer archive's published SHA-256 and extracting into a fresh
directory, enter `rf-button-manager` and run:

```sh
sha256sum -c SHA256SUMS
test -x bin/rf-button-manager || chmod u+x bin/rf-button-manager
./bin/rf-button-manager --version
bash scripts/inspect-zorin.sh
bash scripts/install.sh --binary "$PWD/bin/rf-button-manager"
```

The packaged `SHA256SUMS` covers the binary; the outer archive checksum covers the
complete package. Review the dry run and receiver ownership, then follow
[installation](#install-on-zorin). Use the supplied executable; rebuilding is only
needed for development or an intentional source change. To attach bundled history
without overwriting extracted source, follow the [developer guide](docs/DEVELOPMENT.md).

## Build and verify from source

Build on Linux with **Go 1.27.3**, Git and Make. The embedded frontend needs no Node
build or network-hosted assets. Tests with the race detector require a C compiler;
the release executable uses pure-Go SQLite and is built with `CGO_ENABLED=0`.

```sh
go mod download
go mod verify
make check
make build
./bin/rf-button-manager --version
```

`make test` runs uncached race-enabled tests with a two-minute timeout per package.
Use `make test TEST_PACKAGES=./internal/receiver` for a targeted check, or override
`TEST_FLAGS` deliberately. `make test-deploy` verifies shell syntax and dry-run
behavior without installing anything. These checks do not use production data or
establish physical receiver behavior.

`go.mod` and `go.sum` pin the dependency graph and checksums. `make build` uses a
fixed target (`linux/amd64`), strips machine paths/build IDs and excludes VCS metadata.
Reproduce a binary with the same commit, toolchain and command; verify it with
`sha256sum bin/rf-button-manager`. Dependency downloads require internet access on
the build machine. Runtime decoding still uses an installed `rtl_433` executable.

## First local run

```sh
./bin/rf-button-manager --data-dir ./rf-data --listen 127.0.0.1:8787 --rtl433 /usr/bin/rtl_433
```

Open `http://127.0.0.1:8787` and enter the token from
`rf-data/management.token`. Keep the token private. A missing receiver is reported;
demo data is never substituted automatically. Without explicit decoder settings,
rtl_433 uses its default protocols, which does not prove Switch433 support.

For an interface preview without radio hardware, use a **separate** data directory:

```sh
./bin/rf-button-manager --demo --data-dir ./demo-data
```

Demo mode explicitly labels simulated receiver data, generates synthetic
`Switch433` events every five seconds and disables all real webhook execution.
It cannot verify your radio, decoder or Home Assistant installation.

1. Open Settings, select the receiver serial if available, and enter the exact
   verified flex decoder or supported protocol IDs. Save and check receiver status.
2. Add a remote with six buttons and two columns.
3. Select **A ON**, choose **Learn**, and press the matching physical button once.
4. Inspect the received candidate. Release the button, arm the next capture, and
   repeat until three separate presses match. Confirm the mapping.
5. Add the endpoint's origin to Settings, for example `http://127.0.0.1:8123`.
6. Save the button webhook, normally a POST to
   `http://127.0.0.1:8123/api/webhook/YOUR_PRIVATE_WEBHOOK_ID`.
7. Use **Test webhook** deliberately, then enable the button and test a physical press.

Home Assistant must already contain the intended webhook automation. This app does
not create or change Home Assistant automations. Home Assistant enables POST by
default; GET requires explicit `allowed_methods` configuration there. Treat the
webhook ID as a secret. Use state-setting actions where possible; a network timeout
cannot tell whether an action executed, so this app does not automatically retry it.
At most eight requests run concurrently across different buttons, and each button
can have only one pending request. Extra presses while that button or the global
limit is busy are rejected and logged; they are not queued for delayed execution
or replayed after restart.

## Install on ZORIN

First review the [operations guide](docs/OPERATIONS.md) and run the read-only
inspection script. Do not change the production server until the intended changes
have been reviewed and approved.

```sh
bash scripts/inspect-zorin.sh
bash scripts/install.sh --binary "$PWD/bin/rf-button-manager"
# Only after reviewing the dry-run plan and approving deployment:
sudo bash scripts/install.sh --binary "$PWD/bin/rf-button-manager" --apply
```

The installer creates only RF Button Manager's service, service account, state
directory, executable and narrowly scoped USB permissions. It does not install
packages, modify kernel modules, stop competing receiver services, or change Docker
and Home Assistant. If initial USB permissions have not applied, arrange a
receiver-only reconnect with its operator. Never reset hubs or other devices.

From Windows, keep the default private listener and forward it through SSH:

```powershell
ssh -N -L 8787:127.0.0.1:8787 nino@ZORIN
```

Then open `http://127.0.0.1:8787` in the Windows browser. Retrieve the login token
on ZORIN with `sudo cat /var/lib/rf-button-manager/management.token`.
For direct private-network access, see the explicit binding and HTTPS guidance in
the operations guide. No public exposure is configured by default.
An HTTPS reverse proxy also requires the exact trusted browser origin through
`--public-origin https://rf.home.example` (replace the example with your own origin),
or `RFB_PUBLIC_ORIGIN` in the installed environment file. Without that setting,
strict same-origin checks reject an HTTPS frontend backed by loopback HTTP.

## Project documentation

- [Architecture and technical risks](docs/ARCHITECTURE.md)
- [Receiver configuration and three-press learning](docs/CONFIGURATION.md)
- [API contract and authenticated examples](docs/API.md)
- [Installation, upgrades, backups and troubleshooting](docs/OPERATIONS.md)
- [Physical hardware release gate](docs/HARDWARE-TESTS.md)
- [Persistent implementation progress](docs/PROGRESS.md)
- [Clean builds and developer extension examples](docs/DEVELOPMENT.md)
- [Contributing and forking](CONTRIBUTING.md)
- [Security and private configuration](SECURITY.md)
- [Publication and release procedures](docs/RELEASING.md)

## License and dependencies

Application code: [MIT](LICENSE), copyright Nino Milin. `rtl_433` is a separately
installed project with its own license and source at
<https://github.com/merbanan/rtl_433>. Go dependencies retain their own licenses;
run `python3 scripts/collect-notices.py` when packaging a binary to collect their
license texts and the Go standard library notices into
`dist/THIRD_PARTY_NOTICES.txt`. Include that file and the application's `LICENSE`
beside every distributed executable.

Sources: [rtl_433 operation](https://triq.org/rtl_433/OPERATION.html),
[Home Assistant webhook triggers](https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger),
[modernc SQLite](https://pkg.go.dev/modernc.org/sqlite).
