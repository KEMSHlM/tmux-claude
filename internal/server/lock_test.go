package server_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockManager_WriteAndRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	err := lm.Write(7860, "test-token-abc")
	require.NoError(t, err)

	lock, err := lm.Read(7860)
	require.NoError(t, err)
	assert.Equal(t, "test-token-abc", lock.AuthToken)
	assert.Equal(t, "ws", lock.Transport)
	assert.Greater(t, lock.PID, 0)
}

func TestLockManager_Exists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	assert.False(t, lm.Exists(7860))

	require.NoError(t, lm.Write(7860, "token"))
	assert.True(t, lm.Exists(7860))
}

func TestLockManager_Remove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	require.NoError(t, lm.Write(7860, "token"))
	assert.True(t, lm.Exists(7860))

	require.NoError(t, lm.Remove(7860))
	assert.False(t, lm.Exists(7860))
}

func TestLockManager_Remove_NotExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	err := lm.Remove(9999)
	assert.Error(t, err) // file doesn't exist
}

func TestLockManager_Read_NotExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	_, err := lm.Read(9999)
	assert.Error(t, err)
}

func TestLockManager_CleanStale_RemovesDeadPID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ideDir := filepath.Join(dir, "ide")
	lm := server.NewLockManager(ideDir)

	// Write a lock with a PID that doesn't exist (99999999).
	require.NoError(t, os.MkdirAll(ideDir, 0o700))
	staleData := `{"pid":99999999,"authToken":"stale","transport":"ws"}`
	require.NoError(t, os.WriteFile(filepath.Join(ideDir, "11111.lock"), []byte(staleData), 0o600))

	// Write a lock with our own PID (alive) AND a listening port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	alivePort := ln.Addr().(*net.TCPAddr).Port
	aliveData := fmt.Sprintf(`{"pid":%d,"authToken":"alive","transport":"ws"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", alivePort)),
		[]byte(aliveData), 0o600,
	))

	removed := lm.CleanStale()
	assert.Equal(t, 1, removed, "should remove 1 stale lock")
	assert.False(t, lm.Exists(11111), "stale lock should be removed")
	assert.True(t, lm.Exists(alivePort), "alive lock should remain")
}

func TestLockManager_CleanStale_RemovesPIDAliveButPortDead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ideDir := filepath.Join(dir, "ide")
	lm := server.NewLockManager(ideDir)

	// Write a lock with our own PID (alive) but a port that is NOT listening.
	require.NoError(t, os.MkdirAll(ideDir, 0o700))
	myPID := os.Getpid()
	staleData := fmt.Sprintf(`{"pid":%d,"authToken":"stale","transport":"ws"}`, myPID)
	require.NoError(t, os.WriteFile(filepath.Join(ideDir, "19999.lock"), []byte(staleData), 0o600))

	// Write a lock with our own PID and a port that IS listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	alivePort := ln.Addr().(*net.TCPAddr).Port
	aliveData := fmt.Sprintf(`{"pid":%d,"authToken":"alive","transport":"ws"}`, myPID)
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", alivePort)),
		[]byte(aliveData), 0o600,
	))

	removed := lm.CleanStale()
	assert.Equal(t, 1, removed, "should remove lock with dead port")
	assert.False(t, lm.Exists(19999), "dead-port lock should be removed")
	assert.True(t, lm.Exists(alivePort), "alive-port lock should remain")
}

func TestLockManager_CleanStale_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	removed := lm.CleanStale()
	assert.Equal(t, 0, removed)
}

func TestLockManager_CleanAllExcept_RemovesLazyclaudeLocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ideDir := filepath.Join(dir, "ide")
	lm := server.NewLockManager(ideDir)
	require.NoError(t, os.MkdirAll(ideDir, 0o700))

	// Lock 1: lazyclaude lock on a listening port (should be removed).
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln1.Close()
	port1 := ln1.Addr().(*net.TCPAddr).Port
	data1 := fmt.Sprintf(`{"pid":%d,"authToken":"t1","transport":"ws","app":"lazyclaude"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", port1)),
		[]byte(data1), 0o600,
	))

	// Lock 2: lazyclaude lock — the "self" port (should be kept).
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln2.Close()
	selfPort := ln2.Addr().(*net.TCPAddr).Port
	data2 := fmt.Sprintf(`{"pid":%d,"authToken":"t2","transport":"ws","app":"lazyclaude"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", selfPort)),
		[]byte(data2), 0o600,
	))

	// Lock 3: non-lazyclaude (IDE) lock with explicit app field (should be kept).
	ln3, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln3.Close()
	idePort := ln3.Addr().(*net.TCPAddr).Port
	data3 := fmt.Sprintf(`{"pid":%d,"authToken":"t3","transport":"ws","app":"vscode"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", idePort)),
		[]byte(data3), 0o600,
	))

	removed := lm.CleanAllExcept(selfPort)
	assert.Equal(t, 1, removed, "should remove 1 lazyclaude lock (not self, not IDE)")
	assert.False(t, lm.Exists(port1), "other lazyclaude lock should be removed")
	assert.True(t, lm.Exists(selfPort), "self lock should remain")
	assert.True(t, lm.Exists(idePort), "IDE lock should remain")
}

func TestLockManager_CleanAllExcept_RemovesLegacyLocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ideDir := filepath.Join(dir, "ide")
	lm := server.NewLockManager(ideDir)
	require.NoError(t, os.MkdirAll(ideDir, 0o700))

	// Legacy lock without app field (created by binaries before #33).
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln1.Close()
	legacyPort := ln1.Addr().(*net.TCPAddr).Port
	legacyData := fmt.Sprintf(`{"pid":%d,"authToken":"t1","transport":"ws"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", legacyPort)),
		[]byte(legacyData), 0o600,
	))

	// Current lazyclaude lock — the "self" port (should be kept).
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln2.Close()
	selfPort := ln2.Addr().(*net.TCPAddr).Port
	selfData := fmt.Sprintf(`{"pid":%d,"authToken":"t2","transport":"ws","app":"lazyclaude"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", selfPort)),
		[]byte(selfData), 0o600,
	))

	// IDE lock with explicit app (should be kept).
	ln3, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln3.Close()
	idePort := ln3.Addr().(*net.TCPAddr).Port
	ideData := fmt.Sprintf(`{"pid":%d,"authToken":"t3","transport":"ws","app":"vscode"}`, os.Getpid())
	require.NoError(t, os.WriteFile(
		filepath.Join(ideDir, fmt.Sprintf("%d.lock", idePort)),
		[]byte(ideData), 0o600,
	))

	removed := lm.CleanAllExcept(selfPort)
	assert.Equal(t, 1, removed, "should remove legacy lock without app field")
	assert.False(t, lm.Exists(legacyPort), "legacy lock should be removed")
	assert.True(t, lm.Exists(selfPort), "self lock should remain")
	assert.True(t, lm.Exists(idePort), "IDE lock should remain")
}

func TestLockManager_WriteIncludesApp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lm := server.NewLockManager(filepath.Join(dir, "ide"))

	require.NoError(t, lm.Write(7860, "token"))
	lock, err := lm.Read(7860)
	require.NoError(t, err)
	assert.Equal(t, "lazyclaude", lock.App)
}

func TestLockManager_FilePermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ideDir := filepath.Join(dir, "ide")
	lm := server.NewLockManager(ideDir)

	require.NoError(t, lm.Write(7860, "secret-token"))

	// Lock file should be user-only readable (0600)
	path := filepath.Join(ideDir, "7860.lock")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}