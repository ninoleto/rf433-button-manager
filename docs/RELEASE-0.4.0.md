# RF433 Button Manager 0.4.0

**Turn 433 MHz wireless buttons and remotes into controls for Home Assistant.**

Press a wireless wall switch or remote button to turn on a light, activate a scene
or run another automation. RF433 Button Manager is an open-source Linux app that
listens for supported **433.92 MHz** signals through an **RTL-SDR USB receiver**
and `rtl_433`, then sends a webhook when it recognizes an assigned button.

A webhook is a private web address that starts an automation when the app calls
it. You choose what happens in Home Assistant; RF433 Button Manager connects the
physical button press to that automation. It can also call other services that
accept HTTP webhooks.

[First installation](GETTING-STARTED.md) · [Update guide](UPGRADING.md)
· [Button compatibility](COMPATIBILITY.md)

## Why use it?

An inexpensive wireless button can be useful even when it has no direct Home
Assistant integration. RF433 Button Manager brings receiving, inspecting,
learning and assigning supported buttons into one browser interface, so you do
not need to write and maintain your own receiver and webhook scripts.

Install it on an always-on Linux computer, connect a compatible receiver and
antenna, then manage your buttons from a PC or phone. The service keeps listening
and sending assigned webhooks after you close the browser. No MQTT broker,
Docker container or cloud account is required by RF433 Button Manager.

