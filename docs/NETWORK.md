# Network access, HTTPS and management tokens

These settings change how your browser connects to RF433 Button Manager. Your
RF receiver and saved webhook actions keep running. The app changes its own
listeners and files only; it does not configure a firewall, router, VPN or Home
Assistant.

## On the Linux server or another device?

A new installation starts at `http://127.0.0.1:8787`. Open that address **on the
Linux server**, or first use the SSH tunnel in [Getting started](GETTING-STARTED.md).
`localhost` and `127.0.0.1` always mean the device opening the address: typing them
on your Windows PC or phone does not connect to the Linux server.

An upgrade preserves your existing address, ports and network protocol. No
maintainer's LAN address is supplied as a default. From test.3, HTTP on
`127.0.0.1` remains available independently of the network connection type.
It still requires the management token; it is not an authentication bypass.

| Network setting | On the Linux server | On other devices |
| --- | --- | --- |
| Network access off | HTTP on `127.0.0.1:8787` | No access |
| Network access on, HTTP selected | HTTP on `127.0.0.1:8787` | HTTP at the selected server address |
| Network access on, HTTPS selected | HTTP on `127.0.0.1:8787` | HTTPS at the selected server address; network HTTP is closed |

The table uses the default local HTTP port. The advanced HTTP / localhost port
changes that port. A saved test.2 configuration that enabled both network
protocols remains readable and keeps both until you apply and confirm a new
choice; the interface explains this. There is no separate “keep HTTP” checkbox.

## Allow access from your home network

1. Open **Settings → Network access**. Turn on **Allow access from my local
   network**. This changes a draft; it does not immediately disconnect anything.
2. Tap **Server address** and choose the Linux server's actual LAN address from
   the large list. Each row names its network interface. A Docker bridge normally
   is not reachable from another PC. Choose a VPN address only if both devices
   already use that VPN. The app never guesses or switches interfaces for you.
3. Choose **HTTP** or **HTTPS** for other devices. Prepare client trust first if
   choosing HTTPS, using the instructions below. Localhost HTTP does not need
   a separate switch and does not expose HTTP to other computers.
4. Leave the ports alone unless needed. Defaults are 8787 for HTTP and 8788 for
   HTTPS. **Advanced: address and ports** also allows a manual address.
5. Select **Apply network settings**. Open the proposed URL in another tab, log
   in with your current token, then go to Settings and **Confirm connection**.
   HTTPS changes must be confirmed over the proposed HTTPS address.

The new listeners are opened before the old ones are closed. You have **three
minutes** to confirm. A failed bind, **Cancel change**, timeout, or restart before
confirmation keeps the previously saved settings. The confirmation button is
only enabled in a tab using the proposed address. Your token is never in a link.
The URL cards show **saved** endpoints, not unapplied form edits.

After confirmation, copy the **Network URL** for your other devices and update
bookmarks. The old tab may stop working if its listener was removed. If the new
URL is blocked, cancel and investigate routing, VPN membership or the firewall;
this app does not change those settings. A localhost health check cannot prove
that a Windows PC or phone can reach the server.

Only explicit loopback, private LAN and VPN addresses are accepted, including
Tailscale's 100.64.0.0/10 range. Public IPs and all-interface binds such as
`0.0.0.0` or `::` are refused. Ports must be 1024–65535 and must be free on both
the selected address and its local companion. No other listener is stopped to
free a port.

## Turn off access from other devices

Turn **Allow access from my local network** off and apply. The app warns that you
must open the proposed localhost URL **on the Linux server**, or through an SSH
tunnel to that exact local port, and confirm there. Until confirmation, current
network access remains available. Afterwards, other devices cannot connect
through the previous network listener. The local browser still works, with the
same token and chosen HTTP/HTTPS security.

## HTTP, HTTPS and remote access

HTTP sends your token and application data without encryption. Never use it on
a public or shared network. For access from outside your home, use a trusted VPN
such as Tailscale or WireGuard; connect to the server through the protected VPN
route. A VPN icon does not protect an unrelated direct HTTP route. HTTPS adds
browser-to-server encryption when the certificate is trusted correctly.

**Never forward this app's port to the public internet, even with HTTPS.** The
network switch does not configure a VPN or make this a public hosting product.

## Set up HTTPS on each browser device

This app creates a private certificate authority (CA) and a server certificate.
Browsers do not know your CA until you explicitly trust it. The **HTTPS certificate
— download and setup instructions** section in Settings contains the steps below.
Do this once per device/browser profile, before applying HTTPS.

1. Use a trusted initial connection: a browser on the server, SSH tunnel, or
   properly authenticated VPN. Select **Download local HTTPS trust certificate**.
2. Check the certificate's SHA-256 fingerprint against Settings opened through
   that trusted connection. A downloaded certificate and an untrusted HTTP page
   cannot independently verify each other. Trust only a CA from a server you own.
3. Install the public `rf433-button-manager-ca.crt` on the **client device where
   you open the browser**, using its instructions below. No private key is in this
   download. A web page cannot install trust automatically.
4. Choose HTTPS, apply, open the exact proposed address and confirm in that tab.
   A browser certificate warning means verification has not succeeded; do not
   bypass it or disable verification.

For an independent server-side fingerprint check, this read-only command prints
only the public CA's SHA-256 fingerprint, never its private key:

