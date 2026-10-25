package receiver

import (
	"testing"
	"time"
)

func TestReceiverInventoryAndSelection(t *testing.T) {
	ds := []device{{serial: "desk", name: "Desk tuner", port: "1-2", usbID: "0bda:2838"}, {serial: "roof", name: "Roof tuner", port: "1-3"}}
	list := deviceInventory(ds)
	if len(list) != 2 || list[0].Name != "Desk tuner" || list[0].USBPort != "1-2" || list[0].USBID != "0bda:2838" {
		t.Fatal(list)
	}
	if ValidateSelection(list, "roof") != nil || ValidateSelection(list, "") == nil || ValidateSelection(list, "missing") == nil {
		t.Fatal("unsafe selection")
	}
	ds[1].serial = "desk"
	list = deviceInventory(ds)
	if list[0].Selectable || list[1].Selectable || ValidateSelection(list, "desk") == nil {
		t.Fatal("duplicate serial selected")
	}
	list = deviceInventory([]device{{port: "2-1"}})
	if list[0].Selectable || list[0].Reason == "" || ValidateSelection(list, "") != nil {
		t.Fatal("serial-less singleton")
	}
	if ValidateSelection(deviceInventory(nil), "") == nil {
		t.Fatal("absent receiver selected")
	}
}

func TestSupervisorReportsPhysicalReceiverAndStopsOnAmbiguity(t *testing.T) {
	f := startFixture(t, time.Second)
	first := receive(t, f.processes)
	f.setDevices(device{serial: "serial", incarnation: "port1:2", name: "Fixture tuner", port: "1-2"})
	receive(t, first.stopped)
	p := receive(t, f.processes)
	p.out <- record{data: []byte(signalJSON)}
	s := waitState(t, f.statuses, "connected")
	if s.Name != "Fixture tuner" || s.USBPort != "1-2" || s.Serial != "serial" {
		t.Fatal(s)
	}
	f.setDevices(device{serial: "serial", incarnation: "port1:2"}, device{serial: "other", incarnation: "port2:1"})
	receive(t, p.stopped)
	s = waitState(t, f.statuses, "error")
	if s.USBPort != "" || s.Name != "" {
		t.Fatal("stale device shown as active", s)
	}
}

func TestPinnedReceiverNeverFallsBackAndRejectsNewDuplicate(t *testing.T) {
	f := startConfiguredFixture(t, time.Second, Config{Serial: "serial"})
	first := receive(t, f.processes)
	f.setDevices(device{serial: "unselected", incarnation: "port2:1"})
	receive(t, first.stopped)
	waitState(t, f.statuses, "disconnected")
	select {
	case <-f.processes:
		t.Fatal("fell back to unselected receiver")
	default:
	}
	f.setDevices(device{serial: "serial", incarnation: "newport:2", port: "2-4", name: "Chosen tuner"}, device{serial: "unselected", incarnation: "port2:1"})
	second := receive(t, f.processes)
	second.out <- record{data: []byte(heartbeatJSON)}
	status := waitState(t, f.statuses, "connected")
	if status.Serial != "serial" || status.USBPort != "2-4" {
		t.Fatal("wrong receiver after USB move", status)
	}
	f.setDevices(device{serial: "serial", incarnation: "newport:2"}, device{serial: "serial", incarnation: "duplicate:3"})
	receive(t, second.stopped)
	waitState(t, f.statuses, "error")
}
