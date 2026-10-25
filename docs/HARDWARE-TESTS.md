# Physical hardware acceptance gate

**Browser capture update: physical learning remains pending.** The operator's
2026-10-25 baseline confirms the 0.1 service installation/authentication/configuration
persistence and receiver sample acquisition after one receiver-only reconnect.
That is not a completed physical decoding, webhook or port/hub recovery matrix.
The rows below require their full listed evidence; no row passes from a build.

This checklist deliberately distinguishes physical
evidence from unit tests with fake sysfs entries or fake decoder processes. Do not
mark a row passed based on simulation, a successful build or a working web GUI.

Record app commit/version, binary SHA-256, ZORIN OS/kernel, `rtl_433` version,
receiver VID/PID/serial, exact decoder specification, sample JSON and timestamps.
Keep webhook credentials out of reports. Existing authorization covers this
application's update. Obtain separate approval before stopping another receiver
owner, reconnecting hardware, changing Home Assistant or rebooting the server.

Use a harmless, observable Home Assistant action for acceptance. Keep its existing
automation unchanged unless an explicit change is separately approved. Observe
Home Assistant traces and application events together; HTTP 2xx alone does not
prove that the desired physical state changed.

| Scenario | Procedure | Required evidence | Result |
| --- | --- | --- | --- |
| Baseline ownership | Start manager after the approved handover | One owned `rtl_433`, correct serial, connected status, existing Docker services unchanged | NOT RUN |
| Browser capture cancellation | Cancel during capture/analysis and after a candidate | Normal reception restored; one live owner; previous mapping unchanged; no learning-time webhook | NOT RUN |
| Browser unsupported/mismatch | Send an unsupported or different signal in an agreed test | Visible explanation, retry/diagnostics, no provisional profile saved | NOT RUN |
| Six-button mapping | Learn A/B/C ON and OFF with three separate presses each | Six stable unique identifiers, expected raw JSON, no learning-time actions | NOT RUN |
| Unrelated signal | Transmit a different remote/button during learning | Candidate inspection/mismatch prevents silent acceptance; no webhook | NOT RUN |
| Duplicate mapping | Try learning a saved button onto another button | Conflict shown; previous mapping retained | NOT RUN |
| Held button | Hold one physical button for at least 10 seconds | One action for continuous repeated frames, no periodic retrigger | NOT RUN |
| Rapid presses | Release fully, press again across configured debounce interval | One action per valid separated press; interval documented | NOT RUN |
| Learning while action running | Begin learning around an in-flight test action | UI/action behavior matches documented suppression boundary; no new dispatch after learning starts | NOT RUN |
| Browser closed | Close all management tabs, press each remote button | Actions continue correctly | NOT RUN |
| Unplug while idle | Disconnect receiver without RF traffic | Timely disconnected/recovering status, no busy restart loop | NOT RUN |
| Same-port reconnect | Reconnect the receiver to the original port | Automatic recovery and one successful button action | NOT RUN |
| Different-port reconnect | Move receiver to another USB port | Automatic recovery without path edits or service restart | NOT RUN |
| Different-hub reconnect | Move receiver through a different powered hub | Automatic recovery; hub/power details and recovery time recorded | NOT RUN |
| Brief disappearance | Replug quickly during child restart/backoff | At most one decoder owner; eventual recovery | NOT RUN |
| Decoder crash | Terminate only the application's decoder child in an approved test | Application detects exit, backs off, starts one replacement | NOT RUN |
| Application restart | Restart only RF Button Manager | Mappings survive; no queued actions replayed; receiver reacquired | NOT RUN |
| Server reboot | Reboot ZORIN in an approved window | Service starts automatically; hardware and mappings recover; other services healthy | NOT RUN |
| Long RF silence | Leave receiver idle for at least 30 minutes | No spurious disconnection solely because no buttons were pressed | NOT RUN |
| Ambiguous receivers | Attach two supported receivers with unusable/duplicate serials | Clear error, no guessed receiver acquisition | NOT RUN |
| Unique receiver choice | Attach two uniquely identified receivers, select intended serial | Correct receiver only, stable after port changes | NOT RUN |
| Slow/failed endpoint | Use a controlled test endpoint with delay/error | Bounded resource use, timeout/error recorded, no automatic retry | NOT RUN |
| Restart during action | Restart during a deliberately slow harmless webhook | No replay; outcome ambiguity accurately reported | NOT RUN |
| Backup and restore | Export, alter a test configuration, restore export | Mapping preservation; restored actions disabled until reviewed | NOT RUN |
| Upgrade | Apply an approved replacement binary | Existing data and token retained; one receiver process | NOT RUN |
| Uninstall | Apply uninstall in an approved maintenance window | Only owned service/files removed; state preserved; other services untouched | NOT RUN |

Release may proceed only after failures are fixed and rerun with recorded evidence.
If a hardware topology or protocol was not tested, name that limitation in the
release notes rather than claiming all receivers and hubs are supported.
