package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestCompleteSpaceLifecyclePurgeCommitsExactReceiptAndFenceAtomically(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	spaceID := uuid.MustParse("20000000-0000-4000-8000-000000000301")
	operationID := uuid.MustParse("20000000-0000-4000-8000-000000000302")
	chatID := uuid.MustParse("20000000-0000-4000-8000-000000000303")
	manifest := &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000304", ItemCount: 1}
	manifest.ManifestSha256 = messagingChatManifestSHA(spaceID, operationID, 7, []uuid.UUID{chatID})
	page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding), ItemIds: []string{chatID.String()}, PageSha256: make([]byte, 32)}
	service := &MessagesStore{Pool: pool}
	_, err := service.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{
		SpaceID: spaceID, DeletionOperationID: operationID, ScheduleGeneration: 7, Page: page, SealsManifest: true,
		RequestBytes: []byte("manifest-page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("manifest-receipt"),
	})
	require.NoError(t, err)
	for _, state := range []commonv1.LifecycleFenceState{
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED,
	} {
		generation := uint64(7)
		if state == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
			generation = 8
		}
		_, err = service.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{
			SpaceID: spaceID, DeletionOperationID: operationID, Generation: generation, State: state, Manifest: manifest,
			RequestBytes: []byte(state.String()), RequestSHA256: append([]byte{byte(generation)}, make([]byte, 31)...), ReceiptBytes: []byte("fence-receipt-" + state.String()),
		})
		require.NoError(t, err)
	}
	pageIDs, err := service.SpaceManifestChatPage(ctx, spaceID, operationID, 7, manifest.GetManifestId(), manifest.GetManifestSha256(), 1, 0)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{chatID}, pageIDs)
	request := &messagingv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 8,
		PurgeDecidedAt: timestamppb.New(time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)),
		ParticipantId:  commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding),
	}}
	first, err := service.CompleteSpaceLifecyclePurge(ctx, request, uuid.NewString())
	require.NoError(t, err)
	response := &messagingv1.PurgeSpaceResponse{}
	require.NoError(t, proto.Unmarshal(first, response))
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, response.GetReceipt().GetParticipantId())
	require.Equal(t, uint64(8), response.GetReceipt().GetGeneration())
	require.Len(t, response.GetReceipt().GetRequestSha256(), sha256.Size)

	replay, err := service.CompleteSpaceLifecyclePurge(ctx, request, uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, first, replay, "receipt bytes must remain stable after a retry")
	key := SpacePurgeReceiptKey{SpaceID: spaceID, DeletionOperationID: operationID, PurgeGeneration: 8, SourceScheduleGeneration: 7, MessagingRequestSHA256: spacePurgeRequestHash(request)}
	receipt, err := service.GetSpacePurgeReceipt(ctx, key)
	require.NoError(t, err)
	require.True(t, proto.Equal(response.GetReceipt(), receipt))
	changed := proto.Clone(request).(*messagingv1.PurgeSpaceRequest)
	changed.Purge.Manifest.ItemCount++
	_, err = service.CompleteSpaceLifecyclePurge(ctx, changed, uuid.NewString())
	require.ErrorIs(t, err, ErrSpacePurgeReceiptBinding)

	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM messaging_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state))
	require.Equal(t, "PURGED", state)
}

func spacePurgeRequestHash(request *messagingv1.PurgeSpaceRequest) []byte {
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		panic(err)
	}
	input := append([]byte("voice.messaging.v1.PurgeSpaceRequest\x00"), wire...)
	digest := sha256.Sum256(input)
	return digest[:]
}

func TestImportSpacePurgeManifestPagePersistsSealAndExactReplay(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	spaceID := uuid.MustParse("20000000-0000-4000-8000-000000000001")
	operationID := uuid.MustParse("20000000-0000-4000-8000-000000000002")
	chatID := uuid.MustParse("20000000-0000-4000-8000-000000000010")
	page := &chatv1.SpacePurgeManifestPage{
		ProtocolVersion: 1,
		Manifest:        &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000003", ItemCount: 1},
		ItemIds:         []string{chatID.String()},
		PageSha256:      make([]byte, 32),
	}
	page.Manifest.ManifestSha256 = messagingChatManifestSHA(spaceID, operationID, 7, []uuid.UUID{chatID})
	input := SpacePurgeManifestPageInput{
		SpaceID: spaceID, DeletionOperationID: operationID, ScheduleGeneration: 7,
		Page: page, SealsManifest: true,
		RequestBytes: []byte("request-v1"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("receipt-v1"),
	}
	service := &MessagesStore{Pool: pool}
	saved, err := service.ImportSpacePurgeManifestPage(ctx, input)
	require.NoError(t, err)
	require.Equal(t, input.ReceiptBytes, saved)

	var sealed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT sealed FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&sealed))
	require.True(t, sealed)
	var storedChat uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_id FROM messaging_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&storedChat))
	require.Equal(t, chatID, storedChat)

	input.ReceiptBytes = []byte("new-response-must-not-replace-the-first-receipt")
	saved, err = service.ImportSpacePurgeManifestPage(ctx, input)
	require.NoError(t, err)
	require.Equal(t, []byte("receipt-v1"), saved)

	input.RequestBytes = []byte("changed-body")
	input.RequestSHA256 = append([]byte(nil), input.RequestSHA256...)
	input.RequestSHA256[0] = 1
	_, err = service.ImportSpacePurgeManifestPage(ctx, input)
	require.ErrorIs(t, err, ErrSpaceManifestBinding)

	gap := input
	gap.Page = proto.Clone(page).(*chatv1.SpacePurgeManifestPage)
	gap.Page.PageIndex = 1
	_, err = service.ImportSpacePurgeManifestPage(ctx, gap)
	require.ErrorIs(t, err, ErrSpaceManifestOrder)
}

