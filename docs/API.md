# Management API contract v1

## Receiver inventory and activity (0.3.6-dev)

`GET /api/receivers` requires the management token and normal origin checks. It
returns `{"devices":[...]}`; each item contains `name`, `manufacturer`, `serial`,
`usb_port`, `usb_id`, `selectable`, and an optional `reason`. Empty inventory is
an empty array. It reads Linux sysfs without opening a tuner or stopping a child.
Do not interpret inventory as proof of permission, availability or acquisition.

`PUT /api/settings` continues to accept `receiver_serial` with all other settings.
When that field changes, the server checks the current inventory again. New
missing/duplicate selections are rejected without saving. Empty serial selects
automatic mode only when exactly one receiver is present. An unchanged saved
serial remains valid configuration while disconnected. Learning/inspection blocks
settings changes. Selection never edits remotes, mappings, webhook data or action
flags. Backup restoration preserves its existing offline-settings semantics;
the receiver supervisor still refuses ambiguous acquisition after restore.

`GET /api/state` adds optional `receiver.name` and `receiver.usb_port` alongside
`receiver.serial`, plus `receiver_last_decoded_at` (RFC3339Nano or empty). This is
the last decoded RF event, not a sample heartbeat, button match or webhook result.
New `rf`, `matched` and `receiver` events optionally include a `receiver` object
with `name`, `serial`, `usb_port`; historical events need not have it.

After test.1, an exact supported decoder-equivalence match retains `kind:matched`
and explains that path in the message. `kind:rf_ambiguous` means the decoded payload
could address more than one saved button and **no action was scheduled**. It has
no selected `button_id`; use the All events or RF signals filter to see it. Stored
identities and schema are unchanged. See CONFIGURATION.md for the narrowly scoped
GS558-to-Akhan fallback and test.3's registered 25-bit PWM payload conversion;
unrelated decoder models remain exact-only.


All /api routes require `Authorization: Bearer <token>`; JSON bodies, errors
`{"error":"message"}`. UI login stores token in sessionStorage. GET /healthz is
minimal liveness; authenticated GET /api/state is status and configuration.

Data: Remote {id,name,columns,buttons:[Button]}. Button {id,name,enabled,debounce_ms,
signature,decoder_profile_id,identity_fields:[string],webhook:{url,method,payload,headers,timeout_ms},last_received}.
payload is a JSON string (or empty), headers a string/string object.
Settings {receiver_serial,adaptive_sensitivity?:boolean,flex_decoder,protocols:[int],identity_fields:[string],allowed_origins:[string]}.
Defaults identity_fields = ["id","channel","command","state","button","code","codes"].
Receiver {state,message,serial,restarts}. Event {id,time,kind,message,button_id,data}.
Learning includes session_id, button_id, stage, message, automatic, candidate, raw,
presses, required, expires_at, identity_fields, diagnostics and optional profile.
Active stages: preparing, waiting, receiving, analyzing, captured, ready. Inactive
session is null; learning_notice retains a terminal failure until Cancel or Retry.
Confirmation requires three separately armed captures; see LEARNING.md.

- GET /api/state -> {remotes,settings,receiver,learning,learning_notice,actions_paused,decoder_profiles,events,version}
- POST /api/remotes {name,columns,button_count} -> Remote (default 6, two columns)
- PUT /api/remotes/{id} {name,columns} -> Remote
- POST /api/remotes/{id}/duplicate -> Remote (mappings cleared, actions disabled)
- DELETE /api/remotes/{id} -> {ok:true}
- POST /api/remotes/{id}/buttons {name} -> Button
- PUT /api/buttons/{id} -> full Button editable fields (signature only through learning)
  Optional request-only `allow_webhook_origin` string approves the exact origin
  of this button's URL in the same transaction. It must match the URL's lowercase
  scheme/host/explicit port, with no path. Omitting it retains strict allowlist
  enforcement. The GUI sends it only when the user saves a newly approved address.
  Invalid edits persist neither the button nor permission; saving never invokes
  a webhook. The response and stored Button do not include this request field.
- DELETE /api/buttons/{id} -> {ok:true}
- POST /api/buttons/{id}/clear -> clears mapping+action, disables button
- POST /api/buttons/{id}/test -> {ok:true} (test saved action; blocked in learning)
- POST /api/buttons/{id}/reception/reset {} -> {ok:true} (diagnostic reset only;
  requires normal reception, preserves debounce, action flags and pending actions)
