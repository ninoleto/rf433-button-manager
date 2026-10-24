# RF Button Manager architecture

Status: implementation in progress; no ZORIN access or hardware validation yet.

## Decisions

Go 1.27, standard net/http server, modernc.org/sqlite (pinned pure-Go SQLite),
embedded HTML/CSS/JavaScript with no CDN or frontend build tool. One Linux executable
manages one rtl_433 child, listens on loopback by default, and persists configuration
and bounded event history in SQLite. Native systemd unit, no Docker or MQTT dependency.
MIT license for our code; rtl_433 is installed separately and retains its own license.

## Components

- cmd/rf-button-manager: flags, startup, shutdown and single-instance lock.
- internal/app: SQLite, configuration validation, learning, debounce, webhook dispatch and API.
- internal/receiver: Linux USB discovery and supervised rtl_433 subprocess.
- internal/webui: embedded remote editor and live monitor.
- deploy, scripts: systemd, install/uninstall, read-only ZORIN inspection.
- docs: architecture, API, operations, hardware acceptance and persistent progress.

## RF and recovery

Use explicit argv (never a shell), -c /dev/null to avoid unrelated rtl_433 configs,
JSON stdout, 433.92 MHz, configured protocol IDs and/or exact flex spec. Never guess
Switch433 timings: old analyzer output is not proof of the final tested decoder.
Canonical signatures include model and explicitly selected stable fields; metadata
(timestamps, signal level, counters) cannot be identity by default. Flex code arrays
must be normalized conservatively; uncertain multi-code events require diagnostics.
Learn is a timed global suppression session, requires three separately armed presses
of one candidate and explicit confirmation, and checks mapping uniqueness atomically.
No webhook dispatch during learning. Sliding silence-based debounce prevents a held
button from retriggering continuously. Actions are not replayed across restarts.

USB discovery re-runs across port/hub changes. Select unique serial where available;
automatic selection is permitted only with exactly one supported receiver. Ambiguous
receivers block acquisition. Never persist USB bus/device numbers. Supervise process
exit and USB disappearance independently of RF silence, with capped exponential
backoff, clean child termination and recovery events. Hardware tests remain a release
gate; synthetic subprocess/USB tests are not physical USB evidence.

## Security and actions

Loopback default, random persisted management token, bearer authentication, strict
same-origin checks for browser requests, request size limits and CSP. LAN access
requires an explicit private bind and token; use an SSH tunnel or HTTPS reverse proxy.
Webhook destinations are explicit HTTP(S) origins on an administrator allowlist;
no redirects, no environment proxy, no URL credentials, no unbounded retries.
POST is default. Exactly-once HTTP delivery is impossible: timeout means outcome
unknown; never automatically repeat a physical action. Secrets excluded from logs.
Backups contain secrets and are authenticated downloads; restore validates and
atomically replaces configuration with actions disabled pending review.

## Sources reviewed 2026-10-25

- https://triq.org/rtl_433/OPERATION.html — serial selection, JSON output, flex decoders, diagnostics.
- https://github.com/merbanan/rtl_433/tree/23.11 — target installed version.
- https://pkg.go.dev/modernc.org/sqlite — pure-Go SQLite.
- https://www.home-assistant.io/docs/automation/trigger/#webhook-trigger — POST and webhook semantics.

## Main risks

Unsupported or rolling-code protocols; incorrect flex timings; RF collisions and
false decodes; nonunique USB serials; decoder hangs without USB removal; slow webhook
destinations; uncertain HTTP outcomes; power-loss/disk-full storage failures; local
network management credential exposure. Tests and explicit limits document each.
