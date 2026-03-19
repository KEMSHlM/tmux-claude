package choice

import (
	"regexp"
	"strconv"
	"strings"
)

// optionPattern matches Claude Code's numbered dialog options.
// Handles formats: "1. Yes", "1) Yes", "> 1. Yes", "❯ 1. Yes"
var optionPattern = regexp.MustCompile(`^\s*(?:[>❯]\s+)?(\d+)[.)]`)

const defaultMaxOption = 3

// DetectMaxOption scans tmux capture-pane output for numbered dialog options
// and returns the highest option number found. Returns 3 as default if no
// options are detected.
//
// This is used to clamp the user's choice to valid range before sending
// via tmux send-keys to Claude Code's permission dialog.
func DetectMaxOption(paneContent string) int {
	max := 0
	for _, line := range strings.Split(paneContent, "\n") {
		matches := optionPattern.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		n, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	if max == 0 {
		return defaultMaxOption
	}
	return max
}
