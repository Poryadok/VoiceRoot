package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
)

type continuationAuthRPC func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)

func (f continuationAuthRPC) ObserveSpaceAdmissionSubject(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest, _ ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
	return f(ctx, r)
}

func continuationClient(t *testing.T, now *time.Time, accountType string) *admissionauthority.Client {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return &admissionauthority.Client{Issuer: issuer, Clock: func() time.Time { return *now }, Auth: continuationAuthRPC(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_FIRST_PUBLISH, r.Phase)
		require.Nil(t, r.InteractiveSessionEpoch)
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, err)
		echo := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: echo[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: accountType, AccountSecurityRevision: 2, ProfileRevision: 3, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
}

// The journals and Role receipt here are controlled storage fixtures. Only
// the private current Auth capability is obtained through the real client;
// this does not assert assembled Java Auth, mTLS, Gateway or Role activation.
func prepareContinuationFixture(t *testing.T, st *SpaceStore, savedGuest, invite bool) (*MemberAdmissionIntent, uuid.UUID, []byte) {
	t.Helper()
	ctx := context.Background()
	create, _, owner := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Sealed continuation", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET allow_guests=true WHERE id=$1`, space.ID)
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New(), Guest: savedGuest}
	if invite {
		link, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
		require.NoError(t, err)
		in.InviteID = &link.ID
	}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	_, basis := seedJoinDecisionHistory(t, st, in, a)
	i, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: in.AccountID, ProfileID: in.ProfileID, Guest: in.Guest, InviteID: in.InviteID, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: basis})
	require.NoError(t, err)
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
	require.NoError(t, err)
	return i, role, receipt[:]
}

func TestMemberAdmissionContinuationCurrentGuestAndSharedReceipt(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	for _, saved := range []bool{false, true} {
		for _, current := range []string{"regular", "guest"} {
			t.Run(fmt.Sprintf("saved-%t/current-%s", saved, current), func(t *testing.T) {
				i, role, receipt := prepareContinuationFixture(t, st, saved, true)
				now := time.Now()
				client := continuationClient(t, &now, current)
				proof, err := st.ObserveMemberAdmissionContinuation(ctx, i.Input.OperationID, client, nil)
				require.NoError(t, err)
				effective, err := proof.check(i)
				require.NoError(t, err)
				require.Equal(t, saved || current == "guest", effective)
				_, err = st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, &VerifiedAdmissionContinuation{}, nil)
				require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "empty public struct cannot mint private authority")
				calls := 0
				member, err := st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, proof, func(_ context.Context, _ *SpaceStore, view *MemberAdmissionIntent) error {
					calls++
					view.Input.ProfileID = uuid.New()
					view.Generation = 999
					view.BindingSHA256[0] ^= 1
					view.Input.DecisionBasisSHA256[0] ^= 1
					return nil
				})
				require.NoError(t, err)
				require.Equal(t, i.Input.ProfileID, member.ProfileID)
				require.Equal(t, 1, calls)
				savedIntent, err := st.LookupMemberAdmission(ctx, i.Input.OperationID)
				require.NoError(t, err)
				require.Equal(t, i.Input, savedIntent.Input, "effective guest never rewrites A/P or the configured-default receipt binding")
				require.Equal(t, "PUBLISHED", savedIntent.State)
				now = now.Add(6 * time.Second)
				replayed, err := st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, nil, func(context.Context, *SpaceStore, *MemberAdmissionIntent) error {
					t.Fatal("completed replay called hook")
					return nil
				})
				require.NoError(t, err)
				require.Equal(t, member, replayed)
				var attemptState, basisState string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, i.Input.JoinAttempt.AttemptID).Scan(&attemptState))
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT publication_state FROM space_join_operations WHERE decision_basis_sha256=$1`, i.Input.DecisionBasisSHA256).Scan(&basisState))
				require.Equal(t, "PUBLISHED", attemptState)
				require.Equal(t, "PUBLISHED", basisState)
			})
		}
	}
}

func TestMemberAdmissionContinuationExpiryRollsBackEveryPublicationWrite(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	i, role, receipt := prepareContinuationFixture(t, st, false, true)
	now := time.Now()
	client := continuationClient(t, &now, "regular")
	proof, err := st.ObserveMemberAdmissionContinuation(ctx, i.Input.OperationID, client, nil)
	require.NoError(t, err)
	_, err = st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, proof, func(ctx context.Context, scoped *SpaceStore, _ *MemberAdmissionIntent) error {
		_, err := scoped.db().Exec(ctx, `SELECT 1`)
		now = now.Add(5 * time.Second) // Exact earliest deadline, after local SQL.
		return err
	})
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	saved, err := st.LookupMemberAdmission(ctx, i.Input.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", saved.State)
	var member, events, floor, uses int
	var attemptState, basisState string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2),(SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$3),(SELECT retired_through FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2),(SELECT use_count FROM invites WHERE id=$4)`, i.Input.SpaceID, i.Input.ProfileID, i.Input.OperationID, i.Input.InviteID).Scan(&member, &events, &floor, &uses))
	require.Zero(t, member)
	require.Zero(t, events)
	require.Zero(t, floor)
	require.Zero(t, uses)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, i.Input.JoinAttempt.AttemptID).Scan(&attemptState))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT publication_state FROM space_join_operations WHERE decision_basis_sha256=$1`, i.Input.DecisionBasisSHA256).Scan(&basisState))
	require.Equal(t, "CLAIMED", attemptState)
	require.Equal(t, "PREPARING", basisState)
	proof, err = st.ObserveMemberAdmissionContinuation(ctx, i.Input.OperationID, client, nil)
	require.NoError(t, err)
	_, err = st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, proof, nil)
	require.NoError(t, err, "a new current observation can resume the same P and saved Role receipt")
}

func TestMemberAdmissionContinuationKnownCurrentGuestDenialRetiresOnlyExactP(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	i, role, receipt := prepareContinuationFixture(t, st, false, false)
	now := time.Now()
	proof, err := st.ObserveMemberAdmissionContinuation(ctx, i.Input.OperationID, continuationClient(t, &now, "guest"), nil)
	require.NoError(t, err)
	_, err = st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, proof, nil)
	require.ErrorIs(t, err, ErrGuestsNotAllowed, "current guest still requires exact invitation despite saved regular A")
	saved, err := st.LookupMemberAdmission(ctx, i.Input.OperationID)
	require.NoError(t, err)
	require.Equal(t, "RETIRED", saved.State)
	require.Equal(t, i.Input, saved.Input)
}
