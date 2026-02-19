package main

import (
	"context"
	"log"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/ini.v1"
)

type Account struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Order int    `json:"order"`
}

// App struct
type App struct {
	ctx          context.Context
	DofusWindows []WindowInfo
}

type WindowInfo struct {
	Title         string `json:"title"`
	Hwnd          uint64 `json:"hwnd"`
	CharacterName string
	Class         string
	Order         int
}

type Keybinds struct {
	Action  string
	KeyName string
}

// KeyEvent represents a Linux evdev keyboard event
type KeyEvent struct {
	Code  int32
	Value int32 // 0=up, 1=down, 2=hold
}

var (
	charactersFilePath  string
	configFilePath      string
	configFile          *ini.File
	exeDir              string
	isAlwaysOnTop       bool
	keybindMap          map[int32]Keybinds
	isOrganizerRunning  bool
	isKeyPressed        map[int32]bool
	configFileMutex     sync.Mutex
	keyEventChan        chan KeyEvent
	currentDofusIndex   int // tracks which Dofus window was last active
)

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Gets exe dir of program and store as var
	getExecutableDir()

	// check if config ini file exists
	configFile, err, exists := loadINIFile(configFilePath)
	if !exists {
		a.CreateConfigSection(configFile, exeDir)
	}
	if err != nil {
		runtime.LogError(a.ctx, "Error with the ini file")
	}

	// Initialize X11 connection early (needed by refreshAndUpdateCharacterList)
	if err := initX11(); err != nil {
		log.Printf("Warning: X11 init failed: %v", err)
		log.Println("Window management will not be available. Is DISPLAY set? Is XWayland running?")
	}

	// Initialize our array
	a.refreshAndUpdateCharacterList(exists)

	// Initialize map of keybinds and load our saved keybinds
	keybindMap = make(map[int32]Keybinds)
	isKeyPressed = make(map[int32]bool)
	a.GetAllKeyBindings()

	// Start of our Observers
	runtime.EventsOn(a.ctx, "KeybindsUpdate", func(optionalData ...interface{}) {
		runtime.LogPrint(a.ctx, "Keybinds Updated... Restarting Hooks with updated keybinds...")

		// Stop / Start hook to update their keybinds
		a.UninstallHook()

		err := a.InstallHook()
		if err != nil {
			runtime.LogErrorf(a.ctx, "Error installing Hook.. %v", err)
		}
	})

	// Start foreground window watcher (X11 property notifications)
	go a.foregroundWindowsHook()

	// Start evdev keyboard hook
	if err := a.InstallHook(); err != nil {
		log.Printf("Warning: could not install keyboard hook: %v", err)
		log.Println("To enable global hotkeys, add yourself to the 'input' group:")
		log.Println("  sudo usermod -aG input $USER")
		log.Println("Then log out and back in.")
	}

	// Run the hook event loop (blocks until timeout)
	if err := a.GoHook(); err != nil {
		log.Printf("Hook loop error: %v", err)
	}
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.UninstallHook()
	log.Println("Hook uninstalled.. exiting..")
	return false
}