func TestImportEmptySpaceManifestSealsAndAllowsFreeze(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	spaceID, operationID := uuid.New(), uuid.New()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: messagingChatManifestSHA(spaceID, operationID, 7, nil)}
	page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: manifest, PageSha256: make([]byte, 32)}
	service := &MessagesStore{Pool: pool}
	_, err := service.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{
		SpaceID: spaceID, DeletionOperationID: operationID, ScheduleGeneration: 7, Page: page, SealsManifest: true,
		RequestBytes: []byte("empty-manifest"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("empty-receipt"),
	})
	require.NoError(t, err)
	_, err = service.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{
		SpaceID: spaceID, DeletionOperationID: operationID, Generation: 7, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest,
		RequestBytes: []byte("freeze-empty"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("freeze-receipt"),
	})
	require.NoError(t, err)
	_, err = service.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{SpaceID: spaceID, DeletionOperationID: operationID, Generation: 8, State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, Manifest: manifest, RequestBytes: []byte("purge-empty"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("purge-fence")})
	require.NoError(t, err)
	request := &messagingv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 8, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, Manifest: manifest, PurgeDecidedAt: timestamppb.Now()}}
	first, err := service.CompleteSpaceLifecyclePurge(ctx, request, uuid.NewString())
	require.NoError(t, err)
	replay, err := service.CompleteSpaceLifecyclePurge(ctx, request, uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, first, replay)
}

