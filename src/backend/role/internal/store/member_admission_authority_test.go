package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/role/internal/memberauthority"
)

func preparingObservation(t *testing.T, binding memberadmission.EnsureBinding, owner uuid.UUID, validity time.Duration) memberauthority.Observation {
	t.Helper()
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: binding.SpaceID, ProfileId: binding.ProfileID}
	reply := &spacev1.GetMemberAdmissionAuthorityResponse{ProtocolVersion: 1, SpaceId: binding.SpaceID, ProfileId: binding.ProfileID, SpaceAuthorityRevision: 1,
		Lifecycle: &spacev1.MemberAdmissionLifecycleAuthority{State: spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE},
		Ownership: &spacev1.MemberAdmissionOwnershipAuthority{Generation: 1, State: spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE, CurrentOwnerProfileId: owner.String()},
		Ordinary: &spacev1.MemberAdmissionOrdinaryAuthority{HeadGeneration: binding.MembershipGeneration, State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING,
			PreparingBinding: &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: binding.MembershipGeneration, PublicationIdentity: binding.AdmissionOperationID}}}, PreparingAdmissionBindingSha256: binding.AdmissionBindingSHA256},
		Community: &spacev1.MemberAdmissionCommunityAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE}, SampledAt: timestamppb.Now(), ValidityRemaining: durationpb.New(validity)}
	now := time.Now()
	observation, err := memberauthority.Parse(req, reply, now, now)
	require.NoError(t, err)
	return observation
}

func TestMemberAdmissionQualifiedPreparationAndHistoricalReplay(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	_, err := st.PrepareQualifiedDefaultMemberRole(ctx, binding, memberauthority.Observation{})
	require.ErrorIs(t, err, ErrScopeUnavailable)
	observation := preparingObservation(t, binding, owner, 5*time.Second)
	receipt, err := st.PrepareQualifiedDefaultMemberRole(ctx, binding, observation)
	require.NoError(t, err)
	wrong := binding
	wrong.AdmissionBindingSHA256 = append([]byte(nil), binding.AdmissionBindingSHA256...)
	wrong.AdmissionBindingSHA256[0] = 1
	_, err = st.PrepareQualifiedDefaultMemberRole(ctx, wrong, observation)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict)
	_, err = st.PrepareQualifiedDefaultMemberRole(ctx, ensureMemberBinding(space, uuid.New(), 1), observation)
	require.ErrorIs(t, err, ErrScopeUnavailable, "observation cannot cross subject")

	// A currently published source is not permission to mint a fresh READY, but
	// it permits returning an existing exact historical receipt without effects.
	publishedContext := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	published, ok := memberauthority.FromContext(publishedContext, space.String(), profile.String())
	require.True(t, ok)
	replay, err := st.PrepareQualifiedDefaultMemberRole(ctx, binding, published)
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	next := ensureMemberBinding(space, profile, 2)
	_, err = st.PrepareQualifiedDefaultMemberRole(ctx, next, published)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	removal := removeMemberBinding(space, profile, 1)
	_, err = st.RemoveQualifiedOrdinaryMemberRoles(ctx, removal, published)
	require.NoError(t, err)
	replay, err = st.PrepareQualifiedDefaultMemberRole(ctx, binding, published)
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_preparations WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count, "historical receipt must not recreate removed preparation")
}

func TestMemberAdmissionObservationExpiresDuringActualLockWait(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	for _, kind := range []string{"ensure", "remove"} {
		t.Run(kind, func(t *testing.T) {
			binding := ensureMemberBinding(space, profile, 1)
			tx, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(context.Background())
			_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", space.String())
			require.NoError(t, err)
			app := "member-authority-expiry-" + kind
			second := r22SecondRolePool(t, ctx, st.Pool, app)
			observation := preparingObservation(t, binding, owner, 250*time.Millisecond)
			result := make(chan error, 1)
			go func() {
				other := &RoleStore{Pool: second}
				var err error
				if kind == "ensure" {
					_, err = other.PrepareQualifiedDefaultMemberRole(ctx, binding, observation)
				} else {
					_, err = other.RemoveQualifiedOrdinaryMemberRoles(ctx, removeMemberBinding(space, profile, 1), observation)
				}
				result <- err
			}()
			r22WaitForRoleLock(t, ctx, st.Pool, app)
			deadline := time.NewTimer(300 * time.Millisecond)
			<-deadline.C
			require.NoError(t, tx.Commit(ctx))
			require.ErrorIs(t, <-result, ErrScopeUnavailable)
			var count int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM role_member_admission_receipts)+(SELECT count(*) FROM role_member_source_floors)+(SELECT count(*) FROM role_member_admission_preparations)`).Scan(&count))
			require.Zero(t, count, "expired observation must not write after lock wait")
		})
	}
}

func TestMemberAdmissionObservationExpiresDuringWriteRollsBack(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	_, err := st.Pool.Exec(ctx, `CREATE FUNCTION member_admission_test_delay() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.3); RETURN NEW; END $$; CREATE TRIGGER member_admission_test_delay BEFORE INSERT ON role_member_admission_receipts FOR EACH ROW EXECUTE FUNCTION member_admission_test_delay()`)
	require.NoError(t, err)
	binding := ensureMemberBinding(space, profile, 1)
	for _, kind := range []string{"ensure", "remove"} {
		observation := preparingObservation(t, binding, owner, 250*time.Millisecond)
		if kind == "ensure" {
			_, err = st.PrepareQualifiedDefaultMemberRole(ctx, binding, observation)
		} else {
			_, err = st.RemoveQualifiedOrdinaryMemberRoles(ctx, removeMemberBinding(space, profile, 1), observation)
		}
		require.ErrorIs(t, err, ErrScopeUnavailable)
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM role_member_admission_receipts)+(SELECT count(*) FROM role_member_source_floors)+(SELECT count(*) FROM role_member_admission_preparations)`).Scan(&count))
		require.Zero(t, count, "end-of-write expiry rolls back receipt, floor and preparation atomically")
	}
}

func TestMemberAdmissionHistoricalReceiptWaitCannotOutliveObservation(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	_, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	removal := removeMemberBinding(space, profile, 1)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removal)
	require.NoError(t, err)
	for _, kind := range []string{"ensure", "remove"} {
		t.Run(kind, func(t *testing.T) {
			tx, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(context.Background())
			_, err = tx.Exec(ctx, `LOCK TABLE role_member_admission_receipts IN ACCESS EXCLUSIVE MODE`)
			require.NoError(t, err)
			app := "member-historical-expiry-" + kind
			second := r22SecondRolePool(t, ctx, st.Pool, app)
			observation := preparingObservation(t, binding, owner, 5*time.Second)
			result := make(chan error, 1)
			go func() {
				other := &RoleStore{Pool: second}
				var err error
				if kind == "ensure" {
					_, err = other.PrepareQualifiedDefaultMemberRole(ctx, binding, observation)
				} else {
					_, err = other.RemoveQualifiedOrdinaryMemberRoles(ctx, removal, observation)
				}
				result <- err
			}()
			r22WaitForRoleLock(t, ctx, st.Pool, app)
			<-time.NewTimer(5100 * time.Millisecond).C
			require.NoError(t, tx.Commit(ctx))
			require.ErrorIs(t, <-result, ErrScopeUnavailable, "historical receipt read must recheck expiry after its table wait")
		})
	}
}
