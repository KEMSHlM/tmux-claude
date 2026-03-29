package presentation

// ANSI escape sequences for terminal styling.
// Uses 256-color mode (supported by all modern terminals).
const (
	Reset     = "\x1b[0m"
	Bold      = "\x1b[1m"
	Dim       = "\x1b[2m"
	Italic    = "\x1b[3m"
	Underline = "\x1b[4m"
	Reverse   = "\x1b[7m"

	// Foreground colors
	FgRed     = "\x1b[31m"
	FgGreen   = "\x1b[32m"
	FgYellow  = "\x1b[33m"
	FgBlue    = "\x1b[34m"
	FgMagenta = "\x1b[35m"
	FgCyan    = "\x1b[36m"
	FgWhite   = "\x1b[37m"
	FgGray    = "\x1b[90m"

	// 256-color foreground (for finer control)
	FgPurple  = "\x1b[38;5;141m" // soft purple (crush-inspired primary)
	FgTeal    = "\x1b[38;5;80m"  // teal accent
	FgOrange  = "\x1b[38;5;208m" // warm orange
	FgDimGray = "\x1b[38;5;242m" // dim gray for muted text

	// Background colors (256-color)
	BgDarkGray = "\x1b[48;5;236m" // subtle dark background
	BgGreen    = "\x1b[48;5;22m"  // dark green background
	BgBlue     = "\x1b[48;5;17m"  // dark blue background
	BgRed      = "\x1b[48;5;52m"  // dark red background
)

// Unicode icons for status indicators (crush-inspired).
const (
	IconRunning  = "\x1b[32m●\x1b[0m" // green filled circle
	IconDead     = "\x1b[31m×\x1b[0m" // red cross
	IconOrphan   = "\x1b[33m○\x1b[0m" // yellow empty circle
	IconDetached = "\x1b[90m◆\x1b[0m" // gray diamond
	IconPending  = "\x1b[35m◆\x1b[0m" // magenta diamond (choice waiting)
	IconUnknown  = "\x1b[90m?\x1b[0m" // gray question mark
	IconArrow    = "→"
	IconCheck    = "✓"
	IconDot      = "·"
	IconSep      = "│"
	IconWorktree        = "\x1b[38;5;214m[W]\x1b[0m"  // orange [W] for worktree sessions
	IconPM              = "\x1b[38;5;141m[PM]\x1b[0m" // purple [PM] for PM sessions
	IconProjectExpanded = "\x1b[1m\xe2\x96\xbc\x1b[0m"  // bold ▼ for expanded project
	IconProjectCollapsed = "\x1b[1m\xe2\x96\xb6\x1b[0m" // bold ▶ for collapsed project
)

// StyledKey renders a keybinding hint: key in bold, description in dim.
// Example: StyledKey("n", "new") => "\x1b[1mn\x1b[0m\x1b[2m:new\x1b[0m"
func StyledKey(key, desc string) string {
	return Bold + key + Reset + Dim + ":" + desc + Reset
}

// HintDef is a minimal key/label pair for building options bars from registry data.
type HintDef struct {
	Key   string // display key (e.g. "n", "h/l", "C-y")
	Label string // description (e.g. "new", "fold", "yes")
}

// BuildOptionsBar generates an options bar string from a slice of HintDefs.
// Contract: the result always starts with a leading ASCII space (' ')
// when non-empty, so callers can safely use result[1:] to strip it.
func BuildOptionsBar(hints []HintDef) string {
	if len(hints) == 0 {
		return ""
	}
	var b []byte
	b = append(b, ' ')
	for i, h := range hints {
		if i > 0 {
			b = append(b, ' ', ' ')
		}
		b = append(b, []byte(StyledKey(h.Key, h.Label))...)
	}
	return string(b)
}



