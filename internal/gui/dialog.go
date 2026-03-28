package gui

// DialogKind identifies which input dialog is currently active.
type DialogKind int

const (
	DialogNone             DialogKind = iota // no dialog
	DialogRename                           // rename-input
	DialogWorktree                         // worktree-branch + worktree-prompt (new)
	DialogWorktreeChooser                  // worktree-chooser (select existing)
	DialogWorktreeResume                   // worktree-resume-prompt (prompt only for existing)
)

// DialogState groups all input dialog state into a single struct,
// keeping the App struct focused on core TUI concerns.
type DialogState struct {
	Kind           DialogKind     // current dialog (DialogNone = no dialog)
	RenameID       string         // session ID being renamed (empty = no rename)
	ActiveField    string         // which worktree dialog field has focus
	WorktreeItems  []WorktreeInfo // items in worktree chooser
	WorktreeCursor int            // selected index in chooser (len(items) = "New")
	SelectedPath   string         // path of chosen existing worktree
}

// HasActiveDialog returns true if any input dialog is open.
func (a *App) HasActiveDialog() bool {
	return a.dialog.Kind != DialogNone
}

// ActiveDialogKind returns the current dialog type.
func (a *App) ActiveDialogKind() DialogKind {
	return a.dialog.Kind
}

// dialogFocusView returns the gocui view name that should have focus
// for the current dialog. Returns "" if no dialog is active.
// Used by layoutMain to restore focus after popup dismiss.
func (a *App) dialogFocusView() string {
	switch a.dialog.Kind {
	case DialogRename:
		return "rename-input"
	case DialogWorktree:
		if a.dialog.ActiveField != "" {
			return a.dialog.ActiveField
		}
		return "worktree-branch"
	case DialogWorktreeChooser:
		return "worktree-chooser"
	case DialogWorktreeResume:
		return "worktree-resume-prompt"
	default:
		return ""
	}
}
