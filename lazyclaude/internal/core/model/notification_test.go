package model_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/model"
	"github.com/stretchr/testify/assert"
)

func TestIsDiff_True(t *testing.T) {
	t.Parallel()
	n := model.ToolNotification{ToolName: "Write", OldFilePath: "/tmp/test.go"}
	assert.True(t, n.IsDiff())
}

func TestIsDiff_False(t *testing.T) {
	t.Parallel()
	n := model.ToolNotification{ToolName: "Bash"}
	assert.False(t, n.IsDiff())
}
