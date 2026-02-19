package main

// UMap maps evdev key codes to key name strings
type UMap map[uint32]string

func (a *App) GetKeycodes() UMap {
	return Keycode
}

func (a *App) GetStringFromKey(key int) (string, bool) {
	for k, name := range Keycode {
		if key == int(k) {
			return name, true
		}
	}
	return "", false
}

// Keycode maps Linux evdev KEY_* codes (from linux/input-event-codes.h) to key names
var Keycode = UMap{
	// Function keys
	59:  "F1",
	60:  "F2",
	61:  "F3",
	62:  "F4",
	63:  "F5",
	64:  "F6",
	65:  "F7",
	66:  "F8",
	67:  "F9",
	68:  "F10",
	87:  "F11",
	88:  "F12",
	183: "F13",
	184: "F14",
	185: "F15",
	186: "F16",
	187: "F17",
	188: "F18",
	189: "F19",
	190: "F20",
	191: "F21",
	192: "F22",
	193: "F23",
	194: "F24",

	// Letters
	30: "A",
	48: "B",
	46: "C",
	32: "D",
	18: "E",
	33: "F",
	34: "G",
	35: "H",
	23: "I",
	36: "J",
	37: "K",
	38: "L",
	50: "M",
	49: "N",
	24: "O",
	25: "P",
	16: "Q",
	19: "R",
	31: "S",
	20: "T",
	22: "U",
	47: "V",
	17: "W",
	45: "X",
	21: "Y",
	44: "Z",

	// Numbers row
	11: "0",
	2:  "1",
	3:  "2",
	4:  "3",
	5:  "4",
	6:  "5",
	7:  "6",
	8:  "7",
	9:  "8",
	10: "9",

	// Arrow keys
	103: "ARROW_UP",
	108: "ARROW_DOWN",
	105: "ARROW_LEFT",
	106: "ARROW_RIGHT",

	// Navigation
	104: "PAGEUP",
	109: "PAGEDOWN",
	102: "HOME",
	107: "END",
	110: "INSERT",
	111: "DELETE",

	// Numpad
	82: "NUMPAD0",
	79: "NUMPAD1",
	80: "NUMPAD2",
	81: "NUMPAD3",
	75: "NUMPAD4",
	76: "NUMPAD5",
	77: "NUMPAD6",
	71: "NUMPAD7",
	72: "NUMPAD8",
	73: "NUMPAD9",
	96: "NUMPAD_ENTER",
	78: "NUMPAD_PLUS",
	74: "NUMPAD_MINUS",
	55: "NUMPAD_MULT",
	98: "NUMPAD_DIV",

	// Mouse buttons (BTN_*)
	274: "SOURIS_3",
	275: "SOURIS_4",
	276: "SOURIS_5",
	277: "SOURIS_6",
	278: "SOURIS_7",

	// Special keys
	1:   "ESC",
	15:  "TAB",
	58:  "CAPSLOCK",
	42:  "LSHIFT",
	54:  "RSHIFT",
	29:  "LCTRL",
	97:  "RCTRL",
	56:  "LALT",
	100: "RALT",
	57:  "SPACE",
	28:  "ENTER",
	14:  "BACKSPACE",
	70:  "SCROLLLOCK",
	69:  "NUMLOCK",
	119: "PAUSE",
	99:  "SYSRQ",
}
