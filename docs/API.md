# Internal UI API contract v1

All /api routes require `Authorization: Bearer <token>`; JSON bodies, errors
`{"error":"message"}`. UI login stores token in sessionStorage. GET /healthz is
minimal liveness; authenticated GET /api/state is status and configuration.

Data: Remote {id,name,columns,buttons:[Button]}. Button {id,name,enabled,debounce_ms,
signature,identity_fields:[string],webhook:{url,method,payload,headers,timeout_ms},last_received}.
payload is a JSON string (or empty), headers a string/string object.
Settings {receiver_serial,flex_decoder,protocols:[int],identity_fields:[string],allowed_origins:[string]}.
Defaults identity_fields = ["id","channel","command","state","button","code","codes"].
Receiver {state,message,serial,restarts}. Event {id,time,kind,message,button_id,data}.
Learning {button_id,stage,candidate,raw,presses,required,expires_at}; stages waiting,
captured,ready. Session null when inactive. Confirmation after 3 armed captures.

- GET /api/state -> {remotes,settings,receiver,learning,events,version}
- POST /api/remotes {name,columns,button_count} -> Remote (default 6, two columns)
- PUT /api/remotes/{id} {name,columns} -> Remote
- POST /api/remotes/{id}/duplicate -> Remote (mappings cleared, actions disabled)
- DELETE /api/remotes/{id} -> {ok:true}
- POST /api/remotes/{id}/buttons {name} -> Button
- PUT /api/buttons/{id} -> full Button editable fields (signature only through learning)
- DELETE /api/buttons/{id} -> {ok:true}
- POST /api/buttons/{id}/clear -> clears mapping+action, disables button
- POST /api/buttons/{id}/test -> {ok:true} (test saved action; blocked in learning)
- POST /api/learn/start {button_id,identity_fields:[string]} -> Learning
- POST /api/learn/arm {} -> Learning (arm next independent capture after release)
- POST /api/learn/confirm {} -> {ok:true}
- POST /api/learn/cancel {} -> {ok:true}
- PUT /api/settings -> Settings (receiver restart when changed)
- GET /api/backup -> JSON {schema_version:1,remotes,settings}
- POST /api/restore -> backup JSON (atomic; actions restored disabled)

Poll GET /api/state at 1 second for initial UI (bounded events, no SSE dependency).
Never render external data through innerHTML. All mutating requests Content-Type
application/json. Include visible labels for settings and asynchronous errors.
