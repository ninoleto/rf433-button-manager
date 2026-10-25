# Operations and troubleshooting

## Deployment boundary

Use the test records to distinguish observed deployment from unperformed physical
checks. Apply a reviewed concrete plan only within the operator's authorization;
existing authorization for that scope does not require a redundant confirmation. Run
`scripts/inspect-zorin.sh`; it reads system details, `rtl_433` version/help, supported
USB IDs/serials, process names and config filenames. It does not open the receiver,
read webhook configurations, install packages or stop anything. Review the output
before sharing it because serial numbers identify hardware.

Preserve existing verified decoder configuration during upgrades. Normal learning
uses the browser to capture, analyze and validate supported signals; it never asks
the operator to recover terminal history or copy decoder strings. Analyzer guesses
alone are not proof. Keep real captures and configuration private. If another job
currently owns the receiver, agree on its exact handover before stopping it. The
installer refuses competing rtl_433, rtl_tcp and rtl_test processes outside its unit.
Inspect other SDR/RF software and actual USB file-descriptor ownership too; a
process-name check alone cannot identify every possible receiver owner. A sandbox
that hides host processes cannot establish that the receiver is free.

## Installed files

The supplied binary needs no compiler. Verify the archive and internal binary
checksum, architecture, free space on `/var/lib`, existing rtl_433 version,
receiver owners, serial uniqueness and port availability first. A restricted
shell may expose USB sysfs while hiding host processes/network/systemd; such a
check cannot establish exclusive receiver ownership. Use an authorized host
terminal for those checks. Do not repair sudo ownership or weaken host security
to work around an agent sandbox. A dry run never installs packages or opens USB.

| Path | Purpose | Upgrade / uninstall behavior |
| --- | --- | --- |
| `/usr/local/bin/rf-button-manager` | Application executable | Atomic replacement / removed |
| `/usr/local/bin/rf-button-manager.previous` | Previous executable | Replaced on each upgrade / removed |
| `/etc/systemd/system/rf-button-manager.service` | Owned systemd unit | Replaced / removed |
| `/etc/udev/rules.d/70-rf-button-manager.rules` | Receiver group access | Replaced / removed |
| `/etc/rf-button-manager/manager.env` | Listener/decoder binary settings | Preserved / preserved |
| `/var/lib/rf-button-manager/manager.db` | SQLite application data | Preserved / preserved |
| `/var/lib/rf-button-manager/management.token` | Management credential | Preserved / preserved |

The dedicated `rfbutton` system account has no login shell and owns the private
state directory. The installer preserves it on uninstall so retained files keep
their owner. Inspect the unit or use a systemd drop-in for service customization;
the managed unit itself is replaced on upgrade.

`rtl_433` is not bundled or upgraded. The initial target is the existing 23.11
installation. `/etc/rf-button-manager/manager.env` can override its absolute path.
The service's `-c /dev/null` decoder invocation intentionally does not inherit
unrelated `rtl_433` configurations. Use **Learn** for supported signals; existing
advanced decoder settings are preserved. See [LEARNING.md](LEARNING.md).

## Start, stop and access

```sh
sudo systemctl status rf-button-manager.service --no-pager
sudo journalctl -u rf-button-manager.service --since '10 minutes ago' --no-pager
curl --fail http://127.0.0.1:8787/healthz
sudo systemctl restart rf-button-manager.service
```

`/healthz` answers application liveness; it does not promise receiver connectivity
or successful automation. Inspect the authenticated interface for those states.
No RF transmissions for hours is normal and must not be treated as a fault.

The default listener is `127.0.0.1:8787`. An SSH tunnel from Windows provides
encrypted management access without opening another LAN port:

```powershell
ssh -N -L 8787:127.0.0.1:8787 nino@ZORIN
```

For deliberate direct LAN access, set `RFB_LISTEN` in the owned environment file to
ZORIN's actual private address, such as `192.168.1.20:8787`, then restart only this
service. Replace the example address with the verified server address. The token
remains required. Plain HTTP on the LAN exposes credentials to an on-path observer;
use the tunnel or an explicitly configured HTTPS reverse proxy. Never use a public
bind, router port forward or remote tunnel for this management service by default.
Proxy configuration and firewall changes are separate approved tasks. Keep
same-origin browser checks intact; do not add permissive CORS headers.

For an explicitly configured HTTPS reverse proxy, keep `RFB_LISTEN` on loopback
and set `RFB_PUBLIC_ORIGIN` to the exact browser origin, for example
`https://rf.home.example` or `https://rf.home.example:8443`. Replace the example
with the configured hostname and include the port only when it is part of the
browser origin. The value must not contain a path, query, fragment or credentials.
The service passes it as `--public-origin`. Leave it empty when using an SSH
tunnel or direct HTTP access. An HTTPS browser origin differs from the backend's
HTTP origin, so proxy access is rejected unless this explicit trusted origin is
configured. The token is still required; this setting does not publish the
application, configure TLS or trust arbitrary forwarding headers.

