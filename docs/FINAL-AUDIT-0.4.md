# Final 0.4.0 audit — 2026-11-01

## Scope and conclusion

Reviewed the current test.6 reception and action paths and prepared 0.4.0 with the
maintainer's final presentation changes. No new release-blocking defect was found
in the reviewed paths. This is a bounded code and software verification review,
not certification of RF reliability or a complete penetration test.

The final runtime diff from test.6 is the version constant and two frontend
changes: Method and Timeout are vertically stacked at every viewport size, and
displayed times use local 24-hour time. Midnight displays 00 rather than 24.
Dates follow the browser locale, the timezone remains the browser's timezone,
and raw API/export timestamps retain their original values.

No decoder, RF gain, threshold, sensitivity, debounce, matching, webhook retry,
network binding, schema, mapping or installed configuration change was made in
this final pass. Packaging metadata now points to the actual hardware observations
instead of always claiming that operator testing has not started.

## Reviewed behavior

| Area | Review result |
| --- | --- |
| Reception and framing | One supervised live receiver; bounded pulse parsing; the extra normal-use PWM decoder retains exact complete-word matching and is excluded during learning. Incomplete words are not repaired. |
| Matching and repeated reports | Native/custom equivalents share button debounce. Conflicting mappings are refused. Background reports are not treated as physical button attempts. |
| Recording | Start/stop retains bounded evidence without a different decoder plan or action path. Offline parity does not prove zero live-host timing overhead. |
| Action delivery | Configuration/storage/learning gates remain in effect. One action per button and eight globally are allowed in flight; excess presses are not queued. Redirects, proxies and automatic retries remain disabled. HTTP 2xx is request acceptance, not physical-device confirmation. |
| Management and recovery | Token authentication and browser origin checks protect the API. Localhost HTTP remains available while network settings are staged, confirmed or rolled back. The updater preserves configuration and enabled flags and retains rollback snapshots. |
| Data handling | UI labels and decoded payloads are rendered as text. Diagnostic exports exclude application credentials and webhook settings but still contain private RF identifiers. Raw user captures and production screenshots remain outside Git and release assets. |

## Executed software checks

- Go 1.27.3 module verification, full uncached race tests, vet and JavaScript
  syntax passed. The pinned public RF recordings were available to the tests.
- All 25 frontend controller checks, deployment/history simulations and 10
  updater simulations passed.
- All 16 real headless Chrome for Testing 141 browser groups passed. The action
  fields remain vertical and touch controls remain at least 48 pixels high at
  1440, 820, 390 and 320 pixel widths in light/dark themes. Screens were visually
  inspected. The time checks cover English and Croatian locales, Europe/Zagreb
  and UTC browser timezones, midnight, reception and action timestamps, the
  receiver sidebar and activity events. Original ISO event timestamps are preserved.
- Real isolated demo HTTP/TLS checks passed for local access, network HTTP,
  network HTTPS, network-off, certificate trust/renewal, staged confirmation and
  rollback, token rotation/revocation and restart persistence.
- Four private recordings replayed through rtl_433 23.11, matching, debounce and
  isolated HTTP retained the earlier original/corrected counts: 10/10, 10/10,
  32/32 and 4/8. In the last trial, two incomplete bursts remain undecodable.
  Stored configuration stayed unchanged apart from last-received timestamps.
- A private decoded-report replay produced 10 recognized presses, five repeated
  reports and 10 isolated actions with recording off, on and stopped. Receiver
  plan, settings, action epoch and configuration were unchanged. This is software
  parity on one recording, not a causal explanation for every physical miss.
- Gitleaks 8.30.1 source/history scans and a separate scan for known private RF
  identifiers found no matches. Such scans supplement review; they cannot prove
  the absence of every possible secret.

All test endpoints and data are disposable. No real USB receiver, production
service or Home Assistant action is operated by these checks. Private replay
inputs and the temporary parity harness are not distributed.

## Distribution checks

Before delivery, review current source and all reachable outgoing history for
secrets and private identifiers. Package only clean committed source, the matching
binary, full source history and dependency notices. Verify ZIP integrity, every
internal checksum, the reported version and source tree. Rebuild from the included
Git bundle with the same toolchain and compare binary hashes. The package's exact
commit/hashes belong in release.json and the accompanying archive checksum.

GitHub publication additionally requires CI for that exact commit and verification
of the publicly downloaded archive, as documented in RELEASING.md. Local review
and a downloadable package do not claim those publication checks have happened.

## Remaining limits and future improvement

The maintainer's available equipment and latest observations are recorded in
HARDWARE-TESTS.md. Intermittent misses remain unexplained; no aggregate 98% rate
is established. Other receiver/remote combinations, multi-tuner hardware, hub
recovery and a formal extended soak remain unverified.

A useful future diagnostic improvement would identify a failed physical attempt
and correlate it with receiver evidence and the destination's device state.
Current logs cannot identify a press that produced no decoded report, and an old
successful event can remain visible after a miss. Raw IQ analysis and controlled
receiver/gain comparisons would require new evidence and operator participation.
These are opportunities for later investigation, not grounds for speculative
matching changes or automatic webhook retries in this release.