- POST /api/learn/start {button_id,identity_fields:[string]} -> Learning
- POST /api/inspect/start {} -> Learning with `inspect_only:true`, no button ID
  required. Shares the bounded capture/three-arm workflow and webhook suppression.
- POST /api/inspect/finish {} -> {ok:true}. Requires three matching armed presses
  and stage `ready`; records a successful inspection, changes no mapping, and
  retains suppression until receiver/analysis cleanup completes. Stop inspection
  still uses `/api/learn/cancel`, with an inspection-specific stopped message.
- GET /api/inspect/result -> RF-only inspection report for the active/retained
  inspection dialog: candidate, matched observations, candidate profile and limits.
  Requires authentication. Download before closing the dialog. No configuration,
  webhook URL/header or management token fields are included.
  Inspection uses `/api/learn/arm`, `/api/learn/cancel` and `/api/learn/capture`;
  `/api/learn/confirm` rejects inspection sessions and cannot persist a mapping.
- POST /api/learn/arm {} -> Learning (arm next independent capture after release)
- POST /api/learn/confirm {} -> {ok:true}
- POST /api/learn/cancel {} -> {ok:true}
- PUT /api/settings -> Settings (owned receiver reload on every successful settings save)
- GET /api/backup -> JSON {schema_version:2,remotes,settings,decoder_profiles}
- POST /api/restore -> backup JSON (atomic; actions restored disabled)

Poll GET /api/state at 1 second for initial UI (bounded events, no SSE dependency).
Never render external data through innerHTML. All mutating requests Content-Type
application/json. Include visible labels for settings and asynchronous errors.

## Reception diagnostics (0.4.0-test.4)

`settings.adaptive_sensitivity` defaults to false when absent in an existing
schema-2 configuration. True adds only `-Y autolevel` to both live receive plans.
Every settings PUT still replaces the full settings object and reloads the owned
receiver. Preserve other settings and do not silently enable this option.

`receiver.adaptive_sensitivity` describes the running plan. Optional
`receiver.noise` contains `{dbfs,at}` from rtl_433's bounded stderr estimate, with
an RFC3339Nano observation time. Noise logs are not RF events or sample heartbeats.
Disconnect/restart clears them; unknown formats have no measurement.

Optional `event.levels` contains numeric `rssi_dbfs`, `snr_db`, `noise_dbfs`.
Absent/invalid fields remain absent, and zero is valid. Values are extracted from
the original event before identity normalization and are not part of matching.
Old events remain valid. This additive event JSON needs no database schema change.

`state.reception` contains `{since,buttons:{[button_id]:diagnostics}}`. A diagnostic
has `since`, `reports`, `presses`, `repeats`, optional `last_matched_at`, `levels`,
`action`, `action_at`. Reports match only that saved button, presses pass normal
debounce, and repeats are suppressed reports. Disabled buttons count too; learning,
inspection, cleanup and ambiguous matches do not. Actions retain their own gates.
No counter establishes the number of physical attempts or webhook successes.
These diagnostics are bounded by configured buttons, kept only in memory and
excluded from configuration backups. Receiver restarts/source changes reset them;
removed/relearned mappings lose their old counts. Reset cannot alter debounce.

## Normal-use recording (0.4.0-test.5)

- `POST /api/buttons/{id}/reception/record {}` starts a 60-second recording for a
  saved identity, returning `RecordingStatus`. Normal connected reception must
  be ready; learning, cleanup, unavailable receivers and overlapping recordings
  are rejected. Demo permits explicitly marked synthetic recordings only.
- `POST /api/reception/recording/stop {}` stops retention, preserving the recording.
  Returns status; calling it again on a retained recording is harmless.
- `GET /api/reception/recording` downloads an authenticated JSON attachment, format
  `rf433-button-manager-reception-v1`. A current recording can be exported as a
  snapshot; the UI stops before downloading. Quiet recordings are valid evidence.

`state.reception_recording_supported` indicates availability;
`state.reception_recording` is null or a payload-free summary with ID, selected
button ID/name, start/expiry/end, active/stop reason, remaining seconds, retained
byte/packet/decoded counts, recognized presses and ignored repeat reports.

The export adds complete OOK/FSK `packets` with unchanged `pulse_data`, original
decoded reports and per-report match decisions in `events`, sanitized webhook
event kinds/HTTP statuses, RF-only target/decoder settings, receiver/stream
snapshots and limitations. `report_id` joins a decision to its original JSON;
webhook events are button/timestamp observations, not falsely assigned to a
specific report. Existing or manually tested requests can appear; results after
the stop time are absent. Offsets are application receipt times, not RF timestamps.

