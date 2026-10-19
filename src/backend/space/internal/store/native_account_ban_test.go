package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/space/internal/entrypolicy"
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

func TestNativeAccountBanRejectsUnknownOwnerAndForeignEvictionHint(t *testing.T) {
	for _, legacyOwner := range []bool{true, false} {
		name := "foreign account hint"
		if legacyOwner {
			name = "unknown legacy owner"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var st *SpaceStore
			var space *SpaceRow
			var owner uuid.UUID
			if legacyOwner {
				pool := startSpacePostgresForStoreTest(t, ctx)
				applySpaceMigrationForStoreTest(t, ctx, pool)
				st = &SpaceStore{Pool: pool}
				owner = uuid.New()
				var err error
				space, err = st.CreateSpace(ctx, owner, "Unqualified owner", "", "public")
				require.NoError(t, err)
				applyMemberAuthorityIfPresent(t, st)
				_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(33,false)`)
				require.NoError(t, err)
			} else {
				st = nativeCreatorFixture(t)
				create, _, actor := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Exact eviction", Visibility: "public"})
				owner = actor
				var err error
				space, err = create()
				require.NoError(t, err)
			}
			account := uuid.New()
			alias := nativeBanAliasFixture(t, st, space.ID, account)
			var hint *uuid.UUID
			if !legacyOwner {
				foreign := nativeBanAliasFixture(t, st, space.ID, uuid.New())
				hint = &foreign.Input.ProfileID
			}
			var before int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&before))
			err := st.BanMember(ctx, space.ID, account, owner, nil, hint)
			if legacyOwner {
				require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "unknown owner association is not proof that target account is not Owner")
			} else {
				require.ErrorIs(t, err, ErrMemberAdmissionConflict, "an eviction hint cannot select another account's profile")
			}
			var bans, audit, cleanups, after int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_bans WHERE space_id=$1),(SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'),(SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1),(SELECT member_count FROM spaces WHERE id=$1)`, space.ID).Scan(&bans, &audit, &cleanups, &after))
			require.Zero(t, bans)
			require.Zero(t, audit)
			require.Zero(t, cleanups)
			require.Equal(t, before, after)
			member, err := st.IsSpaceMember(ctx, space.ID, alias.Input.ProfileID)
			require.NoError(t, err)
			require.True(t, member)
			if hint != nil {
				member, err = st.IsSpaceMember(ctx, space.ID, *hint)
				require.NoError(t, err)
				require.True(t, member)
			}
		})
	}
}

func nativeBanAliasFixture(t *testing.T, st *SpaceStore, space, account uuid.UUID) *MemberAdmissionIntent {
	t.Helper()
	ctx := context.Background()
	i, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space, ProfileID: uuid.New(), AccountID: account})
	require.NoError(t, err)
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
	require.NoError(t, err)
	_, err = st.PublishMemberAdmission(ctx, i.Input.OperationID, role, receipt[:], nil)
	require.NoError(t, err)
	return i
}

func TestNativeAccountBanCommunityCaptureUnavailableHasNoPartialEffect(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	in := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "ban-coverage-community", TemplateID: "guild-default"}
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	target := uuid.New()
	require.ErrorIs(t, st.BanMember(ctx, space.ID, target, in.OwnerProfileID, nil, nil), ErrMemberAuthorityUnavailable, "ordinary known Owner does not qualify retained/future community episodes or suppression")
	var bans, audit, cleanup, count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_bans WHERE space_id=$1),(SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'),(SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1),(SELECT member_count FROM spaces WHERE id=$1)`, space.ID).Scan(&bans, &audit, &cleanup, &count))
	require.Zero(t, bans)
	require.Zero(t, audit)
	require.Zero(t, cleanup)
	require.Equal(t, 1, count)
}

func TestNativeAccountBanInvalidatesUnboundAttemptAndRollsBackUnknownClaimed(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	create, _, owner := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Ban35 journal", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	applyJoinAttemptMigration(t, st)
	_, err = st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=35`)
	require.NoError(t, err)
	question := uuid.New()
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{{ID: question, Prompt: "2x2", Kind: "exact_text", ExpectedAnswer: "4"}}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	other, err := st.StartJoinAttempt(ctx, JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()})
	require.NoError(t, err)
	require.NoError(t, st.BanMember(ctx, space.ID, in.AccountID, owner, nil, nil))
	require.NoError(t, st.UnbanMember(ctx, space.ID, in.AccountID, owner))
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "INVALIDATED", state, "ban then unban cannot preserve an unbound historical decision")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, other.ID).Scan(&state))
	require.Equal(t, "COLLECTING", state)
	fresh, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	require.NotEqual(t, a.ID, fresh.ID)
	ready, err := st.SubmitJoinAttempt(ctx, in, fresh.ID, []entrypolicy.Answer{{QuestionID: question, Value: "4"}})
	require.NoError(t, err)
	policy, err := st.GetEntryPolicy(ctx, space.ID)
	require.NoError(t, err)
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackLifecycleTx(ctx, tx)
	require.NoError(t, lockLifecycleSpace(ctx, tx, space.ID))
	scoped := *st
	scoped.tx = tx
	publication := uuid.New()
	require.NoError(t, scoped.VerifyJoinAttemptAdmission(ctx, in, JoinAttemptReference{AttemptID: ready.ID, EvidenceSHA256: ready.EvidenceSHA256}, policy.Version, publication))
	require.NoError(t, tx.Commit(ctx))
	// Deliberately unresolved native P: primitive35 CLAIMED is not a proof of
	// native retirement. Public Ban must roll back its row and every local loss.
	require.ErrorIs(t, st.BanMember(ctx, space.ID, in.AccountID, owner, nil, nil), ErrJoinAttemptUnavailable)
	var bans int
	var savedPublication uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, space.ID, in.AccountID).Scan(&bans))
	require.Zero(t, bans)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,publication_operation_id FROM space_join_attempts WHERE id=$1`, ready.ID).Scan(&state, &savedPublication))
	require.Equal(t, "CLAIMED", state)
	require.Equal(t, publication, savedPublication)
}
