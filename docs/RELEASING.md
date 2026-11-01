# Packaging and release procedure

This branch prepares **0.4.0** as a regular release. The maintainer is closing the
current development cycle after testing his available equipment. The final changes
from test.6 are stacked action fields, 24-hour display and release documentation.
Hardware observations and remaining limits are in HARDWARE-TESTS.md; a regular
release does not establish universal compatibility or a measured success rate.
Historical releases remain identified by their original tags and archives.

## Before packaging

1. Read AGENTS.md and review the diff against the last published commit. Preserve
   installed user data and avoid real USB/webhooks during software tests.
2. Run `make check` with the checksum-pinned public RF fixtures. Run the optional
   private capture replay without committing its input. Run the real browser
   suite where Chromium is available and report any blocked check explicitly.
3. Update TESTING-0.4.md with actual results, not intended outcomes. Physical
   receiving, webhook results, multiple tuners, USB recovery and soak tests are
   separate evidence. A software pass does not authorize a stable label.
4. Review current files and complete outgoing history for secrets and private RF
   identifiers. Exclude databases, tokens, private environment/TLS files, backups,
   captures, toolchains and scratch helper files.
5. Commit reviewed changes. Build with the exact documented Go toolchain and
   Makefile flags. Collect notices without changing dependencies.

```sh
make check
make build
python3 scripts/collect-notices.py --output dist/THIRD_PARTY_NOTICES.txt
python3 scripts/package-release.py --notices dist/THIRD_PARTY_NOTICES.txt --output dist
```

The packager requires clean tracked source, records the tested HEAD commit and
binary hash, and includes source, full reachable commit history, license notices
and a SHA256SUMS covering every packaged file except itself. The bundle exposes
that exact commit as `main`, independent of the development branch name.

Write a companion `.zip.sha256` using the archive's basename. Verify extraction,
all internal checksums and `--version` from a fresh directory. Clone the included
bundle with `git clone --branch main`, build from it using the same toolchain and
compare the executable SHA-256. Do not replace an already distributed archive.

## Operator observations and final review

The maintainer installed test.6, tested all six remote buttons, compared recording
on/off and used the app from desktop and phone. Retain the intermittent-miss
observations in [TESTING-0.4.md](TESTING-0.4.md). Additional hardware, long-duration
soak and the full formal recovery matrix have not been tested. The maintainer's
decision to ship does not turn those open rows into passes. Run the final software
checks and build the release at its own committed/tagged source.

## GitHub publication

Use the maintainer's existing GitHub authentication and verify the account and
repository. Inspect existing refs/releases, preserve remote work and never
force-push or move a distributed tag. CI must pass for the exact packaged commit,
not just a later documentation commit. A private test artifact has no published
CI result unless its commit was actually submitted to GitHub Actions.

Prepare a release body explaining the purpose, prerequisites, tested equipment,
first-install guide, upgrade guide, token location, known limitations, exact
asset names/checksums and commit/CI links. Use the matching tag's documentation.
Publish the final `v0.4.0` as a regular release and select it as Latest when the
maintainer authorizes publication. Keep separately named test builds marked as
development/prerelease if they are published. Upload
only after the checks and publication authorization are satisfied, then download
the public ZIP anonymously and verify its checksum. Publishing never updates an
operator's running server automatically.
