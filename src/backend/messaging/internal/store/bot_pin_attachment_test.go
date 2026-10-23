package store

import (
	"crypto/sha256"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func unqualifiedPinAttachment() *messagingv1.BotChatPinRemovalAttachment {
	return &messagingv1.BotChatPinRemovalAttachment{AttachmentId: uuid.NewString(), OriginalScopeEvidence: &messagingv1.BotChatPinRemovalAttachment_OriginalScopeUnqualified{OriginalScopeUnqualified: true}}
}

// These are controlled Chat-owned facts in the disposable PG fixture, read via
// the production SQL datasource. They are not a protected Chat transport proof.
func prepareBotPinCurrentScope(t *testing.T, s *PinsStore, request *messagingv1.ApplyBotChatPinRemovalRequest) {
	t.Helper()
	ctx := t.Context()
	_, err := s.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS chats(id UUID PRIMARY KEY,space_id UUID,scope_incarnation UUID,type TEXT,managed_by_application_id UUID);
CREATE TABLE IF NOT EXISTS chat_scope_authority_heads(chat_id UUID PRIMARY KEY,space_id UUID,scope_incarnation UUID,revision BIGINT,present BOOLEAN);
CREATE TABLE IF NOT EXISTS chat_space_lifecycle_fences(space_id UUID PRIMARY KEY,generation BIGINT,state TEXT);
CREATE TABLE IF NOT EXISTS chat_match_squad_operations(chat_id UUID);`)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO chats(id,space_id,scope_incarnation,type) VALUES($1,$2,$3,'group') ON CONFLICT DO NOTHING`, request.ChatId, request.SpaceId, uuid.New())
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO chat_scope_authority_heads(chat_id,space_id,scope_incarnation,revision,present) SELECT id,space_id,scope_incarnation,1,true FROM chats WHERE id=$1 ON CONFLICT DO NOTHING`, request.ChatId)
	require.NoError(t, err)
	loader := &SQLChatThreadPolicy{Pool: s.Pool}
	s.BotPinChatScope = loader
	scope, err := loader.LoadBotPinChatScope(ctx, uuid.MustParse(request.ChatId), uuid.MustParse(request.SpaceId))
	require.NoError(t, err)
	q := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: request.ChatId, ExpectedSpaceId: request.SpaceId, OperationId: uuid.NewString()}
	scope.OperationId = q.OperationId
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	digest := sha256.Sum256(raw)
	observation := &messagingv1.BotRemovalChatScopeObservation{Request: q, Response: &chatv1.GetBotChatScopeAuthorityResponse{Scope: scope, RequestSha256: digest[:]}}
	if len(request.Attachments) == 0 {
		request.Attachments = []*messagingv1.BotChatPinRemovalAttachment{unqualifiedPinAttachment()}
	}
	for _, attachment := range request.Attachments {
		attachment.CurrentScope = proto.Clone(observation).(*messagingv1.BotRemovalChatScopeObservation)
	}
}

func nextBotPinDecision(request *messagingv1.ApplyBotChatPinRemovalRequest, purpose string) *messagingv1.ApplyBotChatPinRemovalRequest {
	next := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
	next.OperationId, next.DecisionId, next.Purpose = uuid.NewString(), uuid.NewString(), purpose
	next.DecisionSha256 = make([]byte, 32)
	next.FrozenScopeSha256 = make([]byte, 32)
	return next
}

func TestBotPinAttachmentMixedBatchHasOneEffectAndDirectOrigins(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL mixed attachment coverage")
	}
	s, first, message := newBotPinRemovalFixture(t)
	ctx := t.Context()
	first.Purpose = "space_uninstall"
	origin, err := s.ApplyBotChatPinRemoval(ctx, first)
	require.NoError(t, err)
	chat := uuid.MustParse(first.ChatId)
	require.NoError(t, s.UpsertPin(ctx, chat, message, uuid.New()))
	batch := nextBotPinDecision(first, "global_delete")
	batch.Attachments = append(batch.Attachments, unqualifiedPinAttachment())
	sort.Slice(batch.Attachments, func(i, j int) bool { return batch.Attachments[i].AttachmentId < batch.Attachments[j].AttachmentId })
	prepareBotPinCurrentScope(t, s, batch)
	response, err := s.ApplyBotChatPinRemoval(ctx, batch)
	require.NoError(t, err)
	require.EqualValues(t, 1, response.Receipt.RemovedPinCount)
	require.EqualValues(t, 2, response.Receipt.ExecutionCleanupRevision)
	for _, target := range response.Receipt.Targets {
		if target.AttachmentId == first.Attachments[0].AttachmentId {
			require.Equal(t, first.OperationId, target.CoveringOperationId)
			require.Equal(t, origin.Receipt.ReceiptId, target.CoveringReceiptId)
			require.EqualValues(t, 1, target.CleanupRevision)
		} else {
			require.Empty(t, target.CoveringOperationId)
			require.EqualValues(t, 2, target.CleanupRevision)
		}
	}
	require.NoError(t, s.UpsertPin(ctx, chat, message, uuid.New()))
	// Both targets are completed. No current Chat datasource may be required.
	s.BotPinChatScope = nil
	adopted, err := s.ApplyBotChatPinRemoval(ctx, nextBotPinDecision(batch, "global_delete"))
	require.NoError(t, err)
	require.Zero(t, adopted.Receipt.RemovedPinCount)
	require.Zero(t, adopted.Receipt.ExecutionCleanupRevision)
	for _, target := range adopted.Receipt.Targets {
		if target.AttachmentId == first.Attachments[0].AttachmentId {
			require.Equal(t, first.OperationId, target.CoveringOperationId)
		} else {
			require.Equal(t, batch.OperationId, target.CoveringOperationId)
		}
	}
	pins, err := s.ListPins(ctx, chat)
	require.NoError(t, err)
	require.Len(t, pins, 1)
	var revision int64
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT revision FROM bot_pin_removal_revisions WHERE chat_id=$1 AND actor_profile_id=$2`, first.ChatId, first.ActorProfileId).Scan(&revision))
	require.EqualValues(t, 2, revision, "mixed batch has one revision; all-covered batch has none")
}

