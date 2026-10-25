#!/usr/bin/env bash
set -Eeuo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
binary="$project_dir/bin/rf-button-manager"
apply=0
usage() {
  cat <<'USAGE'
Usage: scripts/install.sh [--binary /absolute/path/rf-button-manager] [--apply]
Default: print the installation/upgrade plan without changing the host.
--apply: install and enable this service (requires root and prior deployment approval).
Existing application data, token and manager.env are preserved.
USAGE
}
while (($#)); do
  case "$1" in
    --apply) apply=1; shift ;;
    --binary) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; binary=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done
[[ -f "$binary" && -x "$binary" ]] || { echo "Build first with make build, or provide an executable using --binary." >&2; exit 1; }
binary=$(realpath -- "$binary")
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || { echo 'This package targets Linux amd64.' >&2; exit 1; }
for source in rf-button-manager.service 70-rf-button-manager.rules manager.env.example; do
  [[ -f "$project_dir/deploy/$source" ]] || { echo "Missing deployment file: $source" >&2; exit 1; }
done

cat <<PLAN
RF Button Manager installation/upgrade plan
Binary source: $binary
Binary target: /usr/local/bin/rf-button-manager
Previous binary: /usr/local/bin/rf-button-manager.previous (replaced on upgrade)
Dedicated system user/group: rfbutton
State: /var/lib/rf-button-manager (0700, preserved)
Settings: /etc/rf-button-manager/manager.env (created only if absent)
Service: /etc/systemd/system/rf-button-manager.service
USB rule: /etc/udev/rules.d/70-rf-button-manager.rules (0bda:2832/2838 only)
Actions: stop only this service during upgrade, reload systemd and udev rules,
         enable/start this service; receiver-only reconnect may be needed for USB access.
No USB trigger, driver blacklist, package install, Home Assistant or Docker changes.
PLAN
sha256sum -- "$binary"
if (( ! apply )); then
  echo 'Dry run complete. No host changes made. Use --apply only after reviewing this plan.'
  exit 0
fi
[[ $EUID -eq 0 ]] || { echo '--apply requires root.' >&2; exit 1; }
[[ -d /run/systemd/system ]] || { echo 'A running systemd system is required.' >&2; exit 1; }
for command in systemctl udevadm pgrep getent useradd groupadd install sha256sum realpath; do
  command -v "$command" >/dev/null || { echo "Required command unavailable: $command" >&2; exit 1; }
done
[[ -x /usr/bin/rtl_433 ]] || { echo '/usr/bin/rtl_433 is missing. Install/verify rtl_433 separately; this script does not change packages.' >&2; exit 1; }

# Fail before any mutation if a different process owns, or could own, the receiver.
bash "$project_dir/scripts/check-receiver-owners.sh"

unit=/etc/systemd/system/rf-button-manager.service
rules=/etc/udev/rules.d/70-rf-button-manager.rules
for target in "$unit" "$rules"; do
  if [[ -e "$target" ]] && ! grep -Fq 'Managed by RF Button Manager installer.' "$target"; then
    echo "Refusing to overwrite unrecognized file: $target" >&2
    exit 1
  fi
done
if [[ -e /usr/local/bin/rf-button-manager && ! -f "$unit" ]]; then
  echo 'Refusing to replace an existing executable without a managed service unit; review ownership manually.' >&2
  exit 1
fi
if getent passwd rfbutton >/dev/null; then
  [[ $(getent passwd rfbutton | cut -d: -f6) == /var/lib/rf-button-manager ]] || { echo 'Existing rfbutton account has an unexpected home; review manually.' >&2; exit 1; }
  getent group rfbutton >/dev/null || { echo 'Existing rfbutton account lacks its matching service group; review manually.' >&2; exit 1; }
else
  getent group rfbutton >/dev/null || groupadd --system rfbutton
  useradd --system --gid rfbutton --home-dir /var/lib/rf-button-manager --no-create-home --shell /usr/sbin/nologin rfbutton
fi
install -d -m 0700 -o rfbutton -g rfbutton /var/lib/rf-button-manager
install -d -m 0750 -o root -g rfbutton /etc/rf-button-manager
if [[ ! -e /etc/rf-button-manager/manager.env ]]; then
  install -m 0640 -o root -g rfbutton "$project_dir/deploy/manager.env.example" /etc/rf-button-manager/manager.env
fi

was_active=0
systemctl is-active --quiet rf-button-manager.service && was_active=1
(( ! was_active )) || systemctl stop rf-button-manager.service
staged=''
recover_on_error() {
  local code=$?
  [[ -z "$staged" ]] || rm -f -- "$staged"
  echo 'Installation failed. Existing data remains. Inspect the error before retrying; see docs/OPERATIONS.md for rollback.' >&2
  if (( was_active )); then
    echo 'The previously running service may be stopped; no unrelated service was changed.' >&2
  fi
  exit "$code"
}
trap recover_on_error ERR
install -d -m 0755 /usr/local/bin
if [[ -f /usr/local/bin/rf-button-manager ]]; then
  install -m 0755 /usr/local/bin/rf-button-manager /usr/local/bin/rf-button-manager.previous
fi
staged=$(mktemp /usr/local/bin/.rf-button-manager.XXXXXX)
install -m 0755 -o root -g root "$binary" "$staged"
mv -f -- "$staged" /usr/local/bin/rf-button-manager
staged=''
install -m 0644 -o root -g root "$project_dir/deploy/rf-button-manager.service" "$unit"
install -m 0644 -o root -g root "$project_dir/deploy/70-rf-button-manager.rules" "$rules"
systemctl daemon-reload
udevadm control --reload-rules
systemctl enable --now rf-button-manager.service
systemctl is-active --quiet rf-button-manager.service
trap - ERR
echo 'Installed and started RF Button Manager. If USB access is denied, arrange a receiver-only reconnect with the operator.'
echo 'Retrieve the login token locally with: sudo cat /var/lib/rf-button-manager/management.token'
echo 'Default UI: http://127.0.0.1:8787 (use an SSH tunnel from Windows).'
