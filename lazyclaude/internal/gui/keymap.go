package gui

import "github.com/jesseduffield/gocui"

// KeyAction identifies a logical action in the keymap.
type KeyAction string

const (
	ActionQuit          KeyAction = "quit"
	ActionEnterFull     KeyAction = "enter_fullscreen"
	ActionExitFull      KeyAction = "exit_fullscreen"
	ActionNormalMode    KeyAction = "normal_mode"
	ActionInsertMode    KeyAction = "insert_mode"
	ActionCursorUp      KeyAction = "cursor_up"
	ActionCursorDown    KeyAction = "cursor_down"
	ActionNewSession    KeyAction = "new_session"
	ActionDeleteSession KeyAction = "delete_session"
	ActionPopupAccept   KeyAction = "popup_accept"
	ActionPopupAllow    KeyAction = "popup_allow"
	ActionPopupReject   KeyAction = "popup_reject"
	ActionPopupCancel   KeyAction = "popup_cancel"
)

// KeyBinding maps a physical key to an action.
type KeyBinding struct {
	Key  gocui.Key
	Rune rune
	Mod  gocui.Modifier
}

// KeyMap holds the mapping from actions to their key bindings.
// An action may have multiple bindings (e.g., both 'k' and ArrowUp for cursor_up).
type KeyMap struct {
	Bindings map[KeyAction][]KeyBinding
}

// DefaultKeyMap returns the default lazyclaude keymap.
func DefaultKeyMap() *KeyMap {
	return &KeyMap{
		Bindings: map[KeyAction][]KeyBinding{
			ActionQuit: {
				{Rune: 'q'},
			},
			ActionEnterFull: {
				{Key: gocui.KeyEnter},
			},
			ActionExitFull: {
				{Key: gocui.KeyCtrlD},
				// 'q' in normal mode is handled by ActionQuit handler
			},
			ActionNormalMode: {
				{Key: gocui.KeyCtrlBackslash},
			},
			ActionInsertMode: {
				{Rune: 'i'},
				// Enter in normal mode is handled by ActionEnterFull handler
			},
			ActionCursorUp: {
				{Rune: 'k'},
				{Key: gocui.KeyArrowUp},
			},
			ActionCursorDown: {
				{Rune: 'j'},
				{Key: gocui.KeyArrowDown},
			},
			ActionNewSession: {
				{Rune: 'n'},
			},
			ActionDeleteSession: {
				{Rune: 'd'},
			},
			ActionPopupAccept: {
				{Rune: 'y'},
				{Rune: '1'},
			},
			ActionPopupAllow: {
				{Rune: 'a'},
				{Rune: '2'},
			},
			ActionPopupReject: {
				// 'n' is NOT listed here — it is handled by ActionNewSession's
				// popup guard (hasPopup → dismissPopup). Only '3' is a pure reject key.
				{Rune: '3'},
			},
			ActionPopupCancel: {
				{Key: gocui.KeyEsc},
			},
		},
	}
}

// Matches returns true if the given key event matches this binding.
func (kb KeyBinding) Matches(key gocui.Key, ch rune, mod gocui.Modifier) bool {
	if mod != kb.Mod {
		return false
	}
	if kb.Rune != 0 {
		return ch == kb.Rune
	}
	return key == kb.Key
}

// HasBinding returns true if the action has any binding matching the event.
func (km *KeyMap) HasBinding(action KeyAction, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	for _, b := range km.Bindings[action] {
		if b.Matches(key, ch, mod) {
			return true
		}
	}
	return false
}

// FirstRune returns the first rune binding for an action, or 0 if none.
func (km *KeyMap) FirstRune(action KeyAction) rune {
	for _, b := range km.Bindings[action] {
		if b.Rune != 0 {
			return b.Rune
		}
	}
	return 0
}

// FirstKey returns the first gocui.Key binding for an action, or 0 if none.
func (km *KeyMap) FirstKey(action KeyAction) gocui.Key {
	for _, b := range km.Bindings[action] {
		if b.Rune == 0 {
			return b.Key
		}
	}
	return 0
}
