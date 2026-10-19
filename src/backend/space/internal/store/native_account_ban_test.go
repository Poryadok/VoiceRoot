package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
)

func TestNativeAccountBanRetiresEveryKnownAliasWithoutResurrection(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	create, ownerAccount, owner := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Account ban", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	account := uuid.New()
	var aliases []*MemberAdmissionIntent
	for index := range 3 {
		i, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: uuid.New(), AccountID: account})
		require.NoError(t, err)
		aliases = append(aliases, i)
		if index < 2 {
			role := uuid.New()
			receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
			require.NoError(t, err)
			_, err = st.PublishMemberAdmission(ctx, i.Input.OperationID, role, receipt[:], nil)
			require.NoError(t, err)
		}
	}
	unrelated := uuid.New()
	publishAdmissionFixture(t, st, space.ID, owner, unrelated)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION reject_native_ban_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='member_banned' THEN RAISE EXCEPTION 'injected ban audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_native_ban_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_native_ban_audit()`)
	require.NoError(t, err)
	require.Error(t, st.BanMember(ctx, space.ID, account, owner, nil, nil))
	var cleanups int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1`, space.ID).Scan(&cleanups))
	require.Zero(t, cleanups, "failed audit rolls back both published losses and inert candidate floors")
	for index, i := range aliases {
		member, err := st.IsSpaceMember(ctx, space.ID, i.Input.ProfileID)
		require.NoError(t, err)
		require.Equal(t, index < 2, member)
		var floor uint64
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_through FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, i.Input.ProfileID).Scan(&floor))
		require.Zero(t, floor)
	}
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER reject_native_ban_audit ON audit_log`)
	require.NoError(t, err)
	require.NoError(t, st.BanMember(ctx, space.ID, account, owner, nil, nil))
	require.NoError(t, st.BanMember(ctx, space.ID, account, owner, nil, nil))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1`, space.ID).Scan(&cleanups))
	require.Equal(t, 3, cleanups, "repeated ban cannot retire another incarnation or queue duplicate losses")
	require.NoError(t, st.UnbanMember(ctx, space.ID, account, owner))
	for _, i := range aliases {
		member, err := st.IsSpaceMember(ctx, space.ID, i.Input.ProfileID)
		require.NoError(t, err)
		require.False(t, member, "account ban removes every known published alias and keeps pending aliases invisible")
		var floor uint64
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_through FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, i.Input.ProfileID).Scan(&floor))
		require.GreaterOrEqual(t, floor, i.Generation)
	}
	member, err := st.IsSpaceMember(ctx, space.ID, unrelated)
	require.NoError(t, err)
	require.True(t, member, "a different account is not an alias")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	require.ErrorIs(t, st.BanMember(ctx, space.ID, ownerAccount, owner, nil, nil), ErrCannotBanOwner, "nil selected eviction cannot bypass known Owner account protection")
}
