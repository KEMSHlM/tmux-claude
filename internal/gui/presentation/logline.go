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

// timestampPrefixLen is the length of "2006/01/02 15:04:05 " (timestamp + space).
// Used by both ClassifyLogLine and ColorizeLogLine to ensure consistency.
const timestampPrefixLen = 20

// ClassifyLogLine determines the visual category from the message portion
// of a standard-library log line.  The expected format is
// "2006/01/02 15:04:05 <message>".  Classification uses prefix matching
// on the message text to avoid false positives from substrings in payloads.
func ClassifyLogLine(line string) LogCategory {
	msg := line
	if len(msg) > timestampPrefixLen {
		msg = msg[timestampPrefixLen:]
	}
	lower := strings.ToLower(msg)

	// Debug (dim gray): noisy per-connection websocket logs and cleanup.
	// Checked first because ws frame errors contain keywords that would
	// otherwise match the error category.
	if strings.HasPrefix(lower, "ws read ") ||
		strings.HasPrefix(lower, "ws parse ") ||
		strings.HasPrefix(lower, "ws marshal ") ||
		strings.HasPrefix(lower, "ws write ") ||
		(strings.HasPrefix(lower, "cleaned") && strings.Contains(lower, "stale")) {
		return LogCatDebug
	}

	// Error (red): known error-only prefixes
	if strings.HasPrefix(lower, "server error:") ||
		strings.HasPrefix(lower, "ws accept:") ||
		strings.HasPrefix(lower, "local resolve failed") {
		return LogCatError
	}
	// Error (red): notify sub-patterns that indicate failures
	if strings.HasPrefix(lower, "notify:") {
		rest := lower[len("notify:"):]
		if strings.Contains(rest, "window not found") ||
			strings.Contains(rest, "no window found") ||
			strings.Contains(rest, "dropped") ||
			strings.Contains(rest, "encode error") ||
			strings.Contains(rest, "encode response:") {
			return LogCatError
		}
	}
	// Error (red): ide_connected sub-patterns
	if strings.HasPrefix(lower, "ide_connected:") {
		rest := lower[len("ide_connected:"):]
		if strings.Contains(rest, "invalid") {
			return LogCatError
		}
	}

	// Warning (yellow): warning prefixes and soft-failure patterns
	if strings.HasPrefix(lower, "warning:") {
		return LogCatWarn
	}
	if strings.HasPrefix(lower, "notify:") {
		rest := lower[len("notify:"):]
		if strings.Contains(rest, "not found") && strings.Contains(rest, "using last pending") {
			return LogCatWarn
		}
	}

	// Notify (cyan): notification pipeline events (not matched above)
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
		strings.HasPrefix(lower, "ws disconnected:") {
		return LogCatConnection
	}
	// ide_connected (not matched by error above)
	if strings.HasPrefix(lower, "ide_connected:") {
		return LogCatConnection
	}

	// DiffMsg (magenta): diff and messaging operations
	if strings.HasPrefix(lower, "opendiff:") ||
		strings.HasPrefix(lower, "msg/") {
		return LogCatDiffMsg
	}

	return LogCatInfo
}

// categoryColor returns the ANSI foreground escape for a category.
func categoryColor(cat LogCategory) string {
	switch cat {
	case LogCatInfo:
		return ""
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
	}
	return ""
}

// ColorizeLogLine wraps a log line with ANSI color escapes based on its
// classified category.  The timestamp portion (first 19 chars) is rendered
// in dim gray, and the message portion (including the separator space)
// uses the category color.
// Lines shorter than the timestamp format are returned unmodified.
func ColorizeLogLine(line string) string {
	cat := ClassifyLogLine(line)
	color := categoryColor(cat)

	// Split at the timestamp boundary (19 chars = timestamp without space).
	// The space separator stays with the message portion for coloring.
	const tsDisplay = timestampPrefixLen - 1 // 19: "2006/01/02 15:04:05"
	if len(line) > tsDisplay {
		ts := line[:tsDisplay]
		rest := line[tsDisplay:]
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
