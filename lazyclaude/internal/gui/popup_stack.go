package gui

import (
	"fmt"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/KEMSHlM/lazyclaude/internal/notify"
)

// popupEntry represents a single popup in the stack.
type popupEntry struct {
	notification *notify.ToolNotification
	scrollY      int
	diffCache    []string
	diffKinds    []presentation.DiffLineKind
	suspended    bool
}

// pushPopup adds a notification to the popup stack.
func (a *App) pushPopup(n *notify.ToolNotification) {
	a.popupStack = append(a.popupStack, popupEntry{notification: n})
	a.popupFocusIdx = len(a.popupStack) - 1
}

// popupCount returns total popups (including suspended).
func (a *App) popupCount() int {
	return len(a.popupStack)
}

// visiblePopupCount returns non-suspended popup count.
func (a *App) visiblePopupCount() int {
	c := 0
	for _, e := range a.popupStack {
		if !e.suspended {
			c++
		}
	}
	return c
}

// activePopup returns the focused popup's notification, or nil.
func (a *App) activePopup() *notify.ToolNotification {
	e := a.activeEntry()
	if e == nil {
		return nil
	}
	return e.notification
}

// activeEntry returns a pointer to the focused popup entry, or nil.
func (a *App) activeEntry() *popupEntry {
	if len(a.popupStack) == 0 || a.popupFocusIdx < 0 || a.popupFocusIdx >= len(a.popupStack) {
		return nil
	}
	e := &a.popupStack[a.popupFocusIdx]
	if e.suspended {
		return nil
	}
	return e
}

// dismissActivePopup removes the focused popup from the stack.
func (a *App) dismissActivePopup() {
	if len(a.popupStack) == 0 || a.popupFocusIdx < 0 || a.popupFocusIdx >= len(a.popupStack) {
		return
	}
	a.popupStack = append(a.popupStack[:a.popupFocusIdx], a.popupStack[a.popupFocusIdx+1:]...)
	if a.popupFocusIdx >= len(a.popupStack) {
		a.popupFocusIdx = len(a.popupStack) - 1
	}
	// Skip suspended entries
	if len(a.popupStack) > 0 && a.popupStack[a.popupFocusIdx].suspended {
		a.popupFocusNext()
	}
}

// popupFocusNext moves focus to the next visible popup (wrapping).
func (a *App) popupFocusNext() {
	n := len(a.popupStack)
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		next := (a.popupFocusIdx + 1 + i) % n
		if !a.popupStack[next].suspended {
			a.popupFocusIdx = next
			return
		}
	}
}

// popupFocusPrev moves focus to the previous visible popup (wrapping).
func (a *App) popupFocusPrev() {
	n := len(a.popupStack)
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		prev := (a.popupFocusIdx - 1 - i + n) % n
		if !a.popupStack[prev].suspended {
			a.popupFocusIdx = prev
			return
		}
	}
}

// suspendActivePopup hides the focused popup without dismissing.
func (a *App) suspendActivePopup() {
	if a.popupFocusIdx < 0 || a.popupFocusIdx >= len(a.popupStack) {
		return
	}
	a.popupStack[a.popupFocusIdx].suspended = true
	a.popupFocusNext()
}

// suspendAllPopups hides all popups without dismissing.
func (a *App) suspendAllPopups() {
	for i := range a.popupStack {
		a.popupStack[i].suspended = true
	}
}

// visibleIndexOf returns the visible-only index for a stack index.
func (a *App) visibleIndexOf(stackIdx int) int {
	idx := 0
	for i := 0; i < stackIdx && i < len(a.popupStack); i++ {
		if !a.popupStack[i].suspended {
			idx++
		}
	}
	return idx
}

// popupViewNames returns the view names for each popup in the stack.
func (a *App) popupViewNames() []string {
	names := make([]string, len(a.popupStack))
	for i := range a.popupStack {
		names[i] = fmt.Sprintf("tool-popup-%d", i)
	}
	return names
}

// popupCascadeOffset returns the top-left position for a cascaded popup.
func popupCascadeOffset(baseX, baseY, index int) (int, int) {
	return baseX + index*2, baseY + index
}

// unsuspendAll makes all suspended popups visible again.
func (a *App) unsuspendAll() {
	for i := range a.popupStack {
		a.popupStack[i].suspended = false
	}
	if len(a.popupStack) > 0 {
		a.popupFocusIdx = len(a.popupStack) - 1
	}
}
