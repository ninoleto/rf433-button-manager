# Management API contract v1

All /api routes require `Authorization: Bearer <token>`; JSON bodies, errors
`{"error":"message"}`. UI login stores token in sessionStorage. GET /healthz is
minimal liveness; authenticated GET /api/state is status and configuration.

Data: Remote {id,name,columns,buttons:[Button]}. Button {id,name,enabled,debounce_ms,
signature,decoder_profile_id,identity_fields:[string],webhook:{url,method,payload,headers,timeout_ms},last_received}.
payload is a JSON string (or empty), headers a string/string object.
Settings {receiver_serial,flex_decoder,protocols:[int],identity_fields:[string],allowed_origins:[string]}.
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
- POST /api/learn/start {button_id,identity_fields:[string]} -> Learning
- POST /api/inspect/start {} -> Learning with `inspect_only:true`, no button ID
  required. Shares the bounded capture/three-arm workflow and webhook suppression.
- GET /api/inspect/result -> RF-only inspection report for the active/retained
  inspection dialog: candidate, matched observations, candidate profile and limits.
  Requires authentication. Download before closing the dialog. No configuration,
  webhook URL/header or management token fields are included.
  Inspection uses `/api/learn/arm`, `/api/learn/cancel` and `/api/learn/capture`;
  `/api/learn/confirm` rejects inspection sessions and cannot persist a mapping.
- POST /api/learn/arm {} -> Learning (arm next independent capture after release)
- POST /api/learn/confirm {} -> {ok:true}
- POST /api/learn/cancel {} -> {ok:true}
- PUT /api/settings -> Settings (receiver restart when changed)
- GET /api/backup -> JSON {schema_version:2,remotes,settings,decoder_profiles}
- POST /api/restore -> backup JSON (atomic; actions restored disabled)

Poll GET /api/state at 1 second for initial UI (bounded events, no SSE dependency).
Never render external data through innerHTML. All mutating requests Content-Type
application/json. Include visible labels for settings and asynchronous errors.

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
token = Path('/var/lib/rf-button-manager/management.token').read_text().strip()
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
400 when no data is retained. `format` is `rf-button-manager-capture-v1`; fields
include `version`, `started_at`, `frequency_hz`, `sample_rate`, `captured_bytes`,
`limit_reached`, `packets` (offset_ms/modulation/pulses/cleanup/pulse_data), and
`decoded_events`. `/api/state` includes only `rf_capture_available`, not the raw
recordings. Config/settings/token/webhooks are never copied into this export.
