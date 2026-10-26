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

**This setup uses two separately installed programs on the computer with the USB
receiver, and an existing Home Assistant installation.** Here is what each does:

| Item | What it does | Where it belongs |
| --- | --- | --- |
| RTL-SDR receiver and antenna | Receives the radio transmission when you press your wireless button. | Plugged into an always-on Linux x86_64 computer. |
| `rtl_433` | Reads the receiver and decodes supported transmissions into data. | Installed first, on that same Linux computer. |
| RF433 Button Manager | Gives you a browser interface, learns buttons, remembers assignments and sends webhook requests. It starts and manages `rtl_433`. | Installed second, on that same Linux computer. |
| Home Assistant | Runs the automation you choose, such as turning on a light. | Already running on that computer or another machine on the same home network. |
| Web browser | Displays the setup and management screens. | Your PC or phone. |

For example, Home Assistant can stay on your Raspberry Pi while a separate Linux
mini PC handles the USB receiver. You would install both `rtl_433` and RF433 Button
Manager on the mini PC. **You do not need to install Home Assistant again.** The
supplied app ZIP is for x86_64 PCs (64-bit Intel/AMD); it is not an ARM/Raspberry Pi
application package.

The Linux computer needs **systemd** (which runs the app in the background) and
**udev** (which manages USB-device permissions). The supplied installer uses both.
It also needs **Python 3** to run the update helper and **unzip** to unpack the ZIP.
The ZIP already contains the built application; installing it does not require Go,
Node.js, Docker or compiling source code.

In this guide, **Linux terminal** means the command window on the computer with
the receiver, or an SSH connection to that computer. Do not paste Linux installation
commands into Home Assistant's automation editor. If you are using a Windows
terminal, first connect to the Linux computer by SSH; the commands then run there.
You need a Linux account allowed to use `sudo`, which requests administrator access.

