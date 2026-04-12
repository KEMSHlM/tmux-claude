package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// askpassTempDir creates a short temp directory under /tmp to avoid macOS
// Unix socket path length limits (~104 chars). t.TempDir() paths are too long.
func askpassTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "lc-askpass-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestAskpassServer_StartStop(t *testing.T) {
	dir := askpassTempDir(t)
	srv := NewAskpassServer(dir, func(prompt string) (string, error) {
		return "unused", nil
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	sockPath := srv.SockPath()
	if _, err := os.Stat(sockPath); err != nil {
		t.Errorf("socket file should exist: %v", err)
	}

	// Verify socket permissions are 0600.
	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("socket permissions = %o, want 0600", perm)
	}

	srv.Stop()

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after Stop()")
	}
}

func TestAskpassServer_PromptResponse(t *testing.T) {
	dir := askpassTempDir(t)
	srv := NewAskpassServer(dir, func(prompt string) (string, error) {
		if prompt != "Password: " {
			t.Errorf("handler received prompt=%q, want %q", prompt, "Password: ")
		}
		return "s3cret", nil
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("unix", srv.SockPath())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send prompt.
	fmt.Fprintln(conn, "Password: ")

	// Read response.
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("expected response line")
	}
	if got := scanner.Text(); got != "s3cret" {
		t.Errorf("response = %q, want %q", got, "s3cret")
	}
}

func TestAskpassServer_Cancel(t *testing.T) {
	dir := askpassTempDir(t)
	srv := NewAskpassServer(dir, func(prompt string) (string, error) {
		return "", nil // empty = cancel
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("unix", srv.SockPath())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	fmt.Fprintln(conn, "Password: ")

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("expected response line")
	}
	if got := scanner.Text(); got != "" {
		t.Errorf("response = %q, want empty string", got)
	}
}

func TestAskpassServer_HandlerError(t *testing.T) {
	dir := askpassTempDir(t)
	srv := NewAskpassServer(dir, func(prompt string) (string, error) {
		return "", fmt.Errorf("dialog cancelled")
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	conn, err := net.Dial("unix", srv.SockPath())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	fmt.Fprintln(conn, "Password: ")

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("expected response line")
	}
	// Handler error -> empty line response.
	if got := scanner.Text(); got != "" {
		t.Errorf("response = %q, want empty string", got)
	}
}

func TestAskpassServer_SockPath(t *testing.T) {
	dir := askpassTempDir(t)
	srv := NewAskpassServer(dir, nil)
	want := filepath.Join(dir, "askpass.sock")
	if got := srv.SockPath(); got != want {
		t.Errorf("SockPath() = %q, want %q", got, want)
	}
}

func TestAskpassServer_DoubleStop(t *testing.T) {
	dir := askpassTempDir(t)
	srv := NewAskpassServer(dir, func(prompt string) (string, error) {
		return "", nil
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Double stop should not panic.
	srv.Stop()
	srv.Stop()
}
