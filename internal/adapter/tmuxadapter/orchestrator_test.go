package tmuxadapter

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPopupClient records DisplayPopup calls and blocks until released.
type mockPopupClient struct {
	tmux.Client
	mu     sync.Mutex
	calls  []tmux.PopupOpts
	gates  chan struct{} // send to release a blocked DisplayPopup
	doneCh chan string   // receives window after DisplayPopup returns
}

func newMockPopupClient() *mockPopupClient {
	return &mockPopupClient{
		gates:  make(chan struct{}, 10),
		doneCh: make(chan string, 10),
	}
}

func (m *mockPopupClient) FindActiveClient(_ context.Context) (*tmux.ClientInfo, error) {
	return &tmux.ClientInfo{Name: "/dev/pts/0"}, nil
}

func (m *mockPopupClient) DisplayPopup(_ context.Context, opts tmux.PopupOpts) error {
	m.mu.Lock()
	m.calls = append(m.calls, opts)
	m.mu.Unlock()

	// Block until released (simulates popup open)
	<-m.gates

	m.doneCh <- opts.Target
	return nil
}

func (m *mockPopupClient) getCalls() []tmux.PopupOpts {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]tmux.PopupOpts, len(m.calls))
	copy(cp, m.calls)
	return cp
}

func TestPopupQueue_SinglePopup(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger)

	p.SpawnToolPopup(context.Background(), "@1", "Bash", "{}", "/tmp")

	// Wait for popup to be spawned
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)

	// Release popup
	mock.gates <- struct{}{}

	// Should complete
	select {
	case w := <-mock.doneCh:
		assert.Equal(t, "@1", w)
	case <-time.After(time.Second):
		t.Fatal("popup did not complete")
	}
}

func TestPopupQueue_SequentialForSameWindow(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger)

	// Spawn 3 popups for the same window rapidly
	p.SpawnToolPopup(context.Background(), "@1", "Tool1", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@1", "Tool2", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@1", "Tool3", "{}", "/tmp")

	// Only 1 should be spawned immediately
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, len(mock.getCalls()), "only 1 popup should be active")

	// Release first popup -> second should spawn
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 2
	}, 2*time.Second, 10*time.Millisecond)

	// Release second -> third should spawn
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 3
	}, 2*time.Second, 10*time.Millisecond)

	// Release third
	mock.gates <- struct{}{}

	// All 3 should have Tool1, Tool2, Tool3 in order (tool name is in env)
	calls := mock.getCalls()
	require.Len(t, calls, 3)
	assert.Equal(t, "Tool1", calls[0].Env["TOOL_NAME"])
	assert.Equal(t, "Tool2", calls[1].Env["TOOL_NAME"])
	assert.Equal(t, "Tool3", calls[2].Env["TOOL_NAME"])
}

func TestPopupQueue_DifferentWindowsConcurrent(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger)

	// Spawn popups for different windows
	p.SpawnToolPopup(context.Background(), "@1", "Tool1", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@2", "Tool2", "{}", "/tmp")

	// Both should spawn immediately (different windows)
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 2
	}, time.Second, 10*time.Millisecond)

	// Release both
	mock.gates <- struct{}{}
	mock.gates <- struct{}{}
}

func TestPopupGlobalLimit_BlocksBeyondMax(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestratorWithMax("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger, 2)

	// Spawn popups for 4 different windows
	p.SpawnToolPopup(context.Background(), "@1", "Tool1", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@2", "Tool2", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@3", "Tool3", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@4", "Tool4", "{}", "/tmp")

	// Only 2 should be spawned (global limit = 2)
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 2
	}, time.Second, 10*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 2, len(mock.getCalls()), "only 2 popups should be active globally")

	// Release one -> third should spawn
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 3
	}, 2*time.Second, 10*time.Millisecond)

	// Release another -> fourth should spawn
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 4
	}, 2*time.Second, 10*time.Millisecond)

	// Release remaining
	mock.gates <- struct{}{}
	mock.gates <- struct{}{}
}

