package presentation

import (
	"testing"
)

func TestClassifyLogLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want LogCategory
	}{
		// Error (red): prefixes
		{name: "server error", line: "2024/04/13 10:00:00 server error: bind failed", want: LogCatError},
		{name: "ws accept", line: "2024/04/13 10:00:00 ws accept: connection refused", want: LogCatError},
		{name: "local resolve failed", line: "2024/04/13 10:00:00 local resolve failed: dial tcp", want: LogCatError},

		// Error (red): notify sub-patterns
		{name: "notify window not found", line: "2024/04/13 10:00:00 notify: window not found for pid=1234 type=tool tool=bash", want: LogCatError},
		{name: "notify DROPPED", line: "2024/04/13 10:00:00 notify: DROPPED — empty toolName for window @1 pid=1234", want: LogCatError},
		{name: "notify encode error", line: "2024/04/13 10:00:00 notify: encode error: EOF", want: LogCatError},
		{name: "notify encode response", line: "2024/04/13 10:00:00 notify: encode response: EOF", want: LogCatError},

		// Error (red): ide_connected invalid
		{name: "ide invalid params", line: "2024/04/13 10:00:00 ide_connected: invalid params: bad json", want: LogCatError},
		{name: "ide invalid pid", line: "2024/04/13 10:00:00 ide_connected: invalid pid: 0", want: LogCatError},

		// Warning (yellow)
		{name: "warning prefix", line: "2024/04/13 10:00:00 warning: write port file: permission denied", want: LogCatWarn},
		{name: "not found using last pending", line: "2024/04/13 10:00:00 notify: pid=1234 not found, using last pending window @1", want: LogCatWarn},

		// Debug (dim gray)
		{name: "ws read", line: "2024/04/13 10:00:00 ws read conn123: EOF", want: LogCatDebug},
		{name: "ws parse", line: "2024/04/13 10:00:00 ws parse conn123: invalid json", want: LogCatDebug},
		{name: "ws marshal", line: "2024/04/13 10:00:00 ws marshal conn123: unsupported type", want: LogCatDebug},
		{name: "ws write", line: "2024/04/13 10:00:00 ws write conn123: broken pipe", want: LogCatDebug},
		{name: "cleaned stale", line: "2024/04/13 10:00:00 cleaned 2 stale lock file(s)", want: LogCatDebug},

		// Notify (cyan)
		{name: "notify event", line: "2024/04/13 10:00:00 notify: type=tool pid=1234 window=@1 tool=bash", want: LogCatNotify},
		{name: "notify delivered", line: "2024/04/13 10:00:00 notify: delivered via broker for window @1", want: LogCatNotify},

		// Lifecycle (green)
		{name: "session-start", line: "2024/04/13 10:00:00 session-start: pid=1234 window=@1", want: LogCatLifecycle},
		{name: "stop", line: "2024/04/13 10:00:00 stop: pid=1234 window=@1 reason=end_turn", want: LogCatLifecycle},
		{name: "prompt-submit", line: "2024/04/13 10:00:00 prompt-submit: pid=1234 window=@1", want: LogCatLifecycle},
		{name: "listening", line: "2024/04/13 10:00:00 listening on 127.0.0.1:8080", want: LogCatLifecycle},

		// Connection (blue)
		{name: "ws connected", line: "2024/04/13 10:00:00 ws connected: abc123", want: LogCatConnection},
		{name: "ws disconnected", line: "2024/04/13 10:00:00 ws disconnected: abc123", want: LogCatConnection},
		{name: "ide_connected normal", line: "2024/04/13 10:00:00 ide_connected: pid=1001 window=@1", want: LogCatConnection},

		// DiffMsg (magenta)
		{name: "openDiff", line: "2024/04/13 10:00:00 openDiff: window=@1 file=/home/user/main.go", want: LogCatDiffMsg},
		{name: "msg/create", line: "2024/04/13 10:00:00 msg/create: session created", want: LogCatDiffMsg},
		{name: "msg/send", line: "2024/04/13 10:00:00 msg/send: sent to pane @1", want: LogCatDiffMsg},
		{name: "msg/sessions", line: "2024/04/13 10:00:00 msg/sessions: encode: EOF", want: LogCatDiffMsg},

		// Info (default)
		{name: "short line", line: "short", want: LogCatInfo},
		{name: "empty line", line: "", want: LogCatInfo},

		// False positive guards: "invalid" and "dropped" must NOT leak
		{name: "no false positive: invalid in openDiff path", line: "2024/04/13 10:00:00 openDiff: window=@1 file=/tmp/invalid.go", want: LogCatDiffMsg},
		{name: "no false positive: invalid in msg payload", line: "2024/04/13 10:00:00 msg/create: invalid session id", want: LogCatDiffMsg},
		{name: "no false positive: dropped in msg payload", line: "2024/04/13 10:00:00 msg/send: packet dropped due to timeout", want: LogCatDiffMsg},
		{name: "no false positive: warn in file path", line: "2024/04/13 10:00:00 openDiff: window=@0 file=/home/user/warnings.go", want: LogCatDiffMsg},
		{name: "no false positive: error in msg payload", line: "2024/04/13 10:00:00 msg/create: nil result with no error", want: LogCatDiffMsg},

		// Error vs Warn precedence at overlap
		{name: "error wins over warn: no window found with using last pending", line: "2024/04/13 10:00:00 notify: no window found, using last pending fallback", want: LogCatError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyLogLine(tt.line)
			if got != tt.want {
				t.Errorf("ClassifyLogLine(%q) = %d, want %d", tt.line, got, tt.want)
			}
		})
	}
}

func TestColorizeLogLine(t *testing.T) {
	ts := "2024/04/13 10:00:00"

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "error: dim timestamp + red message",
			line: ts + " server error: bind",
			want: FgDimGray + ts + Reset + fgLogError + " server error: bind" + Reset,
		},
		{
			name: "warning: dim timestamp + yellow message",
			line: ts + " warning: port file",
			want: FgDimGray + ts + Reset + fgLogWarn + " warning: port file" + Reset,
		},
		{
			name: "notify: dim timestamp + cyan message",
			line: ts + " notify: type=tool",
			want: FgDimGray + ts + Reset + fgLogNotify + " notify: type=tool" + Reset,
		},
		{
			name: "lifecycle: dim timestamp + green message",
			line: ts + " session-start: pid=1",
			want: FgDimGray + ts + Reset + fgLogLifecycle + " session-start: pid=1" + Reset,
		},
		{
			name: "connection: dim timestamp + blue message",
			line: ts + " ws connected: abc",
			want: FgDimGray + ts + Reset + fgLogConnection + " ws connected: abc" + Reset,
		},
		{
			name: "diffmsg: dim timestamp + magenta message",
			line: ts + " msg/create: ok",
			want: FgDimGray + ts + Reset + fgLogDiffMsg + " msg/create: ok" + Reset,
		},
		{
			name: "debug: dim timestamp + dim message",
			line: ts + " ws read conn1: EOF",
			want: FgDimGray + ts + Reset + fgLogDebug + " ws read conn1: EOF" + Reset,
		},
		{
			name: "info: dim timestamp only, message uncolored",
			line: ts + " something unknown",
			want: FgDimGray + ts + Reset + " something unknown",
		},
		{
			name: "short line: no timestamp split",
			line: "short",
			want: "short",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorizeLogLine(tt.line)
			if got != tt.want {
				t.Errorf("ColorizeLogLine(%q)\n  got  %q\n  want %q", tt.line, got, tt.want)
			}
		})
	}
}
