#!/usr/bin/env bash
# Read-only inventory. Does not open the receiver, read webhook configs or stop services.
set -uo pipefail
section() { printf '\n%s\n' "$1"; }
section 'Operating system'
if [[ -r /etc/os-release ]]; then
  awk -F= '$1 == "PRETTY_NAME" || $1 == "VERSION_ID" {print}' /etc/os-release
fi
uname -srmo
section 'rtl_433 executable and version (no receiver opened)'
if command -v rtl_433 >/dev/null; then
  command -v rtl_433
  rtl_433 -c /dev/null -V 2>&1
  section 'rtl_433 help (no receiver opened)'
  rtl_433 -c /dev/null -h 2>&1
else
  echo 'rtl_433 not found in PATH'
fi
section 'USB receivers in sysfs'
found=0
for node in /sys/bus/usb/devices/*; do
  [[ -r "$node/idVendor" && -r "$node/idProduct" ]] || continue
  vendor=$(<"$node/idVendor")
  product=$(<"$node/idProduct")
  [[ "$vendor" == 0bda && ( "$product" == 2832 || "$product" == 2838 ) ]] || continue
  found=1
  printf 'sysfs=%s vendor=%s product=%s\n' "${node##*/}" "$vendor" "$product"
  for attr in manufacturer product serial; do
    if [[ -r "$node/$attr" ]]; then
      printf '  %s=' "$attr"
      tr -cd '[:print:]\n' < "$node/$attr"
    fi
  done
done
(( found )) || echo 'No supported receiver currently visible'
section 'Competing receiver-related processes (no arguments or environment)'
if command -v pgrep >/dev/null; then
  for name in rtl_433 rtl_tcp rtl_test; do
    pgrep -l -x "$name" || true
  done
fi
section 'Relevant systemd unit names and states'
if command -v systemctl >/dev/null && [[ -d /run/systemd/system ]]; then
  systemctl list-units --type=service --all --plain --no-legend --no-pager 2>/dev/null |
    awk '/rtl[_-]?433|rf-button-manager/ {print $1, $2, $3, $4}'
else
  echo 'Running systemd manager unavailable'
fi
section 'Known decoder configuration filenames (contents intentionally omitted)'
for config_dir in /etc/rtl_433 /usr/local/etc/rtl_433 "${XDG_CONFIG_HOME:-$HOME/.config}/rtl_433"; do
  [[ -d "$config_dir" ]] || continue
  find "$config_dir" -maxdepth 2 -type f -name '*.conf' -print 2>/dev/null
done
echo 'Read-only inspection complete. Review before sharing; USB serials identify your receiver.'
