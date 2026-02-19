package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Generic SaveKeybind - saves a keybind to config.ini
func (a *App) SaveKeybind(keycode int32, keyname string, keybindName string) (string, error) {
	// Open Config
	configFile, _, _ = loadINIFile(configFilePath)
	section, _ := configFile.GetSection("KeyBindings")

	if _, exists := keybindMap[keycode]; exists {
		return "failed", nil
	}

	// Create a string combination of the two
	value := fmt.Sprintf("%d,%s", keycode, strings.ToUpper(keyname))

	// delete existing binding for this action
	for existingKeycode, keybind := range keybindMap {
		if keybind.Action == keybindName {
			delete(keybindMap, existingKeycode)
			break
		}
	}

	section.Key(keybindName).SetValue(value)

	err := configFile.SaveTo(configFilePath)
	if err != nil {
		runtime.LogPrintf(a.ctx, "Error saving INI file: %v", err)
		return "", err
	}

	keybindMap[keycode] = Keybinds{
		Action:  keybindName,
		KeyName: strings.ToUpper(keyname),
	}

	a.KeybindUpdatedEvent()

	return "sucess", nil
}

// GetAllKeyBindings loads all keybinds from config.ini
func (a *App) GetAllKeyBindings() map[int32]Keybinds {
	configFile, err, _ := loadINIFile(configFilePath)
	if err != nil && configFile != nil {
		fmt.Printf("Error loading config file: %v\n", err)
		return nil
	}

	// Get the KeyBindings section
	section, err := configFile.GetSection("KeyBindings")
	if err != nil {
		runtime.LogErrorf(a.ctx, "Error getting section 'KeyBindings': %v", err)
		return nil
	}

	// Function to parse the key value: "62,F4" -> (62, "F4")
	parseKey := func(keyName string) (int32, string, error) {
		keyValue := section.Key(keyName).String()
		if keyValue == "" {
			err := fmt.Errorf("'%s' key not found", keyName)
			runtime.LogErrorf(a.ctx, "Error: %v", err)
			return 0, "", err
		}
		parts := strings.SplitN(keyValue, ",", 2)
		if len(parts) != 2 {
			err := fmt.Errorf("invalid key value format for '%s': '%s'", keyName, keyValue)
			runtime.LogErrorf(a.ctx, "Error: %v", err)
			return 0, "", err
		}
		keycode, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 32)
		if err != nil {
			runtime.LogErrorf(a.ctx, "Error parsing keycode '%s': %v", parts[0], err)
			return 0, "", err
		}
		keyname := strings.TrimSpace(parts[1])
		return int32(keycode), keyname, nil
	}

	keybindMap = make(map[int32]Keybinds)

	// Get StopOrganizer key
	stopCode, stopName, err := parseKey("StopOrganizer")
	if err != nil {
		return nil
	}
	keybindMap[stopCode] = Keybinds{
		Action:  "StopOrganizer",
		KeyName: stopName,
	}

	// Get PreviousChar key
	prevCode, prevName, err := parseKey("PreviousChar")
	if err != nil {
		return nil
	}
	keybindMap[prevCode] = Keybinds{
		Action:  "PreviousChar",
		KeyName: prevName,
	}

	// Get NextChar key
	nextCode, nextName, err := parseKey("NextChar")
	if err != nil {
		return nil
	}
	keybindMap[nextCode] = Keybinds{
		Action:  "NextChar",
		KeyName: nextName,
	}

	return keybindMap
}
