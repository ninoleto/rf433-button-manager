> Historical installation handoff. Browser learning in 0.2 supersedes the manual
> decoder prerequisite here; use [LEARNING.md](LEARNING.md). Existing application
> authorization still applies. Physical receiver handover/replug needs participation.

# Ready-to-paste local Codex continuation task

**Continuation note, 2026-10-25:** The operator has explicitly authorized this
application's reviewed installation/configuration. That session authorization
supersedes the older STOP/reconfirmation wording below for the same scope.
Ask before stopping a competing receiver owner or physical USB participation;
do not alter unrelated services. If host execution is unavailable, provide scoped
manual commands and continue independent development. Public publication is
authorized only after installation, agreed testing and documentation are complete.
See PROGRESS.md and TEST-REPORT.md for current evidence and outstanding blockers.

Continue RF Button Manager from the supplied 0.1.0-dev source/binary package.
Read README.md, docs/ARCHITECTURE.md, docs/PROGRESS.md, docs/OPERATIONS.md and
docs/HARDWARE-TESTS.md before acting. The package contains a standalone Git bundle
for restoring the development history. The application was built and software-tested
in an isolated environment; no physical receiver or Home Assistant test has passed.

## Immediate task: inspect ZORIN without changing it

1. Work in an isolated checkout/directory. Use the user's existing authorized SSH
   connection to nino@ZORIN if available. Do not request passwords in chat.
2. Run scripts/inspect-zorin.sh on ZORIN without installing anything. Record OS,
   rtl_433 version, supported USB identity/serial and competing processes/services.
   The script intentionally does not open the receiver or print process arguments.
3. Locate the exact working Switch433 flex decoder and existing test implementation.
   Inspect targeted rtl_433 configuration locations and the user's identified test
   files. Do not infer timings from analyzer suggestions. Keep webhook URLs, IDs,
   tokens and other secrets out of logs, commits and replies.
4. Obtain representative already-recorded decoded JSON for several separate presses
   if available. Do not launch a second rtl_433 process or interrupt its current owner.
   Determine whether code fields are stable and whether the conservative current
   normalizer supports the actual JSON shape. Add sanitized fixtures and regression
   tests in the isolated checkout when samples are available.
5. Review whether existing udev permissions already grant access and whether the
   device serial is unique. Do not change serial EEPROM values, kernel modules,
   Docker, Home Assistant, existing services or USB rules during inspection.
6. Produce a concise report and exact deployment diff: binary, service account,
   directories, unit, environment and USB rule; identify any existing receiver process
   that must be stopped. Include rollback and data-preservation steps. STOP before
   deploying and obtain the user's explicit approval for these concrete changes.

## After deployment approval only

Perform the approved installation and exclusive receiver handover. Import the exact
working decoder through Settings, configure the intended private browser access,
learn A ON with three separate armed presses, save the actual Home Assistant webhook,
test deliberately, enable, and verify the physical action. Do not modify HA automations
unless separately authorized. Complete every relevant row of HARDWARE-TESTS.md:
USB same/different port, different hub, prolonged removal/reconnect, decoder crash,
RF silence, held key, rapid distinct presses, app restart and reboot. A reboot needs
the user's approval because this is a production server with other services.

Record real timestamps, outcomes and failures; mark unperformed checks unverified.
Fix failures in the checkout and rerun appropriate tests. Never declare production
readiness or publish a stable release until the physical acceptance gate passes.
Keep commits logical and update docs/PROGRESS.md so another session can resume.

## GitHub publication

The intended separate repository is ninoleto/rf-button-manager. It has not been
created by this session: the GitHub connector lacks repository creation and the
browser requires sign-in. When authorized account access is available, create the
repository without replacing any existing project, inspect all staged files for
secrets, and push the source history. Keep 0.1.0-dev clearly labeled as unvalidated
on hardware. The package intentionally excludes tokens, databases and test state.