The open-source [rtl_433](https://github.com/merbanan/rtl_433) project decodes the
radio signals. This app adds the browser interface, button learning, saved
assignments, receiver management, diagnostics and webhook actions.

## What is new in 0.4.0?

- **Improved button recognition:** supported fixed-code signals can match the
  same saved button across verified equivalent decoder reports. A bounded framing
  correction also improves supported saved PWM profiles. Existing mappings are retained.
- **Reception diagnostics:** recognized-press counts, ignored repeat reports,
  signal observations and optional normal-use recordings. Actions continue while recording.
- **Network settings:** local HTTP and configurable access on a selected LAN/VPN
  address using HTTP or HTTPS, with connection testing, confirmation and rollback.
  Localhost remains available on the server. Certificate setup and token replacement
  are available in Settings.
- **Touch-friendly interface:** Classic and Unified desktop layouts, phone/tablet
  navigation, a light/dark switch, large receiver/address pickers and minus, plus
  and Reset controls.
- **Clearer action settings and time:** Method above Timeout on every screen,
  plus **24-hour local time**, such as `22:35`, throughout the interface.
- **Preserving updates:** the updater keeps mappings, webhooks, token, environment
  and enabled flags, including migration from the older installation name.

## What do I need to install, and where?

The supplied application is for **Linux x86_64** — a 64-bit Intel/AMD computer
with **systemd and udev**. It is not an ARM/Raspberry Pi application package.
You need a compatible RTL-SDR USB receiver, antenna and supported 433.92 MHz button.

**Install two separate programs on the Linux computer with the USB receiver:**

1. **Install `rtl_433` first.** Our ZIP does not contain it, and our installer does
   not install it. If it is missing on Debian/Ubuntu-based systems such as Zorin,
   run `sudo apt install rtl-433` on that computer. The package uses a hyphen
   (`rtl-433`); the installed executable uses an underscore (`rtl_433`). Follow the
   [decoder installation and checking steps](GETTING-STARTED.md#install-the-required-decoder).
   Keep an existing working installation; physical testing used version **23.11**.
2. **Then install RF433 Button Manager** on the same computer. It starts and
   manages `rtl_433` automatically after setup. You do not start another decoder
   process each time.

For Home Assistant actions, you also need **Home Assistant already running**.
It may be on the same computer or another machine on your network. Its built-in
**Webhook** trigger is enough: no extra integration, add-on, HACS component,
MQTT broker or Home Assistant access token is required for this connection.
The app does not automatically create RF devices or entities in Home Assistant.

Nothing needs to be installed on the Windows PC or phone used to open the web
interface. Initial installation uses a Linux terminal; everyday learning and
action setup happen in the browser. The supplied executable needs no Go, Node.js
or rebuilding. Setup tools and prerequisites are covered in the installation guide.

## Connect a button to Home Assistant

1. Install the two programs, open RF433 Button Manager and log in with its
   **management token**. Check that the receiver is connected.
2. In Home Assistant, create and enable an automation with a **Webhook** trigger
   and your chosen action, such as turning on a light.
3. In RF433 Button Manager, create a remote, select a button and choose **Learn
   signal**. Follow the prompts for three separate presses, then choose **Confirm
   and save**.
4. Paste the automation's **complete webhook URL** into **Webhook URL**, choose
   **POST** and select **Save button**, or **Save & allow destination** if prompted.
   Saving does not run the automation.
5. When ready to run the action, select **Test saved action** and confirm **Send
   test**. After a successful test, turn on **Enabled**, choose **Save button**
   again and try the physical button.

The [complete walkthrough](GETTING-STARTED.md#learn-a-button-and-connect-home-assistant)
explains each screen and the optional webhook-ID helper. Keep webhook URLs private;
the management token is a separate credential for logging into this app.

## Download and install

Under **Assets**, download both files to the same folder on the Linux computer
with your USB receiver:

- **rf433-button-manager-0.4.0-linux-amd64.zip** — ready-to-run executable, source,
  Git history, documentation and dependency notices.
- **rf433-button-manager-0.4.0-linux-amd64.zip.sha256** — the download checksum.

Choose these assets rather than GitHub's automatic **Source code** downloads,
which omit the executable. Verify the archive checksum and the included
`SHA256SUMS` before installing.

**First installation:** follow the [step-by-step setup guide](GETTING-STARTED.md).
It covers the decoder, installation plan, browser access, login and Home Assistant.

The extracted folder is **`rf433-button-manager`**, and the installed service is
**`rf433-button-manager.service`**. Current state uses
`/var/lib/rf433-button-manager/`. The updater handles supported legacy
`rf-button-manager` installations; the [migration guide](UPGRADING.md#what-is-renamed)
lists the old and new paths.

## Update an existing installation

Leave the existing app running. Finish or cancel learning/inspection and any
pending network change. For a read-only update preview and recovery details,
follow the [upgrade guide](UPGRADING.md).

When ready to update, open a terminal **on the Linux server**, in the folder
containing both downloaded files, and run:

```sh
(
  set -e
  sha256sum -c rf433-button-manager-0.4.0-linux-amd64.zip.sha256
  rfb_update_dir=$(mktemp -d "$HOME/rf433-button-manager-update.XXXXXX")
  unzip -q rf433-button-manager-0.4.0-linux-amd64.zip -d "$rfb_update_dir"
  cd "$rfb_update_dir/rf433-button-manager"
  sha256sum --quiet -c SHA256SUMS
  sudo python3 scripts/update-prebuilt.py --apply
)
```

The updater temporarily disables actions, backs up stopped state, installs and
verifies the running version, checks configuration persistence, then restores
the original enabled flags. If verification fails, it attempts rollback. Keep
the printed snapshot path. It does not send a test webhook or reset USB.

Wait for **Update verified**, refresh your browser and confirm **0.4.0** and the
expected receiver status. Existing mappings, webhook settings, browser URL and
login token are preserved. Test.6 users do not need to relearn their buttons.

## Where is my login token?

The app creates its own token on the Linux server during first startup. In that
server's terminal, or through SSH, run:

```sh
sudo cat /var/lib/rf433-button-manager/management.token
```

Paste the result into **Management token** on the login screen and choose
**Connect**. It is not a Home Assistant access token or webhook ID. Keep it private.
Reboots and updates preserve it. With a custom `--data-dir`, look in that folder instead.

## Compatibility and testing

**A “433 MHz” label alone does not guarantee support.** Compatibility depends on
the signal format and receiver setup. Use **Inspect signal** to investigate a
button before assigning it. The app receives RF; it cannot transmit commands
back to radio devices or generally decrypt rolling/encrypted signals.

Physical testing used **Zorin OS 18.1**, one **RTL2838/R820T receiver** and
**rtl_433 23.11**. The maintainer's setup has nine configured button assignments;
the latest detailed trials exercised one six-button remote. He reports the best
results so far on his equipment and has chosen to publish **0.4.0 as a regular
release**. Zorin itself is not a requirement.

Occasional missed presses remain unresolved. Other hardware combinations,
physical multi-tuner operation, USB/hub recovery and a formal extended soak
remain unverified. No universal compatibility or reception percentage is claimed.
See [Compatibility](COMPATIBILITY.md), [Hardware observations](HARDWARE-TESTS.md)
and the [final software audit](FINAL-AUDIT-0.4.md). Reports from additional receiver
and remote combinations are welcome.

**HTTP 200 means the webhook was accepted**, not independent confirmation that
the device changed state. Diagnostic exports exclude application credentials and
webhook settings but contain radio identifiers and surrounding RF traffic;
review them before sharing.

## AI-assisted development and contributing

This community project was built iteratively with AI coding agents and hands-on
testing by its owner — an approach often called **“vibe coding.”** It is shared
under the [MIT license](../LICENSE) so others can inspect, improve and fork it.
Automated checks and limited physical testing are documented; this is not an
independently certified or universally validated product.

Developers and AI coding agents should start with [AGENTS.md](../AGENTS.md),
[Development](DEVELOPMENT.md), [Architecture](ARCHITECTURE.md) and
[Contributing](../CONTRIBUTING.md). Bug reports, code review and hardware test
results are welcome.

## Verified 0.4.0 package

[GitHub CI passed for the exact packaged commit](https://github.com/ninoleto/rf433-button-manager/actions/runs/41358568955),
including race tests, static checks, build and real isolated HTTP/HTTPS management checks.

Source commit: [fae127bd0b2074fd6ceff3f68f1cafe35e1f1be7](https://github.com/ninoleto/rf433-button-manager/commit/fae127bd0b2074fd6ceff3f68f1cafe35e1f1be7).

The final software audit also records 25 frontend controller checks, 16 real
browser test groups and updater/deployment simulations. Software tests and
recorded-signal replay are separate from physical receiver testing.

The published ZIP's **137 internal checksums passed**. Its bundled source matches
the release commit, and a clean rebuild from the included Git bundle produced
an identical executable. The publicly downloaded ZIP matched the published SHA-256.
This documentation update does not replace that archive or move the release tag.

SHA-256 for `rf433-button-manager-0.4.0-linux-amd64.zip`:

```text
e5f2f5b8d274deeccd1be22693d0fa861f0e2503d00bdf6d20041b4c7ae29382
```
