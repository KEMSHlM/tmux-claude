package server_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
	"github.com/KEMSHlM/lazyclaude/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler() (*server.Handler, *server.State, *tmux.MockClient) {
	state := server.NewState()
	mock := tmux.NewMockClient()
	logger := log.New(os.Stderr, "test: ", 0)
	handler := server.NewHandler(state, mock, logger)
	return handler, state, mock
}

func TestHandler_Initialize(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"capabilities":{}}`),
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.Equal(t, json.RawMessage(`1`), resp.ID)

	// Verify result contains expected fields
	data, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"protocolVersion"`)
	assert.Contains(t, string(data), `"lazyclaude"`)
}

func TestHandler_NotificationsInitialized(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	assert.Nil(t, resp) // no response for notifications
}

func TestHandler_IDEConnected(t *testing.T) {
	t.Parallel()
	h, state, mock := newTestHandler()

	// Set up mock: panes and windows
	mock.Panes[""] = []tmux.PaneInfo{
		{ID: "%1", Window: "@1", PID: 1001},
	}
	mock.Sessions["lazyclaude"] = []tmux.WindowInfo{
		{ID: "@1", Name: "lc-abc", Session: "lazyclaude"},
	}
	mock.Messages["@1"] = "lazyclaude"

	req := &server.Request{
		JSONRPC: "2.0",
		Method:  "ide_connected",
		Params:  json.RawMessage(`{"pid":1001}`),
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	assert.Nil(t, resp) // notification, no response

	// Verify state was updated
	cs := state.GetConn("conn-1")
	require.NotNil(t, cs)
	assert.Equal(t, 1001, cs.PID)
	assert.Equal(t, "@1", cs.Window)
}

func TestHandler_IDEConnected_InvalidPID(t *testing.T) {
	t.Parallel()
	h, state, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		Method:  "ide_connected",
		Params:  json.RawMessage(`{"pid":0}`),
	}

	h.HandleMessage(context.Background(), "conn-1", req)
	assert.Nil(t, state.GetConn("conn-1")) // should not be registered
}

func TestHandler_IDEConnected_InvalidParams(t *testing.T) {
	t.Parallel()
	h, state, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		Method:  "ide_connected",
		Params:  json.RawMessage(`{invalid}`),
	}

	h.HandleMessage(context.Background(), "conn-1", req)
	assert.Nil(t, state.GetConn("conn-1"))
}

func TestHandler_IDEConnected_CachedWindow(t *testing.T) {
	t.Parallel()
	h, state, _ := newTestHandler()

	// Pre-populate cache
	state.SetConn("old-conn", &server.ConnState{PID: 2002, Window: "@5"})

	req := &server.Request{
		JSONRPC: "2.0",
		Method:  "ide_connected",
		Params:  json.RawMessage(`{"pid":2002}`),
	}

	h.HandleMessage(context.Background(), "conn-2", req)

	cs := state.GetConn("conn-2")
	require.NotNil(t, cs)
	assert.Equal(t, "@5", cs.Window) // used cached value
}

func TestHandler_OpenDiff(t *testing.T) {
	t.Parallel()
	h, state, _ := newTestHandler()

	// Register connection first
	state.SetConn("conn-1", &server.ConnState{PID: 1001, Window: "@1"})

	req := &server.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "openDiff",
		Params:  json.RawMessage(`{"old_file_path":"/home/user/src/main.go","new_contents":"package main"}`),
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	data, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"window":"@1"`)
	assert.Contains(t, string(data), `"old_path":"/home/user/src/main.go"`)
}

func TestHandler_OpenDiff_UnregisteredConnection(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "openDiff",
		Params:  json.RawMessage(`{"old_file_path":"/home/user/test.go","new_contents":"x"}`),
	}

	resp := h.HandleMessage(context.Background(), "conn-unknown", req)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32603, resp.Error.Code)
}

func TestHandler_OpenDiff_InvalidParams(t *testing.T) {
	t.Parallel()
	h, state, _ := newTestHandler()
	state.SetConn("conn-1", &server.ConnState{PID: 1001, Window: "@1"})

	req := &server.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "openDiff",
		Params:  json.RawMessage(`{invalid}`),
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestHandler_UnknownMethod_Request(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`99`),
		Method:  "unknown_method",
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "unknown_method")
}

func TestHandler_UnknownMethod_Notification(t *testing.T) {
	t.Parallel()
	h, _, _ := newTestHandler()

	req := &server.Request{
		JSONRPC: "2.0",
		Method:  "unknown_notification",
	}

	resp := h.HandleMessage(context.Background(), "conn-1", req)
	assert.Nil(t, resp) // notifications get no error response
}
