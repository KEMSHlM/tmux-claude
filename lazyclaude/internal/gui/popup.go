package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/KEMSHlM/lazyclaude/internal/notify"
	"github.com/jesseduffield/gocui"
)

const popupViewName = "tool-popup"
const popupActionsViewName = "tool-popup-actions"

// hasPopup returns true if any visible (non-suspended) popup exists.
func (a *App) hasPopup() bool {
	return a.visiblePopupCount() > 0
}

// showToolPopup pushes a notification onto the popup stack.
func (a *App) showToolPopup(n *notify.ToolNotification) {
	a.pushPopup(n)
}

// dismissPopup sends the choice to the focused popup and removes it from the stack.
func (a *App) dismissPopup(choice Choice) {
	active := a.activePopup()
	if active == nil {
		return
	}
	window := active.Window
	a.dismissActivePopup()

	if a.sessions != nil {
		go func() {
			// Ignore errors (window may have been closed)
			a.sessions.SendChoice(window, choice)
		}()
	}
}

// dismissAllPopups sends the choice to all popups and clears the stack.
func (a *App) dismissAllPopups(choice Choice) {
	if len(a.popupStack) == 0 {
		return
	}
	entries := make([]popupEntry, len(a.popupStack))
	copy(entries, a.popupStack)
	a.popupStack = nil
	a.popupFocusIdx = 0

	if a.sessions != nil {
		go func() {
			for _, e := range entries {
				a.sessions.SendChoice(e.notification.Window, choice)
			}
		}()
	}
}

// layoutToolPopup renders all visible popups as cascaded overlays.
func (a *App) layoutToolPopup(g *gocui.Gui, maxX, maxY int) error {
	// Clean up old popup views
	a.cleanupPopupViews(g)

	if !a.hasPopup() {
		g.DeleteView(popupActionsViewName)
		return nil
	}

	// Base popup dimensions: 70% width, 60% height, centered
	popW := maxX * 7 / 10
	popH := maxY * 6 / 10
	if popW < 40 {
		popW = maxX - 4
	}
	if popH < 10 {
		popH = maxY - 4
	}
	baseX := (maxX - popW) / 2
	baseY := (maxY - popH) / 2

	// Render each visible popup with cascade offset
	var activeViewName string
	visibleIdx := 0
	for i := range a.popupStack {
		e := &a.popupStack[i]
		if e.suspended {
			continue
		}

		viewName := fmt.Sprintf("tool-popup-%d", i)
		cx, cy := popupCascadeOffset(baseX, baseY, visibleIdx)
		x1 := cx + popW
		y1 := cy + popH - 2

		// Clamp to screen
		if x1 >= maxX {
			x1 = maxX - 1
		}
		if y1 >= maxY-2 {
			y1 = maxY - 3
		}

		v, err := g.SetView(viewName, cx, cy, x1, y1, 0)
		if err != nil && !isUnknownView(err) {
			return err
		}
		v.Clear()

		if e.notification.IsDiff() {
			a.renderDiffPopup(v, e)
		} else {
			a.renderToolPopup(v, e.notification)
		}

		if i == a.popupFocusIdx {
			activeViewName = viewName
		}
		visibleIdx++
	}

	// Bring focused popup to front
	if activeViewName != "" {
		g.SetViewOnTop(activeViewName)
	}

	// Actions bar for focused popup
	focusedEntry := a.activeEntry()
	if focusedEntry != nil {
		// Position actions bar below the focused popup
		cx, cy := popupCascadeOffset(baseX, baseY, a.visibleIndexOf(a.popupFocusIdx))
		ay0 := cy + popH - 1
		ay1 := ay0 + 2
		if ay1 >= maxY {
			ay1 = maxY - 1
		}
		ax1 := cx + popW
		if ax1 >= maxX {
			ax1 = maxX - 1
		}

		v2, err := g.SetView(popupActionsViewName, cx, ay0, ax1, ay1, 0)
		if err != nil && !isUnknownView(err) {
			return err
		}
		v2.Frame = false
		v2.Clear()
		g.SetViewOnTop(popupActionsViewName)

		visible := a.visiblePopupCount()
		n := focusedEntry.notification

		base := " y/a/n"
		if n.IsDiff() {
			base += " j/k:scroll"
		}
		base += " Esc:hide"
		if visible > 1 {
			base += fmt.Sprintf(" Y:all [%d/%d]", a.visibleIndexOf(a.popupFocusIdx)+1, visible)
		}
		fmt.Fprint(v2, base)

		if _, err := g.SetCurrentView(activeViewName); err != nil && !isUnknownView(err) {
			return err
		}
	}

	return nil
}

