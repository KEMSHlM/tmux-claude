package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds all filesystem paths used by lazyclaude.
// All paths are configurable to enable test isolation.
type Paths struct {
	IDEDir     string // lock files: <IDEDir>/<port>.lock
	DataDir    string // state file: <DataDir>/state.json
	RuntimeDir string // choice files, port file
}

// DefaultPaths returns production paths.
// Panics if home directory cannot be determined.
func DefaultPaths() Paths {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("cannot determine home directory: %v", err))
	}
	return Paths{
		IDEDir:     filepath.Join(home, ".claude", "ide"),
		DataDir:    filepath.Join(home, ".local", "share", "lazyclaude"),
		RuntimeDir: os.TempDir(),
	}
}

// TestPaths returns isolated paths under a temporary directory.
// Use this in tests to avoid affecting production state.
func TestPaths(tmpDir string) Paths {
	return Paths{
		IDEDir:     filepath.Join(tmpDir, "ide"),
		DataDir:    filepath.Join(tmpDir, "data"),
		RuntimeDir: filepath.Join(tmpDir, "run"),
	}
}

// StateFile returns the path to the session state file.
func (p Paths) StateFile() string {
	return filepath.Join(p.DataDir, "state.json")
}

// PortFile returns the path to the MCP server port file.
func (p Paths) PortFile() string {
	return filepath.Join(p.RuntimeDir, "lazyclaude-mcp.port")
}

// ChoiceFile returns the path to a choice file for a window.
func (p Paths) ChoiceFile(window string) string {
	return filepath.Join(p.RuntimeDir, "lazyclaude-choice-"+window+".txt")
}

// LockFile returns the path to an IDE lock file for a port.
func (p Paths) LockFile(port int) string {
	return filepath.Join(p.IDEDir, fmt.Sprintf("%d.lock", port))
}
