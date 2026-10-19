//go:build native_admission_integration

package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
)

type submissionOwnerRPC func(context.Context, *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error)

func (f submissionOwnerRPC) ObserveSpaceAdmissionSubject(ctx context.Context, request *authv1.ObserveSpaceAdmissionSubjectRequest, _ ...grpc.CallOption) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
	return f(ctx, request)
}

// Run with the exact native producer composition: no private-call constructor,
// reflection or generic-principal substitution. The owner RPC is a controlled
// signed-service fixture, not evidence of live Auth/User assembly or TLS wiring.
func TestJoinAttempt_SealedSubmissionFromRealAdmissionVerifier(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, mode := range []string{"automatic", "manual", "current guest", "current guest invite", "saved guest", "both guest", "expired", "expired after SQL"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			owner := uuid.New()
			space, err := st.CreateSpace(ctx, owner, "Sealed submission", "", "public")
			require.NoError(t, err)
			applyEntryPolicyMigration(t, st)
			rawQ, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000032_join_request_groups.up.sql"))
			require.NoError(t, err)
			_, err = pool.Exec(ctx, string(rawQ))
			require.NoError(t, err)
			applyJoinAttemptMigration(t, st)
			applyJoinOperationMigration(t, st)
			question := uuid.New()
			_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: mode == "manual", Questions: []entrypolicy.Question{{ID: question, Kind: "exact_text", Prompt: "2x2", ExpectedAnswer: "4"}}}})
			require.NoError(t, err)
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New(), Guest: mode == "saved guest" || mode == "both guest"}
			if mode == "current guest invite" || in.Guest {
				_, err = pool.Exec(ctx, `UPDATE spaces SET allow_guests=true WHERE id=$1`, space.ID)
				require.NoError(t, err)
				maxUses := int32(1)
				invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner, MaxUses: &maxUses})
				require.NoError(t, err)
				in.InviteID = &invite.ID
			}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			req := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.ID.String(), AttemptId: a.ID.String(), OperationId: uuid.NewString(), Answers: []*spacev1.JoinAnswer{{QuestionId: question.String(), Value: "4"}}}
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
			token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, RequestID: id, RequestHash: hash, AccountID: in.AccountID.String(), ProfileID: in.ProfileID.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			seen := map[string]bool{}
			verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock,
				AdmissionEpoch: func(ctx context.Context, account string, epoch int64) error {
					require.Equal(t, in.AccountID.String(), account)
					require.EqualValues(t, 7, epoch)
					return nil // Controlled ingress floor; never account/profile eligibility.
				}, AdmissionReplay: func(_ context.Context, _ string, jti string, _ time.Time) error {
					if seen[jti] {
						return errors.New("replay")
					}
					seen[jti] = true
					return nil
				}}
			ownerCalls := 0
			client := &admissionauthority.Client{Issuer: spaceIssuer, Clock: clock, Auth: submissionOwnerRPC(func(ctx context.Context, request *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				ownerCalls++
				md, _ := metadata.FromOutgoingContext(ctx)
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
				require.NoError(t, err)
				hash, err := principal.RequestHash(request)
				require.NoError(t, err)
				_, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_ObserveSpaceAdmissionSubject_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
				raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
				require.NoError(t, err)
				binding := sha256.Sum256(raw)
				accountType := "regular"
				if mode == "current guest" || mode == "current guest invite" || mode == "both guest" {
					accountType = "guest"
				}
				return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: binding[:], AccountId: request.AccountId, ProfileId: request.ProfileId, AccountType: accountType, AccountSecurityRevision: 2, ProfileRevision: 3, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
			})}
			incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
			var record *JoinSubmissionRecord
			_, err = verifier.Unary()(incoming, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_SubmitJoinAttempt_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				proposal, err := st.PrepareJoinSubmissionDecision(ctx, req)
				if err != nil {
					return nil, err
				}
				expected, err := proposal.OwnerRequest()
				require.NoError(t, err)
				subject, err := client.Observe(ctx, expected)
				require.NoError(t, err)
				decision, err := proposal.Seal(subject)
				require.NoError(t, err)
				if mode == "expired" {
					advance.Store(int64(5 * time.Second))
				}
				var budgetBarrier chan error
				if mode == "expired after SQL" {
					gate, err := pool.Acquire(ctx)
					require.NoError(t, err)
					_, err = gate.Exec(ctx, `SELECT pg_advisory_lock(69010351::bigint)`)
					require.NoError(t, err)
					_, err = pool.Exec(ctx, `CREATE FUNCTION block_submission_basis_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='DECIDED' THEN PERFORM pg_advisory_xact_lock(69010351::bigint); END IF; RETURN NEW; END $$; CREATE TRIGGER block_submission_basis_test BEFORE UPDATE ON space_join_operations FOR EACH ROW EXECUTE FUNCTION block_submission_basis_test()`)
					require.NoError(t, err)
					budgetBarrier = make(chan error, 1)
					go func() {
						var failure error
						defer func() {
							cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
							_, unlockErr := gate.Exec(cleanup, `SELECT pg_advisory_unlock(69010351::bigint)`)
							cancel()
							gate.Release()
							if failure == nil {
								failure = unlockErr
							}
							budgetBarrier <- failure
						}()
						deadline := time.Now().Add(5 * time.Second)
						for time.Now().Before(deadline) {
							var waiting bool
							poll, cancel := context.WithTimeout(context.Background(), time.Second)
							failure = pool.QueryRow(poll, `SELECT EXISTS(SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND NOT l.granted AND a.datname=current_database())`).Scan(&waiting)
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
						failure = errors.New("decision basis SQL did not reach the owned barrier")
					}()
				}
				record, err = st.CommitJoinSubmissionDecision(ctx, decision)
				if budgetBarrier != nil {
					require.NoError(t, <-budgetBarrier)
				}
				return record, err
			})
			require.Equal(t, 1, ownerCalls)
			var state, opState string
			var basis []byte
			require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state))
			require.NoError(t, pool.QueryRow(ctx, `SELECT state,decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ProfileID, uuid.MustParse(req.OperationId)).Scan(&opState, &basis))
			if mode == "current guest" || mode == "expired" || mode == "expired after SQL" {
				if mode == "current guest" {
					require.ErrorIs(t, err, ErrGuestsNotAllowed)
				} else {
					require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
				}
				require.Equal(t, "COLLECTING", state)
				require.Equal(t, "RESERVED", opState)
				require.Nil(t, basis)
			} else {
				require.NoError(t, err)
				wantState := "READY"
				if mode == "manual" {
					wantState = "PENDING"
				}
				require.Equal(t, wantState, state)
				require.Equal(t, "DECIDED", opState)
				require.Equal(t, record.DecisionBasisSHA256, basis)
				if mode == "manual" {
					group, groupErr := st.LookupJoinRequestGroupForAttempt(ctx, space.ID, a.ID)
					require.NoError(t, groupErr)
					require.NotEqual(t, a.ID, group.ID, "Q is a group identity, never an attempt alias")
					require.Equal(t, in.AccountID, group.AccountID)
				}
				_, err = st.LoadJoinDecisionBasis(ctx, basis)
				require.NoError(t, err)
			}
			var members int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&members))
			require.Zero(t, members, "a sealed decision alone never creates membership")
			var savedGuest bool
			var savedInvite *uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT guest,invite_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&savedGuest, &savedInvite))
			require.Equal(t, in.Guest, savedGuest, "current Auth type never rewrites the saved bit")
			require.Equal(t, in.InviteID, savedInvite)
			if in.InviteID != nil {
				invite, err := st.GetInviteByID(ctx, *in.InviteID)
				require.NoError(t, err)
				require.Zero(t, invite.UseCount, "a stored question/approval decision does not consume the invite")
			}
			_, err = st.CommitJoinSubmissionDecision(ctx, &VerifiedJoinSubmissionDecision{})
			require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
		})
	}
}