// cleanupPopupViews deletes all tool-popup-N views that are no longer needed.
func (a *App) cleanupPopupViews(g *gocui.Gui) {
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("tool-popup-%d", i)
		if i < len(a.popupStack) && !a.popupStack[i].suspended {
			continue
		}
		g.DeleteView(name)
	}
}

func (a *App) renderToolPopup(v *gocui.View, n *notify.ToolNotification) {
	v.Title = fmt.Sprintf(" %s ", n.ToolName)
	td := presentation.ParseToolInput(n.ToolName, n.Input, n.CWD)
	for _, line := range presentation.FormatToolLines(td) {
		fmt.Fprintln(v, line)
	}
}

func (a *App) renderDiffPopup(v *gocui.View, entry *popupEntry) {
	n := entry.notification
	v.Title = fmt.Sprintf(" Diff: %s ", filepath.Base(n.OldFilePath))

	diffLines, diffKinds := getDiffLinesForEntry(entry)
	_, viewH := v.Size()
	visibleLines := viewH - 1

	start := entry.scrollY
	end := start + visibleLines
	if end > len(diffLines) {
		end = len(diffLines)
	}
	if start < 0 {
		start = 0
	}

	for i := start; i < end; i++ {
		line := diffLines[i]
		kind := diffKinds[i]
		switch kind {
		case presentation.DiffAdd:
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line)
		case presentation.DiffDel:
			fmt.Fprintf(v, "\x1b[31m%s\x1b[0m\n", line)
		case presentation.DiffHunk:
			fmt.Fprintf(v, "\x1b[36m%s\x1b[0m\n", line)
		case presentation.DiffHeader:
			fmt.Fprintf(v, "\x1b[1m%s\x1b[0m\n", line)
		default:
			fmt.Fprintln(v, line)
		}
	}
}

// getDiffLinesForEntry generates and caches diff output for a popup entry.
func getDiffLinesForEntry(entry *popupEntry) ([]string, []presentation.DiffLineKind) {
	if entry.diffCache != nil {
		return entry.diffCache, entry.diffKinds
	}

	n := entry.notification
	diffOutput := generateDiffFromContents(n.OldFilePath, n.NewContents)
	parsed := presentation.ParseUnifiedDiff(diffOutput)

	lines := make([]string, len(parsed))
	kinds := make([]presentation.DiffLineKind, len(parsed))
	for i, dl := range parsed {
		lines[i] = presentation.FormatDiffLine(dl, 4)
		kinds[i] = dl.Kind
	}

	entry.diffCache = lines
	entry.diffKinds = kinds
	return lines, kinds
}

// generateDiffFromContents creates a unified diff between the old file and new contents.
func generateDiffFromContents(oldFilePath, newContents string) string {
	tmpDir := os.TempDir()
	newFile, err := os.CreateTemp(tmpDir, "lazyclaude-diff-new-*")
	if err != nil {
		return fmt.Sprintf("(error creating temp file: %v)", err)
	}
	defer os.Remove(newFile.Name())
	if _, err := newFile.WriteString(newContents); err != nil {
		newFile.Close()
		return fmt.Sprintf("(error writing temp file: %v)", err)
	}
	if err := newFile.Close(); err != nil {
		return fmt.Sprintf("(error closing temp file: %v)", err)
	}

	if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("--- /dev/null\n+++ %s\n@@ -0,0 +1 @@\n", filepath.Base(oldFilePath)))
		for _, line := range strings.Split(newContents, "\n") {
			if line != "" {
				sb.WriteString("+" + line + "\n")
			}
		}
		return sb.String()
	}

	cmd := exec.Command("git", "diff", "--no-index", "--unified=3", "--", oldFilePath, newFile.Name())
	out, err := cmd.Output()
	if err != nil && len(out) > 0 {
		return string(out)
	}
	if err != nil {
		return fmt.Sprintf("(no differences or error: %v)", err)
	}
	return string(out)
}
