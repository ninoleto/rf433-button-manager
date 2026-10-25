# Security and private configuration

The management token grants full configuration, backup/restore and explicit action
testing access. There are no roles or per-administrator audit identities. Static
assets and minimal `/healthz` are public; `/api/` requests require the credential.
Browser-origin/cross-site checks add protection but do not replace authentication.
The GUI stores the token in tab `sessionStorage`, never a URL or cookie.

The default listener is loopback. An operator can deliberately choose one private
LAN address with authentication retained. HTTP is unencrypted; use a trusted LAN
or independently configured encrypted transport. This project does not configure
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
file input only; it never opens the receiver. Raw captures are not API downloads.

## Secret handling and rotation

The whole state directory and logical backups are private. Backups contain URLs,
IDs, headers and payloads; the token is stored separately and is not exported.
Configuration is not encrypted at rest. Never commit databases, tokens, backups,
production environment files, private RF captures or unredacted screenshots/logs.
Inspect all outgoing history as well as current files before public publication.
Synthetic fixtures use invented identifiers and example endpoints.

Retrieve the installed token only in a trusted local terminal:

```sh
sudo cat /var/lib/rf-button-manager/management.token
```

Do not put it in a URL, process argument, issue or shared log. Avoid shell tracing
around credentials. The API examples read the token file into memory. Review
receiver diagnostics and decoded data before sharing; they can identify hardware.

To rotate a compromised token, stop only this service, securely preserve the old
credential if needed, remove that application's token file, then start the service
to generate a fresh token. Login again using the new local token. Deliberate
replacement must use 64 hexadecimal characters and private ownership/permissions.
Rotate leaked webhook credentials at their destination too; deleting a commit
does not revoke a secret. Rotation is not performed during a normal installation.

## Vulnerability reports

Use the repository's private vulnerability-reporting feature if enabled. Otherwise
request a private contact channel with a minimal non-sensitive description first.
Do not publish a live token, endpoint, database or RF capture in an issue. Include
affected commit/version, sanitized reproduction and impact. The development build
does not promise a response SLA or a maintained stable-version support window.
