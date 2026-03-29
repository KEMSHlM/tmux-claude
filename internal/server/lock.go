package server

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const lockDialTimeout = 500 * time.Millisecond

// LockFile represents the contents of an IDE lock file.
type LockFile struct {
	PID       int    `json:"pid"`
	AuthToken string `json:"authToken"`
	Transport string `json:"transport"`
	App       string `json:"app,omitempty"` // "lazyclaude" for locks created by this binary
}

const lockApp = "lazyclaude"

// LockManager handles IDE lock file lifecycle.
type LockManager struct {
	ideDir string
}

// NewLockManager creates a lock manager.
// ideDir is typically ~/.claude/ide/
func NewLockManager(ideDir string) *LockManager {
	return &LockManager{ideDir: ideDir}
}

// Write creates a lock file at <ideDir>/<port>.lock.
func (m *LockManager) Write(port int, token string) error {
	if err := os.MkdirAll(m.ideDir, 0o700); err != nil {
		return fmt.Errorf("create ide dir: %w", err)
	}

	lock := LockFile{
		PID:       os.Getpid(),
		AuthToken: token,
		Transport: "ws",
		App:       lockApp,
	}
	data, err := json.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}

	path := m.lockPath(port)
	return os.WriteFile(path, data, 0o600)
}

// Read reads a lock file.
func (m *LockManager) Read(port int) (*LockFile, error) {
	data, err := os.ReadFile(m.lockPath(port))
	if err != nil {
		return nil, err
	}
	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	return &lock, nil
}

// Remove deletes a lock file.
func (m *LockManager) Remove(port int) error {
	return os.Remove(m.lockPath(port))
}

// Exists checks if a lock file exists for a port.
func (m *LockManager) Exists(port int) bool {
	_, err := os.Stat(m.lockPath(port))
	return err == nil
}

// CleanStale removes lock files whose server is no longer reachable.
// A lock is considered stale if either the PID is dead OR the port is
// not accepting TCP connections. Returns the number of removed files.
func (m *LockManager) CleanStale() int {
	entries, err := os.ReadDir(m.ideDir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".lock" {
			continue
		}
		path := filepath.Join(m.ideDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var lock LockFile
		if err := json.Unmarshal(data, &lock); err != nil {
			continue
		}

		// Extract port from filename (e.g. "51623.lock" → 51623).
		port, portErr := strconv.Atoi(strings.TrimSuffix(e.Name(), ".lock"))

		// Check 1: PID must be alive.
		pidAlive := false
		if lock.PID > 0 {
			if proc, err := os.FindProcess(lock.PID); err == nil {
				pidAlive = proc.Signal(syscall.Signal(0)) == nil
			}
		}

		// Check 2: TCP port must be reachable.
		tcpAlive := false
		if portErr == nil && port > 0 {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), lockDialTimeout)
			if err == nil {
				conn.Close()
				tcpAlive = true
			}
		}

		// Remove if either check fails.
		if !pidAlive || !tcpAlive {
			os.Remove(path)
			removed++
		}
	}
	return removed
}

// CleanAllExcept removes all lazyclaude lock files except the one for exceptPort.
// Non-lazyclaude locks (e.g. VS Code, JetBrains) are left untouched.
// Returns the number of removed files.
func (m *LockManager) CleanAllExcept(exceptPort int) int {
	entries, err := os.ReadDir(m.ideDir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".lock" {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".lock"))
		if err != nil || port == exceptPort {
			continue
		}
		path := filepath.Join(m.ideDir, e.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var lock LockFile
		if json.Unmarshal(data, &lock) != nil {
			continue
		}
		// Only remove locks created by lazyclaude (app=="lazyclaude" or
		// app=="" for locks from binaries before #33). Keep locks with any
		// other app value (e.g. VS Code, JetBrains).
		if lock.App != "" && lock.App != lockApp {
			continue
		}
		// Send SIGINT so the server can clean up gracefully.
		// Skip our own PID to avoid killing ourselves.
		if lock.PID > 0 && lock.PID != os.Getpid() {
			if proc, findErr := os.FindProcess(lock.PID); findErr == nil {
				_ = proc.Signal(os.Interrupt)
			}
		}
		os.Remove(path)
		removed++
	}
	return removed
}

func (m *LockManager) lockPath(port int) string {
	return filepath.Join(m.ideDir, strconv.Itoa(port)+".lock")
}