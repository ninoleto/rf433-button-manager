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
