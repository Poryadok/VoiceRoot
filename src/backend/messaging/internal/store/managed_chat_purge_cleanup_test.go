package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

func TestManagedSpaceChatPurgeRemovesRelatedRowsAtomicallyAndPreservesControl(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	space, deletion, chat, control := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	cutoff := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	for _, chatID := range []uuid.UUID{chat, control} {
		message, reply, profile := uuid.New(), uuid.New(), uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,created_at,thread_parent_id) VALUES($1,$3,'dm',$4,'root',$5,NULL),($2,$3,'dm',$4,'thread payload',$5,$1)`, message, reply, chatID, profile, cutoff.Add(-time.Hour))
		require.NoError(t, err)
		for _, sql := range []string{
			`INSERT INTO reactions(message_id,profile_id,emoji) VALUES($1,$2,'test')`,
			`INSERT INTO message_hides(message_id,profile_id) VALUES($1,$2)`,
		} {
			_, err = pool.Exec(ctx, sql, message, profile)
			require.NoError(t, err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO pins(chat_id,message_id,pinned_by) VALUES($1,$2,$3)`, chatID, message, profile)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO read_positions(chat_id,profile_id,last_read_message_id) VALUES($1,$2,$3)`, chatID, profile, message)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO read_receipts(chat_id,profile_id,last_read_message_id,last_delivered_message_id) VALUES($1,$2,$3,$3)`, chatID, profile, message)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO scheduled_messages(id,chat_id,sender_profile_id,payload,schedule_kind,scheduled_at,created_at) VALUES($1,$2,$3,'{"content":"future secret"}','at',clock_timestamp()+interval '1 day',$4)`, uuid.New(), chatID, profile, cutoff.Add(-time.Hour))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO game_message_revisions(chat_id,message_id,revision,operation,operation_id,compact_jws,content_sha256) VALUES($1,$2,1,'create',$3,'private signed content',$4)`, chatID, message, uuid.New(), strings.Repeat("a", 64))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO game_message_operation_receipts(application_id,environment_id,operation_id,compact_jws,chat_id,message_id,result_content) VALUES($1,$2,$3,'private signed content',$4,$5,'private receipt content')`, uuid.New(), uuid.New(), uuid.New(), chatID, message)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO game_message_tombstone_actions(action_id,application_id,environment_id,chat_id,message_id,reason_class,revision,compact_jws) VALUES($1,$2,$3,$4,$5,'system_retention',2,'private signed content')`, uuid.New(), uuid.New(), uuid.New(), chatID, message)
		require.NoError(t, err)
	}
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ItemCount: 1, ManifestSha256: messagingChatManifestSHA(space, deletion, 1, []uuid.UUID{chat})}
	_, err := s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{SpaceID: space, DeletionOperationID: deletion, ScheduleGeneration: 1, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, ItemIds: []string{chat.String()}, PageSha256: make([]byte, 32)}, SealsManifest: true, RequestBytes: []byte("page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt")})
	require.NoError(t, err)
	for _, fence := range []struct {
		gen   uint64
		state commonv1.LifecycleFenceState
	}{{1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN}, {2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED}} {
		_, err = s.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{SpaceID: space, DeletionOperationID: deletion, Generation: fence.gen, State: fence.state, Manifest: manifest, RequestBytes: []byte(fmt.Sprint(fence.gen)), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt")})
		require.NoError(t, err)
	}
	child := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", space, deletion, chat)))
	parent := &messagingv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: deletion.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, Manifest: manifest, PurgeDecidedAt: timestamppb.New(cutoff)}}
	hash, err := principal.RequestHash(parent)
	require.NoError(t, err)
	authorized := principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging", RPC: messagingv1.MessagingService_PurgeSpace_FullMethodName, RequestID: deletion.String(), RequestHash: hash})
	authorized, err = SpacePurgeContext(authorized, parent)
	require.NoError(t, err)
	requestHash := sha256.Sum256([]byte("child request"))
	_, err = s.StartManagedChatPurge(authorized, child, chat, cutoff, requestHash[:])
	require.NoError(t, err)
	_, err = s.CompleteManagedChatPurge(ctx, child, make([]byte, 32), make([]byte, 32))
	require.Error(t, err, "public child ID cannot delete frozen content")
	completed, err := s.CompleteManagedChatPurge(authorized, child, make([]byte, 32), make([]byte, 32))
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", completed.State)
	for _, table := range []string{"messages", "read_positions", "read_receipts", "scheduled_messages", "game_message_revisions", "game_message_operation_receipts", "game_message_tombstone_actions"} {
		var target, foreign int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE chat_id=$1", chat).Scan(&target))
		require.Zero(t, target, table)
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE chat_id=$1", control).Scan(&foreign))
		require.Positive(t, foreign, table+" control must survive")
	}
	for _, table := range []string{"reactions", "message_hides", "pins"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Equal(t, 1, count, table+" must contain only the control row")
	}
}

func TestManagedChatPurgeExactNanosecondCutoffReplay(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	cutoff := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond).Add(123 * time.Nanosecond)
	request := &messagingv1.PurgeManagedChatContentRequest{OperationId: uuid.NewString(), ChatId: uuid.NewString(), PurgeAfter: timestamppb.New(cutoff)}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	digest, err := hex.DecodeString(strings.TrimPrefix(hash, "sha256:"))
	require.NoError(t, err)
	first, err := s.StartManagedChatPurge(ctx, uuid.MustParse(request.OperationId), uuid.MustParse(request.ChatId), cutoff, digest)
	require.NoError(t, err)
	replay, err := s.StartManagedChatPurge(ctx, first.OperationID, first.ChatID, cutoff, digest)
	require.NoError(t, err, "PostgreSQL timestamp precision must not break an exact signed retry")
	require.Equal(t, first, replay)
	request.PurgeAfter = timestamppb.New(cutoff.Add(time.Nanosecond))
	changedHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	changed, err := hex.DecodeString(strings.TrimPrefix(changedHash, "sha256:"))
	require.NoError(t, err)
	_, err = s.StartManagedChatPurge(ctx, first.OperationID, first.ChatID, cutoff.Add(time.Nanosecond), changed)
	require.ErrorIs(t, err, ErrManagedChatPurgeOperationConflict)
}
