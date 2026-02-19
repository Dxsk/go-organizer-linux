# Dofus Organizer — Linux Port

Linux port of [go-organizer](https://github.com/Rhyyn/go-organizer) by [Rhyyn](https://github.com/Rhyyn), a multi-window manager for Dofus.

> **Original project (Windows):** https://github.com/Rhyyn/go-organizer
> **This fork (Linux):** https://github.com/Dxsk/go-organizer-linux

## Download

Pre-built binaries are automatically compiled via GitHub Actions and published to the [Releases page](https://github.com/Dxsk/go-organizer-linux/releases).

Download the latest `go-organizer` binary from there — no need to build from source.

---

## Features

- Automatic detection of open Dofus windows
- Switch active window via keyboard shortcuts or mouse buttons
- Global hotkeys (work even when Dofus is not in focus)
- Support for gaming mouse side buttons
- Configurable always-on-top window
- Key binding configuration through the GUI

---

## What changed from the original project

The original project is **Windows-only** and relies on:

| Component | Original (Windows) | This fork (Linux) |
|---|---|---|
| Window management | Win32 API (`user32.dll`, `w32`) | X11/EWMH via `github.com/jezek/xgb` |
| Keyboard/mouse hook | `lxn/go-hook` (WinAPI `SetWindowsHookEx`) | evdev — direct read from `/dev/input/event*` |
| Key codes | Windows Virtual Key codes (`VK_F2`, etc.) | Linux evdev codes (`linux/input-event-codes.h`) |
| Process detection | `kernel32` `GetModuleFileName` | `/proc/[pid]/exe` symlink |
| Window handles | `w32.HWND` | `uint64` (X11 Window ID) |
| Always-on-top | Win32 `SetWindowPos` | `_NET_WM_STATE_ABOVE` (EWMH) |

### Modified / replaced files

- `windows.go` + `win32hook.go` → `windows_linux.go` (X11 via xgb)
- `hook.go`: full hook system rewritten (evdev instead of go-hook)
- `keycode.go`: Windows VK codes → Linux evdev codes
- `helpers.go`: `GetExecutableName` via `/proc/[pid]/exe`
- `event.go`: `w32.HWND` → `uint64`
- `app.go`: removed Windows-specific imports
- `go.mod`: removed `w32` / `lxn/go-hook`, added `jezek/xgb`

### Technical details

**Window switching:**
An X11 watcher (`_NET_ACTIVE_WINDOW`) tracks which Dofus window was last focused. Hotkeys cycle from that index — so you can press shortcuts from any application (terminal, browser, etc.) without Dofus needing to be in the foreground.

**Keyboard/mouse hook:**
Raw evdev events are read from `/dev/input/event*`. Keyboards are detected via `EV_KEY + EV_REP`, mice via `EV_KEY + EV_REL`. Gaming mice that also set `EV_REP` are supported.

**X11 display detection:**
The `DISPLAY` environment variable takes priority, then the app automatically scans `/tmp/.X11-unix/` — compatible with multi-monitor setups and XWayland.

**Mouse side buttons:**
`BTN_SIDE` (code 275) → `SOURIS_4`, `BTN_EXTRA` (code 276) → `SOURIS_5`.

---

## Requirements

### System

- Linux with **XWayland** running (for Dofus AppImage under Wayland)
- `DISPLAY` environment variable set (usually automatic)

### `input` group

The evdev hook requires read access to `/dev/input/event*`:

```bash
sudo usermod -aG input $USER
# Log out and back in afterwards
```

### Gaming mice (side buttons only)

If mouse side buttons are not detected, it is likely a logind ACL issue. Create a udev rule with your device's VID/PID (find it with `lsusb`):

```bash
# Example for Razer Viper V3 Pro (1532:00c1)
echo 'SUBSYSTEM=="input", ATTRS{idVendor}=="1532", ATTRS{idProduct}=="00c1", TAG+="uaccess"' \
  | sudo tee /etc/udev/rules.d/70-mouse-uaccess.rules
sudo udevadm control --reload && sudo udevadm trigger
```

---

## Tested environment

This port was developed and tested on **Fedora Atomic (Bazzite) with KDE Plasma**, running Dofus as an AppImage through XWayland.

The build environment is a [distrobox](https://distrobox.it/) container (Arch Linux) running on top of the immutable Bazzite host. This approach is recommended for immutable distributions where installing Go and Node.js system-wide is not practical.

### Setting up a distrobox build container

```bash
# Create an Arch Linux container
distrobox create --name go-dev --image archlinux:latest
distrobox enter go-dev

# Inside the container — install dependencies
sudo pacman -S go nodejs npm

# Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2
export PATH="$PATH:$(go env GOPATH)/bin"
```

To run the compiled binary on the host from a KDE/GNOME launcher, use `distrobox-export`:

```bash
# Inside the container
distrobox-export --bin /path/to/go-organizer --export-path ~/.local/bin
```

---

## Build

### Requirements

- Go ≥ 1.22
- Node.js ≥ 18
- [Wails v2](https://wails.io): `go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.2`

### Compile

```bash
cd /path/to/go-organizer-linux
wails build
# Binary output at build/bin/go-organizer
```

`config.ini` and `characters.ini` must be in the same directory as the binary.

---

## Key binding configuration

Key bindings are configurable from the GUI and saved to `config.ini`:

```ini
[KeyBindings]
StopOrganizer = 62,F4
PreviousChar  = 275,SOURIS_4
NextChar      = 276,SOURIS_5
```

The format is `evdev_code,display_name`. Available codes are defined in `keycode.go`.

### Default bindings

| Action | Key |
|---|---|
| Enable / disable organizer | F4 |
| Previous window | SOURIS_4 (mouse back side button) |
| Next window | SOURIS_5 (mouse forward side button) |

---

## Desktop launcher (optional)

To launch from KDE/GNOME without a terminal, create `~/.local/share/applications/go-organizer.desktop`:

```ini
[Desktop Entry]
Name=Dofus Organizer
Comment=Multi-window organizer for Dofus
Exec=/path/to/go-organizer
Icon=/path/to/icon.png
Type=Application
Categories=Game;Utility;
StartupNotify=true
```

---

## Credits

- Original project: [Rhyyn/go-organizer](https://github.com/Rhyyn/go-organizer)
- Framework: [Wails v2](https://wails.io)
- X11: [jezek/xgb](https://github.com/jezek/xgb)
