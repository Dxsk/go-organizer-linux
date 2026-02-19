package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	xconn *xgb.Conn
	xroot xproto.Window

	// X11 EWMH atoms
	atomNetClientList   xproto.Atom
	atomNetActiveWindow xproto.Atom
	atomNetWmPid        xproto.Atom
	atomNetWmName       xproto.Atom
	atomUtf8String      xproto.Atom
	atomNetWmState      xproto.Atom
	atomNetWmStateAbove xproto.Atom
)

// findX11Display tries each available X11 socket and returns the first one
// that accepts a connection. Priority: current DISPLAY env var first, then
// sockets found in /tmp/.X11-unix/.
func findX11Display() (*xgb.Conn, string, error) {
	var candidates []string

	// Current DISPLAY env var has priority
	if d := os.Getenv("DISPLAY"); d != "" {
		candidates = append(candidates, d)
	}

	// Add sockets not already listed
	entries, _ := os.ReadDir("/tmp/.X11-unix")
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "X") {
			continue
		}
		d := ":" + e.Name()[1:]
		already := false
		for _, c := range candidates {
			if c == d {
				already = true
				break
			}
		}
		if !already {
			candidates = append(candidates, d)
		}
	}

	for _, d := range candidates {
		os.Setenv("DISPLAY", d)
		conn, err := xgb.NewConn()
		if err == nil {
			return conn, d, nil
		}
	}

	return nil, "", fmt.Errorf("no working X11 display found (tried %v)", candidates)
}

// initX11 connects to the X11 display and interns the required atoms.
// Tries all available X11 sockets automatically.
func initX11() error {
	var err error
	var display string
	xconn, display, err = findX11Display()
	if err != nil {
		return fmt.Errorf("cannot connect to X11 (is XWayland running?): %v", err)
	}
	log.Printf("Connected to X11 display: %s", display)

	setup := xproto.Setup(xconn)
	xroot = setup.DefaultScreen(xconn).Root

	internAtom := func(name string) xproto.Atom {
		reply, err := xproto.InternAtom(xconn, false, uint16(len(name)), name).Reply()
		if err != nil {
			log.Printf("Warning: could not intern X11 atom '%s': %v", name, err)
			return xproto.AtomNone
		}
		return reply.Atom
	}

	atomNetClientList   = internAtom("_NET_CLIENT_LIST")
	atomNetActiveWindow = internAtom("_NET_ACTIVE_WINDOW")
	atomNetWmPid        = internAtom("_NET_WM_PID")
	atomNetWmName       = internAtom("_NET_WM_NAME")
	atomUtf8String      = internAtom("UTF8_STRING")
	atomNetWmState      = internAtom("_NET_WM_STATE")
	atomNetWmStateAbove = internAtom("_NET_WM_STATE_ABOVE")

	return nil
}

// getWindowTitle fetches the window title, trying _NET_WM_NAME (UTF-8) then WM_NAME as fallback.
func getWindowTitle(wid xproto.Window) string {
	// Try _NET_WM_NAME first (UTF-8)
	prop, err := xproto.GetProperty(xconn, false, wid, atomNetWmName, atomUtf8String, 0, 1024).Reply()
	if err == nil && prop.ValueLen > 0 {
		return string(prop.Value)
	}
	// Fallback to WM_NAME (Latin-1)
	prop, err = xproto.GetProperty(xconn, false, wid, xproto.AtomWmName, xproto.AtomString, 0, 1024).Reply()
	if err == nil && prop.ValueLen > 0 {
		return string(prop.Value)
	}
	return ""
}

// getWindowPid returns the PID of the process owning the window via _NET_WM_PID.
func getWindowPid(wid xproto.Window) (uint32, error) {
	prop, err := xproto.GetProperty(xconn, false, wid, atomNetWmPid, xproto.AtomCardinal, 0, 1).Reply()
	if err != nil {
		return 0, err
	}
	if prop.ValueLen == 0 {
		return 0, fmt.Errorf("no _NET_WM_PID property on window %d", wid)
	}
	return binary.LittleEndian.Uint32(prop.Value), nil
}

