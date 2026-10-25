#!/usr/bin/env bash
# Known competing decoders only; inspect actual USB FD owners separately on host.
set -Eeuo pipefail
for receiver_process in rtl_433 rtl_tcp rtl_test; do
  while read -r pid; do
    [[ -n "$pid" ]] || continue
    if ! grep -Eq '/rf-button-manager\.service(/|$)' "/proc/$pid/cgroup" 2>/dev/null; then
      echo "Competing $receiver_process process PID $pid. Inspect it and arrange an approved handover; it was not stopped." >&2
      exit 1
    fi
  done < <(pgrep -x "$receiver_process" || true)
done
