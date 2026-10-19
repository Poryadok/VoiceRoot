package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

func signedApprovalOperationFixture(t *testing.T, request *spacev1.ApproveJoinRequestRequest, account, profile uuid.UUID) (*JoinApprovalOperation, *atomic.Int64) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	elapsed := &atomic.Int64{}
	clock := func() time.Time { return now.Add(time.Duration(elapsed.Load())) }
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: JoinOperationApprove, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
	require.NoError(t, err)
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	verifier := lifecycleprincipal.Verifier{Clock: clock, Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
	result, err := verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: JoinOperationApprove}, func(ctx context.Context, _ any) (any, error) { return PrepareJoinApprovalOperation(ctx, request) })
	require.NoError(t, err)
	return result.(*JoinApprovalOperation), elapsed
}

// Retained publication storage only. Dummy observation/receipt bytes never
// qualify current membership, a Role allow, or the native45 serving producer.
func TestJoinApprovalOriginalOperationRecoveryAndPublishedSafeOutcome(t *testing.T) {
	for _, family := range []string{"v1", "legacy_v2"} {
		for _, mode := range []string{"fresh", "expired", "sql_expiry"} {
			t.Run(family+"/"+mode, func(t *testing.T) {
				ctx := context.Background()
				st, space := joinAdmissionNativeFixture(t)
				_, err := st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: space.OwnerProfileID, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
				require.NoError(t, err)
				in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
				a, err := st.StartJoinAttempt(ctx, in)
				require.NoError(t, err)
				a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
				require.NoError(t, err)
				var q *JoinRequestGroup
				require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { var e error; q, e = local.AttachJoinRequestAttempt(ctx, a.ID); return e }))
				var binding, submission []byte
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
				var parentV1 JoinDecisionBasis
				var parentV2 JoinLegacyDecisionBasis
				var parentSHA []byte
				if family == "v1" {
					parentV1, parentSHA = seedJoinDecisionHistory(t, st, in, a)
				} else {
					parentV2 = JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyPending, OperationOrigin: JoinOperationServerNoKey, SpaceID: space.ID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.New(), RequestMethod: spacev1.SpaceService_JoinSpace_FullMethodName, RequestSHA256: bytes.Repeat([]byte{1}, 32)}, ApplicantObservationSHA256: bytes.Repeat([]byte{2}, 32)}
					parentSHA = seedLegacyJoinDecisionHistory(t, st, parentV2)
				}
				selected := a.ID.String()
				request := &spacev1.ApproveJoinRequestRequest{SpaceId: space.ID.String(), RequestId: q.ID.String(), OperationId: uuid.NewString(), AttemptId: &selected}
				p, elapsed := signedApprovalOperationFixture(t, request, uuid.New(), space.OwnerProfileID)
				_, err = st.ResolveJoinRequestApproval(ctx, p.input, q.ID, &a.ID)
				require.NoError(t, err)
				saved, recovered, err := st.ReadJoinApprovalOperation(ctx, p)
				require.NoError(t, err)
				require.Nil(t, saved)
				require.Nil(t, recovered)
				approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: p.input.ActorAccountID, ActorProfileID: p.input.ActorProfileID, OperationID: p.input.OperationID}
				_, err = st.ReserveJoinAttemptApproval(ctx, approval)
				require.NoError(t, err)
				a, err = st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
				require.NoError(t, err, "explicit retained READY fixture, no public authority claim")
				var basisSHA [32]byte
				var rawBasis []byte
				version := 1
				if family == "v1" {
					basis := JoinDecisionBasis{Context: parentV1.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{3}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{4}, 32)}
					c := &basis.Context
					c.Kind = JoinDecisionApproveReady
					c.ActorAccountID = p.input.ActorAccountID
					c.ActorProfileID = p.input.ActorProfileID
					c.OperationID = p.input.OperationID
					c.RequestMethod = JoinOperationApprove
					c.RequestSHA256 = p.input.RequestSHA256
					c.ReadyEvidenceSHA256 = a.EvidenceSHA256
					c.SubmitBasisSHA256 = parentSHA
					basisSHA, err = basis.SHA256()
					require.NoError(t, err)
					rawBasis, err = json.Marshal(basis)
					require.NoError(t, err)
				} else {
					version = 2
					basis := JoinLegacyDecisionBasis{Context: parentV2.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{3}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{4}, 32)}
					c := &basis.Context
					c.Kind = JoinApproveLegacyReady
					c.OperationOrigin = JoinOperationClientKey
					c.ActorAccountID = p.input.ActorAccountID
					c.ActorProfileID = p.input.ActorProfileID
					c.OperationID = p.input.OperationID
					c.RequestMethod = JoinOperationApprove
					c.RequestSHA256 = p.input.RequestSHA256
					c.ReadyEvidenceSHA256 = a.EvidenceSHA256
					c.PendingBasisSHA256 = parentSHA
					basisSHA, err = basis.SHA256()
					require.NoError(t, err)
					rawBasis, err = json.Marshal(basis)
					require.NoError(t, err)
				}
				_, err = st.Pool.Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',basis_version=$3,decision_basis=$4,decision_basis_sha256=$5 WHERE actor_profile_id=$1 AND operation_id=$2`, p.input.ActorProfileID, p.input.OperationID, version, rawBasis, basisSHA[:])
				require.NoError(t, err)
				saved, recovered, err = st.ReadJoinApprovalOperation(ctx, p)
				require.NoError(t, err)
				require.Nil(t, saved)
				require.NotNil(t, recovered)
				require.Equal(t, basisSHA[:], recovered.DecisionBasisSHA256)
				require.Nil(t, recovered.PublicationOperationID)
				native := &MemberAdmissionIntent{Input: MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: in.AccountID, ProfileID: in.ProfileID, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: basisSHA[:]}, State: "PUBLISHED", Generation: 1, PolicyVersion: a.PolicyVersion}
				require.ErrorIs(t, st.MarkJoinApprovalPublishedOutcome(ctx, p, native), ErrJoinAttemptUnavailable)
				require.Error(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkJoinApprovalPublishedOutcome(ctx, p, native) }), "marker before actual persisted publication rejects")
				joined := time.Now().UTC().Truncate(time.Microsecond)
				require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error {
					if e := local.VerifyVersionedJoinDecisionBasis(ctx, in, *native.Input.JoinAttempt, basisSHA[:], a.PolicyVersion, native.Input.OperationID); e != nil {
						return e
					}
					if e := local.MarkJoinAttemptPublished(ctx, *native.Input.JoinAttempt, native.Input.OperationID); e != nil {
						return e
					}
					if e := local.MarkVersionedJoinDecisionBasisPublished(ctx, *native.Input.JoinAttempt, basisSHA[:], native.Input.OperationID); e != nil {
						return e
					}
					hash := admissionRequestHash(native.Input)
					_, e := local.db().Exec(ctx, `INSERT INTO space_member_admission_intents(operation_id,space_id,profile_id,account_id,guest,generation,request_sha256,binding_sha256,policy_version,state,role_id,role_receipt_sha256,published_at,join_attempt_id,join_attempt_evidence_sha256,decision_basis_sha256) VALUES($1,$2,$3,$4,false,1,$5,$6,$7,'PUBLISHED',$8,$6,$9,$10,$11,$12)`, native.Input.OperationID, in.SpaceID, in.ProfileID, in.AccountID, hash[:], bytes.Repeat([]byte{5}, 32), a.PolicyVersion, uuid.New(), joined, a.ID, a.EvidenceSHA256, basisSHA[:])
					return e
				}))
				saved, recovered, err = st.ReadJoinApprovalOperation(ctx, p)
				require.NoError(t, err)
				require.Nil(t, saved)
				require.Equal(t, native.Input.OperationID, *recovered.PublicationOperationID)
				wrong := *native
				wrong.Input.ProfileID = p.input.ActorProfileID
				require.Error(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkJoinApprovalPublishedOutcome(ctx, p, &wrong) }), "reviewer membership cannot replace applicant outcome")
				if mode == "expired" {
					elapsed.Store(int64(5 * time.Second))
					require.Error(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkJoinApprovalPublishedOutcome(ctx, p, native) }))
				}
				if mode == "sql_expiry" {
					holder, e := st.Pool.Begin(ctx)
					require.NoError(t, e)
					defer holder.Rollback(ctx)
					var id uuid.UUID
					require.NoError(t, holder.QueryRow(ctx, `SELECT operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, p.input.ActorProfileID, p.input.OperationID).Scan(&id))
					var pid int32
					require.NoError(t, holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
					done := make(chan error, 1)
					go func() {
						done <- st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkJoinApprovalPublishedOutcome(ctx, p, native) })
					}()
					require.Eventually(t, func() bool {
						var blocked bool
						return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked) == nil && blocked
					}, 3*time.Second, 5*time.Millisecond)
					elapsed.Store(int64(5 * time.Second))
					require.NoError(t, holder.Commit(ctx))
					require.Error(t, <-done)
				}
				if mode != "fresh" {
					var state string
					require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, p.input.ActorProfileID, p.input.OperationID).Scan(&state))
					require.Equal(t, "DECIDED", state)
					return
				}
				require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkJoinApprovalPublishedOutcome(ctx, p, native) }))
				_, err = st.Pool.Exec(ctx, `UPDATE space_entry_policies SET version=version+1 WHERE space_id=$1`, space.ID)
				require.NoError(t, err)
				saved, recovered, err = st.ReadJoinApprovalOperation(ctx, p)
				require.NoError(t, err)
				require.Nil(t, recovered)
				require.Equal(t, in.ProfileID.String(), saved.Result.SpaceMembership.ProfileId)
				require.NotEqual(t, p.input.ActorProfileID.String(), saved.Result.SpaceMembership.ProfileId)
				require.True(t, saved.Result.SpaceMembership.JoinedAt.AsTime().Equal(joined))
				var count int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&count))
				require.Zero(t, count, "history hook cannot recreate membership")
				changed := proto.Clone(request).(*spacev1.ApproveJoinRequestRequest)
				changed.RequestId = uuid.NewString()
				other, _ := signedApprovalOperationFixture(t, changed, p.input.ActorAccountID, p.input.ActorProfileID)
				_, _, err = st.ReadJoinApprovalOperation(ctx, other)
				require.ErrorIs(t, err, ErrJoinAttemptConflict)
				altered := proto.Clone(request).(*spacev1.ApproveJoinRequestRequest)
				altered.AttemptId = nil
				other, _ = signedApprovalOperationFixture(t, altered, p.input.ActorAccountID, p.input.ActorProfileID)
				_, _, err = st.ReadJoinApprovalOperation(ctx, other)
				require.ErrorIs(t, err, ErrJoinAttemptConflict)
				// Body hash remains independently signed and cannot be inferred from H.
				wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(request)
				sha := sha256.Sum256(wire)
				require.Equal(t, sha[:], p.input.RequestSHA256)
			})
		}
	}
}
