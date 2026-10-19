package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestAdmissionServer_StartAtomicSavedReplayAndCanonicalGuest(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	for _, file := range []string{"000040_join_decision_basis_v2.up.sql", "000043_join_public_operation_context.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", file))
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	var elapsed atomic.Int64
	clock := func() time.Time { return now.Add(time.Duration(elapsed.Load())) }
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	calls := 0
	currentType := "regular"
	driftPolicy := false
	client := &admissionauthority.Client{Issuer: issuer, Clock: clock, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		calls++
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, r.Phase)
		var saved []byte
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT start_selection_sha256 FROM space_join_operations WHERE actor_account_id=$1 AND actor_profile_id=$2 AND state='RESERVED'`, r.AccountId, r.ProfileId).Scan(&saved))
		require.Equal(t, saved, r.AdmissionBindingSha256, "owner observes the durable preselection, not prospective A")
		var selected *uuid.UUID
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT selected_attempt_id FROM space_join_operations WHERE actor_account_id=$1 AND actor_profile_id=$2 AND state='RESERVED'`, r.AccountId, r.ProfileId).Scan(&selected))
		require.Nil(t, selected, "initial START has explicit absent A")
		if driftPolicy {
			_, err := st.Pool.Exec(ctx, `UPDATE space_entry_policies SET version=version+1 WHERE space_id=$1`, space)
			require.NoError(t, err)
		}
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, err)
		sha := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: currentType, AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	server := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: client})
	invoke := func(request *spacev1.StartJoinAttemptRequest, account, profile uuid.UUID) (*spacev1.StartJoinAttemptResponse, error) {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_StartJoinAttempt_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
		require.NoError(t, err)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
		response, err := verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_StartJoinAttempt_FullMethodName}, func(ctx context.Context, _ any) (any, error) { return server.StartJoinAttempt(ctx, request) })
		if err != nil {
			return nil, err
		}
		return response.(*spacev1.StartJoinAttemptResponse), nil
	}
	request := &spacev1.StartJoinAttemptRequest{Target: &spacev1.StartJoinAttemptRequest_SpaceId{SpaceId: space.String()}, OperationId: uuid.NewString()}
	account, profile := uuid.New(), uuid.New()
	response, err := invoke(request, account, profile)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.NotEmpty(t, response.Attempt.AttemptId)
	var selected uuid.UUID
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT selected_attempt_id,state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId).Scan(&selected, &state))
	require.Equal(t, "COMPLETED", state)
	require.Equal(t, response.Attempt.AttemptId, selected.String())
	_, err = st.Pool.Exec(ctx, `UPDATE space_join_operations SET outcome_proto='corrupt'::bytea WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId)
	require.Error(t, err, "completed safe outcome cannot be rewritten")
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET visibility='private' WHERE id=$1`, space)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE space_join_attempts SET state='EXPIRED' WHERE id=$1`, selected)
	require.NoError(t, err)
	again, err := invoke(request, account, profile)
	require.NoError(t, err)
	require.True(t, proto.Equal(response, again))
	require.Equal(t, 1, calls, "completed replay has zero owner calls and never refreshes safe outcome")
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET visibility='public' WHERE id=$1`, space)
	require.NoError(t, err)
	currentType = "guest"
	guestAccount, guestProfile := uuid.New(), uuid.New()
	request.OperationId = uuid.NewString()
	_, err = invoke(request, guestAccount, guestProfile)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "current owner Guest must prevail over regular ingress hint")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE account_id=$1`, guestAccount).Scan(&count))
	require.Zero(t, count)
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET allow_guests=true WHERE id=$1`, space)
	require.NoError(t, err)
	inviteA, err := st.CreateInvite(ctx, store.CreateInviteInput{SpaceID: space, CreatorProfileID: uuid.New()})
	require.NoError(t, err)
	inviteB, err := st.CreateInvite(ctx, store.CreateInviteInput{SpaceID: space, CreatorProfileID: uuid.New()})
	require.NoError(t, err)
	inviteAccount, inviteProfile := uuid.New(), uuid.New()
	request = &spacev1.StartJoinAttemptRequest{Target: &spacev1.StartJoinAttemptRequest_InviteCode{InviteCode: inviteA.Code}, OperationId: uuid.NewString()}
	firstInvite, err := invoke(request, inviteAccount, inviteProfile)
	require.NoError(t, err)
	var savedInvite uuid.UUID
	var guest bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT invite_id,guest FROM space_join_attempts WHERE id=$1`, firstInvite.Attempt.AttemptId).Scan(&savedInvite, &guest))
	require.Equal(t, inviteA.ID, savedInvite)
	require.True(t, guest, "A uses current owner classification")
	request = &spacev1.StartJoinAttemptRequest{Target: &spacev1.StartJoinAttemptRequest_InviteCode{InviteCode: inviteB.Code}, OperationId: uuid.NewString()}
	secondInvite, err := invoke(request, inviteAccount, inviteProfile)
	require.NoError(t, err)
	require.NotEqual(t, firstInvite.Attempt.AttemptId, secondInvite.Attempt.AttemptId, "two InviteIDs never share A")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT use_count FROM invites WHERE id=$1`, inviteA.ID).Scan(&count))
	require.Zero(t, count, "START never redeems an invite")
	currentType = "regular"
	driftPolicy = true
	driftAccount, driftProfile := uuid.New(), uuid.New()
	request = &spacev1.StartJoinAttemptRequest{Target: &spacev1.StartJoinAttemptRequest_SpaceId{SpaceId: space.String()}, OperationId: uuid.NewString()}
	_, err = invoke(request, driftAccount, driftProfile)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "policy drift after owner read must not select a new policy")
	driftPolicy = false
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE account_id=$1`, driftAccount).Scan(&count))
	require.Zero(t, count)
	// Hold the actual final outcome UPDATE after A and its pin have been written.
	// The final private budget check must roll all three back at strict equality.
	currentType = "regular"
	request = &spacev1.StartJoinAttemptRequest{Target: &spacev1.StartJoinAttemptRequest_SpaceId{SpaceId: space.String()}, OperationId: uuid.NewString()}
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION hold_start_outcome() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='COMPLETED' AND OLD.state='RESERVED' THEN PERFORM pg_advisory_xact_lock(90006943); END IF; RETURN NEW; END $$; CREATE TRIGGER hold_start_outcome BEFORE UPDATE ON space_join_operations FOR EACH ROW EXECUTE FUNCTION hold_start_outcome()`)
	require.NoError(t, err)
	holder, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer holder.Release()
	_, err = holder.Exec(ctx, `SELECT pg_advisory_lock(90006943)`)
	require.NoError(t, err)
	expiringAccount, expiringProfile := uuid.New(), uuid.New()
	request.OperationId = uuid.NewString()
	done := make(chan error, 1)
	go func() { _, err := invoke(request, expiringAccount, expiringProfile); done <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=90006943 AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "actual final outcome SQL wait must be reached")
	elapsed.Store(int64(5 * time.Second))
	_, err = holder.Exec(ctx, `SELECT pg_advisory_unlock(90006943)`)
	require.NoError(t, err)
	require.Equal(t, codes.Unavailable, status.Code(<-done))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE account_id=$1`, expiringAccount).Scan(&count))
	require.Zero(t, count, "A rolls back with pin and outcome")
	var pinned *uuid.UUID
	var outcome []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT selected_attempt_id,state,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, expiringProfile, request.OperationId).Scan(&pinned, &state, &outcome))
	require.Nil(t, pinned)
	require.Nil(t, outcome)
	require.Equal(t, "RESERVED", state)
}
