# RF433 Button Manager: download, install, log in and update

Use supported 433.92 MHz wireless buttons to trigger Home Assistant automations
through a USB RTL-SDR receiver. Once installed, the app runs in the background;
you learn buttons and assign webhook URLs in your browser.

**Physical testing is limited to Zorin OS 18.1, Linux x86_64**, one RTL2838/R820T
receiver, rtl_433 23.11 and three home buttons. Other distributions and hardware
combinations are not physically verified. Automated Ubuntu CI is separate software
evidence. This is an AI-assisted ("vibe coded") development prerelease, shared for
review, testing and improvement. It cannot learn every signal sold as "433 MHz".

## Before you start

You need an always-on Linux x86_64 computer with systemd and udev, a compatible
RTL-SDR with an antenna, and `rtl_433` installed at `/usr/bin/rtl_433` for a fresh
installation. The app installer does not install or upgrade rtl_433 or other
packages. Check the receiver is not already being used by another application.
See [operations](OPERATIONS.md) if any prerequisite is missing or another RF service
needs to hand over the receiver. Do not stop unrelated services blindly.

The ZIP includes a ready-to-run executable: you do **not** need Go, Node.js, Docker
or a source build. Python 3, unzip and the normal system administration tools are
needed by these installation/update steps. Home Assistant can run on another machine.

## Download and unpack

1. Open the [0.3.8-dev release page](https://github.com/ninoleto/rf433-button-manager/releases/tag/v0.3.8-dev).
2. Under **Assets**, download both:
   - **rf433-button-manager-0.3.8-dev-linux-amd64.zip** — the application, source and documentation.
   - **rf433-button-manager-0.3.8-dev-linux-amd64.zip.sha256** — its integrity checksum.
3. Put both files in the same folder on the Linux computer where the app will run.
   GitHub's automatic **Source code** downloads do not contain the prebuilt executable.
4. Open a terminal in that folder and run:

```sh
(
  set -e
  sha256sum -c rf433-button-manager-0.3.8-dev-linux-amd64.zip.sha256
  rfb_extract_dir=$(mktemp -d "$HOME/rf433-button-manager.XXXXXX")
  unzip -q rf433-button-manager-0.3.8-dev-linux-amd64.zip -d "$rfb_extract_dir"
  cd "$rfb_extract_dir/rf-button-manager"
  sha256sum -c SHA256SUMS
  chmod u+x bin/rf-button-manager
  ./bin/rf-button-manager --version
  printf 'Extracted project folder: %s\n' "$PWD"
)
```

Every checksum must report **OK**, and the version must be **0.3.8-dev**. Stop on
an error. Copy the printed project folder and `cd` into it for the next steps.
The inner folder is still called `rf-button-manager` for compatibility; this is
intentional. Extraction leaves existing installations and workspaces untouched.

## First installation

Run these from the extracted project folder **on the Linux server**:

```sh
bash scripts/inspect-zorin.sh
bash scripts/install.sh --binary "$PWD/bin/rf-button-manager"
```

The second command only shows the installation plan. Check receiver ownership,
port 8787 availability and the listed changes. When ready to install:

```sh
sudo bash scripts/install.sh --binary "$PWD/bin/rf-button-manager" --apply
```

It installs the app as a background system service, creates its `rfbutton` account
and private data directory, and adds its USB permissions. It does not modify Home
Assistant, Docker, firewall rules or other services. If USB permissions require it,
reconnect **only the RTL-SDR**, with the operator's agreement. See
[troubleshooting](OPERATIONS.md) if the receiver stays disconnected.

## Open the interface

On the Linux server itself, open **http://127.0.0.1:8787**.

From another PC, the default installation needs an SSH tunnel. In that PC's
terminal, use the following command, replacing `YOUR_USER` and `YOUR_SERVER` with
your Linux login name and server's hostname or private IP address:

```sh
ssh -N -L 8787:127.0.0.1:8787 YOUR_USER@YOUR_SERVER
```

Leave that terminal open, then visit **http://127.0.0.1:8787** in the PC's browser.
The forwarding terminal may look idle; that is normal. If you already configured
a private LAN address for the app, continue using that existing URL instead.
[Operations](OPERATIONS.md#start-stop-and-access) explains deliberate LAN binding.
Do not expose the management interface publicly.

## Find your management token and log in

Run this in a terminal **on the Linux server**, including over SSH:

```sh
sudo cat /var/lib/rf-button-manager/management.token
```

Copy the token it prints into **Management token** in the browser and select
**Connect**. Keep the token private; do not put it in an issue, screenshot or chat.
It is the app's own login credential, **not** a Home Assistant token or webhook ID.
The login screen contains the same instructions.

The token is created at first startup and preserved across reboots and updates.
Run the command again whenever you need it. If the file is missing, check that
installation finished and `rf-button-manager.service` is running. Do not disable
authentication or create an empty file. With a custom source-run `--data-dir`, read
`management.token` inside that chosen folder instead.

## Learn a button and connect Home Assistant

1. Check the receiver status; choose the desired tuner under **Settings → Receiver
   device** if needed. Multiple tuners require distinct serial numbers.
2. Create a remote, select a button and choose **Learn**. Follow the prompts for
   three separate matching presses, then **Confirm and save**.
3. In Home Assistant, create and save an automation with a **Webhook** trigger and
   the action you want, such as turning on a light.
4. Paste the **complete webhook URL** into the button's **Webhook URL** field.
   Use POST for a standard Home Assistant webhook. Save and approve the destination
   if prompted. Saving alone does not send a request.
5. When ready for the automation to run, use **Test saved action**, enable the
   button action, then try a physical press. The test sends a real request even
   while automatic button actions are disabled.

The service continues running when the browser is closed. **Inspect signal** can
help investigate a new transmitter without assigning an action, but detection is
not a promise of compatibility. See [learning](LEARNING.md),
[compatibility](COMPATIBILITY.md) and [Home Assistant's webhook documentation](https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger).

## Update an existing installation

1. Leave the existing app running. In its browser interface, finish or cancel any
   learning/inspection and disable all button actions temporarily.
2. Follow **Download and unpack** above. Use a fresh folder; do not extract over
   an old checkout. From the printed project folder, review the update plan:

   ```sh
   sudo python3 scripts/update-prebuilt.py
   ```

3. Apply that plan:

   ```sh
   sudo python3 scripts/update-prebuilt.py --apply
   ```

4. Wait for **Update verified**, then refresh the browser. Confirm **0.3.8-dev**
   and the expected receiver status. Use the same address and management token.
5. Re-enable the button actions you want to use. The updater leaves actions disabled
   so they do not trigger unexpectedly during verification.

The updater backs up the stopped app's data and old executable, replaces only the
executable, restarts only this service and verifies the live version, authentication,
configuration and token. It rolls back if verification fails. It does not call real
webhooks, change the listener/service/USB rule or update a separate development
checkout. Keep the printed snapshot path for [rollback](OPERATIONS.md).
Downloading or publishing a GitHub release alone does not update your server.

## Developers and AI coding agents

Start with [AGENTS.md](../AGENTS.md), [DEVELOPMENT.md](DEVELOPMENT.md) and
[CONTRIBUTING.md](../CONTRIBUTING.md). Use current code and reproducible tests; keep
private configuration and RF captures out of Git. Report hardware tests separately
from simulated or replayed tests. The source, MIT license and full Git history are
included so others can inspect, extend and fork the project.
