package threadlist

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/effectivemembership"
)

func TestBaselineSourceContinuityNeverAssumesAwayRevocations(t *testing.T) {
	e := membershipFixture()
	old := knownMembership{member: baselineMember(e)}
	for _, kind := range []string{"unchanged", "grant_only", "restriction_old_lineage", "restriction_fresh_lineage", "regressing_revision", "regressing_counter", "same_revision_changed_counter", "changed_incarnation", "changed_incarnation_fresh_lineage", "new_dependency", "denied_old_lineage"} {
		t.Run(kind, func(t *testing.T) {
			previous := old
			next := baselineMember(e)
			next.Revision++
			var expected error
			switch kind {
			case "grant_only":
				next.Sources[1].Revision++
			case "restriction_old_lineage":
				next.Sources[1].Revision++
				next.Sources[1].RevocationRevision++
				expected = ErrMembershipUnavailable
			case "restriction_fresh_lineage":
				next.Sources[1].Revision++
				next.Sources[1].RevocationRevision++
				next.Lineage = uuid.New()
			case "regressing_revision":
				next.Sources[1].Revision = 0
				expected = ErrMembershipConflict
			case "regressing_counter":
				previous.member = baselineMember(e)
				previous.member.Sources[1].RevocationRevision = 1
				next.Sources[1].Revision = 2
				expected = ErrMembershipConflict
			case "same_revision_changed_counter":
				next.Sources[1].RevocationRevision = 1
				expected = ErrMembershipConflict
			case "changed_incarnation":
				next.Sources[1].Incarnation = uuid.New()
				expected = ErrMembershipUnavailable
			case "changed_incarnation_fresh_lineage":
				next.Sources[1].Incarnation = uuid.New()
				next.Lineage = uuid.New()
			case "new_dependency":
				id := uuid.New()
				next.Sources = append(next.Sources, effectivemembership.Source{Owner: "space", Scope: id, Incarnation: uuid.New(), Revision: 1}, effectivemembership.Source{Owner: "role", Scope: id, Incarnation: uuid.New(), Revision: 1})
				expected = ErrMembershipUnavailable
			case "denied_old_lineage":
				previous.member = baselineMember(e)
				previous.member.Allowed = false
				expected = ErrMembershipUnavailable
			}
			err := checkBaselineTransition(previous, next, false)
			if expected == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, expected)
			}
		})
	}
}

func TestPostgresBaselineHistoricalRetryAndIdentityCollision(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	_, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	first, source := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, baselineMember(e))
	_, err = inbox.InstallBaseline(ctx, first)
	require.NoError(t, err)
	current, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.EqualValues(t, 10, current.AppliedQ)
	second, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 12, baselineMember(e))
	_, err = inbox.InstallBaseline(ctx, second)
	require.NoError(t, err)
	retried, err := inbox.InstallBaseline(ctx, first)
	require.NoError(t, err)
	require.False(t, retried.Current)
	current, err = inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.EqualValues(t, 12, current.AppliedQ)
	source.h.Q = 13
	source.h.Digest = effectivemembership.ContentDigest(source.h, source.members)
	conflict, err := inbox.ReceiveBaseline(ctx, source, e.ChatID)
	require.NoError(t, err)
	_, err = inbox.InstallBaseline(ctx, conflict)
	require.ErrorIs(t, err, ErrMembershipConflict)
	current, err = inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.True(t, current.Quarantined)
	require.EqualValues(t, 12, current.AppliedQ)
}

func TestPostgresBaselineNewIncarnationRetiresPreviousIdentity(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	require.NoError(t, inbox.Apply(ctx, e))
	_, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	oldEmpty, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10)
	inc := uuid.New()
	member := baselineMember(e)
	member.Sources[0].Incarnation = inc
	unsafe, _ := baselineFixture(t, inbox, e.ChatID, inc, 0, member)
	_, err = inbox.InstallBaseline(ctx, unsafe)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
	member.Lineage = uuid.New()
	fresh, _ := baselineFixture(t, inbox, e.ChatID, inc, 0, member)
	_, err = inbox.InstallBaseline(ctx, fresh)
	require.NoError(t, err)
	require.ErrorIs(t, inbox.Apply(ctx, e), ErrMembershipUnavailable)
	current, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.Equal(t, inc, current.Incarnation)
	// Even a fresh local checkpoint cannot make a retired source incarnation current.
	_, err = inbox.InstallBaseline(ctx, oldEmpty)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
	current, err = inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.Equal(t, inc, current.Incarnation)
}

