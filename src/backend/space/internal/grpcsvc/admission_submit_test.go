package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
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
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestAdmissionServer_SubmitWrongAnswerIsDurableFailedAndReplay(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	question := uuid.New()
	_, err := st.SetEntryPolicy(ctx, store.SetEntryPolicyInput{SpaceID: space, ActorProfileID: uuid.New(), OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{{ID: question, Prompt: "2x2", Kind: "exact_text", ExpectedAnswer: "4"}}}})
	require.NoError(t, err)
	account, profile := uuid.New(), uuid.New()
	a, err := st.StartJoinAttempt(ctx, store.JoinAttemptInput{SpaceID: space, AccountID: account, ProfileID: profile})
	require.NoError(t, err)
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
	client := &admissionauthority.Client{Issuer: issuer, Clock: clock, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		calls++
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT, r.Phase)
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, err)
		sha := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	server := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: client})
	request := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.String(), AttemptId: a.ID.String(), OperationId: uuid.NewString(), Answers: []*spacev1.JoinAnswer{{QuestionId: question.String(), Value: "04"}}}
	var lastResponse *spacev1.SubmitJoinAttemptResponse
	invoke := func() error {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
		response, err := verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_SubmitJoinAttempt_FullMethodName}, func(ctx context.Context, _ any) (any, error) { return server.SubmitJoinAttempt(ctx, request) })
		if err == nil {
			lastResponse = response.(*spacev1.SubmitJoinAttemptResponse)
		}
		return err
	}
	err = invoke()
	require.Equal(t, codes.PermissionDenied, status.Code(err), "wrong configured answer must fail, not enter manual queue")
	require.Equal(t, 1, calls)
	var state string
	var code int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "FAILED", state)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,outcome_code FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId).Scan(&state, &code))
	require.Equal(t, "COMPLETED", state)
	require.Equal(t, int(codes.PermissionDenied), code)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_request_attempts WHERE attempt_id=$1`, a.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&count))
	require.Zero(t, count)
	err = invoke()
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 1, calls, "exact denied replay does not regrade or call owner")
	request.Answers[0].Value = "4"
	err = invoke()
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.Equal(t, 1, calls, "changed body cannot replace the failed immutable submission")
	// A new exact attempt can pass automatic grading and independently wait for
	// manual approval. Q is an actual grouping journal, never an alias for A.
	fresh, err := st.StartJoinAttempt(ctx, store.JoinAttemptInput{SpaceID: space, AccountID: account, ProfileID: profile})
	require.NoError(t, err)
	request.AttemptId = fresh.ID.String()
	request.OperationId = uuid.NewString()
	request.Answers[0].Value = " 4\r\n"
	err = invoke()
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.NotNil(t, lastResponse)
	require.Equal(t, spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING, lastResponse.Result.State)
	requestID, err := uuid.Parse(lastResponse.Result.RequestId)
	require.NoError(t, err)
	require.NotEqual(t, fresh.ID, requestID)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, fresh.ID).Scan(&state))
	require.Equal(t, "PENDING", state)
	before := proto.Clone(lastResponse)
	err = invoke()
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.True(t, proto.Equal(before, lastResponse))
	// The last outcome UPDATE may wait after FAILED was tentatively written.
	// Strict owner expiry must roll back both FAILED and its denied receipt.
	account, profile = uuid.New(), uuid.New()
	expiring, err := st.StartJoinAttempt(ctx, store.JoinAttemptInput{SpaceID: space, AccountID: account, ProfileID: profile})
	require.NoError(t, err)
	request.AttemptId = expiring.ID.String()
	request.OperationId = uuid.NewString()
	request.Answers[0].Value = "04"
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION hold_failed_outcome() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='COMPLETED' AND OLD.state='RESERVED' THEN PERFORM pg_advisory_xact_lock(90006944); END IF; RETURN NEW; END $$; CREATE TRIGGER hold_failed_outcome BEFORE UPDATE ON space_join_operations FOR EACH ROW EXECUTE FUNCTION hold_failed_outcome()`)
	require.NoError(t, err)
	holder, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer holder.Release()
	_, err = holder.Exec(ctx, `SELECT pg_advisory_lock(90006944)`)
	require.NoError(t, err)
	defer func() { _, _ = holder.Exec(ctx, `SELECT pg_advisory_unlock(90006944)`) }()
	done := make(chan error, 1)
	go func() { done <- invoke() }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := holder.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=90006944 AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	elapsed.Store(int64(5 * time.Second))
	_, err = holder.Exec(ctx, `SELECT pg_advisory_unlock(90006944)`)
	require.NoError(t, err)
	require.Equal(t, codes.Unavailable, status.Code(<-done))
	var submission []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,submission_sha256 FROM space_join_attempts WHERE id=$1`, expiring.ID).Scan(&state, &submission))
	require.Equal(t, "COLLECTING", state)
	require.Nil(t, submission)
	var outcome []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, request.OperationId).Scan(&state, &outcome))
	require.Equal(t, "RESERVED", state)
	require.Nil(t, outcome)
}
