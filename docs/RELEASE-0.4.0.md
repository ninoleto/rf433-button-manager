# RF433 Button Manager 0.4.0

Manage supported 433.92 MHz buttons in a browser and use them to trigger Home
Assistant webhooks through an RTL-SDR receiver on Linux.

## Changes

- Improved normal reception for supported fixed-code remotes, including exact
  matching across verified decoder representations and a bounded correction for
  saved PWM profiles that split complete words. Existing mappings are retained.
- Per-button reception counts and signal observations, plus optional normal-use
  recordings to investigate recognition and action results.
- Local HTTP access and configurable LAN/VPN HTTP or HTTPS, with staged connection
  testing, confirmation and rollback. Management token rotation is available.
- Classic and Unified desktop layouts, phone/tablet navigation, a light/dark
  switch, touch receiver/address pickers, and editable number controls with
  minus, plus and Reset buttons.
- Method above Timeout on all screens and 24-hour local time throughout the
  interface, including activity events and receiver status.
- An application updater that preserves mappings, webhooks, token, environment
  and enabled flags, including migration from the older installation name.

## Install or update

The supplied binary is for **Linux x86_64**. The installer uses systemd and udev.
Install `rtl_433` separately; the tested version is **23.11**. Connect a supported
RTL-SDR receiver and antenna, and use an existing Home Assistant installation.

- First installation: [Getting started](GETTING-STARTED.md).
- Existing installation: [Upgrade guide](UPGRADING.md). Test.6 users keep their
  saved signals; no relearning is required.
- Management login: retrieve the token on the Linux server as described in the
  getting-started guide. It is separate from any Home Assistant webhook ID.
- Network access and certificate setup: [Network guide](NETWORK.md).

Download both `rf433-button-manager-0.4.0-linux-amd64.zip` and its matching
`.zip.sha256`. Verify the outer checksum and the included `SHA256SUMS` before
installing. The ZIP includes the executable, source, Git history, documentation
and dependency notices. GitHub's automatic source archives omit the executable.

## Compatibility and testing

Physical testing used Zorin OS 18.1, one RTL2838/R820T receiver and rtl_433 23.11.
The maintainer's existing setup has nine configured assignments; the detailed
latest trials exercised a six-button remote. Other hardware combinations remain
unverified. Occasional missed presses were observed, and their remaining causes
are not fully established. No universal compatibility or reception percentage
is claimed. See [Compatibility](COMPATIBILITY.md), [Hardware observations](HARDWARE-TESTS.md)
and the [final software audit](FINAL-AUDIT-0.4.md).

HTTP 200 means the webhook was accepted; the destination automation and device
determine the physical result. Diagnostic recordings contain private radio
identifiers and surrounding RF traffic, so review them before sharing.
