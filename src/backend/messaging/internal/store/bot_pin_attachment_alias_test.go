package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type blockedBotPinScopeLoader struct {
	inner   BotPinChatScopeLoader
	entered chan struct{}
	release chan struct{}
}

func (b blockedBotPinScopeLoader) LoadBotPinChatScope(ctx context.Context, chat, space uuid.UUID) (*chatv1.GetSpaceChatScopeAuthorityResponse, error) {
	actual, err := b.inner.LoadBotPinChatScope(ctx, chat, space)
	close(b.entered)
	select {
	case <-b.release:
		return actual, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestBotPinAttachmentUsesFrozenBodyAfterSourceWait(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL authenticated-body ownership after source wait")
	}
	for _, mode := range []string{"repair stale incarnation", "remove caller response"} {
		t.Run(mode, func(t *testing.T) {
			s, request, message := newBotPinRemovalFixture(t)
			ctx := t.Context()
			actualIncarnation := request.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation
			request.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation = uuid.NewString()
			barrier := blockedBotPinScopeLoader{inner: s.BotPinChatScope, entered: make(chan struct{}), release: make(chan struct{})}
			s.BotPinChatScope = barrier
			type result struct {
				response *messagingv1.ApplyBotChatPinRemovalResponse
				err      error
				panicked any
			}
			done := make(chan result, 1)
			go func() {
				out := result{}
				defer func() { out.panicked = recover(); done <- out }()
				out.response, out.err = s.ApplyBotChatPinRemoval(ctx, request)
			}()
			select {
			case <-barrier.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("cleanup did not reach actual current-source read")
			}
			// The channel establishes a race-free mutation after body binding and
			// before source qualification. Neither change is authenticated bytes.
			if mode == "repair stale incarnation" {
				request.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation = actualIncarnation
			} else {
				request.Attachments[0].CurrentScope.Response = nil
			}
			close(barrier.release)
			var out result
			select {
			case out = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("cleanup failed to finish after source read")
			}
			require.Nil(t, out.panicked, "caller mutation must not reach a later scope dereference")
			require.ErrorIs(t, out.err, ErrBotPinRemovalConflict, "frozen stale incarnation must remain the checked source")
			require.Nil(t, out.response)
			for _, table := range []string{"bot_pin_removal_decisions", "bot_pin_removal_revisions", "bot_pin_removal_receipts", "bot_pin_removal_attachment_coverage"} {
				var count int
				require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
				require.Zero(t, count, table)
			}
			pins, err := s.ListPins(ctx, uuid.MustParse(request.ChatId))
			require.NoError(t, err)
			require.Len(t, pins, 1)
			require.Equal(t, message, pins[0].MessageID)
		})
	}
}
