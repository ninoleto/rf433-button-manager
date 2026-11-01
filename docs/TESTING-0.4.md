# 0.4 validation record and operator test plan

Version: **0.4.0-test.4**. Base: published RC1 commit
`e1822594297c0d5b208c22b671a3be0f7a183445`. No production host change or GitHub
publication is performed by building this package.

## Test.4: network cleanup and reception comparison

The upper **Server address** picker is now the only address control. **Advanced:
ports** contains only the HTTP/HTTPS port fields. Localhost HTTP remains available
independently of network HTTP or HTTPS; staged confirmation and rollback are
unchanged. This UI does not assign a new IP address to the Linux computer.

**Settings → Reception sensitivity** offers **Standard** (the existing default)
and opt-in **Adaptive** (`rtl_433 -Y autolevel` only). Normal reception and capture
use the same saved setting. No gain, sample rate, frequency, identity or timing
profile is changed. The app obtains rtl_433 signal/SNR/noise observations separately
from matching and displays unavailable or older readings explicitly.

Each selected saved button has **Reception diagnostics**, with recognized-press
and suppressed-repeat counts, last matched levels/time and latest action result.
Other devices cannot inflate that button's count. Counts do not measure physical
attempts or prove delivery; reset does not clear debounce or change action flags.
They are temporary and clear on receiver restart/source change or mapping change.

### First Zorin comparison for this build

1. Upgrade and confirm **0.4.0-test.4**, **Connected**, the same saved buttons and
   webhooks, and **Standard** sensitivity. Keep the existing mapping.
2. Select the problem button and open **Reception diagnostics**. Finish/close
   learning and inspection. If you do not want actions while counting, turn this
   button's **Enabled** off and **Save button** first.
3. Reset counts, wait two seconds, then make **30 presses, two seconds apart**,
   from the normal kitchen position. Keep the same grip/orientation and receiver
   placement. Record attempted presses, recognized presses, repeats and levels.
4. Choose **Adaptive** in Settings, save and wait for **Connected**. Return to the
   button, reset counts, wait two seconds and repeat the same 30-press comparison.
   Do not use Inspect for this comparison: it pauses normal dispatch and resets
   the receiver plan. A missed transmission has no level measurement.
5. Compare results, then check the other eight assignments and idle behavior.
   Keep the mode that behaves best. Restore this button's original enabled flag
   and save it. Check resulting actions separately when ready to operate them.

The operator previously estimated about 95% success from the kitchen, improving
when the remote's front was not covered. This is an informal report with no fixed
attempt count, not a measured acceptance rate or proof that the remote is faulty.
Inspection/USB port swaps explain some reported receiver lifecycle events; no USB
fault was established by the supplied excerpt. Stream Deck cable errors are not
used as evidence against this receiver. See [the audit](RECEIVER-AUDIT-0.4.md).

Software evidence is in TEST-REPORT.md. Physical improvement, false matches, hold
behavior and latency still require this comparison on the operator's equipment.
The published release and earlier private archives remain unchanged.

## Implemented changes

- Desktop Classic/Unified sidebar preference, compact tablet remote selector and
  phone bottom navigation with a separate editor. The same DOM/controller keeps
  selections and unsaved button fields across layout changes.
- Cleaner login with expandable token instructions, Show/Hide and Copy command;
  authenticated token replacement with immediate revocation of the old token.
- Native private HTTP/HTTPS listeners, local CA download and leaf renewal,
  three-minute staged confirmation, cancellation and restart/expiry rollback.
- Current source, package folder, executable, module, service, environment/state
  paths and inspection script use RF433/generic Linux names. Legacy installs use
  the explicit stopped-state migration; old history/archives remain intact.
- A 350 ms first-press comparison gives a built-in rtl_433 decoder priority over
  an early custom-profile event. Custom-only buttons still learn after that bound.
  Three separate matching presses and exact identities remain required. No saved
  mapping or RF timing is silently rewritten; relearn a problematic button.

## Initial test.1 software evidence (historical)

- Full Go race tests, static analysis, **15 Node controller checks** and deployment/
  history simulations passed. **10 updater checks** passed against temporary data
  and fake services. These are software checks, not host installation tests.
- New migration simulations cover success, failed verification, failed stop,
  partial directory migration and refusal of customized legacy units. They check
  preservation of data/token/environment and original enabled flags.
