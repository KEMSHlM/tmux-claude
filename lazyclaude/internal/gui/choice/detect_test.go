package choice_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/choice"
	"github.com/stretchr/testify/assert"
)

func TestDetectMaxOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "3-option dialog (standard)",
			input: ` Do you want to create hello.txt?
 > 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No`,
			expected: 3,
		},
		{
			name: "2-option dialog",
			input: ` Allow this action?
 > 1. Yes
   2. No`,
			expected: 2,
		},
		{
			name: "no options found",
			input: `Some random output
with no numbered options`,
			expected: 3, // default
		},
		{
			name:     "empty string",
			input:    "",
			expected: 3, // default
		},
		{
			name: "options with dot separator",
			input: ` 1. Accept
 2. Reject`,
			expected: 2,
		},
		{
			name: "options with paren separator",
			input: ` 1) Accept
 2) Allow
 3) Reject`,
			expected: 3,
		},
		{
			name: "cursor arrow on option",
			input: `> 1. Yes
  2. Yes, allow always
  3. No`,
			expected: 3,
		},
		{
			name: "mixed content with numbers in text",
			input: `File has 42 lines
 > 1. Yes
   2. No
Some other text with number 99`,
			expected: 2,
		},
		{
			name: "unicode marker",
			input: ` ❯ 1. Yes
   2. Yes, allow all edits during this session (shift+tab)
   3. No`,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := choice.DetectMaxOption(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
