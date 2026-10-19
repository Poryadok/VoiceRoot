package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/memberadmission"
)

type memberAdmissionWriter interface {
	PrepareMemberAdmission(context.Context, MemberAdmissionInput) (*MemberAdmissionIntent, error)
	PublishMemberAdmission(context.Context, uuid.UUID, uuid.UUID, []byte, MemberAdmissionFinalGuard) (*MembershipRow, error)
}

func admissionWriter(t *testing.T, st *SpaceStore) memberAdmissionWriter {
	t.Helper()
	w, ok := any(st).(memberAdmissionWriter)
	require.True(t, ok, "Space must own durable inert preparation and one final publication transaction")
	return w
}

// Store fixtures simulate an already authenticated Role READY receipt; these
// tests assert local SQL behavior and do not claim a live Role RPC path.
func publishAdmissionFixture(t *testing.T, st *SpaceStore, space, owner, profile uuid.UUID) *MembershipRow {
	t.Helper()
	ctx := context.Background()
	invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space, CreatorProfileID: owner})
	require.NoError(t, err)
	i, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space, ProfileID: profile, AccountID: uuid.New(), InviteID: &invite.ID})
	require.NoError(t, err)
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
	require.NoError(t, err)
	m, err := st.PublishMemberAdmission(ctx, i.Input.OperationID, role, receipt[:], nil)
	require.NoError(t, err)
	return m
}

