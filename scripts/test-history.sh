#!/usr/bin/env bash
set -Eeuo pipefail
script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
git init --quiet --initial-branch=main "$scratch/original"
printf 'original\n' > "$scratch/original/tracked.txt"
git -C "$scratch/original" add tracked.txt
git -C "$scratch/original" -c user.name=Fixture -c user.email=fixture@example.invalid commit --quiet -m fixture
git -C "$scratch/original" tag v0.0.0-fixture
git -C "$scratch/original" bundle create "$scratch/history.bundle" --all
mkdir "$scratch/extracted"
printf 'user edit\n' > "$scratch/extracted/tracked.txt"
printf 'untracked user file\n' > "$scratch/extracted/private.txt"
bash "$script_dir/restore-history.sh" "$scratch/extracted" "$scratch/history.bundle" > "$scratch/result.log"
[[ $(cat "$scratch/extracted/tracked.txt") == 'user edit' ]]
[[ $(cat "$scratch/extracted/private.txt") == 'untracked user file' ]]
[[ $(git -C "$scratch/extracted" rev-parse HEAD) == "$(git -C "$scratch/original" rev-parse HEAD)" ]]
[[ $(git -C "$scratch/extracted" tag -l) == v0.0.0-fixture ]]
git -C "$scratch/extracted" diff --name-only | grep -qx tracked.txt
before=$(git -C "$scratch/extracted" rev-parse HEAD)
if bash "$script_dir/restore-history.sh" "$scratch/extracted" "$scratch/history.bundle" > /dev/null 2>&1; then
  echo 'Existing repository was not rejected' >&2
  exit 1
fi
[[ $(git -C "$scratch/extracted" rev-parse HEAD) == "$before" ]]
[[ $(cat "$scratch/extracted/tracked.txt") == 'user edit' ]]
echo 'History restoration preserves local edits, untracked files, tags and existing repositories.'
