package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/ini.v1"
)

// Gets the saved order of Characters from characters.ini
func (a *App) loadCharacterList(cfg *ini.File) ([]string, error) {
	section := cfg.Section("Characters")

	var characterNames []string
	for _, key := range section.Keys() {
		characterNames = append(characterNames, key.Name())
	}

	return characterNames, nil
}

// this deletes our section and re creates it
func (a *App) SaveCharacterList(dofusWindows []WindowInfo) error {
	iniFile, _, _ := loadINIFile(charactersFilePath)

	iniFile.DeleteSection("Characters")

	section := iniFile.Section("Characters")
	for _, window := range dofusWindows {
		if !strings.Contains(window.Title, "Dofus") {
			section.Key(window.CharacterName).SetValue("")
		}
	}

	err := iniFile.SaveTo(charactersFilePath)
	if err != nil {
		runtime.LogPrintf(a.ctx, "saving INI file: %v", err)
	}

	a.DofusWindows = dofusWindows
	return nil
}

// Populate our config.ini Sections and add Default keybinds
// Uses Linux evdev keycodes: F2=60, F3=61, F4=62
func (a *App) CreateConfigSection(cfg *ini.File, exeDir string) {
	section, err := cfg.GetSection("KeyBindings")
	if err != nil {
		section = cfg.Section("KeyBindings")
	}
	section.Key("StopOrganizer").SetValue("62,F4")
	section.Key("PreviousChar").SetValue("60,F2")
	section.Key("NextChar").SetValue("61,F3")

	err = cfg.SaveTo(filepath.Join(exeDir, "config.ini"))
	if err != nil {
		runtime.LogErrorf(a.ctx, "Error saving config file: %v", err)
	}
}

// Load an ini file, if does not exist, we create and return it
func loadINIFile(filePath string) (*ini.File, error, bool) {
	configFileMutex.Lock()
	defer configFileMutex.Unlock()

	if _, err := os.Stat(filePath); err == nil {
		// File exists, load it
		cfg, err := ini.Load(filePath)
		if err != nil {
			return nil, err, false
		}
		return cfg, nil, true
	} else {
		// File doesn't exist, create a new one
		cfg := ini.Empty()
		return cfg, nil, false
	}
}

// gets the binary dir and sets global config paths
func getExecutableDir() {
	exePath, err := os.Executable()

	exeDirTemp := filepath.Dir(exePath)

	if err != nil {
		fmt.Printf("error while getting exe dir %v\n", err)
	} else {
		exeDir = exeDirTemp
		configFilePath = filepath.Join(exeDirTemp, "config.ini")
		charactersFilePath = filepath.Join(exeDirTemp, "characters.ini")
	}
}

// GetExecutableNameFromPid gets the process name from a PID via /proc/[pid]/exe
func GetExecutableNameFromPid(pid uint32) (string, error) {
	exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return "", err
	}
	return filepath.Base(exePath), nil
}

// Extracts char name and class from Dofus Window Title.
// Handles multiple formats:
//   - "CharName - ClassName"           → ("CharName", "ClassName")
//   - "Dofus - CharName - ClassName"   → ("CharName", "ClassName")
//   - "Dofus"                          → ("Dofus", "")
func parseTitleComponents(title string) (string, string) {
	parts := strings.Split(title, " - ")
	switch len(parts) {
	case 1:
		// e.g. "Dofus" with no separator
		return parts[0], ""
	case 2:
		// e.g. "CharName - ClassName"
		return parts[0], parts[1]
	default:
		// e.g. "Dofus - CharName - ClassName": skip leading "Dofus" prefix
		if strings.EqualFold(parts[0], "dofus") {
			return parts[1], parts[2]
		}
		return parts[0], parts[1]
	}
}

// Set main window to always be on top.
// Uses both the Wails GTK method and X11 _NET_WM_STATE_ABOVE for KDE/XWayland compatibility.
func (a *App) SetAlwaysOnTop() {
	isAlwaysOnTop = !isAlwaysOnTop
	runtime.WindowSetAlwaysOnTop(a.ctx, isAlwaysOnTop)

	// Also set via X11 for KDE/XWayland where GTK hint may be ignored.
	// Find our own window by title "go-organizer".
	if xconn != nil {
		for _, wid := range getAllWindows() {
			title := getWindowTitle(wid)
			if strings.Contains(title, "go-organizer") {
				SetWindowAbove(wid, isAlwaysOnTop)
				break
			}
		}
	}
}
