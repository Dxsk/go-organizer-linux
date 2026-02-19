package main

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) KeybindUpdatedEvent() {
	runtime.EventsEmit(a.ctx, "KeybindsUpdate")
}

// CharSelectedEvent emits when a Dofus window becomes active
// hwnd is the X11 Window ID (uint64)
func (a *App) CharSelectedEvent(hwnd uint64) {
	runtime.EventsEmit(a.ctx, "CharSelectedEvent", hwnd)
}

func (a *App) UpdateOrganizerRunning() {
	runtime.EventsEmit(a.ctx, "updateOrganizerRunningState", isOrganizerRunning)
}

func (a *App) UpdateDofusWindows() {
	runtime.EventsEmit(a.ctx, "updatedCharacterOrder", a.DofusWindows)
}
