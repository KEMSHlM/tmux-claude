package gui_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Rect helpers
// ---------------------------------------------------------------------------

func TestRect_Width(t *testing.T) {
	t.Parallel()
	r := gui.Rect{X0: 0, Y0: 0, X1: 79, Y1: 23}
	assert.Equal(t, 79, r.Width())
}

func TestRect_Height(t *testing.T) {
	t.Parallel()
	r := gui.Rect{X0: 0, Y0: 0, X1: 79, Y1: 23}
	assert.Equal(t, 23, r.Height())
}

func TestRect_ZeroSize(t *testing.T) {
	t.Parallel()
	r := gui.Rect{X0: 5, Y0: 5, X1: 5, Y1: 5}
	assert.Equal(t, 0, r.Width())
	assert.Equal(t, 0, r.Height())
}

// ---------------------------------------------------------------------------
// ComputeLayout - main screen
// ---------------------------------------------------------------------------

func TestComputeLayout_NormalSize(t *testing.T) {
	t.Parallel()
	// Standard 80x24 terminal.
	l := gui.ComputeLayout(80, 24)

	// splitX = 80/3 = 26, which is >= 20 and < 80-10=70, so stays 26.
	// leftMidY = (24-2)*2/3 = 22*2/3 = 14
	require.False(t, l.Compact, "80-wide should not be compact")

	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 25, Y1: 14}, l.Sessions)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 15, X1: 25, Y1: 22}, l.Server)
	assert.Equal(t, gui.Rect{X0: 26, Y0: 0, X1: 79, Y1: 22}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 22, X1: 79, Y1: 24}, l.Options)
}

func TestComputeLayout_WideTerminal(t *testing.T) {
	t.Parallel()
	// 200x50 terminal.
	l := gui.ComputeLayout(200, 50)

	// splitX = 200/3 = 66, >= 20 and < 200-10=190, so stays 66.
	// leftMidY = (50-2)*2/3 = 48*2/3 = 32
	require.False(t, l.Compact)

	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 65, Y1: 32}, l.Sessions)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 33, X1: 65, Y1: 48}, l.Server)
	assert.Equal(t, gui.Rect{X0: 66, Y0: 0, X1: 199, Y1: 48}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 48, X1: 199, Y1: 50}, l.Options)
}

func TestComputeLayout_NarrowTerminal(t *testing.T) {
	t.Parallel()
	// 50x24 — below CompactThreshold (60), compact mode.
	l := gui.ComputeLayout(50, 24)

	require.True(t, l.Compact, "50-wide terminal should be compact")

	// splitX = 50/3 = 16, clamped to 20. 20 < 50-10=40, stays 20.
	// leftMidY = (24-2)*2/3 = 14
	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 19, Y1: 14}, l.Sessions)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 15, X1: 19, Y1: 22}, l.Server)
	assert.Equal(t, gui.Rect{X0: 20, Y0: 0, X1: 49, Y1: 22}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 22, X1: 49, Y1: 24}, l.Options)
}

func TestComputeLayout_VeryNarrow(t *testing.T) {
	t.Parallel()
	// 30x24 — extreme compact.
	l := gui.ComputeLayout(30, 24)

	require.True(t, l.Compact)

	// splitX = 30/3 = 10, clamped to 20. 20 >= 30-10=20, so clamped to 30/2=15.
	// leftMidY = (24-2)*2/3 = 14
	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 14, Y1: 14}, l.Sessions)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 15, X1: 14, Y1: 22}, l.Server)
	assert.Equal(t, gui.Rect{X0: 15, Y0: 0, X1: 29, Y1: 22}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 22, X1: 29, Y1: 24}, l.Options)
}

func TestComputeLayout_MinimumSize(t *testing.T) {
	t.Parallel()
	// 10x5 — very small terminal, should not panic.
	l := gui.ComputeLayout(10, 5)

	require.True(t, l.Compact)

	// splitX = 10/3 = 3, clamped to 20. 20 >= 10-10=0, so clamped to 10/2=5.
	// leftMidY = (5-2)*2/3 = 2
	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 4, Y1: 2}, l.Sessions)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 3, X1: 4, Y1: 3}, l.Server)
	assert.Equal(t, gui.Rect{X0: 5, Y0: 0, X1: 9, Y1: 3}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 3, X1: 9, Y1: 5}, l.Options)
}

