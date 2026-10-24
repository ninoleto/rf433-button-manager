# Verification report — 2026-10-25

Build: RF Button Manager 0.1.0-dev, Linux amd64, Go 1.27.3.
Environment: isolated development workspace, not ZORIN.

## Passed

- 42 top-level Go tests (including subtests), `go test -race -count=1 -timeout=2m ./...`.
- `go vet ./...`, `go mod verify`, and shell syntax checks.
- Deployment installer/uninstaller dry-run and invalid-argument tests.
- Static Linux amd64 executable build, version check and ELF static-link inspection.
- Native executable smoke check: HTTP access, configuration surviving process
  restart, second-instance refusal, missing-receiver status and graceful shutdown.
- Nine grouped real Chromium checks against the actual isolated Go demo service:
  login, six-button remote/action editing, unsaved-input preservation, three-press
  learning, HTML-safe labels, backup/restore, duplicate remote, desktop/mobile themes,
  and lock/token removal. No JavaScript/CSP errors reported.
- systemd unit syntax verification with a temporary stub executable; this does
  not test service installation, hardening compatibility or USB access on ZORIN.

Go tests exercise learning arm/quiet intervals, mismatches, duplicate detection,
timeout/cancel preservation, global action suppression, trailing burst suppression,
held-button debounce, simultaneous distinct-button webhooks, learning waiting for
in-flight requests, SQLite persistence and bounded history, corruption refusal,
storage failure stopping actions, authenticated API, cross-origin rejection, trusted
HTTPS proxy origin, redirects, HTTP timeout/no retry, payload/headers, URL validation,
prohibited network addresses, 64-bit IDs, restore canonicalization and limits.

Receiver tests use fake process/sysfs inputs: disconnected/reconnected/moved receiver,
serial ambiguity, crash backoff, RF silence with sample heartbeat, stalled acquisition,
cancellation, child kill escalation, saturated stderr, oversized lines and safe argv.

## Not performed; required before production release

- Actual ZORIN installation or systemd startup.
- Actual RTL2838/R820T acquisition or Switch433 decoder verification.
- Physical learning and Home Assistant webhook/automation execution.
- Physical USB same/different port, different hub, prolonged disconnect and reboot.
- Sustained server soak test and power-loss validation.
- GitHub-hosted CI run; workflow is prepared but repository publication is pending.

See HARDWARE-TESTS.md. Synthetic test evidence must not be recorded as physical
receiver success. Busy limits (eight global requests, one per button) intentionally
reject excess presses with an event; there is no deferred action queue or replay.
