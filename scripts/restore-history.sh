#!/usr/bin/env bash
# Attach bundled history to extracted source without checking out or replacing files.
set -Eeuo pipefail
if [[ ${1:-} == -h || ${1:-} == --help || $# -gt 2 ]]; then
  echo 'Usage: scripts/restore-history.sh [workspace [bundle]]'
  echo 'Defaults: current directory and workspace/rf-button-manager.git.bundle.'
  echo 'Refuses an existing .git; preserves every workspace file and edit.'
  [[ $# -le 2 ]] && exit 0 || exit 2
fi
target=$(realpath -- "${1:-.}")
bundle=$(realpath -- "${2:-$target/rf-button-manager.git.bundle}")
[[ -d "$target" && -f "$bundle" ]] || { echo 'Workspace and Git bundle required.' >&2; exit 1; }
[[ ! -e "$target/.git" && ! -L "$target/.git" ]] || { echo 'Existing .git preserved; inspect/import history manually.' >&2; exit 1; }
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
git init --quiet --bare "$scratch/verify.git"
git -C "$scratch/verify.git" bundle verify "$bundle"
head=$(git bundle list-heads "$bundle" refs/heads/main | awk '{print $1}')
[[ "$head" =~ ^[0-9a-f]{40}$ ]] || { echo 'Bundle must contain one main branch.' >&2; exit 1; }
git init --quiet --initial-branch=main "$target"
git -C "$target" fetch --no-tags "$bundle" 'refs/heads/*:refs/remotes/bundle/*' 'refs/tags/*:refs/tags/*'
git -C "$target" update-ref refs/heads/main "$head"
# read-tree writes only the index. Never use checkout/reset --hard here.
git -C "$target" read-tree HEAD
echo 'Bundled main history restored. Workspace files were not checked out or overwritten.'
git -C "$target" status --short --branch
