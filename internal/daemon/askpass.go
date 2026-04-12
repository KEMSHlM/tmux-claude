package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// AskpassServer listens on a Unix socket for SSH_ASKPASS requests.
// When an askpass client connects, it reads a prompt string, calls
// the handler to get the user's response (typically via a GUI popup),
// and sends the response back.
type AskpassServer struct {
	sockPath string
	listener net.Listener
	handler  func(prompt string) (string, error)

	mu     sync.Mutex
	closed bool
}

// NewAskpassServer creates a new AskpassServer. The handler is called
// for each incoming prompt and should return the user's password or
// an empty string for cancellation.
func NewAskpassServer(runtimeDir string, handler func(prompt string) (string, error)) *AskpassServer {
	return &AskpassServer{
		sockPath: filepath.Join(runtimeDir, "askpass.sock"),
		handler:  handler,
	}
}

// Start begins listening on the Unix socket and accepting connections.
// Each connection is handled in a separate goroutine.
func (s *AskpassServer) Start() error {
	// Remove stale socket file.
	os.Remove(s.sockPath)

	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return fmt.Errorf("listen on askpass socket: %w", err)
	}

	// Set socket permissions to owner-only (0600).
	if err := os.Chmod(s.sockPath, 0o600); err != nil {
		ln.Close()
		os.Remove(s.sockPath)
		return fmt.Errorf("chmod askpass socket: %w", err)
	}

	s.mu.Lock()
	s.listener = ln
	s.closed = false
	s.mu.Unlock()

	go s.acceptLoop()
	return nil
}

// Stop closes the listener and removes the socket file.
func (s *AskpassServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.sockPath)
}

// SockPath returns the Unix socket file path.
func (s *AskpassServer) SockPath() string {
	return s.sockPath
}

func (s *AskpassServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *AskpassServer) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}
	prompt := scanner.Text()

	response, err := s.handler(prompt)
	if err != nil {
		// Send empty line to signal cancellation.
		fmt.Fprintln(conn, "")
		return
	}

	fmt.Fprintln(conn, response)
}
