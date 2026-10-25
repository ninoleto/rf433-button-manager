# Button compatibility and signal inspection

RF Button Manager is not a universal Amazon-button receiver. Amazon is a retailer,
not a radio protocol. Frequency, modulation, encoding, receiver hardware, decoder
version and stable button identity all matter. A listing saying only "433 MHz" is
not sufficient proof that its particular transmitter will work.

## Current scope

| Device/signal | Current status |
| --- | --- |
| 433.92 MHz button decoded by the configured rtl_433 protocols | Can learn when the decoded event supplies a stable device/button identity across three separate presses. Must be tested per model/revision. |
| Fixed-code OOK PWM, PPM or supported PCM, 12–64 bits | Bounded automatic discovery and rtl_433 replay; consistent repeated frames and three separately armed presses required. Not every encoding/timing pattern is supported. |
| FSK with an existing rtl_433 decoder yielding a suitable button identity | Possible through the decoded-event path; no generic automatic FSK profile discovery. |
| Unrecognized pulses | Can be recorded for investigation; a recording does not itself provide a validated button code. |
| Rolling/encrypted/changing payloads | No general support or decryption. A dedicated upstream decoder might expose a stable usable button identity, but that requires per-protocol review/testing. Never classify a failed capture as rolling/encrypted solely from the failure. |
| 315, 868 or 915 MHz buttons | Outside this application's current fixed 433.92 MHz receive configuration, even though upstream rtl_433 can tune other bands. |
| Wi-Fi, Zigbee, Bluetooth, Matter or infrared buttons | Outside this RTL-SDR/433.92 MHz integration. They require their own receiving hardware/integration. |

Only one configured receiver is used. Custom protocol selections and flex settings
remain in effect during inspection. A mismatched antenna, poor signal, USB ownership
or permissions can also prevent capture. Receiver "connected" means sample
acquisition, not successful button decoding. Other nearby transmitters can produce
traffic even when the intended button was not received.

## Evidence, not a universal compatibility claim

| Evidence | What has been checked |
| --- | --- |
| Three assigned buttons at the original operator's home | Operator tested all three and their webhooks and accepted the latest interface within available testing for 0.3.6-dev publication. Commercial model/revision identifiers were not supplied, so these are not a purchase recommendation or named-model guarantee. |
| First-switch/front-door recordings | Private offline PWM replay, three software-armed observations, persistence and local webhook routing. Private RF IDs are not distributed. |
| EV1527, PT2262, Akhan and HT680 example recordings | Checksum-pinned public IQ files pass software tests. This validates those recordings, not every product sharing a chipset or listing name. |
| Future six-button Amazon remote | Not yet available for physical testing. |
| Physical multi-tuner selection, USB hub/port recovery, soak and wider receiver/platform combinations | Unverified; consult HARDWARE-TESTS.md. |

An honest public release describes this as a development tool for supported
433.92 MHz buttons. Community reports should name the exact product/revision,
receiver, rtl_433 version, app version and completed tests. Device support may
differ between rtl_433 releases. This app does not silently replace rtl_433.

## Inspect a button without setting up a webhook

1. Click **Inspect signal** in the browser. No remote/button needs to exist.
   All webhook actions pause for the session and receiver restoration.
2. Wait for the ready instruction; press/release only the intended button once.
3. If decoding succeeds, **Code / button identifier** shows the candidate. For
   a flex decoder, it includes the model and length-qualified hex code. A code
   alone is not globally unique: keep the model/decoder information with it.
4. Use **Ready for next press** and repeat twice. Three matching observations
   support repeatability in this test; they do not prove encryption status,
   universal compatibility, RF authenticity or long-term reliability.
5. Download the **inspection report** before finishing. It includes the candidate,
   repeat count, candidate decoder (if discovered), RF diagnostics and scope limits.
6. If no code was decoded, download the **RF capture** too. This retains bounded
   pulse records and decoded messages, including unidentified FSK traffic. It is
   not an original wideband IQ recording and is not a frequency scanner.
7. **Finish inspection** or **Stop inspection** restores normal reception. Neither
   creates a mapping, changes existing button settings nor sends a test webhook.
   To assign an action, select/create a button and use the normal Learn workflow.

Reports/captures exclude application credentials, webhook URLs and headers. They
contain radio/device identifiers, including potentially nearby devices. Review
them before sharing. A new attempt replaces the retained RF capture. Inspection
reports are available while the session/result dialog is open; download before
closing it. No successful mapping is invented for an undecoded signal.

## Extending support

An unsupported captured protocol can sometimes be added as an upstream rtl_433
decoder or a validated flex profile. Preserve original evidence, identify the
intended transmitter with repeated controlled presses, and test against other
buttons/background captures to avoid false matches. Some protocols need work
beyond a timing preset; an inspector cannot recover a secret encryption key or
turn a different radio technology into a 433.92 MHz signal.

Primary references (reviewed 2026-10-25):

- [rtl_433 project and supported decoder list](https://github.com/merbanan/rtl_433)
- [Receiver frequencies and capture options](https://github.com/merbanan/rtl_433/blob/master/docs/STARTING.md)
- [Protocol selection and flexible decoders](https://github.com/merbanan/rtl_433/blob/master/docs/OPERATION.md)