First-time setup goes in this order: install/check `rtl_433`, unpack our app,
install it, open its interface, log in, then learn and connect a button. Existing
users should follow [Update an existing installation](#update-an-existing-installation).

### Install the required decoder

**Our app does not include `rtl_433`. Install it on the same Linux computer as the
USB RTL-SDR**, before installing RF433 Button Manager. If it is already installed
and working, keep that setup; an app update does not require a decoder upgrade.

In the Linux terminal, check whether it is already installed:

```sh
command -v rtl_433
```

If this prints `/usr/bin/rtl_433`, continue with the version check below. If it
prints nothing, install the missing program. On Debian/Ubuntu-based systems, the
[upstream installation instructions](https://github.com/merbanan/rtl_433#installation)
use this command, which you run in the **same Linux terminal**:

```sh
sudo apt install rtl-433
```

`sudo` may ask for your Linux password; no characters appearing while you type
is normal. Review the package manager's prompt before accepting installation.
If it cannot find the package, refresh its package list with `sudo apt update`
and try again. For other Linux distributions, use the upstream installation guide.

The package name in the install command is `rtl-433` (hyphen). The installed
program is called `rtl_433` (underscore). These are two names for the same decoder,
not two different programs. Confirm its location and version:

```sh
command -v rtl_433
rtl_433 -c /dev/null -V
```

A fresh installation with our supplied installer expects `/usr/bin/rtl_433`.
The second command should print a version number; it does not start listening to
the receiver. If the location is different or either check fails, resolve that
before running our installer; see [operations](OPERATIONS.md). Our physical tests
used version 23.11. Other distributions/package versions need verification.

Also check the two setup tools:

```sh
command -v python3
command -v unzip
```

Each should print a path. On Debian/Ubuntu-based systems, install a missing tool
with `sudo apt install python3` or `sudo apt install unzip`, respectively.

After our app is installed, **it starts `rtl_433` for you**. Do not leave a separate
radio-decoding command or another SDR application using the same USB receiver.
If another service already uses it, identify that service and arrange its handover
first; do not stop unrelated services blindly.

### What is needed in Home Assistant?

You need **Home Assistant already running**, with a saved automation using its
built-in **Webhook** trigger. Home Assistant can be on another computer, NAS or
Raspberry Pi on the same local network; it does not need to be installed on the
Linux receiver computer. Our app must be able to reach the webhook URL.

An **automation** is the instruction you create in Home Assistant, such as "when
this request arrives, turn on the hallway light." Its **webhook URL** is the
private web address that receives that request. Our app recognizes the radio
button and sends the request; Home Assistant controls the light. The light must
already work in Home Assistant before this example can control it.

If you do not have Home Assistant yet, set it up separately using its
[installation guide](https://www.home-assistant.io/installation/). Our installer
does not install Home Assistant, and our app is not a Home Assistant add-on.

No extra Home Assistant add-on, custom integration, HACS component or MQTT broker
is needed for this connection. You do not need a Home Assistant access token:
the webhook URL itself must be kept private. This method triggers an automation;
it does not automatically create RF devices/entities in Home Assistant. See
[Home Assistant's webhook documentation](https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger).

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

Use a light that already works in Home Assistant for your first test. Keep both
browser interfaces open in separate tabs so you can move between them.

### 1. In RF433 Button Manager: check reception and learn the button

1. Check that the receiver status says it is connected and acquiring samples.
   This means the receiver is running; it does not yet prove your button can be decoded.
2. If you have more than one tuner, open **Settings → Receiver device**, select
   the intended tuner and choose **Save settings**. Multiple tuners need distinct
   serial numbers; see [receiver selection](RECEIVERS.md).
3. Choose **Create a remote** if none exists. A remote here is a group of buttons
   in our interface. Give it a name, choose its button count and select **Create remote**.
4. Select the on-screen button you want to assign. Keep **Enabled** off and choose
   **Learn signal**. If you changed its name or settings, click **Save button** first.
5. When prompted, press and release the physical button once. Wait for the app to
   accept that press, then select **Ready for next press** before pressing again.
   Repeat until three separate matching presses are accepted. Do not hold the
   button down to try to count three presses.
6. Choose **Confirm and save**. The app now remembers which radio signal belongs
   to that on-screen button. It still needs the Home Assistant action below.

If learning fails, stop here and use [learning help](LEARNING.md) or **Inspect
signal**. A transmission being detected does not guarantee the button is supported;
see [compatibility](COMPATIBILITY.md).

### 2. In Home Assistant: create the automation

1. Open **Settings → Automations & scenes**. Select **Create automation → Create
   new automation**.
2. Under **When**, choose **Add trigger** and find **Webhook**. Keep its generated
   webhook ID private. Ensure **POST** is allowed and keep **Local only** enabled
   for this same-network setup. Labels may differ with Home Assistant versions.
3. Under **Then do**, add an action that turns on your chosen light. Save the
   automation with a recognizable name and make sure it is enabled.
4. Copy the webhook's complete URL if the editor offers it. If it only shows a
   webhook ID, use our URL helper in the next section.

The full URL contains the Home Assistant address, `/api/webhook/` and the private
ID. The ID alone is not a URL. See the official
[automation editor](https://www.home-assistant.io/docs/automation/editor/) and
[webhook guide](https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger).

### 3. In RF433 Button Manager: enter where to send the request

Return to the button you learned. Paste the **complete webhook URL** into
**Webhook URL**, then select **POST** under **Method**. POST is the type of web
request the app sends; it must match an allowed method in Home Assistant.
For this simple light example, leave the advanced headers and payload empty.

**If you only have a webhook ID:** expand **Only have a Home Assistant webhook ID?**.
In **Home Assistant base URL**, enter the address of your Home Assistant server,
including `http://` or `https://` and its port if used. Enter the ID in **Webhook ID
or complete URL**, then click **Use**. The helper fills in **Webhook URL**; you must
still save the button.

Use the address that the **Linux receiver computer** can reach. When Home Assistant
is on another computer, do not leave the helper's `http://127.0.0.1:8123` default:
`127.0.0.1` means the computer making the request, not the other Home Assistant machine.
Use your Home Assistant machine's actual LAN address instead. The app address on
port **8787** opens RF433 Button Manager; it is not the Home Assistant webhook address.

| Information | What you do with it |
| --- | --- |
| RF433 Button Manager management token | Paste into the app's login screen. |
| Complete Home Assistant webhook URL | Paste into the button's **Webhook URL** field. |
| Home Assistant webhook ID only | Use the helper together with your Home Assistant base URL. |

Keep all three private. None needs to be posted in an issue or sent to an AI agent.

Leave **Enabled** off and click **Save button**. If the button says **Save & allow
destination**, check that the displayed server address is your intended Home
Assistant server before clicking it. This permits the app to send requests to
that server. Saving alone does not send a request or switch on the light.

### 4. Test the automation, then enable physical presses

1. When you are ready for the light to turn on, click **Test saved action**, then
   confirm **Send test**. **This sends a real request, even with Enabled off.**
2. Check the light and the app's event monitor. This test checks the saved webhook
   connection; it does not test radio reception.
3. If the test works, turn on **Enabled** and click **Save button** again. A toggle
   change is not applied until you save it.
4. Press and release the physical button. Confirm that the app recognizes it and
   Home Assistant performs the action. If the light is already on, turn it off in
   Home Assistant before testing the button so the result is visible.

Repeat for each button/automation you want to assign. The service continues
listening after you close the browser, as long as the Linux computer and Home
Assistant are running. Closing the browser does not turn off enabled button actions.

### If it does not work, check which step failed

| What you observe | What to check next |
| --- | --- |
| The app login screen does not open | Recheck **Open the interface** above, the app service and your SSH tunnel or configured LAN address. |
| Receiver disconnected or error | Check the USB connection, selected tuner, permissions and whether another program owns the receiver. See [operations](OPERATIONS.md). |
| Connected, but learning fails | Reception alone is not a decoded press. Use **Inspect signal** and the learning/compatibility guides; do not invent a signal code. |
| Test saved action fails | Check the complete webhook URL, reachability from the Linux host, POST permission and allowed destination. See the event monitor for the request result. |
| A request succeeds but the light does not change | Check that the Home Assistant automation is enabled, its conditions/actions are correct and its trace shows the expected run. A successful web request alone does not prove the light changed. |
| Manual test works, physical press does not | Check the learned mapping, decoded signal events and that **Enabled** was saved. The manual test bypasses the RF button. |

## Update an existing installation

1. Leave the existing app running. In its browser interface, finish or cancel any
   learning/inspection. For each enabled button, turn **Enabled** off and click
   **Save button** to temporarily disable its action.
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
5. Re-enable the button actions you want to use and click **Save button** for each.
   The updater leaves actions disabled so they do not trigger unexpectedly during
   verification.

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
