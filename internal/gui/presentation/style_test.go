package presentation_test

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/gui/presentation"
	"github.com/stretchr/testify/assert"
)

func TestIconNeedsInput_ContainsBang(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconNeedsInput, "!", "IconNeedsInput should contain bang character")
}

func TestIconNeedsInput_IsMagenta(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconNeedsInput, "\x1b[35m", "IconNeedsInput should use magenta color")
}

func TestIconNeedsInput_ResetsColor(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconNeedsInput, "\x1b[0m", "IconNeedsInput should reset color")
}

func TestIconNeedsInput_IsDistinctFromDetached(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, presentation.IconNeedsInput, presentation.IconDetached,
		"IconNeedsInput and IconDetached should differ")
}

func TestIconIdle_ContainsCheckmark(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconIdle, "\xe2\x9c\x93", "IconIdle should contain checkmark")
}

func TestIconIdle_IsCyan(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconIdle, "\x1b[36m", "IconIdle should use cyan color")
}

func TestIconRunning_ContainsSpinner(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconRunning, "\xe2\x9f\xb3", "IconRunning should contain spinner character")
}

func TestIconRunning_IsGreen(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconRunning, "\x1b[32m", "IconRunning should use green color")
}

func TestIconError_ContainsCross(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconError, "\xe2\x9c\x97", "IconError should contain cross character")
}

func TestIconError_IsRed(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconError, "\x1b[31m", "IconError should use red color")
}

func TestIconPM_Exists(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, presentation.IconPM, "IconPM should be defined")
}

func TestIconPM_ContainsPMLabel(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconPM, "[PM]", "IconPM should contain [PM] label")
}

func TestIconPM_IsPurple(t *testing.T) {
	t.Parallel()
	// FgPurple is \x1b[38;5;141m
	assert.Contains(t, presentation.IconPM, "\x1b[38;5;141m", "IconPM should use purple color")
}

func TestIconPM_ResetsColor(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconPM, "\x1b[0m", "IconPM should reset color after label")
}

func TestIconPM_IsDistinctFromWorktree(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, presentation.IconPM, presentation.IconWorktree,
		"IconPM and IconWorktree should be different icons")
}
