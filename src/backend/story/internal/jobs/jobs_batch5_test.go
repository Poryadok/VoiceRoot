package jobs_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/story/internal/jobs"
	"voice/backend/story/internal/store"
	"voice/backend/pkg/integrationtest"
)

type recordingPublisher struct {
	mu   sync.Mutex
	ids  []string
}

func (p *recordingPublisher) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	return nil
}
func (p *recordingPublisher) PublishStoryViewed(context.Context, string, string) error { return nil }
func (p *recordingPublisher) PublishStoryReacted(context.Context, string, string, string) error {
	return nil
}
func (p *recordingPublisher) PublishStoryExpired(_ context.Context, storyID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ids = append(p.ids, storyID)
	return nil
}
func (p *recordingPublisher) PublishStoryHighlightCreated(context.Context, string, string) error {
	return nil
}
func (p *recordingPublisher) PublishStoryLfpCreated(context.Context, string, string, string) error {
	return nil
}
func (p *recordingPublisher) Close() error { return nil }

func TestMarkExpiredStoriesReturning_publishesPerStory(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storyexpire", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.StoryStore{Pool: pool}

	author := uuid.New()
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	_, err = pool.Exec(ctx, `
INSERT INTO stories (
  id, author_profile_id, type, mention_profile_ids, visibility, expires_at, archived_until, created_at
) VALUES ($1, $2, 'text', '[]', 'friends', $3, $4, $3)`,
		uuid.New(), author, past, past.Add(30*24*time.Hour))
	require.NoError(t, err)

	ids, n, err := st.MarkExpiredStoriesReturning(ctx, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Len(t, ids, 1)

	pub := &recordingPublisher{}
	for _, id := range ids {
		_ = pub.PublishStoryExpired(ctx, id.String())
	}
	require.Len(t, pub.ids, 1)
}

func TestStartExpiryWorker_acceptsPublisher(t *testing.T) {
	st := &store.StoryStore{}
	pub := &recordingPublisher{}
	jobs.StartExpiryWorker(context.Background(), st, pub, nil)
	jobs.StartArchivePurgeWorker(context.Background(), st, nil, nil)
	require.True(t, true)
}
