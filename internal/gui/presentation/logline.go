package presentation

import "strings"

// LogCategory classifies a log line into a visual category for coloring.
type LogCategory int

const (
	LogCatInfo       LogCategory = iota // default: unclassified
	LogCatError                         // red: errors and failures
	LogCatWarn                          // yellow: warnings
	LogCatNotify                        // cyan: notification events
	LogCatLifecycle                     // green: session lifecycle
	LogCatConnection                    // blue: connection events
	LogCatDiffMsg                       // magenta: diff/file/msg operations
	LogCatDebug                         // dim gray: noisy per-frame logs
)

// ANSI escape sequences for log category coloring.
const (
	fgLogError      = "\x1b[31m"       // red
	fgLogWarn       = "\x1b[33m"       // yellow
	fgLogNotify     = "\x1b[36m"       // cyan
	fgLogLifecycle  = "\x1b[32m"       // green
	fgLogConnection = "\x1b[34m"       // blue
	fgLogDiffMsg    = "\x1b[35m"       // magenta
	fgLogDebug      = "\x1b[38;5;242m" // dim gray
)

// ClassifyLogLine determines the visual category from the message portion
// of a standard-library log line.  The expected format is
// "2006/01/02 15:04:05 <message>".  Classification uses prefix matching
// on the message text to avoid false positives from substrings in payloads.
func ClassifyLogLine(line string) LogCategory {
	// Skip the timestamp portion (first 20 chars: "2006/01/02 15:04:05 ").
	msg := line
	if len(msg) > 20 {
		msg = msg[20:]
	}
	lower := strings.ToLower(msg)

	// Debug (dim gray): noisy per-connection websocket logs and cleanup.
	// Checked first because ws frame errors contain "invalid" which would
	// otherwise match the error category.
	if strings.HasPrefix(lower, "ws read ") ||
		strings.HasPrefix(lower, "ws parse ") ||
		strings.HasPrefix(lower, "ws marshal ") ||
		strings.HasPrefix(lower, "ws write ") ||
		(strings.HasPrefix(lower, "cleaned") && strings.Contains(lower, "stale")) {
		return LogCatDebug
	}

	// Error (red): known error-only prefixes and error patterns
	if strings.HasPrefix(lower, "server error:") ||
		strings.HasPrefix(lower, "ws accept:") ||
		strings.Contains(lower, "window not found") ||
		strings.Contains(lower, "dropped") ||
		strings.Contains(lower, "encode error") ||
		strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "no window found") ||
		strings.Contains(lower, "local resolve failed") {
		return LogCatError
	}

	// Warning (yellow): warning prefixes and soft-failure patterns
	if strings.HasPrefix(lower, "warning:") ||
		(strings.Contains(lower, "not found") && strings.Contains(lower, "using last pending")) {
		return LogCatWarn
	}

	// Notify (cyan): notification pipeline events
	if strings.HasPrefix(lower, "notify:") {
		return LogCatNotify
	}

	// Lifecycle (green): session lifecycle events
	if strings.HasPrefix(lower, "session-start:") ||
		strings.HasPrefix(lower, "stop:") ||
		strings.HasPrefix(lower, "prompt-submit:") ||
		strings.HasPrefix(lower, "listening on") {
		return LogCatLifecycle
	}

	// Connection (blue): websocket and IDE connection events
	if strings.HasPrefix(lower, "ws connected:") ||
		strings.HasPrefix(lower, "ws disconnected:") ||
		strings.HasPrefix(lower, "ide_connected:") {
		return LogCatConnection
	}

	// DiffMsg (magenta): diff and messaging operations
	if strings.HasPrefix(lower, "opendiff:") ||
		strings.HasPrefix(lower, "msg/") {
		return LogCatDiffMsg
	}

	return LogCatInfo
}

// timestampLen is the length of the "2006/01/02 15:04:05" timestamp.
const timestampLen = 19

// categoryColor returns the ANSI foreground escape for a category.
// Returns empty string for Info (no color override).
func categoryColor(cat LogCategory) string {
	switch cat {
	case LogCatError:
		return fgLogError
	case LogCatWarn:
		return fgLogWarn
	case LogCatNotify:
		return fgLogNotify
	case LogCatLifecycle:
		return fgLogLifecycle
	case LogCatConnection:
		return fgLogConnection
	case LogCatDiffMsg:
		return fgLogDiffMsg
	case LogCatDebug:
		return fgLogDebug
	default:
		return ""
	}
}

// ColorizeLogLine wraps a log line with ANSI color escapes based on its
// classified category.  The timestamp portion (first 19 chars) is rendered
// in dim gray, and the message portion uses the category color.
// Lines shorter than the timestamp format are returned unmodified.
func ColorizeLogLine(line string) string {
	cat := ClassifyLogLine(line)
	color := categoryColor(cat)

	// Split timestamp and message when the line is long enough.
	if len(line) > timestampLen {
		ts := line[:timestampLen]
		rest := line[timestampLen:]
		if color != "" {
			return FgDimGray + ts + Reset + color + rest + Reset
		}
		return FgDimGray + ts + Reset + rest
	}

	if color != "" {
		return color + line + Reset
	}
	return line
}