- New management tests cover token persistence/rotation/write failure, live auth
  checks, TLS trust and IP SANs, secure confirmation, occupied ports, timeout
  rollback and private file modes.
- The built **demo executable** passes HTTP-to-HTTPS staging and confirmation,
  authenticated CA download/trust, token rotation/old-token rejection, persistence
  after restart and certificate renewal with the same CA. No USB or external
  webhook is used.
- UI controller checks cover layout/page changes with unsaved fields, network
  staging/cancellation, token confirmation and subsequent authenticated requests.
- The private other-room recording replayed with rtl_433 23.11 yields **nine
  recorded bursts / 27 native frames**. After three explicitly armed learning
  steps, those bursts produce **nine requests** to an isolated loopback endpoint,
  without duplicates. This establishes behavior on that recording, not RF range
  in the apartment or ten successful physical presses.
- Full Chromium rendering is **not yet verified here**: browser downloads return
  an unavailable-site page. The real-browser harness is updated, but controller
  tests do not establish layout, touch behavior or accessibility in an actual
  browser. Run the browser harness and inspect real clients before stable release.

Private recordings and radio identities are not included in source or package.
Public RF replay fixtures have their own checksums. Dependency versions are unchanged.
The packaged commit and binary hash are recorded in `release.json`; packaging and
clean-build checks must be completed against that commit before delivery. No new
GitHub CI run is claimed while this remains an unpublished local branch.

## Operator feedback — 31 October 2026

The operator applied `0.4.0-test.1` on ZORIN. The supplied updater reported a
successful migration from `0.3.8-dev`, the live packaged commit, 20 seconds of
stable running, authentication enforcement, configuration/token/environment and
enabled-flag preservation, persistence after service restart, and a connected
receiver owned by `rf433-button-manager.service`. This is host update evidence;
it does not establish reboot recovery, physical action success or full acceptance.

The following feedback was recorded before test.2. At that point the operator
requested notes only. Test.2 implementation is recorded below; physical acceptance
on the operator's equipment remains open.

1. **Desktop editor navigation and scrolling.** The screenshots show **Back to
   remote** even while the remote and its editor are visible side by side. Hide
   that control in this arrangement; reserve it for the compact view where the
   editor replaces the remote. The desktop scrollbar also appears displaced from
   the content boundary. Review the content width, scroll-container boundaries
   and sidebar spacing together in a real browser. Desktop layout acceptance is
   not complete.
2. **Touch remote selection.** The native dropdown at smaller screen widths was
   rejected as insufficiently touch friendly. Review the established LRBridge
   interaction before implementing a replacement. The picker needs clear, large
   touch targets and practical navigation with many remotes, while preserving
   the selected button and unsaved fields. Do not treat the current dropdown as
   an accepted tablet solution.
3. **Missed presses in another room.** The operator still reports intermittent
   missed presses and positions where no presses work. The physical reliability
   issue remains unresolved. The nine-burst recording regression is not proof
   that this real-world problem is fixed. Position dependence makes the radio
   path a relevant hypothesis; it does not by itself distinguish missing RF from
   failed decoding, identity matching, debounce or action dispatch. Compare those
   stages for deliberately counted presses at known positions. Do not compensate
   by accepting ambiguous identities or promise software can recover an absent
   signal. The screenshots still show a saved custom PWM profile; whether this
   button was relearned after the update and what alternative decoder events were
   present are not established by the screenshots.
4. **Network settings are not understandable or touch friendly.** The operator
   rejected the raw IP field/native suggestion dropdown, the lack of a visible
   localhost option and an **Allow network access** switch, and the vague
   certificate instructions. The current warning is insufficiently prominent.
   This is an open usability issue even though isolated network/TLS tests passed.

### Requirements for the network settings redesign

- Provide a prominent **Allow network access** control. Off means **Only on the
  Linux computer running this app**; on allows the deliberately selected LAN/VPN
  address. Keep local access available and show its URL explicitly. Preserve an
  existing installation's access settings during upgrades; do not silently switch
  a remote-managed server to local-only access.
- Explain that `localhost` refers to the computer opening the URL. A localhost
  link opened on the operator's Windows PC or phone does not reach the Linux
  server. Label and copy local URLs without presenting them as remote shortcuts.
