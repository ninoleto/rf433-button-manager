# 0.4 validation record and operator test plan

Version: **0.4.0-test.1**. Base: published RC1 commit
`e1822594297c0d5b208c22b671a3be0f7a183445`. No production host change or GitHub
publication is performed by building this package.

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

## Software evidence in this workspace

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

## ZORIN acceptance — pending

Use the private package and UPGRADING.md. Keep the rollback snapshot. Record the
app commit/version, OS/kernel, rtl_433 version, receiver identity and which client
browser/screen was tested. Never include tokens or webhook URLs in shared reports.

1. **Upgrade:** verify same remotes, nine configured buttons, mapped identities,
   webhooks, selected tuner, token, environment and enabled flags; only the new
   service is enabled and owns one decoder child. Confirm the renamed paths.
2. **Reboot:** with operator approval, confirm service and receiver recover after
   a host reboot. An enabled service alone is not evidence of receiver recovery.
3. **Problem button:** first inspect/relearn it using three separate presses and
   automatic identity fields. Confirm the detected model/identity corresponds to
   the intended remote. Re-enable the saved action after deliberate validation.
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
   persistence and local recovery. Check RF receiving continues during changes.
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
