# Upgrade and name migration

**0.4.0-test.2 is a private validation build.** Test it on the agreed ZORIN host
before stable publication. A working RC1 installation does not update itself when
a new ZIP is downloaded or published.

## Apply the supplied package

Leave the old service running. Finish or cancel learning/inspection and any
pending network change. Keep the existing package and development checkout.
Download the new ZIP and its matching `.sha256` into one folder on the Linux
host. Open a terminal there and run:

```sh
(
  set -e
  sha256sum -c rf433-button-manager-0.4.0-test.2-linux-amd64.zip.sha256
  rfb_update_dir=$(mktemp -d "$HOME/rf433-button-manager-update.XXXXXX")
  unzip -q rf433-button-manager-0.4.0-test.2-linux-amd64.zip -d "$rfb_update_dir"
  cd "$rfb_update_dir/rf433-button-manager"
  sha256sum --quiet -c SHA256SUMS
  ./bin/rf433-button-manager --version
  sudo python3 scripts/update-prebuilt.py
  printf 'Verified package folder: %s\n' "$PWD"
)
```

Review the printed plan. Enter the printed package folder and apply it:

```sh
sudo python3 scripts/update-prebuilt.py --apply
```

The updater records which buttons were enabled, temporarily disables their actions
through the API, stops only this application and makes a private stopped-state
snapshot. It does not need Go or a rebuild. It installs the supplied binary,
verifies its live version, restarts the service again to check persistence, checks
authentication and decoder ownership, then restores the original enabled flags.
If a receiver was connected before updating, it must reconnect for verification
to pass. No test webhook or USB reset is performed by the updater. Once the old
action flags are restored, ordinary physical presses can trigger their actions.

Wait for **Update verified**. Refresh the browser and check **0.4.0-test.2**, your
remotes, the selected receiver and action flags. Your existing browser URL and
token stay the same unless you deliberately change them in Settings afterward.
Keep the printed snapshot path for recovery. Run the [acceptance plan](TESTING-0.4.md).

## What is renamed

| Item | Previous installation | Current installation |
| --- | --- | --- |
| Executable | `/usr/local/bin/rf-button-manager` | `/usr/local/bin/rf433-button-manager` |
| Service | `rf-button-manager.service` | `rf433-button-manager.service` |
| Environment directory | `/etc/rf-button-manager` | `/etc/rf433-button-manager` |
| Private state directory | `/var/lib/rf-button-manager` | `/var/lib/rf433-button-manager` |
| USB rule filename | `70-rf-button-manager.rules` | `70-rf433-button-manager.rules` |
| New upgrade snapshots | old upgrade directories are retained | `/var/lib/rf433-button-manager-upgrades` |

The updater detects the legacy managed service, stops/disables it, moves its state
and environment, installs the new unit/rule filename and starts/enables only the
new service. The numeric `rfbutton` account/group stays unchanged; its home moves
to the new state path. Rules are reloaded without triggering or reconnecting USB.
The old binary/unit/rule are retained in the snapshot. Configuration schema remains
2; identities, profiles, webhook fields and token are not rewritten for branding.

Existing archives, old development checkouts and historical backup directories
are preserved. Do not use the fresh installer to create a second service beside
the old one. It refuses that situation and points to this updater instead.

## Conditions that stop the migration

Both old/new paths present, symlinked application paths, a customized legacy unit
or drop-in, a custom account home, insufficient space, an unrelated receiver
process, an inactive service, active learning, a pending network change, invalid
authentication or a checksum mismatch stop the updater for review. These checks
do not grant permission to remove the conflicting files or stop another service.

The dry run is read-only. During apply, a failed startup/verification attempts to
restore the old executable, data, paths, unit, account home and original action
flags. It retains the failed state and reports failure. An OS/disk/service failure
can also prevent rollback: inspect the specific error and application journal
instead of treating an error exit as a successful recovery. The original flags
are saved privately as `enabled-flags.json` in the snapshot.

## Rollback

Automatic rollback is exercised by isolated migration tests, including a failure
halfway through moving directories. A deliberate rollback after a successful
upgrade must use **the matching snapshot**, not just an old executable.

A successful migration snapshot contains `binary`, `state/`, `manager.env`,
`service`, and `usb-rule` when one existed. It also retains retired files, original
enabled flags and `update-report.json`. `state/` was copied only after stopping the
service; it includes the old token and complete SQLite/WAL files with ownership.

For a deliberate rollback, use an operator-reviewed procedure:

1. Finish learning and disable actions in the current GUI. Stop only
   `rf433-button-manager.service`. Preserve the current state/environment/unit/
   binary in a **new private directory**; do not discard changes made after upgrade.
2. If `name_migrated` is true in the snapshot report, disable the new unit and move
   its active paths out of the way. Restore `state/` to `/var/lib/rf-button-manager`,
   `binary` to `/usr/local/bin/rf-button-manager`, `manager.env` to
   `/etc/rf-button-manager/manager.env`, `service` to the old unit path, and
   `usb-rule` to its old filename when present. Preserve numeric ownership/modes.
   Restore `rfbutton`'s home to `/var/lib/rf-button-manager`. Remove only the newer
   application unit/rule after preserving them; never leave both services enabled.
3. If `name_migrated` is false, restore those files to the current `rf433-` paths
   instead. Do not rename an installation that was already using the new paths.
4. Reload systemd and udev rules **without a USB trigger**, enable/start only the
   restored application service and verify its version, authentication, receiver
   and mappings. A snapshot rollback restores its older token/TLS/network settings.
5. The snapshot was taken with actions disabled. Review the private original flag
   record and restore only intended action states through the GUI/API. Do not edit
   SQLite or blindly restore flags from an unrelated snapshot.

Do not run these recovery operations against a live SQLite directory or mix files
from different snapshots. The release remains a test build until migration and
rollback have also been validated on the real host with the operator.
