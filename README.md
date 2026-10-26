# RF433 Button Manager

**Turn ordinary 433 MHz wireless buttons into controls for Home Assistant.**

Press a wall switch or remote button to turn on a light, activate a scene, or run
another automation you choose. RF433 Button Manager listens for supported **433.92 MHz**
signals through a USB RTL-SDR receiver and sends a webhook when it recognizes a
button you have assigned.

A webhook is simply a special web address that starts an automation when the app
calls it. You choose what happens in Home Assistant; RF433 Button Manager connects the
physical button press to that automation.

> [!NOTE]
> **Linux requirements and testing status**
>
> This AI-assisted ("vibe coded") app targets **Linux x86_64 with systemd, udev and rtl_433**. It was originally developed for a Zorin OS server; **Zorin itself is not a requirement**.
>
> Physical testing so far covers **Zorin OS 18.1**, one RTL2838/R820T receiver, rtl_433 23.11 and three home buttons with working webhooks. Other Linux distributions and hardware combinations remain unverified. Ubuntu CI runs automated software tests, not physical receiver tests. This remains a development prerelease.
>
> Legacy names such as `scripts/inspect-zorin.sh` reflect the original setup. That script performs general Linux checks. Use the filename included with your download; generic script names are planned for a future release.

## Why this exists

An inexpensive wireless button can be useful even when it has no direct Home
Assistant integration. This project brings receiving, learning and assigning
supported buttons into one browser interface, so you do not need to maintain a
separate Python script for each remote.

Install it on an always-on Linux computer, connect a compatible RTL-SDR USB receiver
and antenna, then configure your buttons from a PC or phone. The service keeps
listening and sending assigned webhooks after you close the browser. No MQTT broker,
Docker container or cloud account is required.

## How it works

