# Releases, publication and rollback

## Development versus stable

`0.1.0-dev` is a development build. A public source repository or green automated
CI does not establish receiver compatibility, physical learning, Home Assistant
behavior, USB recovery, soak or reboot behavior. Publish a development prerelease
only with those limitations clearly listed. A stable release requires applicable
rows of [HARDWARE-TESTS.md](HARDWARE-TESTS.md), failures resolved and actual evidence
recorded. Never mark an unperformed test passed, including reboot/USB tests that
require the server operator. Development source publication need not claim any
such validation, but must satisfy the publication authorization's prerequisites.

## Before public publication

1. Inspect current changes, branches/tags and all outgoing history. Preserve the
   imported five-commit development history; do not replace it with a single source
   dump. Do not force-push or overwrite an existing repository.
2. Scan tracked files and historical blobs for credentials, private configuration,
   databases/WAL files, backups, RF captures and production screenshots. Review
   suspected matches privately, reporting only paths and categories. `.gitignore`
   does not protect data already present in history. Revoke any actual credential
   leak and coordinate history remediation rather than silently rewriting history.
3. Run clean-build/check commands and compare reproducible binary hashes. Record
   toolchain, dependency verification, commit, test results and environmental limits.
   Update the progress/report documents; do not inherit old claims as fresh evidence.
4. Verify GitHub authentication with the existing credential:
   `gh api user --jq .login`. It must be `ninoleto` for the intended upstream.
   GitHub authentication is independent of a browser/Google login.
5. Inspect `ninoleto/rf-button-manager` before creation. A 404 can mean absence or
   insufficient access; resolve ambiguity rather than assuming permission. If it
   exists, fetch and inspect its refs/history and integrate preserving existing work.

The GitHub connector can inspect identity/repositories/CI; if it cannot create a
repository or push bundled history, use an authenticated GitHub CLI/Git in an
authorized host terminal. Do not print authentication tokens or alter host security.

Once the agreed publication gates are complete and repository absence is established:

```sh
test "$(gh api user --jq .login)" = ninoleto
gh repo create ninoleto/rf-button-manager --public \
  --description 'Native Linux RF remote learning and guarded webhook automation (development)'
git remote add origin https://github.com/ninoleto/rf-button-manager.git
git push --set-upstream origin main
```

Run `git remote -v` first; preserve an existing remote instead of blindly replacing
it. The creation command is only for a confirmed absent repository. Push reviewed
branches/tags explicitly; do not use `--mirror`, `--all` or `--force` as a shortcut.
No credential belongs in a Git URL. Confirm the remote commit equals the local one:

```sh
git rev-parse HEAD
git ls-remote origin refs/heads/main
gh run list --repo ninoleto/rf-button-manager --commit "$(git rev-parse HEAD)"
```

Inspect and wait on the run for that exact pushed commit. Report workflow/run URL,
conclusion and failing step when applicable. Queued/running/skipped/absent checks
are not a verified green CI result. A successful push alone is not a successful CI
run. The current workflow runs formatting, checksums, race tests, vet, deployment
checks, build and version smoke test on Ubuntu; it has no RF hardware runner and
does not currently run the optional browser suite.

## Package a reproducible development release

Use a clean checkout with Go 1.27.3 and the commands in DEVELOPMENT.md. Update
`internal/app/engine.go`'s `Version` intentionally for a new version and keep release
metadata aligned. Do not replace the user's installed supplied binary merely
because documentation has changed.

```sh
export GOTOOLCHAIN=local
go mod download
go mod verify
make check
make build
python3 scripts/collect-notices.py
mkdir -p dist
git bundle create dist/rf-button-manager.git.bundle --all
```

Review which refs `--all` includes **before distributing this bundle**; in a clean
release checkout only reviewed development refs should exist. For a checkout with
private refs, enumerate approved refs instead. The same rule applies to a source
archive. Package source/tests/CI/deploy/docs plus the executable, application
LICENSE and generated THIRD_PARTY_NOTICES.txt. Exclude `.git`, caches, real state,
private captures and local helper logs. Generate checksums after final copying:

```sh
sha256sum bin/rf-button-manager > dist/BINARY-SHA256SUMS
```

For a ZIP package with an internal `SHA256SUMS`, run checksum verification from
its documented root. State exactly which files it covers; the supplied 0.1.0-dev
manifest covers the executable, while the outer ZIP checksum covers the entire
archive. Hash the completed outer archive separately and publish that SHA-256.
Test extraction/checksums/version from a new directory. No Go compiler is needed
by binary-package consumers.

Retain MIT notices and collect pinned dependency licenses for each binary release.
`rtl_433` is a separately installed project with its own license and is not part
of this executable. Do not imply its license is replaced by this project's MIT.

Create a tag only for the tested commit. A development release must be marked
prerelease on GitHub and visibly retain `-dev` (or another prerelease identifier).
Release notes include supported architecture, toolchain, binary/archive hashes,
changes, installation instructions, schema compatibility, actual test evidence
and all remaining hardware limits. Do not fabricate a stable release date.

## Upgrade/rollback/uninstall

Follow [OPERATIONS.md](OPERATIONS.md): private logical backup; stopped full-state
snapshot when needed; checksum verification; installer dry run; scoped apply;
health/authentication/configuration/receiver checks. Existing manager.env, state
and token are preserved. Only this service is restarted. The prior executable is
saved as `/usr/local/bin/rf-button-manager.previous` on upgrade.

An incompatible schema change requires a matching stopped data snapshot for
rollback; retain the newer data separately. Do not start an older binary against
an unsupported newer schema. Uninstall removes only managed service/binary/rule
files and preserves data, token, account and environment. Permanent deletion is
a separate deliberate decision. Neither upgrade nor uninstall authorizes unrelated
service, automation, Docker, firewall, proxy or USB-hub changes.
