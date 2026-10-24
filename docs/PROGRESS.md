# Project progress

2026-10-25: Requirements read; upstream rtl_433, Home Assistant and SQLite researched.
Architecture and API contract established. Implementation started in isolated workspace.
No server connection, service modification or physical receiver test performed.

## Implemented and verified in this workspace

- Native Linux executable built, statically linked, version 0.1.0-dev.
- SQLite persistence, bounded history, remote/button CRUD, three-press learning,
  conservative identity normalization, overlap rejection, learning suppression,
  sliding debounce, authenticated backup/restore and disabled restored actions.
- HTTP(S) origin allowlist, no redirects or automatic retries, per-button in-flight
  guard and eight global action slots, timeouts with unknown-outcome reporting.
- Serial-based USB discovery, ambiguity rejection, process supervision, sample
  heartbeat, capped backoff, clean cancellation and parent-death handling.
- Responsive embedded interface, dark/light themes, token login and live monitor.
- MIT license, dependency notices, systemd and dry-run install/upgrade/uninstall,
  read-only server inspection, troubleshooting, CI and hardware test matrix.
- Final `make check`: uncached race-enabled Go tests, go vet and deployment
  dry-run/argument tests passed. Linux amd64 static build succeeded.
- Nine grouped Chromium integration checks passed against the actual isolated
  demo service; no JavaScript/CSP errors. Screenshots are synthetic demo only.
- Review corrections: exact trusted HTTPS proxy origin, distinct-button webhook
  concurrency, inherited identity defaults, 64-bit-safe mapping comparisons,
  canonical restore validation, case-insensitive header uniqueness and size caps.

No test in this workspace establishes operation with the user's RF hardware or
Home Assistant. The exact final Switch433 decoder was not present in the earlier
analyzer capture; no guessed decoder has been enabled.

## Acceptance gates

1. Architecture: documented components, security boundaries, risks, deployment model.
2. Minimum service: build executable, embedded GUI, receiver supervisor, live history.
3. Learning: 3 separately armed matching presses, explicit confirmation, duplicates,
   timeout/cancel, stable normalization, global action suppression, persistence tests.
4. Webhooks: allowlisted HTTP(S), timeout, no redirects/retries, bounded concurrency,
   debounce, test action, redacted logs and outcome-unknown handling tests.
5. Reliability: synthetic USB disappearance/reappearance, ambiguity, child crash,
   backoff, shutdown, no restart on radio silence; systemd service and installer.
6. UI: remote/button CRUD, editor, learning, history, settings, backup/restore, themes;
   desktop/mobile browser verification.
7. Release: unit/integration/race tests, build, docs/license/CI, source bundle;
   ZORIN inspection + approved deployment + actual hardware matrix before stable release.

## Outstanding external gates

- Exact final Switch433 decoder and real JSON samples from ZORIN.
- Read-only server inspection and production deployment approval.
- New GitHub repository creation/push (connected tool currently has no repository-create operation).
- Actual receiver, port/hub, reboot and Home Assistant tests.

Next on-server task: [ZORIN-CODEX-TASK.md](ZORIN-CODEX-TASK.md). This begins read-only;
installation and receiver handover require the user's explicit approval.
