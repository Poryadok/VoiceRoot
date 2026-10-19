package partysearch

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
)

type nativeProjectionQueueFixture struct{ enqueue, release int }

func (q *nativeProjectionQueueFixture) RecoverParty(context.Context, []queue.SearchRecovery) error {
	q.enqueue++
	return nil
}

type projectionLifetimeFixture struct {
	enter     func()
	released  bool
	err       error
	onRelease func()
}

func (f *projectionLifetimeFixture) EnterForward(context.Context, uuid.UUID, [32]byte) (func(), error) {
	if f.enter != nil {
		f.enter()
	}
	if f.err != nil {
		return nil, f.err
	}
	return func() {
		f.released = true
		if f.onRelease != nil {
			f.onRelease()
		}
	}, nil
}

func TestNativeProjectorActualRedisPositiveAndNegativeBoundaries(t *testing.T) {
	for _, kind := range []string{"current", "snapshot-unknown", "changed", "closed-head", "noninitiator-ban", "readback-unknown", "local-cut-change"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			pool := partyDB(t)
			binding, _, _ := historyReceiptFixture(t, pool)
			p := partyParams(t, pool)
			parties := &store.PartyStore{Pool: pool, NativeAdmission: &binding}
			created, err := parties.CreateSourceParty(ctx, p)
			require.NoError(t, err)
			q := projectionRedis(t)
			lifetime := &projectionLifetimeFixture{}
			validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
				require.Zero(t, pool.Stat().AcquiredConns(), "source RPC cannot run under SQL locks")
				if kind == "snapshot-unknown" {
					return partysource.Snapshot{}, partysource.ErrUnknown
				}
				s := partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}
				if kind == "changed" {
					s.RosterVersion++
				}
				return s, nil
			})}
			lifetime.enter = func() {
				require.Zero(t, pool.Stat().AcquiredConns(), "readback cannot run under SQL locks")
				if kind == "local-cut-change" {
					_, e := pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE party_id=$1`, created.Party.ID)
					require.NoError(t, e)
				}
				if kind == "closed-head" {
					_, e := pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
					require.NoError(t, e)
				}
				if kind == "noninitiator-ban" {
					var account uuid.UUID
					for _, m := range p.MemberAuthorities {
						if m.AccountID != p.InitiatorAccountID {
							account = m.AccountID
							break
						}
					}
					require.NotEqual(t, uuid.Nil, account)
					require.NoError(t, (&store.BanStore{Pool: pool}).InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: account, BannedByProfileID: uuid.New()}))
				}
			}
			if kind == "readback-unknown" {
				lifetime.err = errors.New("explicit external observation fixture unavailable")
			}
			if kind == "current" {
				lifetime.onRelease = func() {
					var acknowledged bool
					require.NoError(t, pool.QueryRow(ctx, `SELECT projected_at IS NOT NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&acknowledged))
					require.True(t, acknowledged, "registration must span durable projection acknowledgement")
				}
			}
			projector := &Projector{Store: parties, Queue: projectionSpy{delegate: q, before: func() { require.Zero(t, pool.Stat().AcquiredConns()); require.False(t, lifetime.released) }}, Validator: validator, ForwardLifetime: lifetime, Resetter: &store.MatchStore{Pool: pool, NativeAdmission: &binding}}
			claimed, err := projector.RunOnce(ctx)
			require.True(t, claimed)
			ids, listErr := q.ListSessionIDs(ctx, p.GameID, p.Mode, "eu", 10)
			require.NoError(t, listErr)
			if kind == "current" {
				require.NoError(t, err)
				require.Len(t, ids, 2)
				require.True(t, lifetime.released)
				_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
				require.NoError(t, err)
				_, err = projector.Resetter.ResetVoiceParty(ctx, created.Party.ID)
				require.NoError(t, err)
				// RELEASE is exact terminal work and does not require a current source or
				// OPEN readback. Real Redis removes both retained session projections.
				projector.Validator = nil
				projector.ForwardLifetime = nil
				projector.Queue = q
				claimed, err = projector.RunOnce(ctx)
				require.True(t, claimed)
				require.NoError(t, err)
				ids, err = q.ListSessionIDs(ctx, p.GameID, p.Mode, "eu", 10)
				require.NoError(t, err)
				require.Empty(t, ids)
			} else {
				require.Error(t, err)
				require.Empty(t, ids)
				var searching int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching'`, created.Party.ID).Scan(&searching))
				if kind == "changed" || kind == "noninitiator-ban" {
					require.Zero(t, searching)
				} else {
					require.Equal(t, 2, searching, "Unknown must not cancel the party")
				}
			}
		})
	}
}

func TestNativeProjectionAuthorizationWaitsForCloseBeforeAccountLocks(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool, NativeAdmission: &binding}
	_, err := parties.CreateSourceParty(ctx, p)
	require.NoError(t, err)
	lease, err := parties.ClaimQueueIntent(ctx, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lease)
	cut, err := parties.ReadQueueIntent(ctx, *lease)
	require.NoError(t, err)
	candidate, err := parties.CaptureQueueProjection(ctx, cut)
	require.NoError(t, err)
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	validated, err := validator.ValidateQueueProjection(ctx, candidate)
	require.NoError(t, err)
	require.NotNil(t, validated.Attempt)
	require.Error(t, parties.AuthorizeQueueProjection(ctx, candidate, nil), "no caller-created boolean may replace the private source proof")
	accounts := candidate.Accounts()
	copyOfParties := candidate.Parties()
	copyOfParties[0].MemberAuthorities[0].AccountID = uuid.New()
	require.Equal(t, accounts, candidate.Accounts(), "public accessors cannot mutate the sealed account witness")
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, mmadmission.AcquireCloseGate(ctx, closer))
	_, err = closer.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		done <- parties.AuthorizeQueueProjection(bounded, candidate, validated.Attempt)
	}()
	var pid int
	require.Eventually(t, func() bool {
		select {
		case e := <-done:
			t.Fatalf("projection bypassed FIRST close gate: %v", e)
		default:
		}
		return pool.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_catalog.pg_advisory_xact_lock_shared%' LIMIT 1`).Scan(&pid) == nil
	}, 3*time.Second, 10*time.Millisecond)
	var locks int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND granted`, pid).Scan(&locks))
	require.Zero(t, locks)
	require.NoError(t, closer.Commit(ctx))
	require.ErrorIs(t, <-done, mmadmission.ErrUnavailable)
}
func (q *nativeProjectionQueueFixture) RecoverRelease(context.Context, queue.SearchRecovery) error {
	q.release++
	return nil
}

func TestNativeProjectorCannotEnqueueWithoutFreshSourceAndAdmission(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool, NativeAdmission: &binding}
	_, err := parties.CreateSourceParty(ctx, p)
	require.NoError(t, err)
	queue := &nativeProjectionQueueFixture{}
	claimed, err := (&Projector{Store: parties, Queue: queue}).RunOnce(ctx)
	require.True(t, claimed)
	require.Error(t, err, "native positive projection requires genuine fresh validation, not only RR source shape")
	require.Zero(t, queue.enqueue)
}
