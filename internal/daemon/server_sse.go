package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/any-context/lazyclaude/internal/core/model"
)

// handleSSE streams real-time notifications via Server-Sent Events.
// On connect it sends a full_sync event with all session state, then
// streams activity and tool_info events as they arrive from the broker.
func (s *DaemonServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial full_sync
	sessions := s.mgr.Sessions()
	infos := make([]SessionInfo, len(sessions))
	for i, sess := range sessions {
		infos[i] = sessionToInfo(sess)
	}
	syncEvent := NotificationEvent{
		ID:       s.nextEventID(),
		Type:     EventFullSync,
		Time:     time.Now(),
		Sessions: infos,
	}
	writeSSEEvent(w, s.log, syncEvent)
	flusher.Flush()

	// Subscribe to broker events
	sub := s.broker.Subscribe(64)
	defer sub.Cancel()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		case evt, ok := <-sub.Ch():
			if !ok {
				return
			}
			notif := s.brokerEventToNotification(evt)
			if notif == nil {
				continue
			}
			writeSSEEvent(w, s.log, *notif)
			flusher.Flush()
		}
	}
}

// brokerEventToNotification converts a model.Event into a NotificationEvent.
// Returns nil for events that should not be sent to SSE clients.
func (s *DaemonServer) brokerEventToNotification(evt model.Event) *NotificationEvent {
	switch {
	case evt.ActivityNotification != nil:
		an := evt.ActivityNotification
		return &NotificationEvent{
			ID:        s.nextEventID(),
			Type:      EventActivity,
			Time:      an.Timestamp,
			SessionID: windowToSessionHint(an.Window),
			Activity:  an.State,
			ToolName:  an.ToolName,
		}
	case evt.Notification != nil:
		n := evt.Notification
		return &NotificationEvent{
			ID:   s.nextEventID(),
			Type: EventToolInfo,
			Time: n.Timestamp,
			ToolNotification: &model.ToolNotification{
				ToolName:  n.ToolName,
				Input:     n.Input,
				CWD:       n.CWD,
				Window:    n.Window,
				Timestamp: n.Timestamp,
				MaxOption: n.MaxOption,
			},
		}
	case evt.StopNotification != nil:
		sn := evt.StopNotification
		state := model.ActivityIdle
		if sn.StopReason == "error" || sn.StopReason == "interrupt" {
			state = model.ActivityError
		}
		return &NotificationEvent{
			ID:        s.nextEventID(),
			Type:      EventActivity,
			Time:      sn.Timestamp,
			SessionID: windowToSessionHint(sn.Window),
			Activity:  state,
		}
	case evt.SessionStartNotification != nil:
		ssn := evt.SessionStartNotification
		return &NotificationEvent{
			ID:        s.nextEventID(),
			Type:      EventActivity,
			Time:      ssn.Timestamp,
			SessionID: windowToSessionHint(ssn.Window),
			Activity:  model.ActivityRunning,
		}
	case evt.PromptSubmitNotification != nil:
		psn := evt.PromptSubmitNotification
		return &NotificationEvent{
			ID:        s.nextEventID(),
			Type:      EventActivity,
			Time:      psn.Timestamp,
			SessionID: windowToSessionHint(psn.Window),
			Activity:  model.ActivityRunning,
		}
	default:
		return nil
	}
}

// windowToSessionHint extracts a session ID hint from a tmux window name.
// Window names follow the pattern "lc-<first8chars>", so we return just
// the 8-char prefix as a hint for client-side matching.
func windowToSessionHint(window string) string {
	if after, ok := strings.CutPrefix(window, "lc-"); ok && after != "" {
		return after
	}
	return window
}

func (s *DaemonServer) nextEventID() string {
	return fmt.Sprintf("%d", s.sseEventID.Add(1))
}

func writeSSEEvent(w http.ResponseWriter, logger *log.Logger, evt NotificationEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		logger.Printf("sse: marshal event: %v", err)
		return
	}
	fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", evt.ID, evt.Type, data)
}