Test.6 adds optional `profile_framing_fallbacks`, containing the additional `-X`
specifications actually used in normal reception. Replay tools must append these
alongside the unchanged `decoder_profiles` to reproduce that capture's decoder
plan. The format remains v1; older recordings omit this optional field.

Retention is RAM-only, at most 60 seconds, 512 packets, 2,048 events and a 2 MiB
encoded-data budget. Oversized individual decoded reports stop retention. Starting
another recording replaces the retained one; restarting the app clears it.
Receiver failure/reload, inspection/learning or a saved configuration edit stops
retention with an explicit reason. Neither starting nor stopping changes receiver
generation, mappings, action flags, quiet-period debounce or action concurrency.
The new normal receiver plan adds `-w OOK:-` continuously; only bounded current
packet assembly occurs outside recording. No second receiver or replay is started.

Exports exclude management tokens, configuration backups, webhook destinations,
headers, payloads and free-form request errors. Radio identifiers and surrounding
traffic remain private. Pulse recordings are not raw IQ and cannot reconstruct
information already lost in reception/demodulation. Counts are not physical press
counts or proof of delivery. See LEARNING.md for text FSK replay limitations.

RF capture exports additionally record `adaptive_sensitivity` when enabled. They
still contain pulse records and decoded JSON, not raw I/Q samples.

## Authentication and HTTP semantics

Use the installed service's private listener (loopback by default, or its explicitly
configured LAN address). Send `Authorization: Bearer TOKEN` in a header; never put
it in a URL. The credential is read from `management.token` in the selected data
directory. Static HTML/CSS/JS and `/healthz` do not require authentication; all
`/api/` routes do. Reading state/backup reveals private configuration, not just
public status. Treat their complete responses as secrets.

Successful implemented API calls return HTTP 200, including creates/deletes.
Validation and application-operation errors return 400 with `{"error":"..."}`;
missing/wrong credentials return 401. A valid token with a forbidden browser
Origin or `Sec-Fetch-Site: cross-site` returns 403. Standard router behavior can
return 404/405 for unknown paths/methods. Clients must inspect the status before
interpreting success JSON. Webhook test success only means scheduling succeeded;
read later events for delivery outcome.

Requests that parse a body require Content-Type starting with `application/json`,
reject unknown JSON fields and multiple JSON values, and have a 2 MiB body limit.
For bodyless action endpoints, send `{}` for consistency. There is no CORS bypass,
cookie login endpoint, public mutation endpoint or RF-injection API. Browser
Origin must match the request scheme/host or one configured exact trusted HTTPS
public origin. Non-browser clients can omit Origin but still need the token.

The API has no version prefix, ETag or optimistic-concurrency token. Serialize
administrative edits: full-object PUT from a stale state can overwrite another
operator's edits. Back up before bulk changes. Do not log full requests/responses.

## Exact data shapes and limits

`GET /api/state` includes `demo` and `storage_error` booleans as well as the fields
listed above. `receiver.state` is `disconnected`, `recovering`, `connected`, `error`
or `demo`; there is no independent physical-test-passed field. `learning` is null
or includes `identity_fields` in addition to candidate/count/expiry. Receiver
status and learning are runtime state, not durable configuration.

| Field/object | Validation |
| --- | --- |
| Remote/button names | Nonblank; maximum 100 bytes |
| IDs | Server-generated 32 lowercase hexadecimal characters |
| Remotes/buttons | Maximum 100 remotes, 64 buttons per remote; create count 1–64 (0 defaults to 6) |
| Columns | 1–6; create value 0 defaults to 2 |
| `debounce_ms` | 100–60,000; default 700 |
| `identity_fields` | 1–16 valid distinct stable field names; details in CONFIGURATION.md |
| `receiver_serial` | At most 128 bytes, no CR/LF/NUL |
| `flex_decoder` | At most 4,096 bytes, no CR/LF/NUL |
| `protocols` | At most 32 integers, each 1–10,000 |
| `allowed_origins` | At most 64 canonical lowercase origins, no path/query/trailing slash |
| Webhook URL | Empty or allowlisted HTTP(S), at most 4,096 bytes, no credentials/fragment |
| Webhook timeout | 100–30,000 ms; default 5,000 |
| Payload | Empty or valid JSON encoded as a string, at most 65,536 bytes; GET requires empty |
| Headers | At most 32; names <=128 bytes, values <=4,096; no reserved/duplicate case-insensitive names |
| Logical backup | Export schema 2; restore accepts 1 (migrated) or 2; accepted configurations fit within restore's 2 MiB request limit |