func TestComputeLayout_SplitX_MinWidth(t *testing.T) {
	t.Parallel()
	// When width/3 < 20, splitX must be clamped to 20.
	// Use width=51 so 51/3=17 < 20, and 20 < 51-10=41, so stays 20.
	l := gui.ComputeLayout(51, 24)

	assert.Equal(t, 0, l.Sessions.X0)
	assert.Equal(t, 19, l.Sessions.X1, "splitX-1 should be 19 (splitX=20)")
	assert.Equal(t, 20, l.Main.X0, "main starts at splitX=20")
}

func TestComputeLayout_SplitX_TooLargeClamp(t *testing.T) {
	t.Parallel()
	// When splitX >= maxX-10, clamp to maxX/2.
	// Use width=25: splitX=25/3=8, clamped to 20; 20 >= 25-10=15, so clamped to 25/2=12.
	l := gui.ComputeLayout(25, 24)

	assert.Equal(t, 11, l.Sessions.X1, "splitX-1 should be 11 (splitX=12=25/2)")
	assert.Equal(t, 12, l.Main.X0)
}

func TestComputeLayout_OptionsBar(t *testing.T) {
	t.Parallel()
	// Options bar must always be the last two rows.
	l := gui.ComputeLayout(80, 24)

	assert.Equal(t, 22, l.Options.Y0, "options bar starts at maxY-2")
	assert.Equal(t, 24, l.Options.Y1, "options bar ends at maxY")
	assert.Equal(t, 0, l.Options.X0)
	assert.Equal(t, 79, l.Options.X1)
}

func TestComputeLayout_ServerPanel(t *testing.T) {
	t.Parallel()
	// Server panel must start just below the sessions panel.
	l := gui.ComputeLayout(80, 24)

	assert.Equal(t, l.Sessions.Y1+1, l.Server.Y0, "server starts one row below sessions")
	assert.Equal(t, l.Sessions.X0, l.Server.X0, "server shares left edge")
	assert.Equal(t, l.Sessions.X1, l.Server.X1, "server shares right edge")
}

func TestComputeLayout_SessionsAndMainShareTopEdge(t *testing.T) {
	t.Parallel()
	l := gui.ComputeLayout(80, 24)

	assert.Equal(t, l.Sessions.Y0, l.Main.Y0, "sessions and main both start at row 0")
}

// ---------------------------------------------------------------------------
// ComputeFullScreenLayout
// ---------------------------------------------------------------------------

func TestComputeFullScreenLayout(t *testing.T) {
	t.Parallel()
	// Main takes full width, status bar sits at bottom two rows.
	l := gui.ComputeFullScreenLayout(80, 24)

	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 79, Y1: 22}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 22, X1: 79, Y1: 24}, l.Options)
}

func TestComputeFullScreenLayout_Wide(t *testing.T) {
	t.Parallel()
	l := gui.ComputeFullScreenLayout(200, 50)

	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 199, Y1: 48}, l.Main)
	assert.Equal(t, gui.Rect{X0: 0, Y0: 48, X1: 199, Y1: 50}, l.Options)
}

// ---------------------------------------------------------------------------
// ComputePopupLayout
// ---------------------------------------------------------------------------

func TestComputePopupLayout(t *testing.T) {
	t.Parallel()
	// Content fills all but last three rows; actions bar at bottom two rows.
	l := gui.ComputePopupLayout(80, 24)

	assert.Equal(t, gui.Rect{X0: 0, Y0: 0, X1: 79, Y1: 21}, l.Main,
		"content ends at maxY-3")
	assert.Equal(t, gui.Rect{X0: 0, Y0: 22, X1: 79, Y1: 24}, l.Options,
		"actions bar at maxY-2 to maxY")
}

func TestComputePopupLayout_NoOverlap(t *testing.T) {
	t.Parallel()
	l := gui.ComputePopupLayout(80, 24)

	// Content and actions must not overlap; actions starts immediately after the
	// gap row that separates them (content ends at maxY-3, actions starts at maxY-2).
	assert.Equal(t, l.Main.Y1+1, l.Options.Y0,
		"actions bar starts immediately after content (no overlap)")
}

// ---------------------------------------------------------------------------
// Existing tab / bar tests (unchanged)
