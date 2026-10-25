#!/usr/bin/env bash
# Verify that the default installer/uninstaller paths cannot call privileged mutations.
set -Eeuo pipefail
script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf -- "$test_dir"' EXIT
mkdir "$test_dir/mock-bin"
export RFB_TEST_MUTATION_LOG="$test_dir/mutation.log"
for tool in install mv systemctl udevadm useradd groupadd; do
  cat > "$test_dir/mock-bin/$tool" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "${0##*/}" >> "$RFB_TEST_MUTATION_LOG"
echo "Unexpected mutation command: ${0##*/}" >&2
exit 97
MOCK
  chmod +x "$test_dir/mock-bin/$tool"
done
PATH="$test_dir/mock-bin:$PATH" bash "$script_dir/install.sh" --binary /bin/true > "$test_dir/install.txt"
grep -q 'Dry run complete' "$test_dir/install.txt"
PATH="$test_dir/mock-bin:$PATH" bash "$script_dir/uninstall.sh" > "$test_dir/uninstall.txt"
grep -q 'Dry run complete' "$test_dir/uninstall.txt"
[[ ! -e "$RFB_TEST_MUTATION_LOG" ]] || { echo 'Dry run called a mutation command' >&2; exit 1; }
if bash "$script_dir/install.sh" --binary "$test_dir/missing" > /dev/null 2>&1; then
  echo 'Missing executable was not rejected' >&2
  exit 1
fi
if bash "$script_dir/install.sh" --unexpected-option > /dev/null 2>&1; then
  echo 'Unknown installer option was not rejected' >&2
  exit 1
fi
if bash "$script_dir/uninstall.sh" --unexpected-option > /dev/null 2>&1; then
  echo 'Unknown uninstaller option was not rejected' >&2
  exit 1
fi
echo 'Deployment dry-run and argument tests passed.'

# Each known competing decoder must fail before installation; no kill is invoked.
cat > "$test_dir/mock-bin/pgrep" <<'MOCK'
#!/usr/bin/env bash
if [[ ${2:-} == "${RFB_TEST_OWNER:-}" ]]; then
  echo "$PPID"
fi
MOCK
chmod +x "$test_dir/mock-bin/pgrep"
for owner in rtl_433 rtl_tcp rtl_test; do
  if PATH="$test_dir/mock-bin:$PATH" RFB_TEST_OWNER="$owner" bash "$script_dir/check-receiver-owners.sh" > "$test_dir/owner.txt" 2>&1; then
    echo "Competing $owner was not rejected" >&2
    exit 1
  fi
  grep -q "Competing $owner process PID" "$test_dir/owner.txt"
done
PATH="$test_dir/mock-bin:$PATH" RFB_TEST_OWNER='' bash "$script_dir/check-receiver-owners.sh"
echo 'Known competing rtl_433/rtl_tcp/rtl_test ownership checks passed.'
