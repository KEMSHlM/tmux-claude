package gui

import (
	"fmt"
	"strings"

	"github.com/KEMSHlM/lazyclaude/internal/gui/keymap"
	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/jesseduffield/gocui"
)

// keybind help view names
const (
	helpInputView   = "keybind-help-input"
	helpListView    = "keybind-help-list"
	helpPreviewView = "keybind-help-preview"
	helpHintView    = "keybind-help-hint"
	helpBorderView  = "keybind-help-border"
)

// layoutKeybindHelp creates or updates the Telescope-style help overlay.
// Called from layoutMain when DialogKeybindHelp is active.
func (a *App) layoutKeybindHelp(g *gocui.Gui, maxX, maxY int) error {
	// Overlay dimensions: 80% width, 70% height, centered.
	w := maxX * 80 / 100
	h := maxY * 70 / 100
	if w < 40 {
		w = maxX - 4
	}
	if h < 10 {
		h = maxY - 4
	}
	x0 := (maxX - w) / 2
	y0 := (maxY - h) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Divider: left pane 40%, right pane 60%.
	leftW := w * 40 / 100
	midX := x0 + leftW

	// Border view (full overlay frame).
	bv, err := g.SetView(helpBorderView, x0, y0, x0+w, y0+h, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	setRoundedFrame(bv)
	bv.Title = " Keybind Help "
	bv.Clear()

	// Input field (top of left pane, inside border).
	iv, err := g.SetView(helpInputView, x0+1, y0+1, midX-1, y0+3, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	iv.Frame = true
	iv.Title = " Filter "
	iv.Editable = true
	iv.Editor = &helpInputEditor{app: a}
	setRoundedFrame(iv)

	// List view (left pane, below input).
	lv, err := g.SetView(helpListView, x0+1, y0+3, midX-1, y0+h-3, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	lv.Frame = false
	lv.Highlight = false
	lv.Clear()
	renderKeybindHelpList(lv, a.dialog.HelpItems, a.dialog.HelpCursor)

	// Preview view (right pane).
	pv, err := g.SetView(helpPreviewView, midX, y0+1, x0+w-1, y0+h-3, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	pv.Frame = true
	pv.Title = " Documentation "
	pv.Wrap = true
	setRoundedFrame(pv)
	pv.Clear()
	if len(a.dialog.HelpItems) > 0 && a.dialog.HelpCursor < len(a.dialog.HelpItems) {
		renderKeybindHelpPreview(pv, a.dialog.HelpItems[a.dialog.HelpCursor])
	}

	// Hint bar (bottom of overlay).
	hv, err := g.SetView(helpHintView, x0+1, y0+h-3, x0+w-1, y0+h-1, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	hv.Frame = false
	hv.Clear()
	fmt.Fprint(hv, presentation.StyledKey("Esc", "close")+"  "+
		presentation.StyledKey("j/k", "move")+"  "+
		presentation.StyledKey("C-j/C-k", "scroll"))

	// Z-order: border at bottom, then list/preview, input on top.
	g.SetViewOnTop(helpBorderView)
	g.SetViewOnTop(helpListView)
	g.SetViewOnTop(helpPreviewView)
	g.SetViewOnTop(helpHintView)
	g.SetViewOnTop(helpInputView)

	return nil
}

// closeKeybindHelp removes all help views and resets dialog state.
func (a *App) closeKeybindHelp(g *gocui.Gui) {
	a.dialog.Kind = DialogNone
	a.dialog.HelpItems = nil
	a.dialog.HelpAllItems = nil
	a.dialog.HelpFilter = ""
	a.dialog.HelpCursor = 0
	a.dialog.HelpScrollY = 0
	g.DeleteView(helpInputView)
	g.DeleteView(helpListView)
	g.DeleteView(helpPreviewView)
	g.DeleteView(helpHintView)
	g.DeleteView(helpBorderView)
	g.Cursor = false
}

// renderKeybindHelpList renders the filtered action list with cursor highlight.
// Uses manual "> " prefix instead of gocui's native Highlight/SelBgColor because
// each line contains mixed ANSI styling (key color + description dim) that
// conflicts with gocui's line-level highlight which applies a uniform SelBgColor.
func renderKeybindHelpList(v *gocui.View, items []keymap.ActionDef, cursor int) {
	for i, item := range items {
		keyLabel := item.HintKeyLabel()
		desc := item.Description

		prefix := "  "
		if i == cursor {
			prefix = presentation.FgCyan + "> " + presentation.Reset
		}

		keyStr := fmt.Sprintf("%-8s", keyLabel)
		if i == cursor {
			fmt.Fprintf(v, "%s%s%s%s %s\n",
				prefix,
				presentation.Bold, keyStr, presentation.Reset,
				desc)
		} else {
			fmt.Fprintf(v, "%s%s%s%s %s%s%s\n",
				prefix,
				presentation.FgTeal, keyStr, presentation.Reset,
				presentation.Dim, desc, presentation.Reset)
		}
	}
}

// renderKeybindHelpPreview renders the documentation for the selected action.
func renderKeybindHelpPreview(v *gocui.View, item keymap.ActionDef) {
	// Header: action description.
	fmt.Fprintf(v, "%s%s%s\n\n", presentation.Bold, item.Description, presentation.Reset)

	// Key bindings summary.
	fmt.Fprintf(v, "%sKey:%s %s%s%s\n",
		presentation.Dim, presentation.Reset,
		presentation.FgTeal, item.HintKeyLabel(), presentation.Reset)
	fmt.Fprintf(v, "%sScope:%s %s\n\n",
		presentation.Dim, presentation.Reset,
		string(item.Scope))

	// Detailed documentation from embedded markdown.
	if item.DocSection != "" {
		doc := keymap.DocSection(item.DocSection)
		if doc != "" {
			fmt.Fprint(v, doc)
		}
	}
}

// filterKeybindItems filters actions by case-insensitive substring match
// on Description, HintLabel, and key display string.
func filterKeybindItems(items []keymap.ActionDef, query string) []keymap.ActionDef {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var result []keymap.ActionDef
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Description), q) ||
			strings.Contains(strings.ToLower(item.HintLabel), q) ||
			strings.Contains(strings.ToLower(item.HintKeyLabel()), q) {
			result = append(result, item)
		}
	}
	return result
}

// helpInputEditor handles text input in the keybind help filter field.
// On each keystroke it re-filters the list and resets the cursor.
type helpInputEditor struct {
	app *App
}

func (e *helpInputEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	switch {
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.TextArea.BackSpaceChar()
	case key == gocui.KeyDelete:
		v.TextArea.DeleteChar()
	case key == gocui.KeySpace:
		v.TextArea.TypeCharacter(" ")
	case ch != 0 && mod == gocui.ModNone:
		v.TextArea.TypeCharacter(string(ch))
	default:
		return false
	}

	v.RenderTextArea()
	e.app.dialog.HelpFilter = v.TextArea.GetContent()
	e.app.dialog.HelpItems = filterKeybindItems(e.app.dialog.HelpAllItems, e.app.dialog.HelpFilter)
	e.app.dialog.HelpCursor = 0
	e.app.dialog.HelpScrollY = 0
	return true
}
