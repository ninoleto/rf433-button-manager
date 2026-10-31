# Security and private configuration

The management token grants full configuration, backup/restore and explicit action
testing access. There are no roles or per-administrator audit identities. Static
assets and minimal `/healthz` are public; `/api/` requests require the credential.
Browser-origin/cross-site checks add protection but do not replace authentication.
The GUI stores the token in tab `sessionStorage`, never a URL or cookie.

The default listener is loopback. An operator can deliberately choose one private
LAN address with authentication retained. HTTP is unencrypted; use the built-in
HTTPS listener with a verified trust certificate, an SSH tunnel or a trusted VPN. This project does not configure
public exposure, tunnels, firewall, proxy or router rules. The supplied service
runs as `rfbutton`, without login shell or capabilities, with private state and
limited USB permissions. Do not run the application as root to solve device access.

## Webhook boundary

Only configured HTTP(S) origins are permitted. URL credentials/fragments, unsafe
headers and invalid payloads are rejected. DNS results are checked before dialing;
unspecified, multicast and link-local IPs are prohibited. Private/loopback addresses
are intentionally allowed for local automation; other unicast destinations can
be used if explicitly allowlisted. Administrators must choose safe destinations;
this is not a general-purpose untrusted-URL or SSRF sandbox.

The authenticated button editor can approve one exact destination while saving:
**Save & allow destination** shows its server address and atomically saves that
permission with the button. Pasting alone makes no change. Invalid edits cannot
widen the allowlist. API callers must provide the matching `allow_webhook_origin`
explicitly or use an already allowed origin. This does not enable disabled actions,
send a test request or change receiver configuration.

Requests do not follow redirects, use environment proxies, reuse connections or
retry automatically. At most eight actions are pending globally and one per button.
A timeout/network failure leaves the physical outcome unknown. Prefer idempotent
state-setting actions. RF is unauthenticated and replayable; static codes are
unsuitable as sole authorization for high-impact operations.

**Test webhook can execute with a disabled button.** Installation and learning
do not require real webhook tests. Learning waits for current requests and blocks
new dispatches, but cannot undo requests already delivered. Suppression continues
until capture analysis and receiver transitions have cleaned up. Learning uses
private bounded pulse files, deleted after offline analysis. One replay child has
file input only; it never opens the receiver. Authenticated
`GET /api/learn/capture` downloads the most recent bounded in-memory pulse/decoded
recording; `GET /api/inspect/result` exports RF-only inspection evidence. These
exclude application credentials and webhook settings, but can contain private
radio identifiers from the intended transmitter or nearby devices. Review before
sharing; they are neither anonymized nor original IQ recordings.

## Secret handling and rotation

The whole state directory and logical backups are private. Backups contain URLs,
IDs, headers and payloads; the token is stored separately and is not exported.
Configuration is not encrypted at rest. Never commit databases, tokens, backups,
production environment files, private RF captures or unredacted screenshots/logs.
Inspect all outgoing history as well as current files before public publication.
Synthetic fixtures use invented identifiers and example endpoints.

Retrieve the installed token only in a trusted local terminal:

```sh
sudo cat /var/lib/rf433-button-manager/management.token
```

Do not put it in a URL, process argument, issue or shared log. Avoid shell tracing
around credentials. The API examples read the token file into memory. Review
receiver diagnostics and decoded data before sharing; they can identify hardware.

Use **Settings → Management token → Replace token** to generate a new random
256-bit token. Confirmation is required. The new credential is persisted atomically
before it replaces the live verifier. Other browser tabs are rejected immediately;
the initiating tab receives the new token in its authenticated response. A failed
write retains the old token. RF reception and webhooks continue during rotation.

If login is impossible, retrieve the current credential locally using the command
above. Token files are mode 0600 in private application state; do not remove a
working token merely because a browser has an old value. Backups can contain old
tokens: a full-state rollback restores the token from that snapshot. Rotate it
again after rollback if it was compromised. Rotate leaked webhook credentials at
their destination too; deleting a commit does not revoke them.

## Local HTTPS trust

`tls.json` holds the local CA key, server key and certificates with mode 0600.
Only the **public CA certificate** can be downloaded through the authenticated
API. Never publish or export `tls.json`. The browser must trust that CA on each
client; the download does not install trust automatically. Verify its fingerprint
through a trusted channel before installing it. Leaf certificates are limited to
the configured IP addresses. This is private-LAN TLS, not a public certificate or
permission to expose the app to the Internet. See [NETWORK.md](docs/NETWORK.md).

A staged network change keeps the previous listeners until the new endpoint is
confirmed within three minutes. HTTPS must be confirmed through HTTPS. Expiry,
cancellation and an unconfirmed restart retain the prior saved configuration.
This protects against an unreachable address; it is not a firewall manager.

## Vulnerability reports

Use the repository's private vulnerability-reporting feature if enabled. Otherwise
request a private contact channel with a minimal non-sensitive description first.
Do not publish a live token, endpoint, database or RF capture in an issue. Include
affected commit/version, sanitized reproduction and impact. The development build
does not promise a response SLA or a maintained stable-version support window.