func TestPostgresBaselineCASAndWriteFailureKeepPriorState(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	_, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)

	member := baselineMember(e)
	member.Revision++
	member.Lineage = uuid.New()
	snapshot, source := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, member)
	require.NoError(t, inbox.Apply(ctx, e))
	_, err = inbox.InstallBaseline(ctx, snapshot)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
	current, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	snapshot, err = inbox.ReceiveBaseline(ctx, source, e.ChatID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_baseline() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected baseline failure'; END $$;
CREATE TRIGGER reject_baseline BEFORE INSERT ON thread_list_membership_snapshot_pairs FOR EACH ROW EXECUTE FUNCTION reject_baseline()`)
	require.NoError(t, err)
	_, err = inbox.InstallBaseline(ctx, snapshot)
	require.ErrorContains(t, err, "injected baseline failure")
	after, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.Equal(t, current, after)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM thread_list_membership_baselines`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM thread_list_retired_lineages`).Scan(&count))
	require.Zero(t, count)
	_, err = pool.Exec(ctx, `DROP TRIGGER reject_baseline ON thread_list_membership_snapshot_pairs`)
	require.NoError(t, err)
	_, err = inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
}

func TestPostgresBaselineExpiredPinCannotCommitThroughReplayQuarantine(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	current, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	tail := e
	tail.EventID, tail.Offset, tail.Revision = uuid.New(), 11, 2
	require.NoError(t, inbox.Apply(ctx, tail))
	_, source := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, baselineMember(e))
	_, err = pool.Exec(ctx, `CREATE FUNCTION delay_baseline() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(3); RETURN NEW; END $$;
CREATE TRIGGER delay_baseline BEFORE INSERT ON thread_list_membership_snapshot_pairs FOR EACH ROW EXECUTE FUNCTION delay_baseline()`)
	require.NoError(t, err)
	source.h.RetainUntil = time.Now().Add(2 * time.Second)
	snapshot, err := inbox.ReceiveBaseline(ctx, source, e.ChatID)
	require.NoError(t, err)
	_, err = inbox.InstallBaseline(ctx, snapshot)
	require.ErrorIs(t, err, ErrMembershipUnavailable, "the pin deadline still applies when buffered replay finds a conflict")
	after, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.Equal(t, current, after)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM thread_list_membership_baselines`).Scan(&count))
	require.Zero(t, count)
}

func TestPostgresBaselineExpiredPinCannotCommitIdentityQuarantine(t *testing.T) {
	for _, collision := range []string{"snapshot", "generation"} {
		t.Run(collision, func(t *testing.T) {
			pool := membershipDB(t)
			inbox := MembershipInbox{Pool: pool}
			ctx := context.Background()
			e := membershipFixture()
			require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
			first, source := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, baselineMember(e))
			_, err := inbox.InstallBaseline(ctx, first)
			require.NoError(t, err)
			before, err := inbox.Checkpoint(ctx, e.ChatID)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `CREATE FUNCTION delay_quarantine() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(3); RETURN NEW; END $$;
CREATE TRIGGER delay_quarantine BEFORE UPDATE OF quarantined ON thread_list_membership_heads FOR EACH ROW WHEN (NEW.quarantined AND NOT OLD.quarantined) EXECUTE FUNCTION delay_quarantine()`)
			require.NoError(t, err)
			if collision == "generation" {
				source.h.SnapshotID = uuid.New()
			}
			source.h.Q = 11
			source.h.RetainUntil = time.Now().Add(2 * time.Second)
			source.h.Digest = effectivemembership.ContentDigest(source.h, source.members)
			bad, err := inbox.ReceiveBaseline(ctx, source, e.ChatID)
			require.NoError(t, err)
			_, err = inbox.InstallBaseline(ctx, bad)
			require.ErrorIs(t, err, ErrMembershipUnavailable)
			after, err := inbox.Checkpoint(ctx, e.ChatID)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}
