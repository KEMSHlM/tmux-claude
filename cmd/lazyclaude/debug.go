package main

import (
	"fmt"
	"os"
	"time"
)

var debugEnabled = os.Getenv("LAZYCLAUDE_DEBUG") != ""

// debugLog appends a timestamped line to /tmp/lazyclaude-debug.log.
// Enabled by setting LAZYCLAUDE_DEBUG=1 environment variable.
func debugLog(format string, args ...any) {
	if !debugEnabled {
		return
	}
	f, err := os.OpenFile("/tmp/lazyclaude-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