## Webhook concurrency and delivery

The dispatcher permits at most **eight simultaneous requests across different
buttons**, with **one pending request per button**. If a button already has a
pending request or all eight slots are occupied, an additional action is rejected
and logged. It is not queued for later, and actions are not replayed after restart.
This prevents an unavailable endpoint from building up stale physical actions.

Debounce remains an independent guard against repeated RF frames from the same
press. There are no automatic HTTP retries, including after a timeout: the remote
server may already have performed the action. Inspect the destination's history
before deliberately testing again. A response status reports HTTP delivery; it
does not establish whether a physical device reached its intended state.

## USB permissions and recovery

The udev rule grants `rfbutton` access only to USB IDs `0bda:2832` and `0bda:2838`.
It does not depend on USB bus numbers or hub ports. The installer reloads rules
without triggering all USB devices; unplug/replug the receiver once after initial
installation. Moving it between ports should then require no configuration change.

Choose the exact receiver serial when it is unique. With no reliable serial,
automatic selection is permitted only while one compatible receiver is present.
With duplicate serials or multiple ambiguous receivers, acquisition must stop and
ask for a deliberate resolution rather than controlling the wrong device. Do not
silently rewrite EEPROM serials. Changing a serial with another utility is a
separate hardware operation requiring deliberate authorization.

The application owns one decoder child, waits between failures and re-enumerates
devices. systemd restarts the application if it exits unexpectedly and stops the
whole service process group on shutdown. `PrivateDevices=no` is intentional: a
private device namespace would hide the receiver and hot-plug nodes. Do not add a
static `DeviceAllow` entry with a USB minor number that changes after reconnect.

After a configured restart, inspect `systemctl show rf-button-manager.service`
for `MainPID`, `ControlGroup`, `ActiveState` and `NRestarts`. Verify one decoder
child's parent PID and service cgroup without publishing full arguments. Check
the authenticated receiver state over at least a heartbeat interval. An enabled
unit and stable HTTP process do not establish sample acquisition; `connected`
requires statistics/RF JSON. Real decoding requires a known physical press.
Keep actions disabled until deliberately tested with the operator.

A specific LAN bind depends on that address being assigned at startup and staying
assigned. If DHCP changes it or boot networking is late, inspect the application
journal and its startup retries; do not claim reboot validation from `is-enabled`.
Changing host network allocation is outside this installer.

## Backups and restore

Use **Backup** in the authenticated GUI for a consistent logical configuration
export while the application is running. It includes remote mappings, webhooks
and settings. Treat it as secret material: URLs, headers and payloads may contain
credentials. Store it privately and encrypt external copies. The logical export
is not a complete event-log archive and does not export the management token.

Use **Restore** to upload the backup. The service validates it and replaces the
configuration atomically; restored actions are disabled for inspection. Review
receiver settings and endpoints before re-enabling buttons. Export the current
configuration before replacing it.

For an exact rollback snapshot, stop **only this service**, archive its whole
state directory with numeric ownership and permissions, and restart it:

```sh
sudo systemctl stop rf-button-manager.service
sudo sh -c 'umask 077; tar --numeric-owner -C /var/lib -czf /root/rf-button-manager-state.tar.gz rf-button-manager'
sudo systemctl start rf-button-manager.service
```

Do not copy only a live SQLite database file: its WAL may contain recent writes.
Keep backups outside the application's state directory. The above command replaces
an existing same-named archive; choose a fresh dated filename when retaining history.

## Upgrade and rollback

For a full-state restore, stop only this service, preserve its current entire
state directory at a fresh private location, and restore the matching archive
under `/var/lib` with numeric ownership and private permissions. Restore the
matching environment file separately if needed; it is outside the state archive.
Verify the restored `rfbutton` ownership and installed binary/schema compatibility
before starting. Do not overlay only `manager.db` over live WAL/SHM files. A
full-state snapshot restores the token it contains and may restore enabled actions;
plan the first start accordingly. GUI logical restore instead disables actions.

1. Read the new release notes and export a configuration backup. Take a stopped
   state-directory snapshot when a schema migration is involved.
2. Build/download the new executable and verify its published SHA-256.
3. Run the installer without `--apply` and review the plan.
4. Apply the approved upgrade. Only this service is stopped; binary replacement is
   atomic, and the data directory and environment file remain intact.
5. Check service health, saved mappings, one safe webhook and one receiver reconnect.

An install failure is reported, and the data is retained; it is not silently
declared successful. A previously active service may remain stopped after failure.
Inspect the journal before deciding whether to start it or roll back.

To roll back an executable without a schema change:

