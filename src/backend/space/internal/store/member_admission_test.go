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
	guardFailure := errors.New("current targeted authority changed")
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
	require.NoError(t, st.RemoveMember(ctx, space.ID, in.ProfileID))
	_, err = w.PublishMemberAdmission(ctx, in.OperationID, roleID, receipt[:], nil)
	require.NoError(t, err, "completed operation may return historical outcome")
	member, err = st.IsSpaceMember(ctx, space.ID, in.ProfileID)
	require.NoError(t, err)
	require.False(t, member, "historical publication replay cannot recreate membership")
}
