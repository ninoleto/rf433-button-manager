# Releases, publication and rollback

## Development versus stable

Versions labeled `-dev` are development builds. A public source repository or green automated
CI does not establish receiver compatibility, physical learning, Home Assistant
behavior, USB recovery, soak or reboot behavior. Publish a development prerelease
only with those limitations clearly listed. A stable release requires applicable
rows of [HARDWARE-TESTS.md](HARDWARE-TESTS.md), failures resolved and actual evidence
recorded. Never mark an unperformed test passed, including reboot/USB tests that
require the server operator. Development source publication need not claim any
such validation, but must satisfy the publication authorization's prerequisites.

## Before public publication

Include [COMPATIBILITY.md](COMPATIBILITY.md) and the model-specific community
report template. Never advertise "all Amazon buttons" or infer compatibility
from a frequency/chipset label alone. Distinguish operator reports, raw-recording
software tests and untested devices. The signal inspector cannot promise a code
for an undecoded, encrypted or out-of-band transmission.

1. Inspect current changes, branches/tags and all outgoing history. Preserve the
   complete imported development history (20 commits in the 0.3.6-dev bundle).
   Do not replace it with a source dump, force-push or overwrite an existing repository.
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
its documented root. The 0.3.6-dev `SHA256SUMS` covers every included file except
itself, including the source bundle and release metadata. The outer ZIP checksum
covers the entire archive. Hash the completed archive and publish that SHA-256.
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

## Publish the existing 0.3.6-dev package

The immutable package and tag are separate from later documentation on `main`:

| Item | Expected value |
| --- | --- |
| Tag | `v0.3.6-dev` |
| Packaged commit | `f9b496f3387cd357c1270c28566c97d26cb48dd8` |
| ZIP | `rf-button-manager-0.3.6-dev-linux-amd64.zip` |
| ZIP SHA-256 | `4a55d123df03763ac79f0a6795e3118d65e0d3ac899126a22d8796f466b7df13` |
| Executable SHA-256 | `f7c4fad9f61a21918bfc7adef756c8a5a421fd8760b8669b7425df98aaafd66f` |

Verify the original ZIP, extract freshly, verify its `SHA256SUMS`, clone its bundle
with `--branch main` and compare HEAD and `release.json` before publication. Do not
repackage it with newer docs or point its tag at the docs commit. Scan the ZIP's
contents as well as every outgoing historical blob, commit message and screenshot.
The checked-in screenshots are historical synthetic demo images, not production.

After checking existing remote refs and completing review, create the tag only
if absent; if present, verify its peeled commit and preserve it. Example for a new tag:

```sh
git tag -a v0.3.6-dev f9b496f3387cd357c1270c28566c97d26cb48dd8 \
  -m 'RF Button Manager 0.3.6-dev: packaged development release'
git push origin main refs/tags/v0.3.6-dev
git ls-remote origin refs/heads/main refs/tags/v0.3.6-dev 'refs/tags/v0.3.6-dev^{}'
gh run list --repo ninoleto/rf-button-manager \
  --commit f9b496f3387cd357c1270c28566c97d26cb48dd8
```

The workflow runs on tag pushes as well as branch pushes. Inspect the exact
release-commit run and its jobs with `gh run view RUN_ID --repo
ninoleto/rf-button-manager`. It must finish with `success` **before** uploading
the ZIP. Also verify CI for the final `main` commit; a passing documentation commit
alone does not validate the packaged release. If CI fails, keep the asset unpublished
and report the failing step; do not move the immutable release tag to hide failure.

Write reviewed release notes to a local file. For a confirmed absent release,
after the release-commit CI succeeds:

```sh
gh release create v0.3.6-dev /path/to/rf-button-manager-0.3.6-dev-linux-amd64.zip \
  --repo ninoleto/rf-button-manager --verify-tag --prerelease \
  --title 'RF Button Manager 0.3.6-dev' --notes-file /path/to/release-notes.md
```

Do not overwrite an existing release asset. Download the published ZIP into a new
directory and verify its hash, the remote tag/commit, public visibility and
prerelease flag. Notes must include Linux amd64/runtime prerequisites, checksum
and installation/upgrade links, schema-2 compatibility, the three operator-tested
home button/webhook assignments and accepted interface, plus unverified physical
multi-tuner, USB/hub recovery, soak and broader compatibility. No universal Amazon
button support is claimed. The ZIP's older documentation and conservative
`hardware_validation` metadata are retained; current notes explain newer evidence.

## Upgrade/rollback/uninstall

Follow [OPERATIONS.md](OPERATIONS.md): private logical backup; stopped full-state
snapshot when needed; checksum verification; installer dry run; scoped apply;
health/authentication/configuration/receiver checks. Existing manager.env, state
and token are preserved. Only this service is restarted. The prior executable is
saved as `/usr/local/bin/rf-button-manager.previous` by install.sh. The prebuilt
update helper instead creates a private stopped-state snapshot under
`/var/lib/rf-button-manager-upgrades/` and restores it automatically on failed
verification. The package includes `release.json` with binary/bundle hashes and
commit, plus `SHA256SUMS`; verify extraction and `scripts/test-update-prebuilt.py`.

An incompatible schema change requires a matching stopped data snapshot for
rollback; retain the newer data separately. Do not start an older binary against
an unsupported newer schema. Uninstall removes only managed service/binary/rule
files and preserves data, token, account and environment. Permanent deletion is
a separate deliberate decision. Neither upgrade nor uninstall authorizes unrelated
service, automation, Docker, firewall, proxy or USB-hub changes.

## Build the reviewed binary archive

With a clean checkout whose `main` points at the tested commit, run the clean build
and test steps above, collect notices, and package:

```sh
python3 scripts/collect-notices.py --output /tmp/rfb-notices.txt
python3 scripts/package-release.py --notices /tmp/rfb-notices.txt --output dist
```

The ZIP contains tracked source, executable, complete main/HEAD Git bundle,
notices, `release.json` and a SHA256SUMS covering every included file except itself.
The packager refuses dirty source or an existing archive. Extract in a fresh
directory, verify SHA256SUMS, clone the bundle with `--branch main`, and compare
its commit to release.json. Build from that clean clone and compare the executable
hash before distribution. The archive's outer SHA-256 is printed by the packager.
