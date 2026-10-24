#!/usr/bin/env bash
set -Eeuo pipefail
case "${1:-}" in
  '') apply=0 ;;
  --apply) [[ $# == 1 ]] || exit 2; apply=1 ;;
  -h|--help) echo 'Usage: scripts/uninstall.sh [--apply]'; exit 0 ;;
  *) echo 'Usage: scripts/uninstall.sh [--apply]' >&2; exit 2 ;;
esac
cat <<'PLAN'
Uninstall plan:
- Stop and disable only rf-button-manager.service.
- Remove the installed unit, USB rule, executable and previous executable.
- Reload systemd and udev rules. Do not trigger USB devices.
- Preserve /var/lib/rf-button-manager, /etc/rf-button-manager and the rfbutton account.
PLAN
(( apply )) || { echo 'Dry run complete. Use --apply to perform these changes.'; exit 0; }
[[ $EUID -eq 0 ]] || { echo '--apply requires root.' >&2; exit 1; }
unit=/etc/systemd/system/rf-button-manager.service
rules=/etc/udev/rules.d/70-rf-button-manager.rules
for target in "$unit" "$rules"; do
  if [[ -e "$target" ]] && ! grep -Fq 'Managed by RF Button Manager installer.' "$target"; then
    echo "Refusing to remove unrecognized file: $target" >&2
    exit 1
  fi
done
[[ -f "$unit" ]] || { echo 'Managed unit not found; refusing to remove binaries without installation evidence.' >&2; exit 1; }
systemctl disable --now rf-button-manager.service
rm -f -- "$unit" "$rules" /usr/local/bin/rf-button-manager /usr/local/bin/rf-button-manager.previous
systemctl daemon-reload
udevadm control --reload-rules
echo 'Uninstalled. Configuration, database, token, backups and service account remain for recovery.'
