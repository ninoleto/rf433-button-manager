# Project progress

2026-10-25: Requirements read; upstream rtl_433, Home Assistant and SQLite researched.
Architecture and API contract established. Implementation started in isolated workspace.
No server connection, service modification or physical receiver test performed.

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
