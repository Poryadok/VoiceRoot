package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/store"
)

// Construct the documented Chat owner root/page, independent of Notification.
func notificationManifestFixture(spaceID, operationID uuid.UUID, generation uint64, chats []uuid.UUID) *chatv1.SpacePurgeManifestPage {
	root := sha256.New()
	root.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	root.Write(spaceID[:])
	root.Write(operationID[:])
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], generation)
	root.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(chats)))
	root.Write(integer[:])
	ids := make([]string, len(chats))
	for i, id := range chats {
		root.Write(id[:])
		ids[i] = id.String()
	}
	digest := root.Sum(nil)
	page := sha256.New()
	page.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	page.Write(digest)
	binary.BigEndian.PutUint64(integer[:], 0)
	page.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(chats)))
	page.Write(integer[:])
	for _, id := range chats {
		page.Write(id[:])
	}
	return &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: digest, ItemCount: uint64(len(chats))}, ItemIds: ids, PageSha256: page.Sum(nil)}
}

func TestNotificationManifestFencesAndPurgesExactChannelSettings(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)
	s := &store.SettingsStore{Pool: pool}
	spaceID, operationID, chatID, otherChatID, profileID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, entry := range []store.NotificationSettings{{ProfileID: profileID, ScopeType: "channel", ScopeID: &chatID, Enabled: false}, {ProfileID: profileID, ScopeType: "chat", ScopeID: &chatID, Enabled: false}, {ProfileID: profileID, ScopeType: "channel", ScopeID: &otherChatID, Enabled: false}} {
		require.NoError(t, s.UpsertSettings(ctx, entry))
	}
	page := notificationManifestFixture(spaceID, operationID, 1, []uuid.UUID{chatID})
	request := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), ScheduleGeneration: 1, Page: page, SealsManifest: true}
	receipt, err := s.ImportSpacePurgeManifestPage(ctx, request)
	require.NoError(t, err, "canonical Chat page must be accepted by the actual participant store")
	replayed, err := s.ImportSpacePurgeManifestPage(ctx, request)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replayed))
	_, err = s.GetSettings(ctx, profileID, "channel", &chatID)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleNotLive, "imported channels are fenced before the barrier")
	require.ErrorIs(t, s.UpsertSettings(ctx, store.NotificationSettings{ProfileID: profileID, ScopeType: "chat", ScopeID: &chatID, Enabled: true}), store.ErrSpaceLifecycleNotLive)
	_, err = s.GetSettings(ctx, profileID, "channel", &otherChatID)
	require.NoError(t, err)
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: page.Manifest}
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	changed := proto.Clone(request).(*notificationv1.ImportSpacePurgeManifestPageRequest)
	changed.Page.ItemIds[0] = otherChatID.String()
	_, err = s.ImportSpacePurgeManifestPage(ctx, changed)
	require.Error(t, err)
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	require.NoError(t, s.UpsertSettings(ctx, store.NotificationSettings{ProfileID: profileID, ScopeType: "channel", ScopeID: &chatID, Enabled: true}))
	started, release, sent := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		sent <- s.WithChatDelivery(ctx, chatID.String(), func(guarded context.Context) error {
			// Dispatch policy reuses the admitted transaction; no nested-lock deadlock.
			_, err := s.GetSettings(guarded, profileID, "channel", &chatID)
			if err != nil {
				return err
			}
			close(started)
			<-release
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("admitted dispatch failed to start")
	}
	newOperation := uuid.New()
	newPage := notificationManifestFixture(spaceID, newOperation, 4, []uuid.UUID{chatID})
	newRequest := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: newOperation.String(), ScheduleGeneration: 4, Page: newPage, SealsManifest: true}
	imported := make(chan error, 1)
	go func() { _, err := s.ImportSpacePurgeManifestPage(ctx, newRequest); imported <- err }()
	select {
	case err := <-imported:
		close(release)
		t.Fatalf("import acknowledged before admitted dispatch drained: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-sent)
	require.NoError(t, <-imported)
	fence = &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: newOperation.String(), Generation: 4, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: newPage.Manifest}
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	called := false
	require.NoError(t, s.WithChatDelivery(ctx, chatID.String(), func(context.Context) error { called = true; return nil }))
	require.False(t, called, "a queued delivery must be suppressed after freeze")
	require.NoError(t, s.WithChatDelivery(ctx, otherChatID.String(), func(context.Context) error { called = true; return nil }))
	require.True(t, called, "unrelated Chat dispatch remains available")
	fence.Generation = 5
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: newOperation.String(), Generation: 5, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION, Manifest: newPage.Manifest, PurgeDecidedAt: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond))}
	purgeReceipt, err := s.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	var scopedCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notification_settings WHERE scope_id=$1`, chatID).Scan(&scopedCount))
	require.Zero(t, scopedCount)
	_, err = s.GetSettings(ctx, profileID, "channel", &otherChatID)
	require.NoError(t, err)
	var retentionMatches bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT retain_until=completed_at+interval '30 days' FROM notification_space_lifecycle_purge_receipts WHERE space_id=$1`, spaceID).Scan(&retentionMatches))
	require.True(t, retentionMatches)
	replayedPurge, err := s.PurgeSpace(ctx, purge)
	require.NoError(t, err)
	require.True(t, proto.Equal(purgeReceipt, replayedPurge))
	_, err = pool.Exec(ctx, `UPDATE notification_space_chat_manifests SET retain_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	_, err = s.DeleteExpiredSpaceLifecycleReceipts(ctx)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM notification_space_chat_manifest_pages WHERE space_id=$1`, spaceID).Scan(&scopedCount))
	require.Zero(t, scopedCount)
	called = false
	require.NoError(t, s.WithChatDelivery(ctx, chatID.String(), func(context.Context) error { called = true; return nil }))
	require.False(t, called, "permanent scope fence suppresses delayed delivery after full pages expire")
	_, err = s.ImportSpacePurgeManifestPage(ctx, newRequest)
	require.ErrorIs(t, err, store.ErrSpaceLifecycleNotLive, "old manifest cannot recreate a purged scope")
}

