package gui_test

import (
	"os"
	"testing"

	"github.com/any-context/lazyclaude/internal/core/config"
	"github.com/any-context/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPaths(t *testing.T) config.Paths {
	t.Helper()
	tmp := t.TempDir()
	p := config.TestPaths(tmp)
	os.MkdirAll(p.RuntimeDir, 0o755)
	return p
}

func TestWriteAndReadChoiceFile(t *testing.T) {
	t.Parallel()
	paths := testPaths(t)

	err := gui.WriteChoiceFile(paths, "test-wr", gui.ChoiceAccept)
	require.NoError(t, err)

	choice, err := gui.ReadChoiceFile(paths, "test-wr")
	require.NoError(t, err)
	assert.Equal(t, gui.ChoiceAccept, choice)

	// File should be removed after reading
	_, err = os.Stat(paths.ChoiceFile("test-wr"))
	assert.True(t, os.IsNotExist(err))
}

func TestWriteChoiceFile_AllChoices(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		choice gui.Choice
		want   int
	}{
		{"accept", gui.ChoiceAccept, 1},
		{"allow", gui.ChoiceAllow, 2},
		{"reject", gui.ChoiceReject, 3},
		{"cancel", gui.ChoiceCancel, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			paths := testPaths(t)

			err := gui.WriteChoiceFile(paths, "test-"+tt.name, tt.choice)
			require.NoError(t, err)

			choice, err := gui.ReadChoiceFile(paths, "test-"+tt.name)
			require.NoError(t, err)
			assert.Equal(t, gui.Choice(tt.want), choice)
		})
	}
}

func TestReadChoiceFile_NotExists(t *testing.T) {
	t.Parallel()
	paths := testPaths(t)

	_, err := gui.ReadChoiceFile(paths, "nonexistent")
	assert.Error(t, err)
}

func TestWriteChoiceFile_Permissions(t *testing.T) {
	t.Parallel()
	paths := testPaths(t)

	err := gui.WriteChoiceFile(paths, "test-perms", gui.ChoiceAccept)
	require.NoError(t, err)

	info, err := os.Stat(paths.ChoiceFile("test-perms"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