func TestBotPinAttachmentConcurrentGlobalAndUninstallHaveOneWinner(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL reverse cleanup race")
	}
	s, global, message := newBotPinRemovalFixture(t)
	uninstall := nextBotPinDecision(global, "space_uninstall")
	ctx := t.Context()
	type result struct {
		response *messagingv1.ApplyBotChatPinRemovalResponse
		err      error
	}
	start, done := make(chan struct{}), make(chan result, 2)
	for _, request := range []*messagingv1.ApplyBotChatPinRemovalRequest{global, uninstall} {
		go func() { <-start; r, e := s.ApplyBotChatPinRemoval(ctx, request); done <- result{r, e} }()
	}
	close(start)
	var effects uint64
	for range 2 {
		r := <-done
		require.NoError(t, r.err)
		effects += r.response.Receipt.ExecutionCleanupRevision
	}
	require.EqualValues(t, 1, effects)
	chat := uuid.MustParse(global.ChatId)
	require.NoError(t, s.UpsertPin(ctx, chat, message, uuid.New()))
	s.BotPinChatScope = nil
	retry, err := s.ApplyBotChatPinRemoval(ctx, nextBotPinDecision(global, "space_uninstall"))
	require.NoError(t, err)
	require.Zero(t, retry.Receipt.RemovedPinCount)
	pins, err := s.ListPins(ctx, chat)
	require.NoError(t, err)
	require.Len(t, pins, 1)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_attachment_coverage`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestBotPinAttachmentRejectsUncoveredSourceConflictBeforeEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL all-target source validation")
	}
	s, request, message := newBotPinRemovalFixture(t)
	ctx := t.Context()
	request.Attachments = append(request.Attachments, unqualifiedPinAttachment())
	prepareBotPinCurrentScope(t, s, request)
	request.Attachments[1].CurrentScope.Response.Scope.ChatScopeIncarnation = uuid.NewString()
	sort.Slice(request.Attachments, func(i, j int) bool { return request.Attachments[i].AttachmentId < request.Attachments[j].AttachmentId })
	_, err := s.ApplyBotChatPinRemoval(ctx, request)
	require.ErrorIs(t, err, ErrBotPinRemovalConflict)
	for _, table := range []string{"bot_pin_removal_decisions", "bot_pin_removal_revisions", "bot_pin_removal_receipts", "bot_pin_removal_attachment_coverage"} {
		var count int
		require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
	pins, err := s.ListPins(ctx, uuid.MustParse(request.ChatId))
	require.NoError(t, err)
	require.Len(t, pins, 1)
	require.Equal(t, message, pins[0].MessageID)
}

func TestBotPinAttachmentAdoptsCompletedUninstallAndPreservesHumanRepin(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL cross-operation attachment coverage")
	}
	s, request, message := newBotPinRemovalFixture(t)
	ctx := t.Context()
	request.Purpose = "space_uninstall"
	request.Attachments = []*messagingv1.BotChatPinRemovalAttachment{unqualifiedPinAttachment()}
	prepareBotPinCurrentScope(t, s, request)
	first, err := s.ApplyBotChatPinRemoval(ctx, request)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.GetReceipt().GetRemovedPinCount())
	chat := uuid.MustParse(request.ChatId)
	require.NoError(t, s.UpsertPin(ctx, chat, message, uuid.New()), "a later authorized human repin is independent of the completed cleanup")
	global := proto.Clone(request).(*messagingv1.ApplyBotChatPinRemovalRequest)
	global.OperationId, global.DecisionId, global.Purpose = uuid.NewString(), uuid.NewString(), "global_delete"
	global.DecisionSha256 = make([]byte, 32)
	global.FrozenScopeSha256 = make([]byte, 32)
	// No hint: receiver-owned discoverable coverage is mandatory, not optional.
	second, err := s.ApplyBotChatPinRemoval(ctx, global)
	require.NoError(t, err)
	pins, err := s.ListPins(ctx, chat)
	require.NoError(t, err)
	require.Len(t, pins, 1, "same attachment must adopt the completed uninstall without deleting a later human repin")
	require.Zero(t, second.GetReceipt().GetRemovedPinCount())
	require.Zero(t, second.GetReceipt().GetExecutionCleanupRevision())
	require.Len(t, second.GetReceipt().GetTargets(), 1)
	target := second.Receipt.Targets[0]
	require.Equal(t, request.OperationId, target.CoveringOperationId)
	require.Equal(t, first.Receipt.ReceiptId, target.CoveringReceiptId)
	responseBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(first)
	require.NoError(t, err)
	digest := sha256.Sum256(responseBytes)
	require.Equal(t, digest[:], target.CoveringResponseSha256)
	var revision int64
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT revision FROM bot_pin_removal_revisions WHERE chat_id=$1 AND actor_profile_id=$2`, request.ChatId, request.ActorProfileId).Scan(&revision))
	require.EqualValues(t, 1, revision)
}