func TestNotificationManifestPartialImportCannotCompleteBarrierOrChangeScope(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startNotificationPostgresForTest(t, ctx)
	applyNotificationMigration(t, ctx, pool)
	s := &store.SettingsStore{Pool: pool}
	spaceID, operationID := uuid.New(), uuid.New()
	ids := make([]uuid.UUID, 1001)
	for i := range ids {
		ids[i] = uuid.MustParse(fmt.Sprintf("20000000-0000-4000-8000-%012x", i+1))
	}
	whole := notificationManifestFixture(spaceID, operationID, 1, ids)
	page := func(index uint64, chats []uuid.UUID, next string) *chatv1.SpacePurgeManifestPage {
		out := proto.Clone(whole).(*chatv1.SpacePurgeManifestPage)
		out.PageIndex = index
		out.NextPageToken = next
		out.ItemIds = nil
		h := sha256.New()
		h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
		h.Write(whole.Manifest.ManifestSha256)
		var integer [8]byte
		binary.BigEndian.PutUint64(integer[:], index)
		h.Write(integer[:])
		binary.BigEndian.PutUint64(integer[:], uint64(len(chats)))
		h.Write(integer[:])
		for _, id := range chats {
			out.ItemIds = append(out.ItemIds, id.String())
			h.Write(id[:])
		}
		out.PageSha256 = h.Sum(nil)
		return out
	}
	first := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), ScheduleGeneration: 1, Page: page(0, ids[:1000], "next")}
	last := proto.Clone(first).(*notificationv1.ImportSpacePurgeManifestPageRequest)
	last.Page = page(1, ids[1000:], "")
	last.SealsManifest = true
	_, err := s.ImportSpacePurgeManifestPage(ctx, last)
	require.Error(t, err, "gap cannot create a header or compact mapping")
	_, err = s.ImportSpacePurgeManifestPage(ctx, first)
	require.NoError(t, err)
	spaceDelivery := false
	require.NoError(t, s.WithSpaceDelivery(ctx, spaceID.String(), func(context.Context) error { spaceDelivery = true; return nil }))
	require.False(t, spaceDelivery, "pending import blocks Space delivery even for a chat outside the imported first page")
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: whole.Manifest}
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.Error(t, err, "partial import cannot acknowledge the freeze barrier")
	bad := proto.Clone(last).(*notificationv1.ImportSpacePurgeManifestPageRequest)
	bad.Page.Manifest.ManifestSha256[0] ^= 1
	_, err = s.ImportSpacePurgeManifestPage(ctx, bad)
	require.Error(t, err)
	last.Page = page(1, ids[:1], "")
	_, err = s.ImportSpacePurgeManifestPage(ctx, last)
	require.Error(t, err, "locally sorted final page must still respect global ordering")
	last.Page = page(1, ids[1000:], "")
	_, err = s.ImportSpacePurgeManifestPage(ctx, last)
	require.NoError(t, err)
	_, err = s.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	otherSpace, otherOperation := uuid.New(), uuid.New()
	other := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: otherSpace.String(), DeletionOperationId: otherOperation.String(), ScheduleGeneration: 1, SealsManifest: true, Page: notificationManifestFixture(otherSpace, otherOperation, 1, ids[:1])}
	_, err = s.ImportSpacePurgeManifestPage(ctx, other)
	require.Error(t, err, "a canonical competing root cannot transfer chat scope")
	emptySpace, emptyOperation := uuid.New(), uuid.New()
	empty := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: emptySpace.String(), DeletionOperationId: emptyOperation.String(), ScheduleGeneration: 1, SealsManifest: true, Page: notificationManifestFixture(emptySpace, emptyOperation, 1, nil)}
	_, err = s.ImportSpacePurgeManifestPage(ctx, empty)
	require.NoError(t, err, "empty Space has one sealed empty page")
}
