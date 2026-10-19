package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"net"
	"sync/atomic"
	"testing"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/timeout"
)

type nativeAckRedisHook struct{ calls atomic.Int32 }

func TestPrecutRecoveryCallbackStillAcknowledgesExactEffects(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	created, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	matchID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO matches(id,game_id,mode,region)VALUES($1,$2,$3,'eu')`, matchID, p.GameID, p.Mode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_recovery_effects(match_id,search_session_id,generation,action)SELECT $1,id,recovery_generation,'enqueue' FROM search_sessions WHERE id=$2`, matchID, created.Sessions[0].ID)
	require.NoError(t, err)
	calls := 0
	matches := &store.MatchStore{Pool: pool}
	apply := func(context.Context, store.SearchSession, string) error { calls++; return nil }
	require.NoError(t, matches.ApplyPendingRecoveryEffects(ctx, 10, apply))
	require.Equal(t, 1, calls)
	require.NoError(t, matches.ApplyPendingRecoveryEffects(ctx, 10, apply))
	require.Equal(t, 1, calls, "committed pre-cut recovery is not dispatched twice")
}

func (h *nativeAckRedisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (h *nativeAckRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error { h.calls.Add(1); return next(ctx, cmd) }
}
func (h *nativeAckRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmd []redis.Cmder) error { h.calls.Add(1); return next(ctx, cmd) }
}

func TestNativeRecoveryAckRequiresActualWholePartyStateAndStamp(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool, NativeAdmission: &binding}
	created, err := parties.CreateSourceParty(ctx, p)
	require.NoError(t, err)
	matches := &store.MatchStore{Pool: pool, NativeAdmission: &binding}
	q := projectionRedis(t)
	// Explicit retained post-recovery SQL fixture. The projection below is the
	// real native projector, not an inserted projection certificate/Ready flag.
	matchID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO matches(id,game_id,mode,region)VALUES($1,$2,$3,'eu')`, matchID, p.GameID, p.Mode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_recovery_effects(match_id,search_session_id,generation,action)SELECT $1,id,recovery_generation,'enqueue' FROM search_sessions WHERE id=$2`, matchID, created.Sessions[0].ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE matchmaking_party_search_queue_intents SET projected_at=clock_timestamp() WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	_, err = matches.AcknowledgeNativeRecoveryEffects(ctx, 10)
	require.Error(t, err, "old projected_at without state/stamp must remain unqualified")
	legacyCalls := 0
	err = (&store.MatchStore{Pool: pool}).ApplyPendingRecoveryEffects(ctx, 10, func(context.Context, store.SearchSession, string) error {
		legacyCalls++
		return nil
	})
	require.Error(t, err, "lost native binding cannot select scalar legacy recovery after admission began")
	require.Zero(t, legacyCalls)
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	projector := &Projector{Store: parties, Queue: q, Validator: validator, ForwardLifetime: &projectionLifetimeFixture{}, Resetter: matches}
	claimed, err := projector.RunOnce(ctx)
	require.True(t, claimed)
	require.NoError(t, err)
	count, err := matches.AcknowledgeNativeRecoveryEffects(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = pool.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, matchID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_recovery_effects(match_id,search_session_id,generation,action)SELECT $1,id,recovery_generation,'enqueue' FROM search_sessions WHERE id=$2`, matchID, created.Sessions[1].ID)
	require.NoError(t, err)
	hook := &nativeAckRedisHook{}
	q.Client.AddHook(hook)
	sweeper := &timeout.Sweeper{Sessions: &store.SessionStore{Pool: pool}, Matches: matches, Queue: q}
	require.NoError(t, sweeper.RunNativeOnce(ctx))
	require.Zero(t, hook.calls.Load(), "native sweeper must acknowledge the committed whole-party proof without scalar Redis recovery under SQL locks")
	count, err = matches.AcknowledgeNativeRecoveryEffects(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, count, "exact ACK replay is read-only")
	// A newer local generation cannot reuse the previous physical projection.
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_recovery_effects(match_id,search_session_id,generation,action)SELECT $1,id,recovery_generation,'enqueue' FROM search_sessions WHERE id=$2`, matchID, created.Sessions[0].ID)
	require.NoError(t, err)
	_, err = matches.AcknowledgeNativeRecoveryEffects(ctx, 10)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	_, err = matches.ResetVoiceParty(ctx, created.Party.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_recovery_effects(match_id,search_session_id,generation,action)SELECT $1,id,recovery_generation,'release' FROM search_sessions WHERE id=$2`, matchID, created.Sessions[0].ID)
	require.NoError(t, err)
	projector.Validator = nil
	projector.ForwardLifetime = nil
	claimed, err = projector.RunOnce(ctx)
	require.True(t, claimed)
	require.NoError(t, err)
	count, err = matches.AcknowledgeNativeRecoveryEffects(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 2, count, "obsolete generation and exact closed RELEASE settle without fresh admission")
	ids, err := q.ListSessionIDs(ctx, p.GameID, p.Mode, "eu", 10)
	require.NoError(t, err)
	require.Empty(t, ids)
}