`PUT /api/buttons/{id}` is a full editable-button replacement, not a patch. Use
the current button object and edit only intended fields. Server-owned `id`,
`signature`, `decoder_profile_id` and `last_received` are retained regardless of supplied values.
Changing identity fields clears the signature and disables the button. A direct
signature assignment is ignored; learning is the supported assignment path.
Duplicate remote copies action configuration but clears mappings and disables
all buttons. Clear-button removes both mapping and webhook, then disables it.

The logical export contains `schema_version`, `remotes`, `settings` and
`decoder_profiles`. Profiles have id, name, model, flex_decoder, bits,
validated_presses (3), and created_at (RFC3339). They are bounded and validated;
ordinary button/settings PUT cannot create them or change their association.
Confirmation saves a new profile together with its mapping. Restore
validates first, replaces configuration atomically and disables restored actions;
the token and event archive are not restored. Duplicate/overlapping mappings,
unknown schema versions and noncanonical signatures are rejected.

## Safe authenticated read example

Run on the server; change `base` to the verified LAN URL if the service no longer
listens on loopback. The token stays in process memory rather than process
arguments, shell history or output. Redirects and environment proxies are disabled.
The printed subset excludes webhook settings and raw signal history.

```sh
sudo python3 - <<'PY'
import json
from pathlib import Path
from urllib.request import Request, build_opener, ProxyHandler, HTTPRedirectHandler

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

base = 'http://127.0.0.1:8787'  # Replace if bound to a verified LAN address.
token = Path('/var/lib/rf433-button-manager/management.token').read_text().strip()
client = build_opener(ProxyHandler({}), NoRedirect())

def call(method, path, body=None):
    data = None if body is None else json.dumps(body).encode()
    req = Request(base + path, data=data, method=method,
                  headers={'Authorization': 'Bearer ' + token,
                           'Content-Type': 'application/json'})
    with client.open(req, timeout=40) as response:
        return json.load(response)

state = call('GET', '/api/state')
print(json.dumps({'version': state['version'], 'demo': state['demo'],
                  'storage_error': state['storage_error'],
                  'receiver': state['receiver'],
                  'remote_count': len(state['remotes'])}, indent=2))
PY
```

For a direct liveness/authentication check without credentials:

```sh
curl --fail http://127.0.0.1:8787/healthz
curl --silent --output /dev/null --write-out '%{http_code}\n' http://127.0.0.1:8787/api/state
```

Use the configured address; expected statuses are 200 then 401. A Windows browser
test is still needed to establish LAN reachability from that client.

## Configuration examples using `call`

The following Python snippets run inside the authenticated example above, after
the `call` definition. They change application configuration deliberately. They
never invoke a webhook or invent an RF signature.

Create the six-button layout only if it is missing:

```python
state = call('GET', '/api/state')
names = ['A ON', 'A OFF', 'B ON', 'B OFF', 'C ON', 'C OFF']
remote = next((r for r in state['remotes']
               if [b['name'] for b in r['buttons']] == names), None)
if remote is None:
    remote = call('POST', '/api/remotes',
                  {'name': 'Six-button remote', 'columns': 2, 'button_count': 6})
for button in remote['buttons']:
    if button['enabled']:
        button['enabled'] = False
        call('PUT', '/api/buttons/' + button['id'], button)
```

Optional receiver selection uses its verified unique serial; preserve the other
settings. Normal learning does not require a manual flex decoder:

```python
settings = call('GET', '/api/state')['settings']
settings['receiver_serial'] = verified_serial
call('PUT', '/api/settings', settings)
```

Learning API sequence, with a real separate press after each start/arm:

```python
target = next(b for b in remote['buttons'] if b['name'] == 'A ON')
session = call('POST', '/api/learn/start', {'button_id': target['id']})
# Wait for stage=waiting, then press/release A ON.
# Poll GET /api/state through receiving/analyzing to captured, presses=1.
# Wait >=1 second after release, then:
session = call('POST', '/api/learn/arm', {})
# Press/release A ON, inspect captured/2. Wait, then arm once more:
session = call('POST', '/api/learn/arm', {})
# Third separate press -> ready/3. Review candidate before confirming:
call('POST', '/api/learn/confirm', {})
```

