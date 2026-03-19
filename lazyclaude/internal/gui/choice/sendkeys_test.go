package choice_test

import (
	"context"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
	"github.com/KEMSHlM/lazyclaude/internal/gui/choice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendToPane_Accept(t *testing.T) {
	t.Parallel()
	mock := tmux.NewMockClient()
	mock.Captured["lazyclaude:@3"] = ` Do you want to create hello.txt?
 > 1. Yes
   2. Yes, allow all edits
   3. No`

	err := choice.SendToPane(context.Background(), mock, "@3", choice.Accept)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, mock.SentKeys["lazyclaude:@3"])
}

func TestSendToPane_Reject(t *testing.T) {
	t.Parallel()
	mock := tmux.NewMockClient()
	mock.Captured["lazyclaude:@5"] = ` 1. Yes
 2. Yes, allow all
 3. No`

	err := choice.SendToPane(context.Background(), mock, "@5", choice.Reject)
	require.NoError(t, err)
	assert.Equal(t, []string{"3"}, mock.SentKeys["lazyclaude:@5"])
}

func TestSendToPane_ClampTo2Options(t *testing.T) {
	t.Parallel()
	mock := tmux.NewMockClient()
	// Only 2 options: Reject(3) should be clamped to 2
	mock.Captured["lazyclaude:@7"] = ` 1. Yes
 2. No`

	err := choice.SendToPane(context.Background(), mock, "@7", choice.Reject)
	require.NoError(t, err)
	assert.Equal(t, []string{"2"}, mock.SentKeys["lazyclaude:@7"],
		"choice 3 should be clamped to maxOption 2")
}

func TestSendToPane_Cancel_NoSend(t *testing.T) {
	t.Parallel()
	mock := tmux.NewMockClient()

	err := choice.SendToPane(context.Background(), mock, "@3", choice.Cancel)
	require.NoError(t, err)
	assert.Empty(t, mock.SentKeys, "Cancel should not send any key")
}

func TestSendToPane_PrependsSessionName(t *testing.T) {
	t.Parallel()
	mock := tmux.NewMockClient()
	mock.Captured["lazyclaude:@1"] = ` 1. Yes
 2. No`

	err := choice.SendToPane(context.Background(), mock, "@1", choice.Accept)
	require.NoError(t, err)
	_, ok := mock.SentKeys["lazyclaude:@1"]
	assert.True(t, ok, "target should have lazyclaude: prefix")
}

func TestSendToPane_AlreadyHasSession(t *testing.T) {
	t.Parallel()
	mock := tmux.NewMockClient()
	mock.Captured["mysession:@2"] = ` 1. Yes
 2. No`

	err := choice.SendToPane(context.Background(), mock, "mysession:@2", choice.Accept)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, mock.SentKeys["mysession:@2"])
}