func TestPopupGlobalLimit_DefaultIs3(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger)

	// Spawn 5 popups for different windows
	for i := 0; i < 5; i++ {
		window := fmt.Sprintf("@%d", i+1)
		p.SpawnToolPopup(context.Background(), window, fmt.Sprintf("Tool%d", i+1), "{}", "/tmp")
	}

	// Exactly 3 should spawn (default global limit)
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 3
	}, time.Second, 10*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 3, len(mock.getCalls()), "default limit should be 3")

	// Release all
	for i := 0; i < 5; i++ {
		mock.gates <- struct{}{}
		if i < 4 {
			// Wait for next popup to potentially spawn
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestPopupGlobalLimit_PerWindowQueueStillWorks(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestratorWithMax("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger, 2)

	// 2 popups for @1, 1 popup for @2
	p.SpawnToolPopup(context.Background(), "@1", "Tool1a", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@1", "Tool1b", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@2", "Tool2", "{}", "/tmp")

	// @1 first popup + @2 popup = 2 global slots used. Tool1b queued per-window.
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 2
	}, time.Second, 10*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 2, len(mock.getCalls()))

	calls := mock.getCalls()
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Env["TOOL_NAME"]
	}
	assert.Contains(t, names, "Tool1a")
	assert.Contains(t, names, "Tool2")

	// Release @1 first popup -> Tool1b drains from the per-window queue.
	// It needs to acquire a global semaphore slot (200ms drain sleep + acquire).
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 3
	}, 5*time.Second, 50*time.Millisecond)
	calls = mock.getCalls()
	assert.Equal(t, "Tool1b", calls[2].Env["TOOL_NAME"])

	// Release remaining (Tool1b + Tool2)
	mock.gates <- struct{}{}
	mock.gates <- struct{}{}
}

func TestPopupGlobalLimit_ContextCancel(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestratorWithMax("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger, 1)

	ctx, cancel := context.WithCancel(context.Background())

	// Fill the single global slot
	p.SpawnToolPopup(ctx, "@1", "Tool1", "{}", "/tmp")
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)

	// This goroutine will block on semaphore acquire
	p.SpawnToolPopup(ctx, "@2", "Tool2", "{}", "/tmp")
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, len(mock.getCalls()), "Tool2 should be blocked on semaphore")

	// Cancel context — Tool2's goroutine should exit without leaking
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Verify @2 is cleaned up (active flag cleared)
	p.mu.Lock()
	_, active2 := p.active["@2"]
	p.mu.Unlock()
	assert.False(t, active2, "window @2 should not be active after ctx cancel")

	// Release Tool1
	mock.gates <- struct{}{}
}

func TestPopupGlobalLimit_ZeroDefaultsToDefault(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestratorWithMax("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger, 0)

	// Should use defaultMaxConcurrentPopups (3), not deadlock
	p.SpawnToolPopup(context.Background(), "@1", "Tool1", "{}", "/tmp")
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)
	mock.gates <- struct{}{}
}

func TestPopupEnv_ContainsSocket(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger)

	p.SpawnToolPopup(context.Background(), "@1", "Bash", "{}", "/tmp")

	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)

	calls := mock.getCalls()
	assert.Equal(t, "lazyclaude", calls[0].Env["LAZYCLAUDE_TMUX_SOCKET"])

	mock.gates <- struct{}{}
}

func TestPopupEnv_EmptySocketOmitted(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "", os.TempDir(), mock, nil, logger)

	p.SpawnToolPopup(context.Background(), "@1", "Bash", "{}", "/tmp")

	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)

	calls := mock.getCalls()
	_, hasSocket := calls[0].Env["LAZYCLAUDE_TMUX_SOCKET"]
	assert.False(t, hasSocket, "empty socket should not be in env")

	mock.gates <- struct{}{}
}

func TestPopupQueue_QueueLength(t *testing.T) {
	mock := newMockPopupClient()
	logger := log.New(os.Stderr, "test: ", 0)
	p := NewPopupOrchestrator("lazyclaude", "lazyclaude", os.TempDir(), mock, nil, logger)

	assert.Equal(t, 0, p.QueueLen("@1"))

	p.SpawnToolPopup(context.Background(), "@1", "Tool1", "{}", "/tmp")
	require.Eventually(t, func() bool {
		return len(mock.getCalls()) == 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, p.QueueLen("@1"), "active popup, no queue")

	p.SpawnToolPopup(context.Background(), "@1", "Tool2", "{}", "/tmp")
	p.SpawnToolPopup(context.Background(), "@1", "Tool3", "{}", "/tmp")
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, p.QueueLen("@1"), "2 queued")

	// Release first
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return p.QueueLen("@1") == 1
	}, 2*time.Second, 10*time.Millisecond)

	// Release second
	mock.gates <- struct{}{}
	require.Eventually(t, func() bool {
		return p.QueueLen("@1") == 0
	}, 2*time.Second, 10*time.Millisecond)

	// Release third
	mock.gates <- struct{}{}
}