Do not run the sequence without pausing for the physical observations. Polling
does not count as a press. Cancel with `POST /api/learn/cancel {}` when needed.
Confirmation leaves the action disabled. An overlap or incomplete count returns
400 and does not silently overwrite another mapping.

## Webhook and backup behavior

The button's webhook object contains `url`, `method`, `payload`, `headers`,
`timeout_ms`. A JSON payload is serialized inside a JSON string, e.g.
`"payload":"{\"source\":\"rf\"}"`; do not send a nested object there. Saving a
webhook does not test it. `POST /api/buttons/{id}/test` is a deliberate real action
even if the button is disabled, and is blocked during learning/demo/storage fault.
It can return before delivery finishes. Observe `webhook_start`, `webhook_ok`,
`webhook_error`, `webhook_unknown` or `webhook_busy` events. No automatic retry or
deferred replay occurs.

Use `GET /api/backup` to export a consistent configuration, save privately with
mode 0600, and inspect before `POST /api/restore`. Do not print exports or attach
them to issues. Export the current configuration before replacing it. Full-state
backup/rollback procedures are in [OPERATIONS.md](OPERATIONS.md).

## Learning status and diagnostics

`actions_paused` stays true through capture cleanup and restoration, even when
`learning` has become null. Do not infer action availability from the session alone.
`learning_notice.stage` can be receiver_unavailable, timeout, press_timeout,
no_capture, malformed_capture, capture_error, capture_limit or analysis_error;
its message explains the next step. Since 0.2.2, `unsupported` and `inconsistent`
analysis results are capture rejections: the active session returns to `waiting`,
retains its candidate/profile, and does not create a terminal notice or count a press.
Retry uses POST /api/learn/start with the previous button_id after cleanup. Cancel
also dismisses a retained notice. The GUI provides these operations directly.

`receiver_mode` and `receiver_generation` identify the currently applied receiver
plan. Learning includes `capture_generation`, `capture_receiver`,
`capture_receiver_at` and, on completion, `finished_at`. Capture receiver status is
frozen on failure; top-level `receiver` can subsequently describe restored normal
reception. Generations are process-local and restart with the application.

Diagnostics include `observed_packets`, `rejected_packets`, `ook_packets`,
`fsk_packets`, `malformed_packets`, `empty_packets`, `discarded_packets`, `packets`
(accepted for replay), `pulses` (valid timing rows observed), `bytes` (observed
packet bytes), `accepted_bytes`, `candidates`, `replays`, `source` and `detail`.
Rejected and discarded categories may overlap during cleanup. `stream` contains
`bytes`, `lines`, `json_records`, `headers`, `completed`, `pulse_rows`,
`packet_incomplete`, `expected_pulses`, `current_rows` and `closed`. These describe
stdout records actually read; they are not raw USB sample counts.
`failure_code` / `failure_reason` preserve the terminal cause; `rejection_code` /
`rejection_reason` identify the last rejected packet, including late cleanup.
Examples include `invalid_timing`, `pulse_count_mismatch`, `missing_end`,
`invalid_timescale`, `no_pulse_packet` and `stream_read_error`. No raw rejected line
is echoed into the reason. Failure/cleanup metadata is persisted in `learning` and
`learning_diagnostics` events; the active session/notice itself is not resumed
after application restart.

Automatic learning includes `window_expires_at`, `window_analyses` and
`window_analysis_limit` (40). The 20-second window starts at initial receiver
readiness or an explicit successful `/api/learn/arm`; it covers receiving and
analysis as well as waiting. Ignored traffic never moves it or `expires_at`.
At most 40 pulse analyses are started per window, separated by at least 500 ms.
Live decoded button identities remain eligible until the same fixed deadline.
`press_timeout` has `failure_code=press_window_timeout`. The overall 180-second
session timeout remains independent, including time spent awaiting browser input.

Diagnostics add `analysis_attempts`, `rejected_captures` and `recent_rejections`
(at most 8). Each rejection contains `attempt`, `press_window`, `time`, `code`,
`reason`, `packets`, `bytes`, `candidates` and `replays`. Every unsuccessful analysis
is individually stored in a `learning_capture` event with `session_id` and a
`rejection` object. It contains no raw waveform, token or decoded RF identity.
The recent rejection ring is omitted from aggregate lifecycle events to avoid
duplicating it and exceeding the event payload limit. The existing 500-event
retention/150-event state view still applies. Packet rejection counts and analyzed
capture rejection counts have different meanings and must not be combined.

