package timeout

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/store"
)

type recordingPublisher struct {
	nudges    int
	timeouts  int
	nudgeErr  error
	timeoutErr error
}

func (p *recordingPublisher) PublishSearchStarted(context.Context, string, string, string, string, string) error {
	return nil
}
func (p *recordingPublisher) PublishSearchCancelled(context.Context, string, string) error { return nil }
func (p *recordingPublisher) PublishMatchFound(context.Context, mmevents.MatchFoundEvent) error {
	return nil
}
func (p *recordingPublisher) PublishMatchCompleted(context.Context, mmevents.MatchCompletedEvent) error {
	return nil
}
func (p *recordingPublisher) PublishRatingSubmitted(context.Context, mmevents.RatingSubmittedEvent) error {
	return nil
}
func (p *recordingPublisher) PublishSearchNudge(context.Context, string, string, string, string) error {
	if p.nudgeErr != nil {
		return p.nudgeErr
	}
	p.nudges++
	return nil
}
func (p *recordingPublisher) PublishSearchTimeout(context.Context, string, string, string, string) error {
	if p.timeoutErr != nil {
		return p.timeoutErr
	}
	p.timeouts++
	return nil
}
func (p *recordingPublisher) Close() error { return nil }

func TestSweeper_RunOnce_NudgeAndExpire(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &store.GameStore{Pool: pool}
	games, err := gameStore.List(ctx, store.ListGamesParams{PageSize: 1, Status: store.StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, games.Games)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisQueue := &queue.RedisQueue{Client: rdb, Prefix: "mm"}

	sessions := &store.SessionStore{Pool: pool}
	profileID := uuid.New()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-16 * time.Minute)
	timeoutAt := now.Add(-1 * time.Minute)

	sess, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileID,
		GameID:    games.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: timeoutAt,
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET created_at = $2 WHERE id = $1`, sess.ID, createdAt)
	require.NoError(t, err)

	require.NoError(t, redisQueue.AcquireLock(ctx, profileID, sess.ID))
	require.NoError(t, redisQueue.Enqueue(ctx, games.Games[0].ID, "5v5 Ranked", "eu", sess.ID, createdAt))

	pub := &recordingPublisher{}
	sweeper := &Sweeper{
		Sessions: sessions,
		Queue:    redisQueue,
		Events:   pub,
		Timing: runtimeconfig.SearchTiming{
			NudgeAfter: 15 * time.Minute,
			Timeout:    30 * time.Minute,
		},
		Now: func() time.Time { return now },
	}

	require.NoError(t, sweeper.RunOnce(ctx))
	require.Equal(t, 1, pub.nudges)
	require.Equal(t, 1, pub.timeouts)

	loaded, err := sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusTimeout, loaded.Status)
	require.NotNil(t, loaded.NudgedAt)
}

func TestSweeper_RunOnce_NudgePublishFailureRetries(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &store.GameStore{Pool: pool}
	games, err := gameStore.List(ctx, store.ListGamesParams{PageSize: 1, Status: store.StatusActive})
	require.NoError(t, err)

	sessions := &store.SessionStore{Pool: pool}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	sess, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    games.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: now.Add(30 * time.Minute),
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET created_at = $2 WHERE id = $1`, sess.ID, now.Add(-16*time.Minute))
	require.NoError(t, err)

	pub := &recordingPublisher{nudgeErr: context.DeadlineExceeded}
	sweeper := &Sweeper{
		Sessions: sessions,
		Events:   pub,
		Timing:   runtimeconfig.SearchTiming{NudgeAfter: 15 * time.Minute},
		Now:      func() time.Time { return now },
	}
	require.NoError(t, sweeper.RunOnce(ctx))

	loaded, err := sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Nil(t, loaded.NudgedAt)
}

func TestSweeper_RunOnce_ExpireWithoutQueue(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &store.GameStore{Pool: pool}
	games, err := gameStore.List(ctx, store.ListGamesParams{PageSize: 1, Status: store.StatusActive})
	require.NoError(t, err)

	sessions := &store.SessionStore{Pool: pool}
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	sess, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    games.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: now.Add(-time.Minute),
	})
	require.NoError(t, err)

	pub := &recordingPublisher{}
	sweeper := &Sweeper{Sessions: sessions, Events: pub, Now: func() time.Time { return now }}
	require.NoError(t, sweeper.RunOnce(ctx))

	loaded, err := sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusTimeout, loaded.Status)
	require.Equal(t, 1, pub.timeouts)
}

func TestSweeper_RunOnce_RedisFailureSkipsExpire(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := store.StartMatchmakingDBForStoreTest(t, ctx)
	store.ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &store.GameStore{Pool: pool}
	games, err := gameStore.List(ctx, store.ListGamesParams{PageSize: 1, Status: store.StatusActive})
	require.NoError(t, err)

	sessions := &store.SessionStore{Pool: pool}
	profileID := uuid.New()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	sess, err := sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileID,
		GameID:    games.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: now.Add(-time.Minute),
	})
	require.NoError(t, err)

	// No redis — Dequeue unavailable; session must remain searching.
	sweeper := &Sweeper{Sessions: sessions, Queue: &queue.RedisQueue{}, Now: func() time.Time { return now }}
	require.NoError(t, sweeper.RunOnce(ctx))

	loaded, err := sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, store.SessionStatusSearching, loaded.Status)
}

func TestSweeper_RunOnce_NilSweeper(t *testing.T) {
	var s *Sweeper
	require.NoError(t, s.RunOnce(context.Background()))
}
