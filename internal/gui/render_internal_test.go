package gui

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"
)

func TestTruncateToWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		maxW   int
		expect string
	}{
		{"short ASCII", "hello", 10, "hello"},
		{"exact fit", "hello", 5, "hello"},
		{"truncate ASCII", "hello world", 5, "hello"},
		{"empty string", "", 5, ""},
		{"zero width", "hello", 0, ""},
		{"CJK truncate", "あいう", 4, "あい"},
		{"CJK exact", "あいう", 6, "あいう"},
		{"CJK odd boundary", "あいう", 5, "あい"},
		{"mixed ASCII CJK", "aあb", 3, "aあ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncateToWidth(tt.input, tt.maxW)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestPadRight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		targetW int
		wantW   int
	}{
		{"short needs padding", "hi", 10, 10},
		{"exact no padding", "hello", 5, 5},
		{"longer no padding", "hello world", 5, 11},
		{"CJK padding", "あ", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := padRight(tt.input, tt.targetW)
			gotW := runewidth.StringWidth(got)
			assert.Equal(t, tt.wantW, gotW)
		})
	}
}