They never include credentials or raw IQ data. Profile timing and decoded identities
may identify a remote; treat full state and diagnostics as private. There is no
public pulse-injection endpoint or unauthenticated capture control. Profile limits
are 16 profiles, fixed-code OOK_PWM/OOK_PPM/OOK_PCM (NRZ), 12–64 bits, with exact measured timings and
repeated frames. See LEARNING.md for resource bounds and validation semantics.

## Learning in 0.3.0-dev

The API paths and schema remain unchanged. Automatic learning now accepts live
rtl_433 button events. Identity calculation aliases `cmd`, `action`, or `data`
to `command` when no command exists, and combines active `button1`–`buttonN`
fields into `button`. The original event remains available for inspection.
A live decoded candidate can cancel background file analysis; a late result cannot
overwrite it. Diagnostics source is `rtl_433 decoded button`. Three separately
armed matching presses and explicit confirmation are still required.

## GET /api/learn/capture (0.3.1)

Authenticated read-only download of the most recent in-memory RF attempt. Returns
400 when no data is retained. `format` is `rf433-button-manager-capture-v1`; fields
include `version`, `started_at`, `frequency_hz`, `sample_rate`, `captured_bytes`,
`limit_reached`, `packets` (offset_ms/modulation/pulses/cleanup/pulse_data), and
`decoded_events`. `/api/state` includes only `rf_capture_available`, not the raw
recordings. Config/settings/token/webhooks are never copied into this export.


## Management API added in 0.4

All routes below require the current Bearer token and the same origin checks as
other `/api/` endpoints. JSON request bodies here are limited to **8 KiB**, reject
unknown fields and require `Content-Type: application/json`. Errors return JSON
with an `error` field. Never log a token-rotation response.

| Method and path | Request / behavior |
| --- | --- |
| GET `/api/manage/network` | Current `config`, `urls`, `local_urls`, `network_enabled`, optional `pending`, local `interfaces`, certificate expiry/fingerprint |
| POST `/api/manage/network/stage` | Full network object; opens proposed listeners and creates a three-minute pending change |
| POST `/api/manage/network/confirm` | `{ "id": "pending-id" }`; must arrive at proposed endpoint, over TLS if HTTPS enabled |
| POST `/api/manage/network/cancel` | Cancels the pending change; old configuration remains |
| POST `/api/manage/certificate/prepare` | Creates/loads a local CA and valid leaf for current/pending addresses |
| GET `/api/manage/certificate` | Public CA PEM attachment; never a private key; prepare first |
| POST `/api/manage/certificate/renew` | Renews leaf with same CA; refuses during a pending network change |
| POST `/api/manage/token/rotate` | `{ "confirm": true }`; returns `{ "token": "new-64-hex-token" }`, immediately revoking the old credential |

Network object example (replace with the actual host's private address):

```json
{"address":"192.168.50.20","http":false,"http_port":8787,"https":true,"https_port":8788}
```

Test.3 keeps authenticated HTTP on `127.0.0.1:http_port` independently of the
selected network protocol. `urls` keeps the configured address first, followed
by local HTTP without duplicates. `local_urls` contains that HTTP endpoint.
`network_enabled` is false when the configured address is loopback. To disable
network access, stage `address: "127.0.0.1", http: true, https: false` and confirm
at that endpoint. New staged settings must choose HTTP or HTTPS, never both.
Saved test.2 dual-protocol settings remain loadable and unchanged until an
explicit replacement is confirmed. Selecting network HTTPS does not enable
network HTTP just to preserve localhost access.
A companion loopback endpoint cannot confirm a proposed non-loopback endpoint;
the selected network address must be tested. Certificates cover the selected and
pending addresses plus `localhost`, `127.0.0.1` and `::1`. Local URLs use IPv4
loopback so an IPv6-disabled host is supported.

Ports are 1024–65535; at least one protocol must be enabled. A pending object has
`id`, `config` and `expires_at`. Only successful confirmation persists settings.
Logical `/api/backup` still exports RF configuration only; it excludes the token,
network settings and TLS keys. Full stopped-state snapshots include them.

For scripts, read the current token locally into memory and use a TLS context
that trusts the downloaded CA. Do not disable TLS verification, embed tokens in
URLs/command arguments or automatically confirm an unreachable connection. See
NETWORK.md for the browser flow and trust setup.