```sh
sudo python3 - <<'PYCA'
import hashlib, json, ssl
from pathlib import Path
import base64
ca = base64.b64decode(json.loads(Path('/var/lib/rf433-button-manager/tls.json').read_text())['ca'])
der = ssl.PEM_cert_to_DER_cert(ca.decode())
print(hashlib.sha256(der).hexdigest())
PYCA
```

Compare the hexadecimal digits, ignoring uppercase/lowercase and colon spacing.
To inspect a downloaded PEM certificate on a computer with OpenSSL:
`openssl x509 -in rf433-button-manager-ca.crt -noout -fingerprint -sha256`.
Hashing the text file itself gives a different value; the app hashes DER certificate
bytes. A custom `--data-dir` uses that directory instead of the installed path.

### Windows: Chrome and Edge

Save the file on the Windows PC. Press **Win + R**, enter `certmgr.msc`, and open
**Trusted Root Certification Authorities → Certificates**. Right-click that
Certificates folder, choose **All Tasks → Import**, and select the downloaded
file. Keep the destination set to the trusted root store for your user. Inspect
and verify the certificate before accepting the trust prompt. Restart the browser,
then open and confirm the HTTPS URL. Managed PCs may require administrator help.

### Linux: Chrome and Chromium

On the computer running the browser, open `chrome://certificate-manager` in the
address bar. In **Local certificates**, find **Custom / Installed by you** and
import the file under **Trusted certificates**. Chrome 134+ has this manager;
labels can vary. Older builds use `chrome://settings/certificates`, then
**Authorities → Import** and website-identification trust. Do not import it as a
personal/client certificate. Restart the browser and confirm the HTTPS URL.

If the browser exposes a read-only system list, use its documented CA import
procedure. Chromium's Linux guidance describes the NSS database fallback; its
path differs between versions. Do not blindly edit every user's trust store or
use a command copied for a different distribution/browser installation.

### Firefox: desktop

Open **Settings → Privacy & Security → Certificates → View Certificates →
Authorities → Import**. Choose the verified file and allow it to identify websites.
Restart Firefox and confirm HTTPS. Recent Firefox versions on Windows/macOS may
already trust a CA installed in the operating system. Linux/browser packaging
can differ; use Firefox's own Authorities import when needed.

### Android: Chrome

Download the file on your phone. In Android Settings, search for **Install a
certificate** or **Encryption & credentials** (usually under Security & privacy
and More security settings). Select **CA certificate**, verify the source, unlock
the phone, and select the file. This is not a Wi-Fi or personal client certificate.
Restart Chrome and confirm HTTPS. Manufacturer labels vary. Managed devices or
some apps may reject user-installed CAs; do not bypass warnings to continue.

### macOS: Safari and Chrome

Import the verified file into your login keychain with **Keychain Access**. Open
**RF433 Button Manager local CA**, expand **Trust**, and allow **Secure Sockets
Layer (SSL)** trust. Save and authenticate if asked. Restart the browser, then
open and confirm the HTTPS URL.

### iPhone/iPad: Safari

Download the certificate in Safari and install its profile through **Settings →
General → VPN & Device Management** or **Profile Downloaded**. Then open
**General → About → Certificate Trust Settings** and enable full trust for the
verified **RF433 Button Manager local CA**. Reopen Safari and confirm HTTPS.
Device management policies may prevent installation.

### Warnings, renewal and removal

A wrong address, wrong browser/profile trust store, expired certificate or bad
host clock can cause warnings. Use the exact displayed URL. Select **Renew server
certificate** to renew the leaf with the same CA; client trust need not be
reinstalled. The leaf lasts one year and is refreshed on preparation/startup when
less than 30 days remain. The CA lasts ten years. A replacement CA must be trusted
afresh. To remove trust, delete only this app's CA in the same certificate manager.

Private CA/server keys stay in mode-0600 `tls.json` under the private data directory.
Never share that file. Stopped-state upgrade snapshots include it; GUI logical
configuration backups do not. Certificates include the selected/pending IPs plus
localhost/loopback; TLS 1.2+ is required.

### Platform references

- [Microsoft certificate stores and current-user manager](https://learn.microsoft.com/en-us/windows-server/identity/ad-cs/inspect-certificates-manage-certificate-stores)
- [Chrome Certificate Manager](https://chromium.googlesource.com/chromium/src/+/main/net/data/ssl/chrome_root_store/faq.md) and [Chromium Linux certificate management](https://chromium.googlesource.com/chromium/src/+/main/docs/linux/cert_management.md)
- [Mozilla CA guidance](https://support.mozilla.org/en-US/kb/setting-certificate-authorities-firefox)
- [Android credential settings](https://support.google.com/pixelphone/answer/2844832) (the linked Wi-Fi certificate example is not this app's CA-import purpose)
- [Apple Keychain trust](https://support.apple.com/guide/keychain-access/kyca11871/mac) and [iOS certificate trust](https://support.apple.com/en-us/102390)

The instructions are platform guidance, not a claim that this release was manually
installed into every listed browser/OS trust store. Actual Windows/phone trust
setup remains part of operator acceptance.

Confirmed settings are stored privately in `network.json` and take precedence
over `RFB_LISTEN` / `--listen`. Network changes do not restart the RF decoder.

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