- Replace the browser-native IP suggestions with a touch-friendly picker of
  server addresses. Use large targets and understandable labels for this computer,
  LAN and VPN, retaining the interface name as secondary detail. Offer manual
  address entry under Advanced. Do not bind to every interface as a shortcut.
- Show the resulting complete browser URLs, protocol and connection scope, with
  copy/open controls where appropriate. Separate ordinary connection choices
  from advanced port configuration. Preserve the timed test/confirmation rollback;
  explain the loss of remote access before a local-only change and require a
  reachable confirmation path rather than locking out the operator.
- Replace generic "import into your certificate store" wording with expandable,
  numbered instructions for Windows (including Chrome/Edge), Linux (including
  Zorin/Ubuntu and Chrome/Chromium), and browsers with a separate trust store such
  as Firefox. Include mobile guidance for the supported phone flow. Verify exact
  current steps against official platform/browser documentation before writing
  the final guide. Explain which device needs the certificate, download/import,
  the correct trust category, reopening the browser if required and how to verify
  the HTTPS connection. Do not imply a download automatically establishes trust
  or instruct people to bypass certificate errors as the normal setup.
- Explain what the downloaded public trust certificate does, how to verify its
  fingerprint through a trusted source, and that the server's private key is never
  downloaded. Keep renewal and expiration information in Advanced unless action
  is required. Check certificate names cover every offered local/LAN/VPN URL.
- Put a readable warning beside the access controls: HTTP does not encrypt the
  management token or settings; do not use plain HTTP on public/untrusted networks
  or expose the application directly to the internet. Recommend Tailscale or
  WireGuard for remote access, with HTTPS where practical. Explain that a VPN only
  protects traffic actually routed through its tunnel. These settings must not
  enable router forwarding or change firewalls/proxies/other services.
- Verify the entire flow on actual desktop and touch browsers with a first-time
  user, including changing scope, importing trust, confirming HTTPS and recovering
  from an unconfirmed connection change. Documentation links alone do not satisfy
  the in-app guidance requirement.

Production screenshots include private webhook details and signal identifiers;
they are not copied into the repository or repeated in these notes.

## RF investigation — 1 November 2026

Read-only comparison of the three existing private recordings found a concrete
mapping difference. The custom PWM profile and exact 25-bit code in the operator's
latest screenshot match the earlier close-range recording. In the latest of the
three recordings, that exact custom identity appears in three JSON events across
two burst times. The existing `Akhan-100F14` decoder emits 27 frames across nine
burst times, with one consistent device ID and command. Other emitted decoder
names do not independently identify additional physical devices.

The production `Receive` path matches the saved canonical identity including its
decoder model. An event emitted only by the built-in decoder cannot trigger a
button still mapped to the custom model. This explains a decoding/mapping loss
in those recorded bursts before webhook dispatch, even though the tuner received
enough signal for another decoder. It does not establish how many physical presses
never reached the receiver or what happened during the new positional failures.

The distributed test build changes **new learning selection**, not existing saved
identities. The pending live check is to use Relearn with automatic identity
fields, inspect the chosen model and download that attempt's RF capture before
confirming a replacement. A fresh capture from the installed version is needed
if it still selects the custom profile or fails. After a verified replacement,
count physical presses, matched events and action outcomes at the problem location.
Do not change antenna placement or gain during that initial comparison.

