package main

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/daemon"
	"github.com/stretchr/testify/assert"
)

func TestResolveRemotePath_NonLocalPath_Passthrough(t *testing.T) {
	t.Parallel()
	a := &guiCompositeAdapter{
		cp:               daemon.NewCompositeProvider(nil, nil),
		localProjectRoot: "/local/project",
	}
	// A path that is neither "." nor localProjectRoot passes through unchanged.
	assert.Equal(t, "/home/user/other-project", a.resolveRemotePath("/home/user/other-project", "remote"))
}

func TestResolveRemotePath_DotPath_NoProvider_Passthrough(t *testing.T) {
	t.Parallel()
	a := &guiCompositeAdapter{
		cp:               daemon.NewCompositeProvider(nil, nil),
		localProjectRoot: "/local/project",
	}
	// "." path with no remote provider falls back to the original path.
	assert.Equal(t, ".", a.resolveRemotePath(".", "remote"))
}

func TestResolveRemotePath_LocalProjectRoot_NoProvider_Passthrough(t *testing.T) {
	t.Parallel()
	a := &guiCompositeAdapter{
		cp:               daemon.NewCompositeProvider(nil, nil),
		localProjectRoot: "/local/project",
	}
	// localProjectRoot with no remote provider falls back to the original path.
	assert.Equal(t, "/local/project", a.resolveRemotePath("/local/project", "remote"))
}
