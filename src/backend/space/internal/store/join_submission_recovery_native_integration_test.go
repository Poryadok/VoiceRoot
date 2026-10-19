//go:build native_admission_integration

package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

func TestJoinAttempt_SubmissionRecoveryRequiresExactSignedOriginalRequest(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Exact recovery", "", "public")
	require.NoError(t, err)
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	req := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.ID.String(), AttemptId: a.ID.String(), OperationId: uuid.NewString()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	require.NoError(t, err)
	sha := sha256.Sum256(raw)
	op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.MustParse(req.OperationId), Method: JoinOperationSubmit, RequestSHA256: sha[:]}
	_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
	require.NoError(t, err)
	var binding, submission []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
	// Historical storage fixture, not a newly minted decision/owner proof.
	basis := JoinDecisionBasis{Context: JoinDecisionContext{Kind: JoinDecisionSubmitReady, SpaceID: space.ID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ReadyEvidenceSHA256: a.EvidenceSHA256, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: op.OperationID, RequestMethod: op.Method, RequestSHA256: op.RequestSHA256}, ApplicantObservationSHA256: bytes.Repeat([]byte{9}, 32)}
	basisSHA := persistJoinDecisionHistory(t, st, op, basis)
	_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET state='INVALIDATED' WHERE id=$1`, a.ID)
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	// A generic principal never substitutes for the private exact-method call.
	generic := principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "space", Subject: in.AccountID.String(), AccountID: in.AccountID.String(), ProfileID: in.ProfileID.String(), RequestID: uuid.NewString(), RPC: JoinOperationSubmit, RequestHash: hash, SessionEpoch: 7, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)})
	_, err = st.RecoverJoinSubmissionDecision(generic, req)
	require.Error(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	clock := func() time.Time { return now }
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: JoinOperationSubmit, RequestID: id, RequestHash: hash, AccountID: in.AccountID.String(), ProfileID: in.ProfileID.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }, AdmissionEpoch: func(_ context.Context, account string, epoch int64) error {
		require.Equal(t, in.AccountID.String(), account)
		require.EqualValues(t, 7, epoch)
		return nil
	}}
	incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	_, err = verifier.Unary()(incoming, req, &grpc.UnaryServerInfo{FullMethod: JoinOperationSubmit}, func(ctx context.Context, _ any) (any, error) {
		recovered, err := st.RecoverJoinSubmissionDecision(ctx, req)
		require.NoError(t, err)
		require.Equal(t, a.ID, recovered.AttemptID)
		require.Equal(t, basisSHA, recovered.DecisionBasisSHA256)
		require.Nil(t, recovered.PublicationOperationID)
		changed := proto.Clone(req).(*spacev1.SubmitJoinAttemptRequest)
		changed.AttemptId = uuid.NewString()
		_, err = st.RecoverJoinSubmissionDecision(ctx, changed)
		require.Error(t, err, "private proof cannot move to changed body/A")
		return nil, nil
	})
	require.NoError(t, err)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
	require.Equal(t, "INVALIDATED", state)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&count))
	require.Zero(t, count)
}