The recordings include pulse-level RSSI/SNR/noise headers, but they are not a
calibrated field-strength survey. rtl_433's maintainer explains that level values
are relative to full scale and depend on current gain:
[upstream measurement guidance](https://github.com/merbanan/rtl_433/discussions/2076).
No receiver setting, host service, saved mapping or webhook was changed during
this source/recording investigation. Private radio identities remain outside Git.

The operator subsequently supplied a fresh `0.4.0-test.1` capture and a learning
screenshot showing **3 of 3 matching presses**, ready to save, with the built-in
`Akhan-100F14` candidate. The exported file contains eight native frames in three
timestamp groups (4, 2, 2), all with the same identity, matching the earlier
other-room recording. No capture limit was reached. This confirms the installed
learning path selected that decoder in this attempt. Saving the replacement,
reenabling the action and counted presses at the troublesome position remain
pending; the live capture does not yet establish improved room coverage.

Follow-up normal-reception screenshots show unmatched RF events and three visible
matched/start/HTTP-200 sequences. Those sequences establish accepted HTTP requests,
not independently observed light states. The RF-only filter hides matching and
webhook events, so absence in that filtered view alone is not a dispatch diagnosis.

One expanded unmatched `Smoke-GS558` event has the same complete 24-bit payload as
the learned native button: reversing its code's 24 bits yields the native 20-bit
device ID followed by the four command bits, without discarding or changing bits.
This same alternate code is present with the native decoder in all three earlier
recordings. File-only replay of one OOK packet from the fresh capture produced
both representations with exact payload equivalence. Thus this decoder label
must not be dismissed as proof of an unrelated smoke detector. The expanded
Toyota TPMS and Nexus sensor events do not match the learned button identity.

The current matcher includes the model name and has no verified cross-decoder
equivalence path. Consequently the shown alternate representation does not match
an Akhan-only binding. Native-only relearning is incomplete for such a burst.
The failed-location raw pulse capture is still needed to examine the timing and
framing that caused the alternate-only decode, and to verify the correction on
actual failed input. Any correction must retain the complete intended payload,
reject unrelated identities, handle mapping collisions and share per-button
debounce so two decoder outputs do not send two toggles. Do not blindly map all
Smoke/Nexus/Toyota events, remove model checks globally or infer a transmitter's
identity from temporal proximity alone. No updated executable has been deployed.

The next supplied raw capture began at 00:56:46 Europe/Zagreb on 1 November. It is
a valid `0.4.0-test.1` export with raw pulses, 11 packets (10 OOK, one FSK) and no
capture-limit flag. Nine native frames and three equivalent Smoke frames form
three time groups; every native identity matches the previously learned button.
This file contains no observed Smoke-only decoded group. The operator confirmed
exactly **three physical presses**, so all three are represented in this capture.
That does not measure action latency, Home Assistant outcomes or coverage at
other positions, and does not reproduce an alternate-only physical failure.

### Post-test.1 matching correction — source validation

The targeted review found no alternate-decoder matching path in test.1. The
source now has one explicit, one-way fallback: a complete, internally consistent
`Smoke-GS558` report can satisfy an existing `Akhan-100F14` mapping only when
reversing all 24 reported code bits produces its exact 20-bit ID and supported
four-bit command. Pairing reports, malformed/inconsistent fields, changed bits,
unknown decoder names and additional required mapping fields are not relaxed.
The saved identity, schema, profiles, receiver configuration and enabled flags
are preserved. There is no new waiting window or action retry.

Both reports use the same button's existing sliding debounce and dispatch gates.
An ambiguous payload associated with two saved buttons is blocked, including
when one action is disabled. Existing configurations remain readable; new learning
cannot save this cross-decoder collision. Direct Smoke-only mappings are not
broadened to accept native Akhan messages.

This is equivalence of the **reported payload**, not proof of transmitter model
or complete RF framing: GS558's JSON does not preserve its original row length.
It does not recover unreceived/undecodable data or establish universal RF safety.
The receiver is still a receive-only, unauthenticated fixed-code system.

Software validation on 1 November:

- A fresh checkout of the unchanged test.1 packaged commit reproduced the bug:
  one synthetic alternate-only report produced zero isolated HTTP requests when
  one was expected. The same case in corrected source produces one.
- Replaying the latest private recording's raw OOK packets with rtl_433 23.11
  yields three target bursts / 12 decoder messages and exactly three isolated
  requests. Deliberately retaining only its three alternate reports also yields
  three requests. The latter is a simulation, not an observed physical failure.
- The earlier other-room recording yields nine target bursts / 36 messages and
  nine requests. Retaining only its nine alternate reports yields nine requests.
  These are recorded-burst counts, not a newly established physical press count.
- Full `make check` passed with both private regressions and public replay fixtures
  supplied: race tests, vet, 15 Node controller checks, deployment/history tests
  and 10 updater/migration simulations. Synthetic negatives cover every changed
  payload bit, unrelated models, malformed fields and extra saved identity fields.
  Mixed decoder output/held bursts, collision refusal and action gates pass too.

The next executable/package and ZORIN deployment remain pending. The distributed
test.1 archive was not overwritten. At this matching-only checkpoint, the UI/network
feedback was still open; the following test.2 work addresses its software changes.

## Test.2: matching, touch navigation and network access

The operator authorized completing these changes on 1 November 2026. Test.2
includes the exact-payload matching correction above, without a new dispatch
wait, approximate identities or changes to saved mappings. A received GS558
representation may match an existing complete Akhan mapping only under the
validated transformation. This cannot recover a transmission that the receiver
never decodes, and does not establish RF range or a measured physical latency.

- Desktop Classic/Unified layouts retain one editor and its drafts. The Back
  control is hidden beside a visible remote, the main scroll container reaches
  the viewport edge, and sidebar spacing is corrected.
- Tablets and phones use a searchable dialog with large remote rows. Compact
  editing is a separate view with Back to remote. Phone bottom navigation remains.
  The public LRBridge controller's large jump-choice pattern was inspected as a
  reference; no LRBridge code or services were modified.
- Network access follows the supplied Storage Watch reference: local-network
  switch, explicit address picker, current network/local URL cards with copy,
  large HTTP/HTTPS choices, advanced address/ports, and staged confirmation.
  The switch edits a draft; Apply and confirmation are deliberately explicit.
- Each enabled protocol has a companion IPv4 loopback listener. Disabling network
  access requires confirmation at the proposed local endpoint; the old listener
  remains available until then. Existing selected address/protocols are preserved.
- Certificates cover the selected address and local loopback/localhost. The UI
  and NETWORK.md explain importing the public CA on Windows, Linux, Firefox,
  Android, macOS and iOS, fingerprint verification, VPN use and no public forwarding.
  These are platform instructions, not completed client-OS trust-install tests.
- Token UI/rotation, schema 2, updater action restoration and test.1 branding
  migration remain intact. No new runtime dependency is added.

Software validation for this build:

- `make check` passed: race-enabled Go tests, vet, **18 controller checks**, shell
  deployment/history checks and **10 updater simulations**. The optional latest
  three-press and earlier nine-burst private replay inputs were supplied; they
  were not skipped. Original and alternate-only replay checks produce one isolated
  request per recorded burst, with no production webhook involved.
- The built demo executable passes authenticated selected/companion HTTP and HTTPS,
  CA-verified TLS, refusal to confirm a selected listener through its companion,
  token revocation on both endpoints, settings/token persistence after restart,
  and leaf renewal preserving the CA. Go tests also cover a busy companion port
  without losing current access and confirmation of local-only access.
- Chromium 141 completed **12 browser groups** against the real demo
  service, including learning, draft preservation, desktop Classic/Unified,
  820 px tablet, 390 px phone, 320 px overflow checks, certificate download and
  staged-network cancellation. A touch-capable context also checked 32 remotes,
  picker search/scrolling, a filtered bottom sheet that shrinks to its content,
  large selectable rows and unsaved editor preservation. CSS was visually inspected; screenshots
  in docs are synthetic demo state, not the operator's configuration.
- Final package reproducibility and ZIP/internal checksum checks are recorded
  separately during packaging. This local private build has no new GitHub CI or
  publication claim. Browser emulation does not replace actual phone/tablet testing.

No physical receiver, Home Assistant action, USB reconnect, ZORIN service restart,
Windows/phone CA installation, new RF range test or soak test was performed here.

## Test.3: saved Smoke/profile matching and independent localhost HTTP

The latest saved-button screenshot identifies the problem mapping as Smoke,
not Akhan. Its command is outside Akhan's four supported command values. Two
private recordings include both Smoke reports and a registered custom 25-bit
PWM representation of the same complete payload. The previous fallback only
addressed an existing Akhan mapping, so it did not cover this saved button.

A regression test failed against test.2: retaining only the custom reports
produced zero requests for the saved Smoke mapping. The new bounded conversion
produces three isolated requests, one per recorded matching burst, from either
representation or both together. This does not establish that all reported
misses had this cause: the unfiltered recordings also contain direct Smoke
reports and already produced three requests before the fix. No attempted-press
count or radio range is inferred from unrelated traffic in a recording.

The conversion requires a registered, persisted, three-press-validated 25-bit
PWM profile and all 24 payload bits, the trailing mark and zero hex padding.
GS558's redundant identity fields and zero pairing flag are validated. Commands
remain distinct; no error correction, guessed timings, RF gain/sample changes,
new receiver process or dispatch delay is introduced. A conflicting saved mapping
blocks dispatch with an explanation. Existing saved mappings and actions remain
unchanged; this retest should not start with another relearning cycle.

New synthetic tests cover each changed bit, unknown and invalid profiles, valid
profiles of another width/modulation, additional identity fields, all command
values, mixed-report/hold debounce, action gates, legacy duplicate configurations,
atomic learning overlap rejection and reopening the saved database. The private
recording tests substitute only the custom model label with a valid synthetic
profile because exports omit timing configuration. Payload bits, fragments,
timestamps and event order are unchanged. Filtering out one decoder's output
is explicitly simulated missing-output coverage, not a new physical test.

The network correction removes “Also keep HTTP available”. Local HTTP on
`127.0.0.1` stays authenticated and available independently of the selected LAN
protocol. New network choices are HTTP or HTTPS exclusively. Turning network
access off selects local HTTP only. Test.2's saved dual-protocol settings load
unchanged until an explicit replacement is applied and confirmed. Staged rollback
and certificate trust still apply; no firewall, router or VPN is configured.

Software checks for this private build are recorded in TEST-REPORT.md. Actual
room-by-room reception, nine button assignments and Windows/phone certificate
trust remain operator acceptance tasks. Test.1/test.2 downloads and the published
RC1 remain unchanged.

## ZORIN acceptance — in progress

Use the private package and UPGRADING.md. Keep the rollback snapshot. Record the
app commit/version, OS/kernel, rtl_433 version, receiver identity and which client
browser/screen was tested. Never include tokens or webhook URLs in shared reports.

1. **Upgrade:** verify same remotes, nine configured buttons, mapped identities,
   webhooks, selected tuner, token, environment and enabled flags; only the new
   service is enabled and owns one decoder child. Confirm the renamed paths.
2. **Reboot:** with operator approval, confirm service and receiver recover after
   a host reboot. An enabled service alone is not evidence of receiver recovery.
3. **Problem button:** keep the existing mapping for this retest. Test.3 supports
   the saved Smoke representation and the registered 25-bit PWM profile without
   relearning, as well as the earlier complete Akhan fallback. Close learning or
   inspection and wait for normal reception; actions are paused during capture.
   Test one press at a time from each room, with someone watching the monitor.
   Record attempted presses, recognized bursts and resulting actions separately.
   Check different buttons do not trigger each other's actions and a held press
   does not create unwanted repeated toggles.
4. **Known buttons:** verify every existing assignment with an agreed harmless
   action. No automatic process may claim physical Home Assistant outcomes.
5. **Layouts:** desktop both modes, many remotes/search, portrait tablet, phone
   Remotes/Activity/Settings, editor/back/keyboard, learning dialog, light/dark,
   scrolling and touch targets. Change layout while a field is unsaved. Check
   320–430 px phone widths, tablet rotation, keyboard navigation and browser zoom.
6. **Network:** test HTTP/HTTPS on the actual LAN, certificate trust on Windows
   and phone, cancel/timeout of an unconfirmed change, secure confirmation,
   persistence and local recovery. In all three network modes, authenticate through
   localhost HTTP on the server. With HTTPS selected and confirmed, verify LAN
   HTTP closes. With network access off, verify only local HTTP remains. A legacy
   dual-protocol setup changes only after Apply and confirmation. Check RF
   receiving continues during changes.
7. **Token:** replace it, reconnect a second tab using the new token, confirm the
   old one fails and local retrieval still works. Do not paste the value into chat.
8. **Recovery:** only with the operator, test receiver-only unplug/replug and
   selected serial behavior. Multiple tuners, duplicate serials, hub moves and
   other hardware remain pending until physically exercised.
9. **Rollback:** validate restoration of the matching old snapshot and service
   paths in a controlled window. Preserve the newer state separately.
10. **Observation:** use it normally for an agreed period and review missed or
    repeated actions, restarts and errors. Report duration and observations; do
    not label informal use a formal soak test.

Only a reviewed final commit/package that passes the agreed checks should receive
a stable version and public release. Untested hardware remains explicitly unverified.
