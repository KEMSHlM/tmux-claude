// Package shell provides shell-related utilities.
package shell

import "strings"

// Quote wraps a string in single quotes for safe shell interpolation.
// Single quotes within the string are escaped using the '\'' pattern.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
