package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/role/permissions"
)

type reactionRoleAuthority struct {
	allowed bool
	err     error
	check   func(uuid.UUID, uuid.UUID, uuid.UUID, string)
}

func (r *reactionRoleAuthority) HasSpacePermission(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return r.allowed, r.err
}

func (r *reactionRoleAuthority) HasChatPermission(_ context.Context, spaceID, profileID, chatID uuid.UUID, permission string) (bool, error) {
	if r.check != nil {
		r.check(spaceID, profileID, chatID, permission)
	}
	return r.allowed, r.err
}

// The RPC must decide before either the reaction or its durable event is written.
func TestMessagingReactionPermission(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	applyBaseMessagingMigrations(t, ctx, pool)
	chatID, spaceID, author, viewer := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chatID, author, viewer)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)
	messageID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content) VALUES ($1,$2,'group',$3,'react to me')`, messageID, chatID, author)
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		role *reactionRoleAuthority
		want codes.Code
	}{
		{"denied", &reactionRoleAuthority{}, codes.PermissionDenied},
		{"missing", nil, codes.Unavailable},
		{"unavailable", &reactionRoleAuthority{err: errors.New("private upstream detail")}, codes.Unavailable},
		{"allowed", &reactionRoleAuthority{allowed: true}, codes.OK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			if tc.role != nil {
				tc.role.check = func(gotSpace, gotProfile, gotChat uuid.UUID, permission string) {
					calls++
					require.Equal(t, spaceID, gotSpace)
					require.Equal(t, viewer, gotProfile)
					require.Equal(t, chatID, gotChat)
					require.Equal(t, permissions.TextChatAddReactions, permission)
				}
			}
			wire := messagingWire{}
			if tc.role != nil {
				wire.RolePermissions = tc.role
			}
			client, cleanup := startMessagingServerWired(t, pool, wire)
			defer cleanup()
			_, err := client.AddReaction(withProfileCtx(ctx, uuid.New(), viewer), &messagingv1.AddReactionRequest{MessageId: messageID.String(), Emoji: "👍"})
			require.Equal(t, tc.want, status.Code(err))
			if err != nil {
				require.NotContains(t, err.Error(), "private upstream detail")
			}
			if tc.role != nil {
				require.Equal(t, 1, calls)
			}
			wantRows := 0
			if tc.want == codes.OK {
				wantRows = 1
			}
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reactions WHERE message_id=$1`, messageID).Scan(&count))
			require.Equal(t, wantRows, count)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE message_id=$1 AND subject='message.reaction_added'`, messageID).Scan(&count))
			require.Equal(t, wantRows, count)
		})
	}

	// Losing ADD_REACTIONS must not trap a member's existing reaction.
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{RolePermissions: &reactionRoleAuthority{}})
	defer cleanup()
	_, err = client.RemoveReaction(withProfileCtx(ctx, uuid.New(), viewer), &messagingv1.RemoveReactionRequest{MessageId: messageID.String(), Emoji: "👍"})
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reactions WHERE message_id=$1`, messageID).Scan(&count))
	require.Zero(t, count)
}
