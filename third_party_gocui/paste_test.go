package gocui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendGocuiKeyToBuilder_Rune(t *testing.T) {
	var buf strings.Builder
	appendGocuiKeyToBuilder(&buf, GocuiEvent{Type: eventKey, Ch: 'a'})
	assert.Equal(t, "a", buf.String())
}

func TestAppendGocuiKeyToBuilder_Enter(t *testing.T) {
	var buf strings.Builder
	appendGocuiKeyToBuilder(&buf, GocuiEvent{Type: eventKey, Key: KeyEnter})
	assert.Equal(t, "\n", buf.String())
}

func TestAppendGocuiKeyToBuilder_Tab(t *testing.T) {
	var buf strings.Builder
	appendGocuiKeyToBuilder(&buf, GocuiEvent{Type: eventKey, Key: KeyTab})
	assert.Equal(t, "\t", buf.String())
}

func TestAppendGocuiKeyToBuilder_Space(t *testing.T) {
	var buf strings.Builder
	appendGocuiKeyToBuilder(&buf, GocuiEvent{Type: eventKey, Key: KeySpace})
	assert.Equal(t, " ", buf.String())
}

func TestAppendGocuiKeyToBuilder_Esc(t *testing.T) {
	var buf strings.Builder
	appendGocuiKeyToBuilder(&buf, GocuiEvent{Type: eventKey, Key: KeyEsc})
	assert.Equal(t, "\x1b", buf.String())
}

// TestFilterEscSequence_PasteMarkerDetected verifies that ESC[200~ followed
// by content and ESC[201~ produces a single eventPasteContent on gEvents.
func TestFilterEscSequence_PasteMarkerDetected(t *testing.T) {
	g := &Gui{
		gEvents:   make(chan GocuiEvent, 20),
		rawEvents: make(chan GocuiEvent, 256),
		stop:      make(chan struct{}),
	}

	// Feed the paste marker chars (after ESC) + content + end marker into rawEvents.
	go func() {
		// pasteStartMarker chars: [, 2, 0, 0, ~
		for _, ch := range "[200~" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
		// Content
		for _, ch := range "hello" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
		// End marker: ESC + [201~
		g.rawEvents <- GocuiEvent{Type: eventKey, Key: KeyEsc}
		for _, ch := range "[201~" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
	}()

	esc := GocuiEvent{Type: eventKey, Key: KeyEsc}
	g.filterEscSequence(esc)

	select {
	case ev := <-g.gEvents:
		assert.Equal(t, eventPasteContent, ev.Type)
		assert.Equal(t, "hello", ev.PasteText)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for eventPasteContent")
	}
}

// TestFilterEscSequence_NonPasteFlushes verifies that ESC followed by
// non-matching characters flushes all as normal events.
func TestFilterEscSequence_NonPasteFlushes(t *testing.T) {
	g := &Gui{
		gEvents:   make(chan GocuiEvent, 20),
		rawEvents: make(chan GocuiEvent, 256),
		stop:      make(chan struct{}),
	}

	// Send a non-matching char after ESC
	g.rawEvents <- GocuiEvent{Type: eventKey, Ch: 'x'}

	esc := GocuiEvent{Type: eventKey, Key: KeyEsc}
	g.filterEscSequence(esc)

	// Should get ESC and 'x' as normal events
	var events []GocuiEvent
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case ev := <-g.gEvents:
			events = append(events, ev)
			if len(events) >= 2 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}
done:
	require.Len(t, events, 2, "should have 2 events (ESC + x)")
	assert.Equal(t, Key(KeyEsc), events[0].Key)
	assert.Equal(t, 'x', events[1].Ch)
}

// TestFilterEscSequence_TimeoutFlushes verifies that ESC with no following
// events within escSeqTimeout is flushed as a standalone Esc.
func TestFilterEscSequence_TimeoutFlushes(t *testing.T) {
	g := &Gui{
		gEvents:   make(chan GocuiEvent, 20),
		rawEvents: make(chan GocuiEvent, 256),
		stop:      make(chan struct{}),
	}

	// Don't send anything to rawEvents — let the timeout fire.
	esc := GocuiEvent{Type: eventKey, Key: KeyEsc}
	g.filterEscSequence(esc)

	select {
	case ev := <-g.gEvents:
		assert.Equal(t, Key(KeyEsc), ev.Key, "standalone ESC should be forwarded")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for flushed ESC")
	}
}

// TestAccumulatePasteFromChannel_EndMarker verifies that paste content is
// accumulated until the end marker is detected.
func TestAccumulatePasteFromChannel_EndMarker(t *testing.T) {
	g := &Gui{
		rawEvents: make(chan GocuiEvent, 256),
		stop:      make(chan struct{}),
	}

	go func() {
		for _, ch := range "line1\n" {
			if ch == '\n' {
				g.rawEvents <- GocuiEvent{Type: eventKey, Key: KeyEnter}
			} else {
				g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
			}
		}
		for _, ch := range "line2" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
		// End marker
		g.rawEvents <- GocuiEvent{Type: eventKey, Key: KeyEsc}
		for _, ch := range "[201~" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
	}()

	text := g.accumulatePasteFromChannel()
	assert.Equal(t, "line1\nline2", text)
}

// TestAccumulatePasteFromChannel_NestedEsc verifies that an Esc inside
// paste content that doesn't form the end marker is treated as content.
func TestAccumulatePasteFromChannel_NestedEsc(t *testing.T) {
	g := &Gui{
		rawEvents: make(chan GocuiEvent, 256),
		stop:      make(chan struct{}),
	}

	go func() {
		for _, ch := range "before" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
		// Esc + non-matching char
		g.rawEvents <- GocuiEvent{Type: eventKey, Key: KeyEsc}
		g.rawEvents <- GocuiEvent{Type: eventKey, Ch: 'X'}
		for _, ch := range "after" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
		// Real end marker
		g.rawEvents <- GocuiEvent{Type: eventKey, Key: KeyEsc}
		for _, ch := range "[201~" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
	}()

	text := g.accumulatePasteFromChannel()
	assert.Contains(t, text, "before")
	assert.Contains(t, text, "after")
	assert.Contains(t, text, "\x1bX", "nested Esc+X should be preserved as content")
}

// TestAccumulatePasteFromChannel_Timeout verifies that the accumulator
// returns partial content on timeout.
func TestAccumulatePasteFromChannel_Timeout(t *testing.T) {
	g := &Gui{
		rawEvents: make(chan GocuiEvent, 256),
		stop:      make(chan struct{}),
	}

	go func() {
		for _, ch := range "partial" {
			g.rawEvents <- GocuiEvent{Type: eventKey, Ch: ch}
		}
		// Don't send end marker — let timeout fire.
	}()

	text := g.accumulatePasteFromChannel()
	assert.Equal(t, "partial", text)
}
