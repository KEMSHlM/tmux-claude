package keymap_test

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/gui/keymap"
	"github.com/stretchr/testify/assert"
)

func TestDocSection_ValidSection(t *testing.T) {
	t.Parallel()
	content := keymap.DocSection("new_session")
	assert.NotEmpty(t, content, "new_session section should exist")
	assert.Contains(t, content, "session")
}

func TestDocSection_UnknownSection(t *testing.T) {
	t.Parallel()
	content := keymap.DocSection("nonexistent_section_xyz")
	assert.Empty(t, content, "unknown section should return empty string")
}

func TestDocSection_AllRegisteredActions(t *testing.T) {
	t.Parallel()
	r := keymap.Default()
	defs := r.AllActions()

	for _, d := range defs {
		if d.DocSection == "" {
			continue
		}
		content := keymap.DocSection(d.DocSection)
		assert.NotEmpty(t, content, "DocSection %q for action %s should resolve to content", d.DocSection, d.Action)
	}
}
