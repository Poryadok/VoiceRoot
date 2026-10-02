package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/spacemutationlock"
)

func TestSpaceChatCreationUsesCanonicalSpaceMutationLock(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	spaceID := uuid.New()
	assertWaitsForSpaceLock := func(name string, lockedSpaceID uuid.UUID, run func() error) {
		t.Helper()
		lockConn, err := pool.Acquire(ctx)
		require.NoError(t, err)
		lockKey := spacemutationlock.Key(lockedSpaceID)
		_, err = lockConn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockKey)
		require.NoError(t, err)

		result := make(chan error, 1)
		go func() { result <- run() }()
		waited := false
		var earlyErr error
		select {
		case earlyErr = <-result:
		case <-time.After(150 * time.Millisecond):
			waited = true
		}

		var unlocked bool
		err = lockConn.QueryRow(ctx, `SELECT pg_advisory_unlock($1)`, lockKey).Scan(&unlocked)
		require.NoError(t, err)
		require.True(t, unlocked)
		lockConn.Release()
		if !waited {
			t.Fatalf("%s did not wait for the canonical Space mutation lock: %v", name, earlyErr)
		}
		select {
		case err := <-result:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatalf("%s did not proceed after the canonical lock was released: %v", name, ctx.Err())
		}
	}

	assertWaitsForSpaceLock("Space chat creation", spaceID, func() error {
		_, err := (&DMStore{Pool: pool}).CreateSpaceGroupChat(ctx, uuid.New(), spaceID, "serialized group", nil)
		return err
	})
	manifestSpaceID := uuid.New()
	assertWaitsForSpaceLock("Chat manifest capture", manifestSpaceID, func() error {
		_, err := (&SpaceLifecycleStore{Pool: pool}).PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{
			ProtocolVersion: 1, SpaceId: manifestSpaceID.String(), DeletionOperationId: uuid.NewString(), ScheduleGeneration: 1,
		})
		return err
	})
}

func TestSpaceChatCreationIsFencedAfterLifecycleFreeze(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}
	spaceID, operationID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO chat_space_lifecycle_fences(
		space_id,deletion_operation_id,generation,state,schedule_generation,manifest_id,manifest_sha256,manifest_item_count,
		source_manifest_id,source_manifest_sha256,source_manifest_item_count)
		VALUES($1,$2,1,'FROZEN',1,$3,$4,0,$3,$4,0)`, spaceID, operationID, operationID, make([]byte, 32))
	require.NoError(t, err)

	for name, create := range map[string]func() error{
		"group": func() error {
			_, err := store.CreateSpaceGroupChat(ctx, uuid.New(), spaceID, "blocked group", nil)
			return err
		},
		"channel": func() error {
			_, err := store.CreateSpaceChannelChat(ctx, uuid.New(), spaceID, "blocked channel", nil)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := create()
			require.ErrorIs(t, err, ErrSpaceLifecycleState)
		})
	}

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chats WHERE space_id=$1`, spaceID).Scan(&count))
	require.Zero(t, count, "a frozen manifest must not gain chats after capture")
}
