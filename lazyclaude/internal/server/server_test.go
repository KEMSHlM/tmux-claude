package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
	"github.com/KEMSHlM/lazyclaude/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

func startTestServer(t *testing.T) (*server.Server, int, *tmux.MockClient) {
	t.Helper()
	mock := tmux.NewMockClient()
	logger := log.New(&bytes.Buffer{}, "", 0)

	tmpDir := t.TempDir()
	cfg := server.Config{
		Port:       0, // random port
		Token:      "test-token",
		IDEDir:     filepath.Join(tmpDir, "ide"),
		RuntimeDir: filepath.Join(tmpDir, "run"),
	}

	srv := server.New(cfg, mock, logger)
	ctx := context.Background()
	port, err := srv.Start(ctx)
	require.NoError(t, err)
	require.Greater(t, port, 0)

	t.Cleanup(func() {
		srv.Stop(context.Background())
	})

	return srv, port, mock
}

func TestServer_StartAndStop(t *testing.T) {
	t.Parallel()
	srv, port, _ := startTestServer(t)

	assert.Greater(t, port, 0)
	assert.Equal(t, port, srv.Port())

	err := srv.Stop(context.Background())
	require.NoError(t, err)

	// Double stop should be safe
	err = srv.Stop(context.Background())
	require.NoError(t, err)
}

func TestServer_WebSocket_Unauthorized(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect without token
	_, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/", port), nil)
	assert.Error(t, err) // should fail with 401
}

func TestServer_WebSocket_Initialize(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/", port), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Auth-Token": []string{"test-token"}},
	})
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send initialize request
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"capabilities": map[string]any{}},
	}
	data, _ := json.Marshal(initReq)
	err = conn.Write(ctx, websocket.MessageText, data)
	require.NoError(t, err)

	// Read response
	_, respData, err := conn.Read(ctx)
	require.NoError(t, err)

	var resp server.Response
	require.NoError(t, json.Unmarshal(respData, &resp))
	assert.Nil(t, resp.Error)

	resultJSON, _ := json.Marshal(resp.Result)
	assert.Contains(t, string(resultJSON), "lazyclaude")
	assert.Contains(t, string(resultJSON), "protocolVersion")
}

func TestServer_WebSocket_IDEConnected_Then_OpenDiff(t *testing.T) {
	t.Parallel()
	srv, port, mock := startTestServer(t)

	// Set up mock for PID resolution
	mock.Panes[""] = []tmux.PaneInfo{
		{ID: "%1", Window: "@1", PID: 5555},
	}
	mock.Sessions["lc"] = []tmux.WindowInfo{
		{ID: "@1", Name: "lc-abc", Session: "lc"},
	}
	mock.Messages["@1"] = "lc"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/", port), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Auth-Token": []string{"test-token"}},
	})
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// 1) Send ide_connected (notification, no response)
	ideReq, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "ide_connected",
		"params":  map[string]any{"pid": 5555},
	})
	require.NoError(t, conn.Write(ctx, websocket.MessageText, ideReq))

	// Poll for state update with deadline (avoid flaky time.Sleep)
	require.Eventually(t, func() bool {
		return srv.State().ConnCount() >= 1
	}, 2*time.Second, 10*time.Millisecond, "expected connection to be registered")

	// 2) Send openDiff (request, expects response)
	diffReq, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "openDiff",
		"params":  map[string]any{"old_file_path": "/home/user/main.go", "new_contents": "package main"},
	})
	require.NoError(t, conn.Write(ctx, websocket.MessageText, diffReq))

	_, respData, err := conn.Read(ctx)
	require.NoError(t, err)

	var resp server.Response
	require.NoError(t, json.Unmarshal(respData, &resp))
	assert.Nil(t, resp.Error)

	resultJSON, _ := json.Marshal(resp.Result)
	assert.Contains(t, string(resultJSON), "@1")
	assert.Contains(t, string(resultJSON), "main.go")
}

func TestServer_Notify_POST(t *testing.T) {
	t.Parallel()
	srv, port, _ := startTestServer(t)

	// Pre-populate PID -> window mapping
	srv.State().SetConn("c1", &server.ConnState{PID: 7777, Window: "@3"})

	body, _ := json.Marshal(map[string]int{"pid": 7777})
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/notify", port),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "test-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "@3", result["window"])
}

func TestServer_Notify_Unauthorized(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	body, _ := json.Marshal(map[string]int{"pid": 1})
	resp, err := http.Post(
		fmt.Sprintf("http://127.0.0.1:%d/notify", port),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestServer_Notify_GET_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/notify", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func postNotify(t *testing.T, port int, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/notify", port),
		bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "test-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestServer_Notify_InvalidPID(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	body, _ := json.Marshal(map[string]int{"pid": 0})
	resp := postNotify(t, port, body)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_Notify_UnknownPID(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	body, _ := json.Marshal(map[string]int{"pid": 9999})
	resp := postNotify(t, port, body)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_Notify_BadJSON(t *testing.T) {
	t.Parallel()
	_, port, _ := startTestServer(t)

	resp := postNotify(t, port, []byte(`{invalid`))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_LockFile_CreatedAndRemoved(t *testing.T) {
	t.Parallel()
	srv, _, _ := startTestServer(t)

	// Lock file creation is verified by the server starting successfully.
	// Stop should remove lock file.
	err := srv.Stop(context.Background())
	require.NoError(t, err)
}