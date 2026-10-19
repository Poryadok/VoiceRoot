//go:build native_admission_integration

package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

// Historical submit basis is an explicit storage fixture. Current approval uses
// real signed admission interception, concrete Q selection, private owner/Role
// capabilities and the atomic writer. Owner RPCs remain controlled fixtures.
func TestJoinAttempt_ApprovalWriterCommitsExactQAndOriginalParent(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "commit"
		if expired {
			name = "expiry after SQL"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			st, space := joinAdmissionNativeFixture(t)
			pool := st.Pool
			owner := space.OwnerProfileID
			_, err := st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
			require.NoError(t, err)
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
			require.NoError(t, err)
			_, parentSHA := seedJoinDecisionHistory(t, st, in, a)
			// Pending deliberately outlives collection and has no applicant-session TTL.
			_, err = pool.Exec(ctx, `UPDATE space_join_attempts SET created_at=clock_timestamp()-interval '2 hours',collection_deadline=clock_timestamp()-interval '1 hour' WHERE id=$1`, a.ID)
			require.NoError(t, err)
			var group *JoinRequestGroup
			require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error {
				var attachErr error
				group, attachErr = local.AttachJoinRequestAttempt(ctx, a.ID)
				return attachErr
			}))
			selected := a.ID.String()
			req := &spacev1.ApproveJoinRequestRequest{SpaceId: space.ID.String(), RequestId: group.ID.String(), OperationId: uuid.NewString(), AttemptId: &selected}
			reviewerAccount := uuid.New()
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			now := time.Now().UTC().Truncate(time.Second)
			var advance atomic.Int64
			clock := func() time.Time { return now.Add(time.Duration(advance.Load())) }
			gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
			require.NoError(t, err)
			spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key, Clock: clock})
			require.NoError(t, err)
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			id := uuid.NewString()
			token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: JoinOperationApprove, RequestID: id, RequestHash: hash, AccountID: reviewerAccount.String(), ProfileID: owner.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			verifyService := func(ctx context.Context, r proto.Message, audience, rpc string) {
				md, _ := metadata.FromOutgoingContext(ctx)
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
				require.NoError(t, err)
				hash, err := principal.RequestHash(r)
				require.NoError(t, err)
				_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: audience, ExpectedRPC: rpc, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
			}
			ownerCalls, roleCalls := 0, 0
			client := &admissionauthority.Client{Issuer: spaceIssuer, Clock: clock, Auth: submissionOwnerRPC(func(ctx context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				ownerCalls++
				verifyService(ctx, r, "auth", authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName)
				if r.SubjectRole == authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT {
					require.Nil(t, r.InteractiveSessionEpoch)
					require.Equal(t, in.AccountID.String(), r.AccountId)
					require.Equal(t, in.ProfileID.String(), r.ProfileId)
				} else {
					require.Equal(t, reviewerAccount.String(), r.AccountId)
					require.Equal(t, owner.String(), r.ProfileId)
					require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
				}
				raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
				require.NoError(t, err)
				binding := sha256.Sum256(raw)
				return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: binding[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 2, ProfileRevision: 3, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
			})}
			reviewerClient := &admissionauthority.ReviewerClient{Issuer: spaceIssuer, Clock: clock, Role: approvalPermissionRPC(func(ctx context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
				roleCalls++
				verifyService(ctx, r, "role", rolev1.RoleService_CheckPermission_FullMethodName)
				require.Equal(t, admissionauthority.ReviewJoinPermission, r.PermissionName)
				require.Equal(t, space.ID.String(), r.SpaceId)
				require.Equal(t, owner.String(), r.ProfileId)
				return &rolev1.CheckPermissionResponse{Allowed: true}, nil
			})}
			verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }, AdmissionEpoch: func(_ context.Context, account string, epoch int64) error {
				require.Equal(t, reviewerAccount.String(), account)
				require.EqualValues(t, 7, epoch)
				return nil
			}}
			incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
			_, err = verifier.Unary()(incoming, req, &grpc.UnaryServerInfo{FullMethod: JoinOperationApprove}, func(ctx context.Context, _ any) (any, error) {
				proposal, err := st.PrepareJoinApprovalDecision(ctx, req)
				require.NoError(t, err)
				applicantRequest, r, err := proposal.OwnerRequests()
				require.NoError(t, err)
				applicant, err := client.Observe(ctx, applicantRequest)
				require.NoError(t, err)
				reviewerSubject, err := client.Observe(ctx, r)
				require.NoError(t, err)
				reviewer, err := reviewerClient.Observe(ctx, reviewerSubject, r)
				require.NoError(t, err)
				decision, err := proposal.Seal(applicant, reviewer)
				require.NoError(t, err)
				require.Equal(t, parentSHA, decision.basis.Context.SubmitBasisSHA256)
				identity, err := decision.currentApplicant()
				require.NoError(t, err)
				require.Equal(t, in.AccountID, identity.AccountID)
				projection := proposal.Context()
				projection.SubmitBasisSHA256[0] ^= 1
				require.Equal(t, parentSHA, decision.basis.Context.SubmitBasisSHA256)
				var barrier <-chan error
				if expired {
					barrier = holdJoinApprovalDecisionSQL(t, st, &advance)
				}
				record, err := st.CommitJoinApprovalDecision(ctx, decision)
				if barrier != nil {
					require.NoError(t, <-barrier)
				}
				if expired {
					require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
					return nil, nil
				}
				require.NoError(t, err)
				require.Equal(t, "READY", record.Attempt.State)
				saved, err := st.LoadJoinDecisionBasis(ctx, record.DecisionBasisSHA256)
				require.NoError(t, err)
				require.Equal(t, parentSHA, saved.Context.SubmitBasisSHA256)
				require.Equal(t, reviewerAccount, saved.Context.ActorAccountID)
				require.Equal(t, owner, saved.Context.ActorProfileID)
				var publication *uuid.UUID
				require.NoError(t, pool.QueryRow(ctx, `SELECT publication_operation_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&publication))
				require.Nil(t, publication)

				return nil, nil
			})
			require.NoError(t, err)
			require.Equal(t, 2, ownerCalls)
			require.Equal(t, 1, roleCalls)
			var state, operationState string
			require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
			wantState, wantOperation := "READY", "DECIDED"
			if expired {
				wantState, wantOperation = "PENDING", "RESERVED"
			}
			require.Equal(t, wantState, state)
			require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, owner, uuid.MustParse(req.OperationId)).Scan(&operationState))
			require.Equal(t, wantOperation, operationState)
		})
	}
}

// A real second connection holds the decision UPDATE. Advancing the controlled
// monotonic clock only after PostgreSQL reports that wait discriminates a final
// post-SQL freshness check from an earlier check before the decision write.
func holdJoinApprovalDecisionSQL(t *testing.T, st *SpaceStore, advance *atomic.Int64) <-chan error {
	t.Helper()
	ctx := context.Background()
	gate, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = gate.Exec(ctx, `SELECT pg_advisory_lock(69010352::bigint)`)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION block_approval_basis_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='DECIDED' THEN PERFORM pg_advisory_xact_lock(69010352::bigint); END IF; RETURN NEW; END $$; CREATE TRIGGER block_approval_basis_test BEFORE UPDATE ON space_join_operations FOR EACH ROW EXECUTE FUNCTION block_approval_basis_test()`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		var failure error
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, unlockErr := gate.Exec(cleanup, `SELECT pg_advisory_unlock(69010352::bigint)`)
			cancel()
			gate.Release()
			if failure == nil {
				failure = unlockErr
			}
			done <- failure
		}()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			var waiting bool
			poll, cancel := context.WithTimeout(context.Background(), time.Second)
			failure = st.Pool.QueryRow(poll, `SELECT EXISTS(SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND NOT l.granted AND a.datname=current_database())`).Scan(&waiting)
			cancel()
			if failure != nil {
				return
			}
			if waiting {
				advance.Store(int64(5 * time.Second))
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		failure = errors.New("approval decision never reached owned SQL barrier")
	}()
	return done
}
