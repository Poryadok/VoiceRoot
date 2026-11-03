package threadlist

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func coveredRemoval(e MembershipRecord) MembershipRecord {
	r := e
	r.EventID, r.Offset, r.Revision, r.Kind = uuid.New(), 3, 2, MemberRemoved
	r.Sources = append([]MembershipSource(nil), e.Sources...)
	r.Sources[0].Revision = 2
	r.Sources[0].RevocationRevision = 1
	return r
}

func TestPostgresBaselineCannotDiscardBufferedRevocation(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	require.NoError(t, inbox.Apply(ctx, e))
	removed := coveredRemoval(e)
	require.NoError(t, inbox.Apply(ctx, removed), "Q2 is missing, so Q3 is durable but not applied")
	bad := baselineMember(e)
	bad.Revision = 3
	unsafe, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 5, bad)
	_, err := inbox.InstallBaseline(ctx, unsafe)
	require.ErrorIs(t, err, ErrMembershipConflict, "the seal cannot erase a received removal of this lineage")
	good := baselineMember(e)
	good.Revision = 3
	good.Lineage = uuid.New()
	good.Sources[0].Revision = 3
	good.Sources[0].RevocationRevision = 1
	fresh, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 5, good)
	_, err = inbox.InstallBaseline(ctx, fresh)
	require.NoError(t, err)
	pair, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 5)
	require.NoError(t, err)
	require.Equal(t, good.Lineage, pair.Lineage)
	var retired bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_retired_lineages WHERE chat_id=$1 AND profile_id=$2 AND lineage=$3)`, e.ChatID, e.ProfileID, e.Lineage).Scan(&retired))
	require.True(t, retired)
}

func TestPostgresCoveredLateRemovalQuarantinesAResurrectedNonce(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	m := baselineMember(e)
	m.Revision = 3
	m.Sources[0].Revision = 5
	m.Sources[0].RevocationRevision = 1
	snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, m)
	_, err := inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
	require.ErrorIs(t, inbox.Apply(ctx, coveredRemoval(e)), ErrMembershipConflict)
	checkpoint, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.True(t, checkpoint.Quarantined)
	_, err = inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 10)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
}

func TestPostgresCoveredLateRemovalPreservesNewLineageAndExactIdentity(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	m := baselineMember(e)
	m.Revision = 3
	m.Lineage = uuid.New()
	m.Sources[0].Revision = 5
	m.Sources[0].RevocationRevision = 1
	snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, m)
	_, err := inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
	removed := coveredRemoval(e)
	require.NoError(t, inbox.Apply(ctx, removed))
	require.NoError(t, inbox.Apply(ctx, removed))
	pair, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 10)
	require.NoError(t, err)
	require.Equal(t, m.Lineage, pair.Lineage)
	var retired bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_retired_lineages WHERE chat_id=$1 AND profile_id=$2 AND lineage=$3)`, e.ChatID, e.ProfileID, e.Lineage).Scan(&retired))
	require.True(t, retired)
	removed.Kind = MemberJoined
	require.ErrorIs(t, inbox.Apply(ctx, removed), ErrMembershipConflict, "covered history still retains exact event identity")
}

func TestPostgresCoveredHistoricalJoinNeverBecomesANewGrant(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10)
	_, err := inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
	require.NoError(t, inbox.Apply(ctx, e))
	var live, retired bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_membership_pairs WHERE chat_id=$1 AND profile_id=$2 AND status='BUILDING')`, e.ChatID, e.ProfileID).Scan(&live))
	require.False(t, live)
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM thread_list_retired_lineages WHERE chat_id=$1 AND profile_id=$2 AND lineage=$3)`, e.ChatID, e.ProfileID, e.Lineage).Scan(&retired))
	require.True(t, retired)
}

func TestPostgresBaselineCapabilityRequiresFreshIntakeForAnotherPool(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	snapshot, source := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, baselineMember(e))
	otherPool, err := pgxpool.NewWithConfig(ctx, pool.Config())
	require.NoError(t, err)
	t.Cleanup(otherPool.Close)
	other := MembershipInbox{Pool: otherPool}
	_, err = other.InstallBaseline(ctx, snapshot)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
	fresh, err := other.ReceiveBaseline(ctx, source, e.ChatID)
	require.NoError(t, err)
	_, err = other.InstallBaseline(ctx, fresh)
	require.NoError(t, err)
}