func TestImportSpacePurgeManifestPageSealsOnlyAfterCompleteOrderedRoot(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	spaceID := uuid.MustParse("20000000-0000-4000-8000-000000000101")
	operationID := uuid.MustParse("20000000-0000-4000-8000-000000000102")
	ids := make([]uuid.UUID, 1001)
	for i := range ids {
		ids[i] = uuid.MustParse(fmt.Sprintf("20000000-0000-4000-8000-%012x", i+1))
	}
	manifestHash := messagingChatManifestSHA(spaceID, operationID, 9, ids)
	manifest := &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000103", ManifestSha256: manifestHash, ItemCount: uint64(len(ids))}
	first := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding), NextPageToken: "next", PageSha256: make([]byte, 32)}
	for _, id := range ids[:1000] {
		first.ItemIds = append(first.ItemIds, id.String())
	}
	second := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding), PageIndex: 1, ItemIds: []string{ids[1000].String()}, PageSha256: make([]byte, 32)}
	service := &MessagesStore{Pool: pool}
	firstInput := SpacePurgeManifestPageInput{SpaceID: spaceID, DeletionOperationID: operationID, ScheduleGeneration: 9, Page: first,
		RequestBytes: []byte("first-page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("first-receipt")}
	_, err := service.ImportSpacePurgeManifestPage(ctx, firstInput)
	require.NoError(t, err)
	var sealed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT sealed FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&sealed))
	require.False(t, sealed, "the partial page set must not become purge evidence")

	secondInput := SpacePurgeManifestPageInput{SpaceID: spaceID, DeletionOperationID: operationID, ScheduleGeneration: 9, Page: second, SealsManifest: true,
		RequestBytes: []byte("second-page"), RequestSHA256: append([]byte{1}, make([]byte, 31)...), ReceiptBytes: []byte("second-receipt")}
	saved, err := service.ImportSpacePurgeManifestPage(ctx, secondInput)
	require.NoError(t, err)
	require.Equal(t, secondInput.ReceiptBytes, saved)
	require.NoError(t, pool.QueryRow(ctx, `SELECT sealed FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&sealed))
	require.True(t, sealed, "only a complete root-matching page set can seal")
}

func TestApplySpaceLifecycleFenceNeedsSealedManifestAndBlocksMessageWrites(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	spaceID := uuid.MustParse("20000000-0000-4000-8000-000000000201")
	operationID := uuid.MustParse("20000000-0000-4000-8000-000000000202")
	chatID := uuid.MustParse("20000000-0000-4000-8000-000000000203")
	manifest := &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000204", ItemCount: 1}
	manifest.ManifestSha256 = messagingChatManifestSHA(spaceID, operationID, 7, []uuid.UUID{chatID})
	manifestPage := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding), ItemIds: []string{chatID.String()}, PageSha256: make([]byte, 32)}
	service := &MessagesStore{Pool: pool}
	input := SpaceLifecycleFenceInput{
		SpaceID: spaceID, DeletionOperationID: operationID, Generation: 7,
		State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest,
		RequestBytes: []byte("freeze-request"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("freeze-receipt"),
	}
	_, err := service.ApplySpaceLifecycleFence(ctx, input)
	require.ErrorIs(t, err, ErrSpaceManifestNotSealed, "a fence must not precede the complete imported manifest")

	_, err = service.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{
		SpaceID: spaceID, DeletionOperationID: operationID, ScheduleGeneration: 7, Page: manifestPage, SealsManifest: true,
		RequestBytes: []byte("manifest-page"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("manifest-receipt"),
	})
	require.NoError(t, err)
	beforeFreeze := MessageRow{ID: uuid.New(), ChatID: chatID, ChatType: "group", SenderProfileID: uuid.New(), Content: "before freeze", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	_, err = service.InsertMessage(ctx, beforeFreeze)
	require.NoError(t, err)
	reactionProfileID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO reactions(message_id,profile_id,emoji) VALUES($1,$2,'👍')`, beforeFreeze.ID, reactionProfileID)
	require.NoError(t, err)
	hideProfileID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO message_hides(message_id,profile_id) VALUES($1,$2)`, beforeFreeze.ID, hideProfileID)
	require.NoError(t, err)

	_, err = service.ApplySpaceLifecycleFence(ctx, input)
	require.NoError(t, err)
	replay, err := service.ApplySpaceLifecycleFence(ctx, input)
	require.NoError(t, err)
	require.Equal(t, []byte("freeze-receipt"), replay)

	afterFreeze := MessageRow{ID: uuid.New(), ChatID: chatID, ChatType: "group", SenderProfileID: uuid.New(), Content: "after freeze", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	_, err = service.InsertMessage(ctx, afterFreeze)
	require.Error(t, err, "database mutation gate must reject writes to a frozen Space chat")
	_, err = pool.Exec(ctx, `INSERT INTO reactions(message_id,profile_id,emoji) VALUES($1,$2,'👍')`, beforeFreeze.ID, uuid.New())
	require.Error(t, err, "reaction writes must also be rejected while the Space chat is frozen")
	_, err = pool.Exec(ctx, `UPDATE reactions SET emoji='🔥' WHERE message_id=$1 AND profile_id=$2`, beforeFreeze.ID, reactionProfileID)
	require.Error(t, err, "reaction updates must also be rejected while the Space chat is frozen")
	_, err = pool.Exec(ctx, `DELETE FROM message_hides WHERE message_id=$1 AND profile_id=$2`, beforeFreeze.ID, hideProfileID)
	require.Error(t, err, "message-hide mutations must also be rejected while the Space chat is frozen")
	_, err = pool.Exec(ctx, `INSERT INTO read_positions(chat_id,profile_id,last_read_message_id) VALUES($1,$2,$3)`, chatID, uuid.New(), beforeFreeze.ID)
	require.Error(t, err, "read-position writes must also be rejected while the Space chat is frozen")

	restore := input
	restore.Generation = 8
	restore.State = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	restore.RequestBytes = []byte("restore-request")
	restore.RequestSHA256 = append([]byte{1}, make([]byte, 31)...)
	restore.ReceiptBytes = []byte("live-receipt")
	_, err = service.ApplySpaceLifecycleFence(ctx, restore)
	require.NoError(t, err)
	afterRestore := MessageRow{ID: uuid.New(), ChatID: chatID, ChatType: "group", SenderProfileID: uuid.New(), Content: "after restore", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	_, err = service.InsertMessage(ctx, afterRestore)
	require.NoError(t, err, "a durable LIVE fence reopens the restored Space chat")
	stale := input
	stale.DeletionOperationID = uuid.MustParse("20000000-0000-4000-8000-000000000205")
	stale.Generation = 7
	stale.RequestBytes = []byte("stale-freeze-request")
	stale.ReceiptBytes = []byte("stale-response-must-not-replace-current-receipt")
	replay, err = service.ApplySpaceLifecycleFence(ctx, stale)
	require.NoError(t, err, "a stale generation must be acknowledged without requiring an old manifest")
	require.Equal(t, []byte("live-receipt"), replay)
}