// getActiveWindow returns the currently focused X11 window via _NET_ACTIVE_WINDOW.
func getActiveWindow() xproto.Window {
	prop, err := xproto.GetProperty(xconn, false, xroot, atomNetActiveWindow, xproto.AtomWindow, 0, 1).Reply()
	if err != nil || prop.ValueLen == 0 {
		return 0
	}
	return xproto.Window(binary.LittleEndian.Uint32(prop.Value))
}

// getAllWindows returns the list of managed windows from _NET_CLIENT_LIST on the root window.
func getAllWindows() []xproto.Window {
	prop, err := xproto.GetProperty(xconn, false, xroot, atomNetClientList, xproto.AtomWindow, 0, ^uint32(0)).Reply()
	if err != nil || prop.ValueLen == 0 {
		return nil
	}
	count := int(prop.ValueLen)
	windows := make([]xproto.Window, count)
	for i := 0; i < count; i++ {
		windows[i] = xproto.Window(binary.LittleEndian.Uint32(prop.Value[i*4:]))
	}
	return windows
}

// getWindowClass returns the WM_CLASS instance name for a window.
func getWindowClass(wid xproto.Window) string {
	prop, err := xproto.GetProperty(xconn, false, wid, xproto.AtomWmClass, xproto.AtomString, 0, 256).Reply()
	if err != nil || prop.ValueLen == 0 {
		return ""
	}
	// WM_CLASS contains two null-separated strings: instance\0class\0
	raw := string(prop.Value)
	if idx := strings.IndexByte(raw, 0); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

// isDofusWindow checks whether a window belongs to the Dofus process.
// Primary: /proc/[pid]/exe contains "dofus".
// Fallback (container/permission denied): title or WM_CLASS contains "dofus".
func isDofusWindow(wid xproto.Window) bool {
	pid, err := getWindowPid(wid)
	if err == nil {
		name, err := GetExecutableNameFromPid(pid)
		if err == nil {
			return strings.Contains(strings.ToLower(name), "dofus")
		}
	}
	title := getWindowTitle(wid)
	if strings.Contains(strings.ToLower(title), "dofus") {
		return true
	}
	class := getWindowClass(wid)
	return strings.Contains(strings.ToLower(class), "dofus")
}

// activateWindow sends a _NET_ACTIVE_WINDOW client message to ask the WM
// (KWin on KDE) to focus the given window. Works with XWayland windows.
func activateWindow(wid xproto.Window) {
	data := xproto.ClientMessageDataUnionData32New([]uint32{
		2, // source indication: 2 = pager/external application
		0, // timestamp (0 = current time)
		0, // requestor's currently active window
		0,
		0,
	})
	evt := xproto.ClientMessageEvent{
		Format: 32,
		Window: wid,
		Type:   atomNetActiveWindow,
		Data:   data,
	}
	xproto.SendEvent(
		xconn,
		false,
		xroot,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(evt.Bytes()),
	)
	// xgb flushes automatically on next I/O operation
}

// foregroundWindowsHook subscribes to X11 PropertyNotify events on the root window
// and emits CharSelectedEvent whenever a Dofus window becomes the active window.
// This replaces the Windows WinEventHook mechanism.
func (a *App) foregroundWindowsHook() {
	if xconn == nil {
		return
	}

	// Subscribe to PropertyNotify events on the root window.
	// This gives us notifications when _NET_ACTIVE_WINDOW changes.
	err := xproto.ChangeWindowAttributesChecked(
		xconn, xroot, xproto.CwEventMask,
		[]uint32{xproto.EventMaskPropertyChange},
	).Check()
	if err != nil {
		log.Printf("Warning: could not subscribe to X11 root window events: %v", err)
		// Fall back to polling
		a.foregroundWindowsPoll()
		return
	}

	lastActive := xproto.Window(0)
	for {
		event, err := xconn.WaitForEvent()
		if err != nil {
			log.Printf("X11 event error: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		switch e := event.(type) {
		case xproto.PropertyNotifyEvent:
			if e.Window == xroot && e.Atom == atomNetActiveWindow {
				activeWin := getActiveWindow()
				if activeWin != 0 && activeWin != lastActive {
					lastActive = activeWin
					if isDofusWindow(activeWin) {
						a.updateCurrentDofusIndex(uint64(activeWin))
						a.CharSelectedEvent(uint64(activeWin))
					}
				}
			}
		}
	}
}

// foregroundWindowsPoll is a fallback that polls the active window every 200ms.
// Used when X11 event subscription fails.
func (a *App) foregroundWindowsPoll() {
	lastActive := xproto.Window(0)
	for {
		time.Sleep(200 * time.Millisecond)
		if xconn == nil {
			return
		}
		activeWin := getActiveWindow()
		if activeWin != 0 && activeWin != lastActive {
			lastActive = activeWin
			if isDofusWindow(activeWin) {
				a.updateCurrentDofusIndex(uint64(activeWin))
				a.CharSelectedEvent(uint64(activeWin))
			}
		}
	}
}

// updateCurrentDofusIndex finds the index of hwnd in DofusWindows and updates currentDofusIndex.
func (a *App) updateCurrentDofusIndex(hwnd uint64) {
	for i, w := range a.DofusWindows {
		if w.Hwnd == hwnd {
			currentDofusIndex = i
			return
		}
	}
}

// SetWindowAbove uses _NET_WM_STATE_ABOVE to ask KWin to keep our window on top.
// This works for XWayland windows where GTK's gtk_window_set_keep_above may be ignored.
// onTop=true to pin above all windows, onTop=false to release.
func SetWindowAbove(wid xproto.Window, onTop bool) {
	if xconn == nil || atomNetWmState == xproto.AtomNone || atomNetWmStateAbove == xproto.AtomNone {
		return
	}
	action := uint32(0) // _NET_WM_STATE_REMOVE
	if onTop {
		action = 1 // _NET_WM_STATE_ADD
	}
	data := xproto.ClientMessageDataUnionData32New([]uint32{
		action,
		uint32(atomNetWmStateAbove),
		0, 0, 0,
	})
	evt := xproto.ClientMessageEvent{
		Format: 32,
		Window: wid,
		Type:   atomNetWmState,
		Data:   data,
	}
	xproto.SendEvent(
		xconn,
		false,
		xroot,
		xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify,
		string(evt.Bytes()),
	)
}

func (a *App) UpdateTemporaryDofusWindows(tempChars []WindowInfo) {
	if len(tempChars) != 0 {
		a.DofusWindows = tempChars
	} else {
		runtime.LogErrorf(a.ctx, "error while updating temporary chars to a.DofusWindows %v", tempChars)
	}
}

func (a *App) ActivateNextChar() {
	if len(a.DofusWindows) == 0 {
		return
	}
	nextIndex := (currentDofusIndex + 1) % len(a.DofusWindows)
	currentDofusIndex = nextIndex
	a.WinActivate(a.DofusWindows[nextIndex].Hwnd)
}

func (a *App) ActivatePreviousChar() {
	if len(a.DofusWindows) == 0 {
		return
	}
	nextIndex := (currentDofusIndex - 1 + len(a.DofusWindows)) % len(a.DofusWindows)
	currentDofusIndex = nextIndex
	a.WinActivate(a.DofusWindows[nextIndex].Hwnd)
}

// WinActivate activates (focuses) the window with the given X11 Window ID.
// Called from frontend when user clicks a character name.
func (a *App) WinActivate(hwnd uint64) {
	if xconn == nil || hwnd == 0 {
		return
	}
	activateWindow(xproto.Window(hwnd))
}

// refreshAndUpdateCharacterList enumerates all X11 windows and filters for Dofus windows.
func (a *App) refreshAndUpdateCharacterList(exists bool) {
	if xconn == nil {
		return
	}

	a.DofusWindows = []WindowInfo{}

	windows := getAllWindows()

	for _, wid := range windows {
		title := getWindowTitle(wid)

		// Check via /proc/[pid]/exe first, fall back to title/WM_CLASS
		matched := false
		pid, err := getWindowPid(wid)
		if err == nil {
			exeName, err := GetExecutableNameFromPid(pid)
			if err == nil {
				matched = strings.Contains(strings.ToLower(exeName), "dofus")
			} else {
				matched = strings.Contains(strings.ToLower(title), "dofus")
				if !matched {
					matched = strings.Contains(strings.ToLower(getWindowClass(wid)), "dofus")
				}
			}
		} else {
			matched = strings.Contains(strings.ToLower(title), "dofus")
			if !matched {
				matched = strings.Contains(strings.ToLower(getWindowClass(wid)), "dofus")
			}
		}

		if !matched {
			continue
		}

		characterName, class := parseTitleComponents(title)

		a.DofusWindows = append(a.DofusWindows, WindowInfo{
			Title:         title,
			Hwnd:          uint64(wid),
			CharacterName: characterName,
			Class:         class,
		})
	}

	if !exists {
		a.SaveCharacterList(a.DofusWindows)
	}

	a.UpdateDofusWindowsOrder(a.DofusWindows)
}

// UpdateDofusWindowsOrder reorders the window list according to the saved order in characters.ini
func (a *App) UpdateDofusWindowsOrder(loggedInCharacters []WindowInfo) ([]WindowInfo, error) {
	if len(loggedInCharacters) == 0 {
		return a.DofusWindows, nil
	}

	iniFile, _, _ := loadINIFile(charactersFilePath)

	savedOrder, err := a.loadCharacterList(iniFile)
	if err != nil {
		runtime.LogError(a.ctx, "Error loading character list")
		return nil, err
	}

	var newOrderKnown []WindowInfo
	var newOrderUnknown []WindowInfo

	loggedInMap := make(map[string]WindowInfo)
	for _, char := range loggedInCharacters {
		loggedInMap[char.CharacterName] = char
	}

	processed := make(map[string]bool)
	processedHWND := make(map[uint64]bool)

	for _, loggedChar := range loggedInCharacters {
		if !strings.Contains(loggedChar.CharacterName, "Dofus") {
			for _, savedChar := range savedOrder {
				if _, exists := processed[savedChar]; exists {
					continue
				}

				if loggedChar, exists := loggedInMap[savedChar]; exists {
					newOrderKnown = append(newOrderKnown, loggedChar)
					processed[savedChar] = true
					processedHWND[loggedChar.Hwnd] = true
				} else {
					processedHWND[loggedChar.Hwnd] = true
					processed[savedChar] = true
				}
				processedHWND[loggedChar.Hwnd] = true
				processed[savedChar] = true
			}

			for _, loggedChar := range loggedInCharacters {
				if _, exists := processed[loggedChar.CharacterName]; !exists {
					newOrderUnknown = append(newOrderUnknown, loggedChar)
					processed[loggedChar.CharacterName] = true
					processedHWND[loggedChar.Hwnd] = true
				}
			}
		} else {
			if _, exists := processedHWND[loggedChar.Hwnd]; !exists {
				processed[loggedChar.CharacterName] = true
				processedHWND[loggedChar.Hwnd] = true
				newOrderUnknown = append(newOrderUnknown, loggedChar)
			}
		}
	}

	newOrderKnown = append(newOrderKnown, newOrderUnknown...)
	a.DofusWindows = newOrderKnown

	return newOrderKnown, nil
}

// GetDofusWindows is called by the frontend to refresh and fetch the current window list.
func (a *App) GetDofusWindows() []WindowInfo {
	_, err, exists := loadINIFile(charactersFilePath)
	if err != nil {
		runtime.LogError(a.ctx, "Error with the ini file")
	}

	a.refreshAndUpdateCharacterList(exists)

	if len(a.DofusWindows) > 0 {
		return a.DofusWindows
	}
	return nil
}

// IsWindowDofus checks if the currently active X11 window is in our Dofus window list.
// Returns (true, index) if found, (false, 0) otherwise.
func (a *App) IsWindowDofus() (bool, int) {
	if xconn == nil {
		return false, 0
	}

	activeWin := getActiveWindow()
	if activeWin == 0 {
		return false, 0
	}

	for i, window := range a.DofusWindows {
		if window.Hwnd == uint64(activeWin) {
			return true, i
		}
	}

	return false, 0
}
