package receiver

import "errors"

// DeviceInfo is a read-only sysfs inventory, not evidence of sample acquisition.
// USBPort is display-only: selection is by unique serial, never USB enumeration order.
type DeviceInfo struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Serial       string `json:"serial"`
	USBPort      string `json:"usb_port"`
	USBID        string `json:"usb_id"`
	Selectable   bool   `json:"selectable"`
	Reason       string `json:"reason,omitempty"`
}

func Devices() ([]DeviceInfo, error) {
	if !platformSupported {
		return nil, errors.New("receiver discovery is available on Linux only")
	}
	ds, err := defaultRuntime().discover()
	if err != nil {
		return nil, errors.New("cannot list USB receivers; check service access to sysfs")
	}
	return deviceInventory(ds), nil
}

func deviceInventory(ds []device) []DeviceInfo {
	counts := map[string]int{}
	for _, d := range ds {
		counts[d.serial]++
	}
	out := make([]DeviceInfo, 0, len(ds))
	for _, d := range ds {
		info := DeviceInfo{Name: d.name, Manufacturer: d.manufacturer, Serial: d.serial, USBPort: d.port, USBID: d.usbID, Selectable: true}
		if info.Name == "" {
			info.Name = "RTL-SDR receiver"
		}
		switch {
		case d.serial == "":
			info.Selectable = false
			info.Reason = "No USB serial; automatic mode works only while this is the sole receiver"
		case counts[d.serial] != 1:
			info.Selectable = false
			info.Reason = "Duplicate USB serial; disconnect duplicates or assign unique serials before selecting"
		}
		out = append(out, info)
	}
	return out
}

// ValidateSelection rejects ambiguous/missing choices without opening any receiver.
func ValidateSelection(ds []DeviceInfo, serial string) error {
	if serial == "" {
		if len(ds) != 1 {
			return errors.New("automatic selection requires exactly one connected receiver; choose a device with a unique serial")
		}
		return nil
	}
	matches := 0
	for _, d := range ds {
		if d.Serial == serial {
			matches++
			if !d.Selectable {
				return errors.New(d.Reason)
			}
		}
	}
	if matches != 1 {
		return errors.New("selected receiver is missing or ambiguous; refresh the device list")
	}
	return nil
}
