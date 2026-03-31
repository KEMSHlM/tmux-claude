package server_test

import (
	"encoding/json"
	"testing"

	"github.com/any-context/lazyclaude/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRequest_MethodCall(t *testing.T) {
	t.Parallel()
	data := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)

	req, err := server.ParseRequest(data)
	require.NoError(t, err)
	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, "initialize", req.Method)
	assert.False(t, req.IsNotification())
	assert.JSONEq(t, `{"capabilities":{}}`, string(req.Params))
}

func TestParseRequest_Notification(t *testing.T) {
	t.Parallel()
	data := []byte(`{"jsonrpc":"2.0","method":"ide_connected","params":{"pid":1234}}`)

	req, err := server.ParseRequest(data)
	require.NoError(t, err)
	assert.Equal(t, "ide_connected", req.Method)
	assert.True(t, req.IsNotification())
}

func TestParseRequest_NullID(t *testing.T) {
	t.Parallel()
	data := []byte(`{"jsonrpc":"2.0","id":null,"method":"test"}`)

	req, err := server.ParseRequest(data)
	require.NoError(t, err)
	assert.True(t, req.IsNotification())
}

func TestParseRequest_StringID(t *testing.T) {
	t.Parallel()
	data := []byte(`{"jsonrpc":"2.0","id":"abc-123","method":"test"}`)

	req, err := server.ParseRequest(data)
	require.NoError(t, err)
	assert.False(t, req.IsNotification())
	assert.Equal(t, `"abc-123"`, string(req.ID))
}

func TestParseRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := server.ParseRequest([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestParseRequest_WrongVersion(t *testing.T) {
	t.Parallel()
	_, err := server.ParseRequest([]byte(`{"jsonrpc":"1.0","method":"test"}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported jsonrpc version")
}

func TestParseRequest_MissingVersion(t *testing.T) {
	t.Parallel()
	_, err := server.ParseRequest([]byte(`{"method":"test"}`))
	assert.Error(t, err)
}

func TestNewResponse(t *testing.T) {
	t.Parallel()
	id := json.RawMessage(`1`)
	resp := server.NewResponse(id, map[string]string{"status": "ok"})

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, json.RawMessage(`1`), resp.ID)
	assert.Nil(t, resp.Error)

	data, err := server.MarshalResponse(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"ok"`)
}

func TestNewErrorResponse(t *testing.T) {
	t.Parallel()
	id := json.RawMessage(`42`)
	resp := server.NewErrorResponse(id, -32601, "Method not found")

	assert.Equal(t, "2.0", resp.JSONRPC)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)

	data, err := server.MarshalResponse(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"code":-32601`)
}

func TestMarshalResponse_Roundtrip(t *testing.T) {
	t.Parallel()
	id := json.RawMessage(`"req-1"`)
	original := server.NewResponse(id, map[string]any{
		"capabilities": map[string]any{},
	})

	data, err := server.MarshalResponse(original)
	require.NoError(t, err)

	var decoded server.Response
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, json.RawMessage(`"req-1"`), decoded.ID)
}