package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// inputEvent matches the Linux kernel's struct input_event on 64-bit systems
// Size: 8 (sec) + 8 (usec) + 2 (type) + 2 (code) + 4 (value) = 24 bytes
type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

const (
	evKey    uint16 = 1
	evKeyDown int32 = 1
	evKeyUp   int32 = 0
)

var hookFiles []*os.File

func (a *App) ActivateAction(action string) {
	switch action {
	case "NextChar":
		a.ActivateNextChar()
	case "PreviousChar":
		a.ActivatePreviousChar()
	}
}

// PauseHook pauses the organizer
func (a *App) PauseHook() {
	isOrganizerRunning = false
}

// ResumeHook resumes the organizer
func (a *App) ResumeHook() {
	isOrganizerRunning = true
}

// findInputDevices parses /proc/bus/input/devices to find keyboard and mouse event devices.
// Keyboard: EV_KEY (bit 1) + EV_REP (bit 20), no EV_REL.
// Mouse:    EV_KEY (bit 1) + EV_REL (bit 2)   (EV_REP optional, some gaming mice have it).
func findInputDevices() []string {
	content, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil
	}

	var devices []string
	var currentHandlers string
	var hasKey, hasRep, hasRel bool

	for _, line := range strings.Split(string(content), "\n") {
		switch {
		case strings.HasPrefix(line, "H: Handlers="):
			currentHandlers = line

		case strings.HasPrefix(line, "B: EV="):
			evHexStr := strings.TrimPrefix(line, "B: EV=")
			var evHex uint64
			fmt.Sscanf(evHexStr, "%x", &evHex)
			hasKey = (evHex & (1 << 1)) != 0  // EV_KEY
			hasRep = (evHex & (1 << 20)) != 0 // EV_REP (keyboard repeat)
			hasRel = (evHex & (1 << 2)) != 0  // EV_REL (mouse relative movement)

		case line == "":
			isKeyboard := hasKey && hasRep && !hasRel
			isMouse := hasKey && hasRel // EV_REP optional for mice
			if (isKeyboard || isMouse) && currentHandlers != "" {
				parts := strings.Fields(strings.TrimPrefix(currentHandlers, "H: Handlers="))
				for _, handler := range parts {
					if strings.HasPrefix(handler, "event") {
						devices = append(devices, "/dev/input/"+handler)
					}
				}
			}
			currentHandlers = ""
			hasKey = false
			hasRep = false
			hasRel = false
		}
	}

	return devices
}

// InstallHook finds keyboard and mouse devices and starts reading from them
func (a *App) InstallHook() error {
	if keyEventChan == nil {
		keyEventChan = make(chan KeyEvent, 100)
	}

	devices := findInputDevices()
	if len(devices) == 0 {
		return fmt.Errorf("no keyboard/mouse devices found in /proc/bus/input/devices")
	}

	opened := 0
	for _, device := range devices {
		f, err := os.Open(device)
		if err != nil {
			// Likely a permission error - user needs to be in 'input' group
			continue
		}
		hookFiles = append(hookFiles, f)
		opened++
		go a.readEvdevDevice(f)
	}

	if opened == 0 {
		return fmt.Errorf(
			"could not open any input device (permission denied)\n" +
				"Fix: sudo usermod -aG input $USER  (then log out and back in)",
		)
	}

	fmt.Printf("Input hook installed on %d/%d device(s).\n", opened, len(devices))
	return nil
}

// UninstallHook closes all open evdev file handles
func (a *App) UninstallHook() {
	for _, f := range hookFiles {
		f.Close()
	}
	hookFiles = nil
	fmt.Println("Keyboard hook uninstalled.")
}

// readEvdevDevice reads raw input events from a /dev/input/eventN device in a goroutine.
// It sends KEY events into keyEventChan. Returns when the file is closed.
func (a *App) readEvdevDevice(f *os.File) {
	buf := make([]byte, 24) // sizeof(struct input_event) on 64-bit Linux
	for {
		_, err := io.ReadFull(f, buf)
		if err != nil {
			// File was closed (UninstallHook) or device disconnected
			return
		}

		evType := binary.LittleEndian.Uint16(buf[16:18])
		evCode := binary.LittleEndian.Uint16(buf[18:20])
		evValue := int32(binary.LittleEndian.Uint32(buf[20:24]))

		if evType == uint16(evKey) {
			select {
			case keyEventChan <- KeyEvent{Code: int32(evCode), Value: evValue}:
			default:
				// Channel full, drop the event
			}
		}
	}
}

// GoHook is the main event processing loop. Blocks until timeout (59 min).
func (a *App) GoHook() error {
	for {
		select {
		case <-time.After(59 * time.Minute):
			fmt.Println("Received timeout signal")
			return nil
		case evt := <-keyEventChan:
			switch evt.Value {
			case evKeyDown:
				a.handleKeyDown(evt.Code)
			case evKeyUp:
				a.handleKeyUp(evt.Code)
			}
		}
	}
}

func (a *App) handleKeyDown(eventKey int32) {
	if keybinds, exists := keybindMap[eventKey]; exists {
		if keybinds.Action == "StopOrganizer" && !isKeyPressed[eventKey] {
			isOrganizerRunning = !isOrganizerRunning
			a.UpdateOrganizerRunning()
		} else if isOrganizerRunning && !isKeyPressed[eventKey] {
			a.ActivateAction(keybindMap[eventKey].Action)
		}
		isKeyPressed[eventKey] = true
	}
}

func (a *App) handleKeyUp(eventKey int32) {
	if _, exists := keybindMap[eventKey]; exists {
		if isKeyPressed[eventKey] {
			isKeyPressed[eventKey] = false
		}
	}
}
