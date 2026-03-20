package shell_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/shell"
	"github.com/stretchr/testify/assert"
)

func TestQuote_Simple(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "'hello'", shell.Quote("hello"))
}

func TestQuote_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "''", shell.Quote(""))
}

func TestQuote_ContainsSingleQuote(t *testing.T) {
	t.Parallel()
	// "it's" -> 'it'\''s'
	assert.Equal(t, "'it'\\''s'", shell.Quote("it's"))
}

func TestQuote_MultipleSingleQuotes(t *testing.T) {
	t.Parallel()
	// "a'b'c" -> 'a'\''b'\''c'
	assert.Equal(t, "'a'\\''b'\\''c'", shell.Quote("a'b'c"))
}

func TestQuote_SpecialChars(t *testing.T) {
	t.Parallel()
	// Spaces and other special chars are safely wrapped
	result := shell.Quote("hello world")
	assert.Equal(t, "'hello world'", result)
}

func TestQuote_DollarSign(t *testing.T) {
	t.Parallel()
	// $ is safely wrapped inside single quotes
	assert.Equal(t, "'$HOME'", shell.Quote("$HOME"))
}
