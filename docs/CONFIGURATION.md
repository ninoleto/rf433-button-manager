# Receiver, identity and learning configuration

## Process configuration

The installed unit reads `/etc/rf-button-manager/manager.env` and translates its
variables into CLI flags. A directly launched executable does not read this file.

| Flag | Default | Installed unit input |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8787` | `RFB_LISTEN` |
| `--rtl433` | `/usr/bin/rtl_433` | `RFB_RTL433` |
| `--public-origin` | empty | `RFB_PUBLIC_ORIGIN` |
| `--data-dir` | `./data` | Unit fixes `/var/lib/rf-button-manager` |
| `--demo` | false | Not enabled by supplied unit |
| `--version` | Prints version and exits | Not a runtime setting |

For direct LAN access, verify the server's actual private address and unused port,
set `RFB_LISTEN` to that specific address and restart only this service. Wildcard
and public binds are rejected. Code also accepts loopback/private IPs and
`100.64.0.0/10`; acceptance does not configure routing or exposure. HTTP is
unencrypted. The management token remains required on every interface.

## Saved settings

Use Settings or the authenticated API, never SQLite edits. `PUT /api/settings`
replaces the full settings object: first GET state, preserve unchanged fields,
then PUT. Every successful settings update notifies and restarts the owned decoder.

| Field | Meaning |
| --- | --- |
| `receiver_serial` | Exact serial; empty means unambiguous singleton selection |
| `flex_decoder` | One verified `rtl_433 -X` argument, without `-X`, shell quotes or a whole command |
| `protocols` | Explicit positive `rtl_433 -R` protocol IDs, if required |
| `identity_fields` | Stable defaults copied into newly created buttons |
| `allowed_origins` | Exact lowercase `scheme://host[:port]` origins for webhooks |

Defaults: empty serial/decoder/protocols; identity fields
`id,channel,command,state,button,code,codes`; allowed origin
`http://127.0.0.1:8123`. An allowed origin alone does not create or enable actions.
With no explicit decoder/protocols, rtl_433 uses built-in defaults. Sample
acquisition then does not establish that Switch433 is supported.

## Receiver ownership and decoder provenance

Run `scripts/inspect-zorin.sh` in a normal host terminal. The basic script lists
Realtek receivers and known RF process names without opening hardware. It cannot
establish host ownership inside a restricted process namespace. Inspect
`rtl_433`, `rtl_tcp`, `rtl_test`, other SDR programs, relevant service units, and
open file descriptors for the actual USB node. Arrange handover of an exact
process/service before stopping it. Never use blanket process kills.

Compare the intended serial against all connected compatible receivers. A factory
serial such as `00000001` is not globally unique; use it only when unique among
currently connected compatible devices. Automatic selection requires exactly one
receiver. Never persist bus/device numbers or hub ports. Duplicate serials require
deliberate resolution; this application does not rewrite EEPROM serials.

Preserve any existing verified custom decoder and explicit protocol selection.
New fixed-code OOK PWM/PPM/PCM profiles are added separately; ordinary Settings updates do
not remove them. Normal learning needs no decoder string, terminal history search,
configuration-file edit or receiver command. The browser workflow captures and
validates supported signals internally. It does not inherit arbitrary configuration
files or treat sample timings as a working Switch433 decoder.

The receiver needs USB access as `rfbutton`. A new udev rule may require an
operator to reconnect that receiver. Ask before unplugging; never reset hubs,
unload drivers or change unrelated devices as an installation shortcut.

## Stable identity

Canonical signal JSON always includes `model`, plus selected stable fields.
A model alone is unsafe and rejected. A valid identity needs either `code`/`codes`,
or a device field (`id`, `address`, `device_id`) together with a command field
(`command`, `state`, `button`, `action`). Missing selected fields are skipped.
Scalar strings, numbers and booleans are accepted except empty strings or booleans
used as device IDs/codes. Field names are case-sensitive. Integers retain JSON
precision; a numeric ID and the same text as a string are different identities.

`codes` must be an array of strings normalizing to one distinct nonempty value.
Whitespace and case are normalized and exact repeats collapsed. Bit-count prefixes
and leading zeros remain significant. Multiple distinct codes in a frame are
rejected, not reduced to whichever happens to come first.

Synthetic example, **not a real mapping**:

```json
{"model":"ExampleRemote","id":1234,"command":"on","rssi":-20}
```

