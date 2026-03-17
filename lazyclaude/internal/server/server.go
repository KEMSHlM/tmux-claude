package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
	"nhooyr.io/websocket"
)

// Config holds server configuration.
type Config struct {
	Port       int
	Token      string
	BinaryPath string
	IDEDir     string // lock files directory
	PortFile   string // path to write the listening port
	RuntimeDir string // choice files directory
}

// Server is the MCP WebSocket + HTTP server.
type Server struct {
	config  Config
	state   *State
	handler *Handler
	lock    *LockManager
	tmux    tmux.Client
	log     *log.Logger

	listener net.Listener
	httpSrv  *http.Server

	mu       sync.Mutex
	shutdown bool
}

// New creates a new Server.
func New(cfg Config, tmuxClient tmux.Client, logger *log.Logger) *Server {
	state := NewState()
	handler := NewHandler(state, tmuxClient, logger)
	lockMgr := NewLockManager(cfg.IDEDir)

	s := &Server{
		config:  cfg,
		state:   state,
		handler: handler,
		lock:    lockMgr,
		tmux:    tmuxClient,
		log:     logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/notify", s.handleNotify)
	mux.HandleFunc("/", s.handleWebSocket)

	s.httpSrv = &http.Server{Handler: mux}
	return s
}

// Start begins listening. Returns the actual port (useful when port=0).
func (s *Server) Start(ctx context.Context) (int, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", s.config.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("listen %s: %w", addr, err)
	}
	s.listener = ln

	port := ln.Addr().(*net.TCPAddr).Port
	s.config.Port = port

	// Write lock file
	if err := s.lock.Write(port, s.config.Token); err != nil {
		ln.Close()
		return 0, fmt.Errorf("write lock: %w", err)
	}

	// Write port file
	if err := s.writePortFile(port); err != nil {
		s.log.Printf("warning: write port file: %v", err)
	}

	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Printf("server error: %v", err)
		}
	}()

	s.log.Printf("listening on 127.0.0.1:%d", port)
	return port, nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.shutdown {
		s.mu.Unlock()
		return nil
	}
	s.shutdown = true
	s.mu.Unlock()

	// Remove lock file
	if err := s.lock.Remove(s.config.Port); err != nil {
		s.log.Printf("warning: remove lock: %v", err)
	}

	return s.httpSrv.Shutdown(ctx)
}

// Port returns the listening port.
func (s *Server) Port() int {
	return s.config.Port
}

// State returns the server's shared state (for testing).
func (s *Server) State() *State {
	return s.state
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Verify auth token (header only — never accept via URL query to avoid log leakage)
	token := r.Header.Get("X-Auth-Token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.config.Token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		s.log.Printf("ws accept: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	connID := fmt.Sprintf("ws-%s", r.RemoteAddr)
	s.log.Printf("ws connected: %s", connID)

	ctx := r.Context()
	s.serveConn(ctx, conn, connID)

	s.state.RemoveConn(connID)
	s.log.Printf("ws disconnected: %s", connID)
}

func (s *Server) serveConn(ctx context.Context, conn *websocket.Conn, connID string) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			s.log.Printf("ws read %s: %v", connID, err)
			return
		}

		req, err := ParseRequest(data)
		if err != nil {
			s.log.Printf("ws parse %s: %v", connID, err)
			continue
		}

		resp := s.handler.HandleMessage(ctx, connID, req)
		if resp == nil {
			continue
		}

		respData, err := MarshalResponse(*resp)
		if err != nil {
			s.log.Printf("ws marshal %s: %v", connID, err)
			continue
		}

		if err := conn.Write(ctx, websocket.MessageText, respData); err != nil {
			s.log.Printf("ws write %s: %v", connID, err)
			return
		}
	}
}

type notifyRequest struct {
	PID int `json:"pid"`
}

func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.Header.Get("X-Auth-Token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.config.Token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB cap
	var req notifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.PID <= 0 {
		http.Error(w, "invalid pid", http.StatusBadRequest)
		return
	}

	window := s.state.WindowForPID(req.PID)
	if window == "" {
		// Try to resolve
		w2, err := tmux.FindWindowForPid(r.Context(), s.tmux, req.PID)
		if err != nil || w2 == nil {
			http.Error(w, "window not found", http.StatusNotFound)
			return
		}
		window = w2.ID
	}

	s.log.Printf("notify: pid=%d window=%s", req.PID, window)

	// TODO: trigger tool popup via tmux display-popup

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"window": window}); err != nil {
		s.log.Printf("notify: encode response: %v", err)
	}
}

func (s *Server) writePortFile(port int) error {
	if s.config.PortFile == "" {
		return nil
	}
	return os.WriteFile(s.config.PortFile, []byte(strconv.Itoa(port)), 0o600)
}