1. **Listen:** the RTL-SDR receives the radio signal from your button.
2. **Decode:** the open-source [rtl_433](https://github.com/merbanan/rtl_433) decoder
   turns supported signals into data. RF433 Button Manager manages reception and helps
   discover supported fixed-code signals.
3. **Learn:** choose a button in the web interface, select **Learn**, and follow the
   prompts for three separate matching presses. Confirm the result.
4. **Assign:** paste a webhook URL and save the action for that button.
5. **Use:** enable the action. A recognized press calls the webhook, and Home
   Assistant runs your automation. The event monitor shows reception and request results.

## What must be installed, and where?

**You must install two separate programs on the Linux computer where you plug in
the USB RTL-SDR receiver:**

1. **Install `rtl_433` first.** This free program reads the radio signal from the
   receiver and converts supported signals into data the app can recognize. It
   does not provide our button-management interface. RF433 Button Manager does
   **not** include or install it for you. Follow the
   [decoder installation steps](docs/GETTING-STARTED.md#install-the-required-decoder).
2. **Then install RF433 Button Manager.** Download our application ZIP and follow
   the [first-installation guide](docs/GETTING-STARTED.md). This is the app that
   displays your buttons in a browser, remembers their signals and sends the
   requests that trigger Home Assistant. Once installed, it starts and manages
   `rtl_433` automatically; you do not start the decoder manually each time.

You also need **Home Assistant already installed and working**. Our installer does
not install Home Assistant. It can run on the same Linux computer or on another
computer on your home network. For example, you can keep Home Assistant on your
existing Raspberry Pi and connect the receiver to a separate Linux x86_64 mini PC
running the two programs above. You do not move your existing Home Assistant setup.

On the PC or phone you use to configure everything, open a web browser. If the
receiver is connected to a separate Linux computer, you do not install either
program on your Windows PC or phone. Initial Linux installation uses terminal
commands; everyday learning and webhook setup use the browser interface.

This connection needs **no extra software installed inside Home Assistant**.
Its built-in Webhook trigger is enough. A webhook is a private web address for an
automation: our app sends a request to that address when it recognizes your button.
No custom integration, add-on, HACS component, MQTT broker or Home Assistant access
token is required. The app's management token is a separate credential for logging
into our interface.

## Connect a button to Home Assistant

For example, use a wireless wall button to switch on your living-room lights:

1. Install the two programs above, open RF433 Button Manager and log in using its
   management token. Check that the receiver is connected.
2. In Home Assistant, create an automation: its **Webhook** trigger receives the
   request, and its action turns on your chosen light. Save and enable it.
3. In RF433 Button Manager, select a button and use **Learn signal**. Follow the
   prompts for three separate presses, then choose **Confirm and save**.
4. Copy the automation's **complete webhook URL** into **Webhook URL** in our app
   and select **POST**. Choose **Save button**, or **Save & allow destination** if
   asked to approve your Home Assistant address. Saving does not run the automation.
5. When ready for the light to turn on, choose **Test saved action** and confirm
   **Send test**. After a successful test, turn on **Enabled**, click **Save button**
   again, and press the physical button.

The [complete walkthrough](docs/GETTING-STARTED.md#learn-a-button-and-connect-home-assistant)
explains each screen, what to paste, what to do if you only have a webhook ID and
how to find the cause if the light does not turn on. This method triggers an
automation; it does not automatically add your RF button as a Home Assistant
device or entity.

RF433 Button Manager runs as a separate Linux service and connects to Home Assistant over
the network. It does not need to run on the Home Assistant machine. You create and
manage the automation in Home Assistant; the app sends its trigger. It can also
call other services that accept HTTP webhooks.

Keep the webhook URL private. See [Home Assistant's webhook guide](https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger)
for trigger settings and [security guidance](SECURITY.md) for this app.

## What you need

- An always-on **Linux x86_64** computer; the supplied installer uses systemd and udev.
- A compatible **RTL-SDR USB receiver** and antenna, with `rtl_433` installed.
- A supported **433.92 MHz wireless button or remote**.
- Home Assistant with a webhook automation, or another service with an HTTP webhook.
- A browser for setup and management.

**Compatibility depends on the remote's signal format.** The frequency alone does
not guarantee support. Use **Inspect signal** to investigate a new button before
assigning it. The app receives radio signals; it cannot transmit commands back to
RF devices. See the [compatibility and inspection guide](docs/COMPATIBILITY.md).

See the [test report](docs/TEST-REPORT.md) and [hardware test plan](docs/HARDWARE-TESTS.md) for recorded evidence and remaining checks.

[Download 0.3.8-dev](https://github.com/ninoleto/rf433-button-manager/releases/tag/v0.3.8-dev)
· [Install or update: step-by-step guide](docs/GETTING-STARTED.md)
· [Find your login token](#first-login-where-is-my-token)
· [Developer guide](docs/DEVELOPMENT.md)
· [Instructions for AI coding agents](AGENTS.md)

Previously called **RF Button Manager**. The interface and new ZIP filename now use
**RF433 Button Manager**. The extracted `rf-button-manager` folder, executable,
service and data paths keep their old names so existing installations continue to
work. Updating preserves buttons, webhooks, receiver settings and the management
token; no data migration is needed.

## First login: where is my token?

The app creates its own login token on the Linux server during first startup.
Open a terminal **on that server** (or connect to it by SSH) and run:

```sh
sudo cat /var/lib/rf-button-manager/management.token
```

Copy the text it prints into **Management token** on the login screen, then select
**Connect**. It is **not** a Home Assistant access token or webhook ID. Keep it
private. Restarting or updating the app preserves it; you can run the same command
again whenever you need it. The login screen also shows these instructions.

If you run directly from source with a custom `--data-dir`, the token is in that
folder instead. For the source-run example below: `cat rf-data/management.token`.
If the installed file is missing, check that installation completed and the service
is running; do not create an empty token file or disable authentication.

## AI-assisted development ("vibe coded")

This community project was built iteratively with AI coding agents and hands-on
testing by its owner. That is what we mean by **vibe coded**. It is shared so others
can inspect, improve and fork it. Automated checks and the limited physical tests
are documented; this is not an independently audited or universally validated
product. Bug reports, code review and hardware test results are welcome.

## For developers and AI coding agents

Working on this 433 MHz / RTL-SDR / rtl_433 / Home Assistant project? Start with
[AGENTS.md](AGENTS.md), the [developer guide](docs/DEVELOPMENT.md),
[architecture](docs/ARCHITECTURE.md) and [contribution guide](CONTRIBUTING.md). They
explain how to build, test and extend the app, preserve existing data and report
what has actually been verified. Public Markdown is available to readers and
search tools; search-engine indexing is not guaranteed.

![Running application in explicitly labeled demo mode](docs/screenshots/dashboard.png)

[Mobile interface preview](docs/screenshots/mobile.png). These historical 0.1.0-dev
screenshots use synthetic demo data; they illustrate the interface but do not show
every current control or establish physical receiver behavior.

## What it provides

- A six-button remote layout, plus additional remotes and custom button counts.
- Learn/relearn with three separately armed matching presses and final confirmation.
- Inspect signal without creating a remote: display the decoded identifier,
  compare three presses, and download an RF-only inspection report/capture.
- Global webhook suppression during learning, duplicate-mapping checks and debounce.
- Per-button GET/POST webhooks, headers, JSON payload, timeout and explicit test action.
- Receiver state and recent signal/action events, light/dark themes and mobile layout.
- Persistent SQLite configuration and authenticated configuration backup/restore.
- Native supervision of `rtl_433`, USB rediscovery and bounded restart backoff.
- Loopback binding and a management token by default.

Use **Settings → Receiver device** to choose one connected RTL-SDR by unique serial.
The dashboard identifies the active tuner and shows decoded RF activity. See
[receiver selection and limitations](docs/RECEIVERS.md) for multiple tuners,
duplicate serials and preservation of existing mappings.

Learning is entirely browser-driven: choose **Learn**, press/release the button,
verify three separate presses, then **Confirm and save**. Button events decoded by rtl_433 are learned directly, including common
EV1527, PT2262 and HT680 event formats. For supported 12–64 bit fixed-code OOK PWM/PPM/PCM signals, the application captures
pulses, derives measured timing candidates, and validates them with rtl_433 replay.
A suggestion alone is never saved. Unrecognized traffic is ignored within a bounded press window. Rolling/encrypted
codes and ambiguous or changing identities cannot be saved as fixed buttons. This is
not universal RF learning. See [the learning workflow](docs/LEARNING.md).

## Requirements and supplied binary

The supplied package targets Linux x86_64. Native installation needs systemd,
udev, the existing `rtl_433` executable (initial target 23.11), and a supported
RTL-SDR and a remote using a supported signal format. Runtime state belongs on
reliable internal storage. A browser is the management client; no Node server,
Go compiler, Docker container or MQTT broker is required to install the binary.

The release package is `rf433-button-manager-0.3.8-dev-linux-amd64.zip`, available
from the release page. Download its matching `.zip.sha256` asset as well, then
verify before extracting into a fresh directory:

```sh
sha256sum -c rf433-button-manager-0.3.8-dev-linux-amd64.zip.sha256
rfb_extract_dir=$(mktemp -d "$HOME/rf433-button-manager.XXXXXX")
unzip rf433-button-manager-0.3.8-dev-linux-amd64.zip -d "$rfb_extract_dir"
cd "$rfb_extract_dir/rf-button-manager"
```

Then verify the package and review the installation plan:

```sh
sha256sum -c SHA256SUMS
test -x bin/rf-button-manager || chmod u+x bin/rf-button-manager
./bin/rf-button-manager --version
bash scripts/inspect-zorin.sh
bash scripts/install.sh --binary "$PWD/bin/rf-button-manager"
```

The packaged `SHA256SUMS` covers the binary and included update artifacts; the outer archive checksum covers the
complete package. Review the dry run and receiver ownership, then follow
[installation](#install-on-zorin). Use the supplied executable; rebuilding is only
needed for development or an intentional source change. To attach bundled history
without overwriting extracted source, follow the [developer guide](docs/DEVELOPMENT.md).

## Update an existing installation with a prebuilt package

Extract the new archive to a fresh directory on the internal disk. Close learning
and leave actions disabled in the GUI, then run from the extracted project:

```sh
sha256sum -c SHA256SUMS
sudo python3 scripts/update-prebuilt.py --apply
```

No Go installation or rebuild is involved. The helper snapshots stopped state and
the old executable, replaces only the application executable, checks the **live**
HTTP version and 20 seconds of stable operation, and verifies authentication and
configuration/token/environment preservation. It restores the old binary and state
if verification fails. Refresh the browser, check the live version and receiver,
then re-enable the button actions you want to use. It does not modify the service unit, USB rule, firewall or
other services. Full source and history are included separately in the bundle;
this update does not overwrite your existing development checkout.

## Build and verify from source

Build on Linux with **Go 1.27.3**, Git, Make, Bash and Python 3.12. The frontend controller tests
in `make check` additionally use Node.js 20 or later (no npm packages). The embedded frontend needs no Node
build or network-hosted assets. Tests with the race detector require a C compiler;
the release executable uses pure-Go SQLite and is built with `CGO_ENABLED=0`.

```sh
git clone https://github.com/ninoleto/rf433-button-manager.git
cd rf433-button-manager
export GOTOOLCHAIN=local
go mod download
go mod verify
make check
make build
./bin/rf-button-manager --version
```

`rtl_433` is also needed for the offline pulse replay integration tests (CI installs
it). Those tests never open USB; without it they report a skip.

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
rtl_433 uses its default protocols. Learn discovers supported unknown PWM/PPM/PCM
profiles automatically; a connected receiver alone does not prove a button decoded.

For an interface preview without radio hardware, use a **separate** data directory:

```sh
./bin/rf-button-manager --demo --data-dir ./demo-data
```

Demo mode explicitly labels simulated receiver data, generates synthetic
`Switch433` events every five seconds and disables all real webhook execution.
It cannot verify your radio, decoder or Home Assistant installation.

1. Check receiver status. Select its unique serial in Settings if necessary.
   Normal learning does not require custom decoder settings.
2. Add a remote with six buttons and two columns.
3. Select **A ON**, choose **Learn**, wait for the receiver-ready instruction,
   and briefly press/release the matching physical button once.
4. Inspect the received candidate. Release the button, arm the next capture, and
   repeat until three separate presses match. Confirm and save the mapping and
   validated profile. Keep actions disabled until deliberate action testing.
5. Paste the complete URL copied from Home Assistant into **Webhook URL**.
   If you only have an ID, expand the optional Home Assistant helper; it accepts
   either an ID with a base URL or a complete URL.
6. Click **Save button**, or **Save & allow destination** for a new server address.
   The editor shows the exact address being allowed. Saving does not send a test
   request; the button and destination permission are saved together.
7. Use **Test saved action** deliberately, then enable the button and test a physical press.

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

The installer creates only RF433 Button Manager's service, service account, state
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
- [Project rules for coding agents](AGENTS.md)
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