Fields `id,command` keep model/device/command and omit RSSI. Known volatile fields
including time, RSSI, SNR, noise, frequency, modulation, counts, temperature,
humidity, battery status and integrity metadata are prohibited. Model is always
included and cannot be listed as a selected field. Inspect real frames before
choosing fields; rolling/encrypted protocols may not support static mappings.

Mappings overlap if their models match and all shared identity fields agree.
A broad mapping can therefore conflict with a more specific one. Confirmation
rejects such overlaps. Changing a button's identity fields clears its mapping and
disables it. Changing global defaults affects newly created buttons, not existing
ones. Ordinary button PUT requests cannot assign a signature; use learning.

## Learn A ON with three separate presses

Prerequisites: intended receiver acquiring samples, actions kept
disabled. Create a six-button, two-column remote if missing; default labels are
A ON, A OFF, B ON, B OFF, C ON, C OFF. Do not invent signal codes.

1. Select **A ON**, save pending editor changes and choose **Learn signal**.
   Keep other remotes idle. Wait for the receiver-ready instruction. Learning has
   a 180-second overall limit, waits for existing requests to finish, and suppresses
   new actions globally. Capture and replay are managed by the application.
2. Briefly press physical **A ON once** and release fully. Wait for **1 of 3
   matching presses** and inspect the decoded candidate.
3. Wait at least one second after release, select **Ready for next press**, then
   press and release **A ON once**. Expect **2 of 3**.
4. Again wait at least one second, arm and make the third distinct press. Expect
   **3 of 3** and the confirmation control.
5. Review the candidate and choose **Confirm and save**. The saved action remains
   disabled. The validated profile is saved atomically with the mapping. Later
   buttons on this remote preferentially reuse that profile.

Failures remain visible with a reason, **Retry learning** and expandable
**Diagnostics and receiver status**. See [LEARNING.md](LEARNING.md) for supported
signals, states, capture limits and recovery. Cancel/expiry preserve any previous mapping. Arming alone never counts as a press;
a held transmission cannot satisfy three captures. Restart only this service to
verify persistence. Learning proves those observed decodes, not webhook behavior
or recovery through untested USB ports/hubs. Record these separately with an operator.

## Debounce and webhook setup

Each matching frame advances a silence timer. A new action needs a gap at least
`debounce_ms` since the prior matching frame. Default 700 ms; range 100–60,000 ms.
Held bursts keep extending the interval. Distinct presses faster than the interval
can be suppressed. Automatic learning requires a release interval and explicit arming between
captures. The synthetic demo retains the older 750 ms decoded-event interval.

Paste a complete GET/POST webhook URL into the button editor. For a new address,
the editor shows that address and **Save & allow destination** saves its permission
and the button together. An invalid button does not add a destination. Existing
permissions can still be reviewed or edited in Settings. Pasting or saving does
not send a test request. The optional Home Assistant ID helper also accepts complete
URLs without combining them with its base URL or encoding the whole URL as an ID.
Test only in an agreed safe window. **Test webhook sends a real request even if the button is
disabled.** Disabled blocks RF dispatch, not deliberate manual testing. Demo
mode suppresses all real requests. Installation/learning never require testing one.

Allowed origins have no path, trailing slash, credentials, query or fragment.
They authorize an entire exact origin, not one path. Explicit/default port spelling
matters. POST is default; payload is a JSON string or empty; headers map strings to
strings. Timeout is 100–30,000 ms, default 5,000 ms. GET cannot have a payload.
Home Assistant automation setup is separate; treat its webhook ID as a credential.

## Interpreting evidence

| Observation | Establishes | Does not establish |
| --- | --- | --- |
| `/healthz` returns 200 | HTTP application liveness | Storage health, USB access or decoding |
| `receiver.state=connected` | Sample statistics, decoded JSON or captured pulses arrived | A particular button decoded |
| Real press produces an RF event | Some frame decoded | Correct mapping/action |
| Three real captures confirmed | Mapping saved for that identity | HTTP delivery or recovery |
| `webhook_ok` / HTTP 2xx | Endpoint accepted the request | Intended physical state |
| Physical result and destination trace observed | That specific action worked | Untested hardware topologies |

Other receiver states are `disconnected`, `recovering`, `error` and explicitly
synthetic `demo`. A quiet RF channel is normal while sample heartbeats continue.