```sh
sudo systemctl stop rf-button-manager.service
sudo install -m 0755 /usr/local/bin/rf-button-manager.previous /usr/local/bin/rf-button-manager.rollback
sudo mv -f /usr/local/bin/rf-button-manager.rollback /usr/local/bin/rf-button-manager
sudo systemctl start rf-button-manager.service
```

If a schema changed incompatibly, restore the matching stopped snapshot as well;
do not run an old binary against an unsupported new schema. Retain the newer data
directory separately before replacement. Future releases must document migrations.

## Uninstall

```sh
bash scripts/uninstall.sh
# Review the plan, then apply when intended:
sudo bash scripts/uninstall.sh --apply
```

The uninstaller stops and removes only this application, preserving its data,
token, environment file and service account. Replug the receiver to refresh USB
permissions after rule removal. Permanently deleting retained state is a separate
manual decision; no purge flag is provided.

## Troubleshooting

| Symptom | Investigation |
| --- | --- |
| Disconnected | Check USB visibility and power, then UI recovery events. No bus path should need editing. |
| Permission denied | Replug after rule installation; verify `rfbutton` group ownership on the newly created device node. |
| Device busy | Inspect named processes and their owning units. Stop only the explicitly approved previous receiver service. |
| Ambiguous receiver | Remove the unintended compatible receiver or configure a genuinely unique serial. |
| Connected, no decoded events | A quiet channel is normal. Choose Learn and follow its capture instructions; inspect Diagnostics if a press is unsupported. |
| Candidate changes between presses | Do not confirm. Inspect actual frames, bit length, variable fields and protocol support. |
| One held press runs repeatedly | Increase the release/debounce interval and collect timing evidence; repeat the held-button acceptance test. |
| HTTP 4xx/5xx | Verify endpoint, method, Home Assistant automation and headers; response means transport reached the endpoint, not successful action. |
| Timeout/outcome unknown | Inspect Home Assistant history before testing again. The first action may have executed. |
| Webhook busy/rejected | One request is already pending for that button, or eight requests are active globally. Check slow endpoints; rejected actions are not queued or retried. |
| GUI authorization failure | Use the current token from the service's data directory; avoid embedding tokens in URLs. |
| HTTPS proxy origin rejected | Set the exact trusted `RFB_PUBLIC_ORIGIN` including any non-default port; keep token authentication and same-origin checks enabled. |
| Service restart limit reached | Inspect the journal, fix the startup cause, then `systemctl reset-failed rf-button-manager.service` and start it. |
| Database write failure | Check free space, permissions and storage health. Preserve the database and logs; do not delete the database as a first repair. |

## Learning diagnostics

Use **Learn** and expand **Diagnostics and receiver status**. The application
reports whether it acquired pulses, how many candidates/replays were checked, and
whether it reused a profile. **Retry learning** starts a fresh three-press attempt
after receiver restoration; it never silently saves a partial candidate. No user
receiver command or service shutdown is needed for normal learning.

No pulse capture means no usable pulse packet reached analysis; sample acquisition
alone is not proof of a transmission. Malformed capture means parser input failed:
retain the specific row/header/boundary reason from Diagnostics for investigation.
FSK-only input is excluded from OOK learning and may come from another transmitter;
it does not identify the intended remote as unsupported. Complete OOK input with
no validated decoder is a separate replay outcome. Inconsistent means different
identities were received; keep other remotes idle. A rolling or
encrypted remote remains unsupported, even if several presses look superficially
similar. Captures are temporary/private, bounded and deleted after analysis.

The dialog's capture receiver snapshot is distinct from current normal reception.
Do not infer capture success from a restored **connected** indicator. Counters and
the original rejection survive cleanup and are stored as bounded event metadata.
See [CAPTURE-INVESTIGATION.md](CAPTURE-INVESTIGATION.md) for the corrected 0.2.0
failure classification. No extra live receiver or terminal capture is needed.

An unsupported modulation may need a deliberate developer extension and new
fixtures/tests; see [DEVELOPMENT.md](DEVELOPMENT.md) and [LEARNING.md](LEARNING.md).
Do not run a second live receiver process or convert an analyzer suggestion into
an asserted working decoder. Physical USB recovery tests remain separate and
require operator participation; no hub reset or reboot is part of learning.

References: [rtl_433 operation and analysis](https://triq.org/rtl_433/OPERATION.html),
[systemd service semantics](https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html),
[Home Assistant webhooks](https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger).

## 0.2 development upgrade and rollback

The browser capture workflow changes logical configuration from schema 1 to 2.
Startup validates and migrates the existing configuration without dropping remotes,
mappings, legacy decoder settings, profiles or token. Schema-1 logical backups are
accepted and migrated on restore; restores still disable all actions. An older
0.1 binary rejects schema 2. Before upgrading, take a stopped application-state
snapshot on internal storage. A rollback to 0.1 requires that matching snapshot
as well as the old executable; retain the newer state separately. Do not edit
SQLite or delete profiles to force an old executable to start.
