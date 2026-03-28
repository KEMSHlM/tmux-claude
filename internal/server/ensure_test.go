package server_test

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/core/config"
	"github.com/KEMSHlM/lazyclaude/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureServer_SkipsIfPortFileExistsAndAlive(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Start a real TCP listener to simulate a running server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	// Write port file pointing to our listener
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte(strconv.Itoa(port)), 0o600))

	result, err := server.EnsureServer(server.EnsureOpts{
		Binary:   "/does-not-matter",
		PortFile: paths.PortFile(),
	})
	require.NoError(t, err)
	assert.Equal(t, port, result.Port)
	assert.False(t, result.Started, "should not start a new server")
}

func TestEnsureServer_StartsIfNoPortFile(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Port file does not exist → should attempt to start
	_, err := server.EnsureServer(server.EnsureOpts{
		Binary:   "/nonexistent-binary",
		PortFile: paths.PortFile(),
	})
	// Binary doesn't exist, so start will fail
	assert.Error(t, err)
}

func TestEnsureServer_StartsIfPortFileStale(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Get a port that is guaranteed to have nothing listening
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stalePort := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // close immediately so nothing is listening

	// Write port file pointing to the now-closed port
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte(strconv.Itoa(stalePort)), 0o600))

	_, err = server.EnsureServer(server.EnsureOpts{
		Binary:   "/nonexistent-binary",
		PortFile: paths.PortFile(),
	})
	// Port file is stale, binary doesn't exist → error
	assert.Error(t, err)

	// Stale port file should have been removed
	_, statErr := os.Stat(paths.PortFile())
	assert.True(t, os.IsNotExist(statErr), "stale port file should be removed")
}

func TestEnsureServer_InvalidPortFile(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Write garbage to port file
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte("not-a-number"), 0o600))

	_, err := server.EnsureServer(server.EnsureOpts{
		Binary:   "/nonexistent-binary",
		PortFile: paths.PortFile(),
	})
	// Invalid port file → removed, then start fails
	assert.Error(t, err)

	// Invalid port file should have been removed
	_, statErr := os.Stat(paths.PortFile())
	assert.True(t, os.IsNotExist(statErr), "invalid port file should be removed")
}

func TestRestartServer_ReusesRecentlyStartedServer(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Start a real TCP listener to simulate a running server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	// Write port file with current mtime (just created = recent)
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte(strconv.Itoa(port)), 0o600))

	// RestartServer should detect the recently-started, alive server and reuse it
	result, err := server.RestartServer(server.EnsureOpts{
		Binary:   "/does-not-matter",
		PortFile: paths.PortFile(),
	})
	require.NoError(t, err)
	assert.Equal(t, port, result.Port)
	assert.False(t, result.Started, "should reuse recently started server, not restart")
}

func TestRestartServer_RestartsOldServer(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Start a real TCP listener to simulate a running server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	// Write port file and backdate its mtime to make it look old
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte(strconv.Itoa(port)), 0o600))
	oldTime := time.Now().Add(-10 * time.Second)
	require.NoError(t, os.Chtimes(paths.PortFile(), oldTime, oldTime))

	// RestartServer should proceed to kill and restart since port file is old
	_, err = server.RestartServer(server.EnsureOpts{
		Binary:   "/nonexistent-binary",
		PortFile: paths.PortFile(),
	})
	// Binary doesn't exist, so start will fail, but it should attempt to restart
	assert.Error(t, err, "should attempt restart and fail due to nonexistent binary")
}

func TestRestartServer_RestartsDeadServer(t *testing.T) {
	t.Parallel()
	paths := config.TestPaths(t.TempDir())

	// Get a port that nothing is listening on
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadPort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Write port file with current mtime (recent), but server is dead
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte(strconv.Itoa(deadPort)), 0o600))

	// RestartServer should not reuse a dead server even if port file is recent
	_, err = server.RestartServer(server.EnsureOpts{
		Binary:   "/nonexistent-binary",
		PortFile: paths.PortFile(),
	})
	assert.Error(t, err, "should attempt restart since server is dead")
}
