package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/any-context/lazyclaude/internal/core/model"
)

func TestSSE_FullSyncOnConnect(t *testing.T) {
	_, ts, _ := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/notifications", nil)
	req.Header.Set(AuthHeader, testToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %s", ct)
	}

	// Read first SSE event (full_sync)
	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
		if line == "" && eventType != "" {
			break
		}
	}

	if eventType != string(EventFullSync) {
		t.Fatalf("want event type full_sync, got %s", eventType)
	}

	var evt NotificationEvent
	if err := json.Unmarshal([]byte(data), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.Type != EventFullSync {
		t.Errorf("want full_sync, got %s", evt.Type)
	}
}

func TestSSE_ActivityEvent(t *testing.T) {
	srv, ts, _ := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/notifications", nil)
	req.Header.Set(AuthHeader, testToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Publish an activity event after a small delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		srv.broker.Publish(model.Event{
			ActivityNotification: &model.ActivityNotification{
				Window:    "lc-abc12345",
				State:     model.ActivityRunning,
				ToolName:  "Bash",
				Timestamp: time.Now(),
			},
		})
	}()

	// Read events until we get the activity event
	scanner := bufio.NewScanner(resp.Body)
	foundActivity := false
	eventCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: activity") {
			foundActivity = true
			break
		}
		eventCount++
		if eventCount > 20 {
			break
		}
	}

	if !foundActivity {
		t.Error("did not receive activity event")
	}
}

func TestSSE_Unauthorized(t *testing.T) {
	_, ts, _ := newTestServer(t)

	resp, err := http.Get(ts.URL + "/notifications")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestBrokerEventToNotification(t *testing.T) {
	srv, _, _ := newTestServer(t)
	now := time.Now()

	tests := []struct {
		name     string
		event    model.Event
		wantType NotificationEventType
		wantNil  bool
	}{
		{
			name: "activity",
			event: model.Event{ActivityNotification: &model.ActivityNotification{
				Window: "lc-abc", State: model.ActivityRunning, Timestamp: now,
			}},
			wantType: EventActivity,
		},
		{
			name: "tool_info",
			event: model.Event{Notification: &model.ToolNotification{
				ToolName: "Bash", Window: "lc-abc", Timestamp: now,
			}},
			wantType: EventToolInfo,
		},
		{
			name: "stop",
			event: model.Event{StopNotification: &model.StopNotification{
				Window: "lc-abc", StopReason: "end_turn", Timestamp: now,
			}},
			wantType: EventActivity,
		},
		{
			name: "session_start",
			event: model.Event{SessionStartNotification: &model.SessionStartNotification{
				Window: "lc-abc", Timestamp: now,
			}},
			wantType: EventActivity,
		},
		{
			name: "prompt_submit",
			event: model.Event{PromptSubmitNotification: &model.PromptSubmitNotification{
				Window: "lc-abc", Timestamp: now,
			}},
			wantType: EventActivity,
		},
		{
			name:    "empty event",
			event:   model.Event{},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := srv.brokerEventToNotification(tt.event)
			if tt.wantNil {
				if result != nil {
					t.Fatal("expected nil")
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil")
			}
			if result.Type != tt.wantType {
				t.Errorf("want type %s, got %s", tt.wantType, result.Type)
			}
		})
	}
}

func TestWindowToSessionHint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"lc-abc12345", "abc12345"},
		{"@42", "@42"},
		{"lc-", "lc-"},
		{"short", "short"},
	}

	for _, tt := range tests {
		got := windowToSessionHint(tt.input)
		if got != tt.want {
			t.Errorf("windowToSessionHint(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
