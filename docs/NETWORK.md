# Network access, HTTPS and management tokens

These settings affect the browser connection. RF reception and saved webhooks
continue while you change them. The app changes only its own listeners and files;
it does not open firewall ports, configure a router or change Home Assistant.

## Open the app for the first time

A fresh install listens at `http://127.0.0.1:8787` on the Linux host. From another
computer, first use the SSH tunnel in GETTING-STARTED.md. Existing installations
keep their current address during an upgrade. No author's LAN address is shipped
as a default.

In the browser open **Settings → Network access**. Choose the actual private IP
address of the Linux host; the address field suggests local interfaces. Do not
choose a Docker bridge, loopback or a VPN interface unless that is how your browser
reaches the host. You can enable HTTP, HTTPS or both, on different ports. Defaults
are 8787 for HTTP and 8788 for HTTPS. Ports must be 1024–65535. Public and wildcard
addresses such as `0.0.0.0` are rejected.

## Test and confirm a new connection

1. Select the address, protocols and ports, then click **Test new connection**.
2. The current connection stays available. The app shows a new connection link
   and a **three-minute** countdown. Open that link in another tab and log in
   with the same management token. Tokens are never placed in the link.
3. Open Settings in the new tab and click **Confirm connection**. If HTTPS is
   enabled, confirmation must come from the new HTTPS connection, with the
   certificate trusted as described below.
4. Only then are the settings saved and obsolete listeners closed. Update your
   bookmark. The old tab may stop connecting if its listener was disabled.

If the link is unreachable, use **Cancel change**, or wait for the countdown to
expire. The previous settings remain. An unconfirmed service restart also returns
to the last saved settings. A busy port or an address absent from this host fails
without replacing the working listener. This does not diagnose or change a
network firewall; check the actual blocker before changing unrelated networking.

Confirmed settings are stored in private `network.json` in the data directory.
They take precedence over `RFB_LISTEN` / `--listen`, which supplies the initial
HTTP listener. A network change does not restart the RF decoder.

## Enable HTTPS and trust its certificate

The app creates its own local certificate authority (CA) and a server certificate
for the configured IP. It is not signed by a public Internet CA. Therefore your
browser needs the public trust certificate installed before it can verify this
server without a warning.

1. Establish a trusted initial connection, such as the local browser or SSH
   tunnel. In Settings, choose **Download trust certificate**. The file is
   `rf433-button-manager-ca.crt`. It contains no private key.
2. Verify its SHA-256 certificate fingerprint against the value shown in Settings
   over that trusted connection. Do not install an unverified certificate from
   an email, issue comment or an untrusted network response.
3. Import it into the trust store used by your client/browser. On Windows, use
   the current user's **Trusted Root Certification Authorities** store; Firefox
   may use its own certificate settings. See [Microsoft certificate stores](https://learn.microsoft.com/en-us/windows-hardware/drivers/install/certificate-stores)
   and [Mozilla CA guidance](https://support.mozilla.org/en-US/kb/setting-certificate-authorities-firefox). Phone certificate installation and trust
   settings vary by OS. Installing a local CA is an administrator trust decision,
   not something a web page can do automatically.
4. Enable HTTPS and test the new connection. Use the exact IP and port shown by
   the app. A different hostname or IP will not match the certificate.
5. Confirm from the working HTTPS tab. You can disable HTTP in the same proposed
   configuration; the old HTTP connection remains until confirmation succeeds.

Downloading a certificate alone does not enable HTTPS or install client trust.
Do not disable certificate verification as a permanent workaround. The server
certificate lasts one year; Settings shows its expiry and provides **Renew HTTPS
certificate**. Renewal retains the CA, so clients normally do not need to import
trust again. Startup or certificate preparation also renews a leaf with less than
30 days remaining. The CA lasts ten years; replacing it requires clients to trust
the replacement. Keep the host clock correct. TLS 1.2 or newer is required.

Private CA and server keys live in mode-0600 `tls.json` inside the private data
directory. Never share that file. Full-state backups include it; the GUI logical
configuration backup does not. The authenticated download endpoint returns only
the public CA certificate.

## Replace the login token

Open **Settings → Management token → Replace token**, then confirm. The app
creates and saves a new random token. This tab keeps working and can **Show** or
**Copy** the replacement; other tabs must log in again. RF reception and button
actions continue. The app does not accept a short custom password as a token.

If a tab closes or the response is lost, the saved token is still available on
the Linux host:

```sh
sudo cat /var/lib/rf433-button-manager/management.token
```

The login screen's **Copy command** copies that command for the terminal. It
cannot read the token before login. The token is stored in this browser tab's
session storage and removed by **Lock**. Use HTTPS or a trusted tunnel to protect
it in transit. A full-state rollback restores the token contained in its snapshot.

## Local recovery when the saved address is no longer available

A DHCP address change can make a previously confirmed address unavailable. Keep
a suitable DHCP reservation for an always-on server. To recover locally, stop
only this app, preserve `network.json` under a new name, set `RFB_LISTEN` in the
owned environment file to a reachable explicit address (or `127.0.0.1:8787` for
an SSH tunnel), then start the app. Do not delete the database, token or TLS keys.
For example, after inspecting the file and choosing to reset only network settings:

```sh
(
  set -e
  sudo systemctl stop rf433-button-manager.service
  sudo test ! -e /var/lib/rf433-button-manager/network.before-recovery.json
  sudo mv -n /var/lib/rf433-button-manager/network.json /var/lib/rf433-button-manager/network.before-recovery.json
  sudo test ! -e /var/lib/rf433-button-manager/network.json
  sudoedit /etc/rf433-button-manager/manager.env
  sudo systemctl start rf433-button-manager.service
)
```

If the backup name already exists, choose a new name; do not overwrite it. The
`mv -n` must actually move the original file before restarting with the initial
listener. In the browser, review and confirm network settings again. Existing
reverse proxies remain optional and independently administered; their exact
trusted origin still uses `RFB_PUBLIC_ORIGIN` as described in OPERATIONS.md.
