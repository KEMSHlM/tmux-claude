package daemon

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockClientAPI is a minimal mock for ClientAPI that only implements Health.
type mockClientAPI struct {
	healthResp *HealthResponse
	healthErr  error
}

func (m *mockClientAPI) Health(_ context.Context) (*HealthResponse, error) {
	return m.healthResp, m.healthErr
}

// Stubs for all other ClientAPI methods.
func (m *mockClientAPI) CreateSession(context.Context, SessionCreateRequest) (*SessionCreateResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) DeleteSession(context.Context, string) error                     { return nil }
func (m *mockClientAPI) RenameSession(context.Context, string, string) error             { return nil }
func (m *mockClientAPI) Sessions(context.Context) ([]SessionInfo, error)                 { return nil, nil }
func (m *mockClientAPI) PurgeOrphans(context.Context) (int, error)                       { return 0, nil }
func (m *mockClientAPI) CapturePreview(context.Context, string, int, int) (*PreviewResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) CaptureScrollback(context.Context, string, int, int, int) (*ScrollbackResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) HistorySize(context.Context, string) (*HistorySizeResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) SendKeys(context.Context, string, string) error        { return nil }
func (m *mockClientAPI) SendChoice(context.Context, string, string, int) error { return nil }
func (m *mockClientAPI) AttachSession(context.Context, string) (*AttachResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) CreateWorktree(context.Context, WorktreeCreateRequest) (*WorktreeCreateResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) ResumeWorktree(context.Context, WorktreeResumeRequest) (*WorktreeResumeResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) ListWorktrees(context.Context, string) ([]WorktreeInfo, error) {
	return nil, nil
}
func (m *mockClientAPI) MsgSend(context.Context, MsgSendRequest) (*MsgSendResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) MsgCreate(context.Context, MsgCreateRequest) (*MsgCreateResponse, error) {
	return nil, nil
}
func (m *mockClientAPI) MsgSessions(context.Context) (*MsgSessionsResponse, error) { return nil, nil }
func (m *mockClientAPI) Shutdown(context.Context, ShutdownRequest) error            { return nil }
func (m *mockClientAPI) SubscribeNotifications(context.Context) (<-chan NotificationEvent, error) {
	return nil, nil
}
func (m *mockClientAPI) PendingNotifications(context.Context) ([]*ToolNotificationInfo, error) {
	return nil, nil
}

func TestExponentialBackoff(t *testing.T) {
	b := NewExponentialBackoff(1*time.Second, 30*time.Second, 2)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // capped at max
		{6, 30 * time.Second}, // stays capped
	}

	for _, tt := range tests {
		got := b.Next()
		if got != tt.want {
			t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.want)
		}
	}

	if b.Attempts() != 7 {
		t.Errorf("Attempts() = %d, want 7", b.Attempts())
	}

	b.Reset()
	if b.Attempts() != 0 {
		t.Errorf("after Reset(), Attempts() = %d, want 0", b.Attempts())
	}
	if got := b.Next(); got != 1*time.Second {
		t.Errorf("after Reset(), Next() = %v, want %v", got, 1*time.Second)
	}
}

func TestRemoteConnection_InitialState(t *testing.T) {
	ssh := newMockSSH()
	lm := NewLifecycleManager(ssh)
	rc := NewRemoteConnection("user@host", lm, nil)

	if rc.State() != Disconnected {
		t.Errorf("State() = %v, want %v", rc.State(), Disconnected)
	}

	_, err := rc.Client()
	if err == nil {
		t.Error("Client() should return error when disconnected")
	}
}

func TestRemoteConnection_OnStateChange(t *testing.T) {
	ssh := newMockSSH()
	lm := NewLifecycleManager(ssh)
	rc := NewRemoteConnection("user@host", lm, nil)

	var mu sync.Mutex
	var states []ConnectionState
	rc.OnStateChange(func(s ConnectionState) {
		mu.Lock()
		states = append(states, s)
		mu.Unlock()
	})

	// Trigger a state change by calling Connect with a failing SSH.
	ssh.onRun("cat /tmp/lazyclaude-$(whoami)/daemon.json", "", fmt.Errorf("not found"))
	ssh.onRun("lazyclaude daemon --port 0", "", fmt.Errorf("connection refused"))

	rc.Connect(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(states) < 2 {
		t.Fatalf("expected at least 2 state changes, got %d: %v", len(states), states)
	}
	if states[0] != Connecting {
		t.Errorf("first state = %v, want %v", states[0], Connecting)
	}
	if states[1] != ConnectionError {
		t.Errorf("second state = %v, want %v", states[1], ConnectionError)
	}
}

func TestRemoteConnection_ConnectDiscover_HealthFail(t *testing.T) {
	ssh := newMockSSH()
	ssh.onRun("cat /tmp/lazyclaude-$(whoami)/daemon.json",
		`{"port":9999,"token":"tok"}`, nil)

	lm := NewLifecycleManager(ssh)

	client := &mockClientAPI{
		healthErr: fmt.Errorf("connection refused"),
	}
	factory := func(addr, token string) ClientAPI { return client }

	rc := NewRemoteConnection("user@host", lm, factory)
	err := rc.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if rc.State() != ConnectionError {
		t.Errorf("State() = %v, want %v", rc.State(), ConnectionError)
	}
}

func TestRemoteConnection_ConnectDiscover_VersionMismatch(t *testing.T) {
	ssh := newMockSSH()
	ssh.onRun("cat /tmp/lazyclaude-$(whoami)/daemon.json",
		`{"port":9999,"token":"tok"}`, nil)

	lm := NewLifecycleManager(ssh)

	client := &mockClientAPI{
		healthResp: &HealthResponse{APIVersion: 999},
	}
	factory := func(addr, token string) ClientAPI { return client }

	rc := NewRemoteConnection("user@host", lm, factory)
	err := rc.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if rc.State() != ConnectionError {
		t.Errorf("State() = %v, want %v", rc.State(), ConnectionError)
	}
}

func TestRemoteConnection_Disconnect_NotConnected(t *testing.T) {
	ssh := newMockSSH()
	lm := NewLifecycleManager(ssh)
	rc := NewRemoteConnection("user@host", lm, nil)

	if err := rc.Disconnect(); err != nil {
		t.Errorf("Disconnect() on disconnected connection: %v", err)
	}
	if rc.State() != Disconnected {
		t.Errorf("State() = %v, want %v", rc.State(), Disconnected)
	}
}

func TestRemoteConnection_MultipleCallbacks(t *testing.T) {
	ssh := newMockSSH()
	lm := NewLifecycleManager(ssh)
	rc := NewRemoteConnection("user@host", lm, nil)

	var mu sync.Mutex
	calls := make([]int, 2)
	rc.OnStateChange(func(_ ConnectionState) {
		mu.Lock()
		calls[0]++
		mu.Unlock()
	})
	rc.OnStateChange(func(_ ConnectionState) {
		mu.Lock()
		calls[1]++
		mu.Unlock()
	})

	ssh.onRun("cat /tmp/lazyclaude-$(whoami)/daemon.json", "", fmt.Errorf("not found"))
	ssh.onRun("lazyclaude daemon --port 0", "", fmt.Errorf("fail"))

	rc.Connect(context.Background())

	mu.Lock()
	defer mu.Unlock()
	// Both callbacks should have been called for both Connecting and ConnectionError.
	if calls[0] < 2 {
		t.Errorf("callback 0 called %d times, want >= 2", calls[0])
	}
	if calls[1] < 2 {
		t.Errorf("callback 1 called %d times, want >= 2", calls[1])
	}
}
