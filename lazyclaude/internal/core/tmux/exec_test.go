package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClients(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []ClientInfo
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "single client",
			in:   "/dev/ttys001|||main|||200|||50|||1710000000",
			want: []ClientInfo{
				{Name: "/dev/ttys001", Session: "main", Width: 200, Height: 50, Activity: 1710000000},
			},
		},
		{
			name: "multiple clients",
			in:   "/dev/ttys001|||main|||200|||50|||100\n/dev/ttys002|||claude|||180|||40|||200",
			want: []ClientInfo{
				{Name: "/dev/ttys001", Session: "main", Width: 200, Height: 50, Activity: 100},
				{Name: "/dev/ttys002", Session: "claude", Width: 180, Height: 40, Activity: 200},
			},
		},
		{
			name: "malformed line",
			in:   "bad|||data",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseClients(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseWindows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []WindowInfo
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "single window",
			in:   "@1|||0|||lc-abc12345|||claude|||1",
			want: []WindowInfo{
				{ID: "@1", Index: 0, Name: "lc-abc12345", Session: "claude", Active: true},
			},
		},
		{
			name: "multiple windows",
			in:   "@1|||0|||lc-abc|||claude|||1\n@2|||1|||lc-def|||claude|||0",
			want: []WindowInfo{
				{ID: "@1", Index: 0, Name: "lc-abc", Session: "claude", Active: true},
				{ID: "@2", Index: 1, Name: "lc-def", Session: "claude", Active: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseWindows(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePanes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []PaneInfo
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "alive pane",
			in:   "%1|||@1|||12345|||0",
			want: []PaneInfo{
				{ID: "%1", Window: "@1", PID: 12345, Dead: false},
			},
		},
		{
			name: "dead pane",
			in:   "%2|||@1|||0|||1",
			want: []PaneInfo{
				{ID: "%2", Window: "@1", PID: 0, Dead: true},
			},
		},
		{
			name: "multiple panes",
			in:   "%1|||@1|||1001|||0\n%2|||@2|||1002|||0\n%3|||@2|||0|||1",
			want: []PaneInfo{
				{ID: "%1", Window: "@1", PID: 1001, Dead: false},
				{ID: "%2", Window: "@2", PID: 1002, Dead: false},
				{ID: "%3", Window: "@2", PID: 0, Dead: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parsePanes(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Verify ExecClient implements Client interface at compile time.
var _ Client = (*ExecClient)(nil)

func TestNewExecClient(t *testing.T) {
	t.Parallel()
	c := NewExecClient()
	require.NotNil(t, c)
	assert.Equal(t, "", c.Socket())
}

func TestNewExecClientWithSocket(t *testing.T) {
	t.Parallel()
	c := NewExecClientWithSocket("lazyclaude")
	require.NotNil(t, c)
	assert.Equal(t, "lazyclaude", c.Socket())
}

func TestPrependSocket(t *testing.T) {
	t.Parallel()

	t.Run("no socket", func(t *testing.T) {
		c := NewExecClient()
		args := c.prependSocket([]string{"list-sessions"})
		assert.Equal(t, []string{"-u", "list-sessions"}, args)
	})

	t.Run("with socket", func(t *testing.T) {
		c := NewExecClientWithSocket("lc")
		args := c.prependSocket([]string{"list-sessions"})
		assert.Equal(t, []string{"-u", "-L", "lc", "list-sessions"}, args)
	})
}