func TestPostgresBaselineCannotHideNonceReuseInBufferedHistory(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	require.NoError(t, inbox.Apply(ctx, e))
	removed := coveredRemoval(e)
	require.NoError(t, inbox.Apply(ctx, removed))
	joined := removed
	joined.Sources = append([]MembershipSource(nil), removed.Sources...)
	joined.EventID, joined.Offset, joined.Revision, joined.Kind, joined.Lineage = uuid.New(), 4, 3, MemberJoined, uuid.New()
	joined.Sources[0].Revision = 3
	require.NoError(t, inbox.Apply(ctx, joined))
	removedAgain := joined
	removedAgain.Sources = append([]MembershipSource(nil), joined.Sources...)
	removedAgain.EventID, removedAgain.Offset, removedAgain.Revision, removedAgain.Kind = uuid.New(), 5, 4, MemberRemoved
	removedAgain.Sources[0].Revision = 4
	removedAgain.Sources[0].RevocationRevision = 2
	require.NoError(t, inbox.Apply(ctx, removedAgain))
	reuse := removedAgain
	reuse.Sources = append([]MembershipSource(nil), removedAgain.Sources...)
	reuse.EventID, reuse.Offset, reuse.Revision, reuse.Kind, reuse.Lineage = uuid.New(), 6, 5, MemberJoined, e.Lineage
	reuse.Sources[0].Revision = 5
	require.NoError(t, inbox.Apply(ctx, reuse))
	m := baselineMember(e)
	m.Revision = 6
	m.Lineage = uuid.New()
	m.Sources[0].Revision = 6
	m.Sources[0].RevocationRevision = 2
	snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 8, m)
	_, err := inbox.InstallBaseline(ctx, snapshot)
	require.ErrorIs(t, err, ErrMembershipConflict, "a later fresh lineage cannot hide a known ABA inside the covered gap")
}

func TestPostgresCoveredJoinCannotHideInterveningFenceChange(t *testing.T) {
	for _, change := range []string{"revocation", "incarnation"} {
		t.Run(change, func(t *testing.T) {
			pool := membershipDB(t)
			inbox := MembershipInbox{Pool: pool}
			ctx := context.Background()
			e := membershipFixture()
			require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
			m := baselineMember(e)
			m.Revision = 3
			if change == "revocation" {
				m.Sources[0].Revision = 5
				m.Sources[0].RevocationRevision = 1
			} else {
				m.Sources[1].Incarnation = uuid.New()
			}
			snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, m)
			_, err := inbox.InstallBaseline(ctx, snapshot)
			require.NoError(t, err)
			require.ErrorIs(t, inbox.Apply(ctx, e), ErrMembershipConflict, "no continuity proof allows this same nonce across the new fence")
			c, err := inbox.Checkpoint(ctx, e.ChatID)
			require.NoError(t, err)
			require.True(t, c.Quarantined)
		})
	}
}

func TestPostgresReplaySecondBatchConflictIsDurablyQuarantined(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	first := membershipFixture()
	require.NoError(t, inbox.Track(ctx, first.ChatID, first.Incarnation))
	for offset := int64(2); offset <= 257; offset++ {
		e := membershipFixture()
		e.ChatID, e.Incarnation, e.Offset = first.ChatID, first.Incarnation, offset
		e.Sources[0] = first.Sources[0]
		require.NoError(t, inbox.Apply(ctx, e))
	}
	bad := first
	bad.EventID, bad.Offset, bad.Revision = uuid.New(), 258, 2
	require.NoError(t, inbox.Apply(ctx, bad))
	require.NoError(t, inbox.Apply(ctx, first))
	_, err := inbox.Replay(ctx, first.ChatID, first.Incarnation)
	require.ErrorIs(t, err, ErrMembershipConflict)
	c, err := inbox.Checkpoint(ctx, first.ChatID)
	require.NoError(t, err)
	require.EqualValues(t, 257, c.AppliedQ)
	require.True(t, c.Quarantined)
	restarted := MembershipInbox{Pool: pool}
	_, err = restarted.Pair(ctx, first.ChatID, first.Incarnation, first.ProfileID, 257)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
}