func TestMemberAdmissionPreparationIsInvisibleAndPublicationIsAtomic(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Inert admission", "", "public")
	require.NoError(t, err)
	invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	w := admissionWriter(t, st)
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: uuid.New(), AccountID: uuid.New(), InviteID: &invite.ID}
	intent, err := w.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	require.Positive(t, intent.Generation)
	require.Equal(t, "PREPARING", intent.State)
	_, err = pool.Exec(ctx, `UPDATE space_ordinary_member_heads SET admission_binding_sha256=$3 WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID, make([]byte, 32))
	require.Error(t, err, "a prepared immutable binding cannot be replaced in place")
	for _, mutation := range []string{
		`account_id=gen_random_uuid()`,
		`guest=NOT guest`,
		`request_sha256=decode(repeat('09',32),'hex')`,
		`generation=generation+1`,
	} {
		_, err = pool.Exec(ctx, `UPDATE space_member_admission_intents SET `+mutation+` WHERE operation_id=$1`, in.OperationID)
		require.Error(t, err, "native immutable input/provenance cannot be rebound after preparation: %s", mutation)
	}
	member, err := st.IsSpaceMember(ctx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.False(t, member)
	gotInvite, err := st.GetInviteByID(ctx, invite.ID)
	require.NoError(t, err)
	require.Zero(t, gotInvite.UseCount)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 1, count)
	retry, err := w.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	require.Equal(t, intent, retry, "retry cannot allocate another generation")
	roleID := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), roleID.String())
	require.NoError(t, err)
	guardFailure := ErrInviteRevoked
	_, err = w.PublishMemberAdmission(ctx, in.OperationID, roleID, receipt[:], func(context.Context, *SpaceStore, *MemberAdmissionIntent) error { return guardFailure })
	require.ErrorIs(t, err, guardFailure)
	member, err = st.IsSpaceMember(ctx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.False(t, member, "failed final guard cannot expose temporary membership")
	_, err = w.PublishMemberAdmission(ctx, in.OperationID, roleID, receipt[:], nil)
	require.Error(t, err, "a denied final attempt is retired; a new attempt needs a new generation")
	in.OperationID = uuid.New()
	intent, err = w.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	require.Greater(t, intent.Generation, retry.Generation)
	receipt, err = memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), roleID.String())
	require.NoError(t, err)
	wrong := append([]byte(nil), receipt[:]...)
	wrong[0] ^= 1
	_, err = w.PublishMemberAdmission(ctx, in.OperationID, roleID, wrong, nil)
	require.Error(t, err)
	published, err := w.PublishMemberAdmission(ctx, in.OperationID, roleID, receipt[:], nil)
	require.NoError(t, err)
	require.Equal(t, in.ProfileID, published.ProfileID)
	_, err = pool.Exec(ctx, `UPDATE space_ordinary_member_heads SET role_receipt_sha256=$3 WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID, make([]byte, 32))
	require.Error(t, err, "published provenance cannot be replaced without a new ordinary generation")
	gotInvite, err = st.GetInviteByID(ctx, invite.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, gotInvite.UseCount)
	_, err = w.PublishMemberAdmission(ctx, in.OperationID, roleID, receipt[:], nil)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	var events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$1`, in.OperationID).Scan(&events))
	require.Equal(t, 1, events)
	pending, err := st.ListPendingMemberAdmissionEvents(ctx, 100)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	wrongEvent := pending[0]
	wrongEvent.Generation++
	require.ErrorIs(t, st.MarkMemberAdmissionEventDelivered(ctx, wrongEvent, "chat_events", 9), ErrMemberAdmissionConflict)
	require.ErrorIs(t, st.MarkMemberAdmissionEventDelivered(ctx, pending[0], "other_stream", 9), ErrMemberAdmissionConflict)
	require.NoError(t, st.MarkMemberAdmissionEventDelivered(ctx, pending[0], "chat_events", 9))
	require.NoError(t, st.MarkMemberAdmissionEventDelivered(ctx, pending[0], "chat_events", 9), "same first PubAck replays")
	require.ErrorIs(t, st.MarkMemberAdmissionEventDelivered(ctx, pending[0], "chat_events", 10), ErrMemberAdmissionConflict, "saved first acknowledgement cannot be rebound")
	pending, err = st.ListPendingMemberAdmissionEvents(ctx, 100)
	require.NoError(t, err)
	require.Empty(t, pending)
	require.NoError(t, st.RemoveMember(ctx, space.ID, in.ProfileID))
	_, err = w.PublishMemberAdmission(ctx, in.OperationID, roleID, receipt[:], nil)
	require.NoError(t, err, "completed operation may return historical outcome")
	member, err = st.IsSpaceMember(ctx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.False(t, member, "historical publication replay cannot recreate membership")
}

func TestMemberAdmissionUnavailableGuardPreservesCandidate(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Guard retry", "", "public")
	require.NoError(t, err)
	invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New(), InviteID: &invite.ID}
	i, err := st.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
	require.NoError(t, err)
	for _, failure := range []error{context.DeadlineExceeded, ErrMemberAdmissionConflict, errors.New("owner observation unavailable")} {
		_, err = st.PublishMemberAdmission(ctx, in.OperationID, role, receipt[:], func(ctx context.Context, scoped *SpaceStore, _ *MemberAdmissionIntent) error {
			_, writeErr := scoped.db().Exec(ctx, `UPDATE invites SET use_count=use_count+1 WHERE id=$1`, invite.ID)
			require.NoError(t, writeErr)
			return failure
		})
		require.ErrorIs(t, err, failure)
		var state string
		var generation, floor int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT state,generation,retired_through FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID).Scan(&state, &generation, &floor))
		require.Equal(t, "PREPARING", state, "an unavailable observation is not a known denial")
		require.EqualValues(t, i.Generation, generation)
		require.Zero(t, floor)
		replay, err := st.PrepareMemberAdmission(ctx, in)
		require.NoError(t, err)
		require.Equal(t, i, replay)
		var cleanups, members, events, uses int
		require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1 AND profile_id=$2),(SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2),(SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$3),(SELECT use_count FROM invites WHERE id=$4)`, in.SpaceID, in.ProfileID, in.OperationID, invite.ID).Scan(&cleanups, &members, &events, &uses))
		require.Zero(t, cleanups)
		require.Zero(t, members)
		require.Zero(t, events)
		require.Zero(t, uses, "a failing guard's SQL must roll back")
	}
	_, err = st.PublishMemberAdmission(ctx, in.OperationID, role, receipt[:], nil)
	require.NoError(t, err, "same candidate may publish after a fresh successful observation")
}
