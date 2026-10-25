# Choosing and identifying a receiver

Open **Settings → Receiver device**. The list shows detected RTL-SDR tuners by
product name, USB serial and current USB port. Select the tuner to use, then
choose **Save settings**. **Refresh devices** rescans without opening a receiver
or interrupting reception. Changes preserve remotes, mappings and webhooks.
Finish or cancel any learning/inspection session before changing receivers.

The app runs **one receiving tuner at a time**, even when several are connected.
It starts only its own rtl_433 child with the selected serial. It does not stop
other applications or claim that a listed device is free to use. A busy receiver,
missing USB permission or driver conflict is reported by the reception status.

## Identity and activity

The dashboard displays the selected/active tuner's product, serial and USB port.
The receiving process supplies this identity; it is not inferred from the first
item in the device list. New RF, matched-button and receiver events retain their
source so switching tuners does not relabel past events. Older events have no
source metadata. Serial and port are device identifiers; avoid public screenshots
or sharing private event histories unnecessarily.

- **Connected** means samples or pulse records are arriving.
- **Decoded RF signal received** marks a recent decoded rtl_433 event. It can be
  another transmitter or an event rejected by learning; it is not proof of a
  matched button or successful webhook. The indicator then shows the last time.
- **Physical button received** in the event monitor means a saved mapping matched.
  Webhook results remain separate events.
- **Recovering** means the receiver is being started, not confirmed acquisition.

Activity is cleared when reception disconnects or changes receiver. Inventory
alone never lights the RF activity indicator. Refreshing the browser does not
choose or save a different tuner.

## Several tuners, missing devices and duplicate serials

Choose a **unique serial** when several tuners are attached. The choice persists
in the application database and follows that tuner when it changes USB ports or
hubs. If it is unplugged, the app waits for it; another tuner is not substituted.
If its serial becomes ambiguous, acquisition stops until the ambiguity clears.

**Automatic** works only with exactly one detected tuner. Adding another tuner
pauses automatic acquisition until a unique serial is selected or only one tuner
remains. A sole tuner without a serial can use automatic mode.

Some tuners ship with identical factory serials. The list shows each USB port but
disables those individual choices and explains the duplicate. This release does
not write tuner EEPROMs or use changing enumeration indexes as persistent
identity. Disconnect duplicates or arrange unique serials before selecting one.
Simultaneous acquisition from several tuners and port-based selection of duplicate
serials are not implemented.

USB discovery is Linux sysfs-only and does not open devices. It recognizes the
USB IDs listed by the receiver integration; it is not a list of every possible
DVB/SoapySDR device. USB access rules and support for an individual tuner still
need verification on its host.

## Existing mappings and inspection correction in 0.3.6-dev

This update does not migrate or rewrite learned codes, decoder profiles, webhook
configuration or action flags. It preserves normal matching of saved assignments.
The prebuilt updater still requires actions to be disabled before deployment.
Re-enable the actions you want after checking the updated GUI.

New automatic learning and inspection reject a saved profile's fragment when
the same raw record contains repeated complete frames of a different width.
The same rule applies to existing-profile offline replay and explicitly bounded
custom flex decoders. It does not pick the longest row blindly: a genuine repeated
12-bit frame remains valid despite a single longer trailing fragment. Existing
mappings are not silently changed; a mapping based on a partial row may still be
less reliable and should be reviewed/relearned deliberately.

## Validation boundary

Automated tests cover inventory, serial selection, missing/duplicate devices,
USB moves, process supervision, API authentication, settings persistence, three
preserved assignments and UI controls. Saved private RF recordings exercise the
inspection correction with upstream rtl_433. These tests simulate USB topology;
they do not establish physical multi-tuner or hub recovery acceptance. Record
those results in [HARDWARE-TESTS.md](HARDWARE-TESTS.md) when performed.
