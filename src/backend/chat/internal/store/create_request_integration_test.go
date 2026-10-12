package store

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateChatWithRequestID_replaysAndRejectsChangedRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	chatStore := &DMStore{Pool: pool}
	creatorID, requestID := uuid.New(), uuid.New()
	request := ChatCreateRequest{
		CreatorProfileID: creatorID,
		RequestID:        requestID,
		Type:             "group",
		Name:             "stable request",
	}

	first, replayed, err := chatStore.CreateChatWithRequestID(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.False(t, replayed)
	require.NoError(t, pool.QueryRow(ctx, `UPDATE chats SET name='later edit' WHERE id=$1 RETURNING id`, first.ID).Scan(new(uuid.UUID)))

	second, replayed, err := chatStore.CreateChatWithRequestID(ctx, request)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, "stable request", *second.Name, "retry must return the saved create result, not a later chat projection")
	blankTopic := "  \t"
	emptyTopicRequest := ChatCreateRequest{CreatorProfileID: uuid.New(), RequestID: uuid.New(), Type: "group", Name: "blank topic", Topic: &blankTopic}
	blankFirst, replayed, err := chatStore.CreateChatWithRequestID(ctx, emptyTopicRequest)
	require.NoError(t, err)
	require.False(t, replayed)
	emptyTopicRequest.Topic = nil
	blankReplay, replayed, err := chatStore.CreateChatWithRequestID(ctx, emptyTopicRequest)
	require.NoError(t, err)
	require.True(t, replayed, "blank topic and absent topic have the same NULL create side effect")
	require.Equal(t, blankFirst.ID, blankReplay.ID)
	emptyFirstRequest := ChatCreateRequest{CreatorProfileID: uuid.New(), RequestID: uuid.New(), Type: "group", Name: "absent topic"}
	emptyFirst, replayed, err := chatStore.CreateChatWithRequestID(ctx, emptyFirstRequest)
	require.NoError(t, err)
	require.False(t, replayed)
	emptyFirstRequest.Topic = &blankTopic
	emptyReplay, replayed, err := chatStore.CreateChatWithRequestID(ctx, emptyFirstRequest)
	require.NoError(t, err)
	require.True(t, replayed, "absent topic and blank topic also replay in the reverse direction")
	require.Equal(t, emptyFirst.ID, emptyReplay.ID)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chats WHERE creator_profile_id=$1 AND id IN (SELECT chat_id FROM chat_create_requests WHERE creator_profile_id=$1 AND request_id=$2)`, creatorID, requestID).Scan(&count))
	require.Equal(t, 1, count)

	changed := request
	changed.Name = "different request"
	_, _, err = chatStore.CreateChatWithRequestID(ctx, changed)
	require.ErrorIs(t, err, ErrChatCreateRequestConflict)

	otherCreator, replayed, err := chatStore.CreateChatWithRequestID(ctx, ChatCreateRequest{
		CreatorProfileID: uuid.New(),
		RequestID:        requestID,
		Type:             request.Type,
		Name:             request.Name,
	})
	require.NoError(t, err)
	require.False(t, replayed, "request keys are scoped to authenticated creator")
	require.NotEqual(t, first.ID, otherCreator.ID)
}

func TestCreateChatWithRequestID_concurrentSameKeyCreatesOneChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	chatStore := &DMStore{Pool: pool}
	creatorID, requestID := uuid.New(), uuid.New()
	request := ChatCreateRequest{
		CreatorProfileID: creatorID,
		RequestID:        requestID,
		Type:             "group",
		Name:             "concurrent request",
	}

	const contenders = 8
	start := make(chan struct{})
	ids := make(chan uuid.UUID, contenders)
	errs := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			row, _, err := chatStore.CreateChatWithRequestID(ctx, request)
			if err != nil {
				errs <- err
				return
			}
			ids <- row.ID
		}()
	}
	close(start)
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var first uuid.UUID
	for id := range ids {
		if first == uuid.Nil {
			first = id
		}
		require.Equal(t, first, id)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chats WHERE creator_profile_id=$1`, creatorID).Scan(&count))
	require.Equal(t, 1, count)
}
