package catalogassets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

type postgresActorObserver struct {
	mu      sync.Mutex
	state   ProfileState
	floor   int64
	entered chan struct{}
	release chan struct{}
}

func (o *postgresActorObserver) Observe(ctx context.Context, _ Actor) (ActorObservation, error) {
	o.mu.Lock()
	observation := ActorObservation{SessionEpochFloor: o.floor, Profile: o.state}
	entered, release := o.entered, o.release
	o.entered = nil
	o.release = nil
	o.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return ActorObservation{}, ctx.Err()
		}
	}
	return observation, nil
}
func (o *postgresActorObserver) set(state ProfileState, floor int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state = state
	o.floor = floor
}
func (o *postgresActorObserver) blockNext(entered, release chan struct{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.entered = entered
	o.release = release
}

func TestCatalogPostgresGateLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	_, path, _, _ := runtime.Caller(0)
	migrations := filepath.Join(filepath.Dir(path), "..", "..", "..", "migrations", "chat_db")
	up, err := os.ReadFile(filepath.Join(migrations, "000028_catalog_assets.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(migrations, "000028_catalog_assets.down.sql"))
	require.NoError(t, err)
	pool := integrationtest.StartPostgres(t, ctx, "chat_catalog_actor", "")
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err, "empty schema rollback")
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	keys, err := NewActorKeyring("old", map[string][]byte{"old": bytes.Repeat([]byte{1}, 32)})
	require.NoError(t, err)
	observer := &postgresActorObserver{state: ProfileLive, floor: 7}
	repo := &PostgresRepository{Pool: pool, Keys: keys, Observer: observer}
	require.NoError(t, repo.Preflight(ctx))
	actor := Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
	capture, err := repo.CaptureActor(ctx, actor, true)
	require.NoError(t, err)
	require.True(t, validCapture(capture))
	binding := Binding{Actor: actor, ActorCapture: capture, Selection: Selection{Provider: "fixture", ProviderID: "one"}, SourceID: uuid.New(), FileID: uuid.New()}
	require.NoError(t, repo.SaveBinding(ctx, binding, capture))
	got, found, err := repo.FindBinding(ctx, actor, binding.Selection, binding.FileID, capture)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, binding, got)

	const contenders = 8
	start := make(chan struct{})
	errors := make(chan error, contenders)
	captures := make(chan ActorCapture, contenders)
	for range contenders {
		go func() {
			<-start
			c, err := repo.CaptureActor(ctx, actor, true)
			if err == nil {
				err = repo.SaveBinding(ctx, binding, c)
			}
			captures <- c
			errors <- err
		}()
	}
	close(start)
	for range contenders {
		require.NoError(t, <-errors)
		require.Equal(t, capture, <-captures)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_asset_bindings`).Scan(&count))
	require.Equal(t, 1, count)
	changed := binding
	changed.SourceID = uuid.New()
	require.ErrorIs(t, repo.SaveBinding(ctx, changed, capture), ErrBindingConflict)

	// Rotation keeps one logical gate; key removal/mutation cannot lose old fences.
	rotated, err := NewActorKeyring("new", map[string][]byte{"old": bytes.Repeat([]byte{1}, 32), "new": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	next := &PostgresRepository{Pool: pool, Keys: rotated, Observer: observer}
	require.NoError(t, next.Preflight(ctx))
	current, err := next.CaptureActor(ctx, actor, true)
	require.NoError(t, err)
	require.Equal(t, capture, current)
	require.Error(t, repo.Preflight(ctx), "missing historical/registered key denies startup")
	repo = next
	event := uuid.New()
	require.NoError(t, repo.PurgeAccount(ctx, actor.AccountID, event))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_asset_bindings`).Scan(&count))
	require.Zero(t, count)
	require.ErrorIs(t, repo.SaveBinding(ctx, binding, capture), ErrActorRetired, "post-purge Save must not recreate a mapping")
	observer.set(ProfileLive, 8)
	restored, err := repo.CaptureActor(ctx, actor, true)
	require.NoError(t, err)
	require.NotEqual(t, capture.AccountGeneration, restored.AccountGeneration)
	fresh := binding
	fresh.FileID = uuid.New()
	fresh.SourceID = uuid.New()
	fresh.ActorCapture = restored
	require.NoError(t, repo.SaveBinding(ctx, fresh, restored))
	require.NoError(t, repo.PurgeAccount(ctx, actor.AccountID, event), "same deletion event cannot erase a restored generation")
	_, found, err = repo.FindBinding(ctx, actor, fresh.Selection, fresh.FileID, restored)
	require.NoError(t, err)
	require.True(t, found)

	observer.set(ProfileUnknown, 8)
	_, err = repo.CaptureActor(ctx, actor, true)
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_asset_bindings`).Scan(&count))
	require.Equal(t, 1, count, "UNKNOWN is not erasure proof")
	observer.set(ProfileSoftDeleted, 8)
	require.ErrorIs(t, repo.SaveBinding(ctx, fresh, restored), ErrActorRetired)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_asset_bindings`).Scan(&count))
	require.Zero(t, count)
	require.ErrorIs(t, repo.SaveBinding(ctx, binding, capture), ErrActorRetired)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "nonempty negative state cannot be rolled back")

	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name IN ('catalog_actor_keys','catalog_actor_gates','catalog_actor_aliases','catalog_actor_event_fences') AND column_name IN ('account_id','profile_id','provider','source_url','request_json')`).Scan(&count))
	require.Zero(t, count, "terminal gates must not retain raw identities or provider data")

	// The owner read deliberately waits while Save holds both gate rows. Purge
	// races on a second transaction and must delete the committed Save, rather
	// than leave an actor row inserted after its barrier.
	observer.set(ProfileLive, 9)
	racingActor := Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
	racingCapture, err := repo.CaptureActor(ctx, racingActor, true)
	require.NoError(t, err)
	racingBinding := Binding{Actor: racingActor, ActorCapture: racingCapture, Selection: Selection{Provider: "fixture", ProviderID: "race"}, SourceID: uuid.New(), FileID: uuid.New()}
	entered, release := make(chan struct{}), make(chan struct{})
	observer.blockNext(entered, release)
	saved, purged := make(chan error, 1), make(chan error, 1)
	go func() { saved <- repo.SaveBinding(ctx, racingBinding, racingCapture) }()
	<-entered
	started := make(chan struct{})
	go func() { close(started); purged <- repo.PurgeAccount(ctx, racingActor.AccountID, uuid.New()) }()
	<-started
	close(release)
	require.NoError(t, <-saved)
	require.NoError(t, <-purged)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_asset_bindings WHERE profile_id=$1`, racingActor.ProfileID).Scan(&count))
	require.Zero(t, count)
	require.ErrorIs(t, repo.SaveBinding(ctx, racingBinding, racingCapture), ErrActorRetired)
}
