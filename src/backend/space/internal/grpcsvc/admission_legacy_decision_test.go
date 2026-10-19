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
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestLegacyDecisionUsesTrueContext2AndIndependentManualGate(t *testing.T) {
	for _, mode := range []string{"automatic", "manual", "questions_require_input", "captcha_require_input", "phone_unavailable", "becomes_guest", "final_sql_expiry"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			st, space := admissionReadStore(t)
			for _, name := range []string{"000040_join_decision_basis_v2.up.sql", "000043_join_public_operation_context.up.sql", "000045_join_legacy_selection_context.up.sql"} {
				raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
				require.NoError(t, err)
				_, err = st.Pool.Exec(ctx, string(raw))
				require.NoError(t, err)
			}
			if mode != "automatic" && mode != "becomes_guest" {
				var owner uuid.UUID
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT profile_id FROM space_members WHERE space_id=$1 LIMIT 1`, space).Scan(&owner))
				policy := entrypolicy.Policy{SchemaVersion: 1, Questions: []entrypolicy.Question{}, RequireManualApproval: mode == "manual" || mode == "final_sql_expiry", RequireCaptcha: mode == "captcha_require_input", RequirePhone: mode == "phone_unavailable"}
				if mode == "questions_require_input" {
					policy.Questions = []entrypolicy.Question{{ID: uuid.New(), Prompt: "2x2?", Kind: "exact_text", ExpectedAnswer: "4"}}
				}
				_, err := st.SetEntryPolicy(ctx, store.SetEntryPolicyInput{SpaceID: space, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: policy})
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
			var phases []authv1.SpaceAdmissionPhase
			var finalBinding []byte
			subjects := &admissionauthority.Client{Issuer: issuer, Clock: clock, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
				phases = append(phases, r.Phase)
				if r.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT {
					finalBinding = append([]byte(nil), r.AdmissionBindingSha256...)
				}
				raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
				require.NoError(t, err)
				sha := sha256.Sum256(raw)
				accountType := "regular"
				if mode == "becomes_guest" && r.Phase == authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT {
					accountType = "guest"
				}
				return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: accountType, AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
			})}
			op := uuid.NewString()
			request := &spacev1.JoinSpaceRequest{SpaceId: space.String(), OperationId: &op}
			account, profile := uuid.New(), uuid.New()
			hash, err := principal.RequestHash(request)
			require.NoError(t, err)
			id := uuid.NewString()
			token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_JoinSpace_FullMethodName, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
			verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
			invoke := func() (any, error) {
				return verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_JoinSpace_FullMethodName}, func(callctx context.Context, _ any) (any, error) {
					start, err := st.PrepareLegacyJoinStart(callctx, request)
					if err != nil {
						return nil, err
					}
					expected, err := start.OwnerRequest()
					if err != nil {
						return nil, err
					}
					proof, err := subjects.Observe(callctx, expected)
					if err != nil {
						return nil, err
					}
					sealed, err := start.Seal(proof)
					if err != nil {
						return nil, err
					}
					_, err = st.CommitLegacyJoinStart(callctx, sealed)
					if err != nil {
						return nil, err
					}
					decision, err := st.PrepareLegacyJoinDecision(callctx, start)
					if err != nil {
						return nil, err
					}
					expected, err = decision.OwnerRequest()
					if err != nil {
						return nil, err
					}
					proof, err = subjects.Observe(callctx, expected)
					if err != nil {
						return nil, err
					}
					accepted, err := decision.Seal(proof)
					if err != nil {
						return nil, err
					}
					return st.CommitLegacyJoinDecision(callctx, accepted)
				})
			}
			var result any
			if mode == "final_sql_expiry" {
				_, err = st.Pool.Exec(ctx, `CREATE FUNCTION hold_legacy_decision() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='DECIDED' AND OLD.state='RESERVED' THEN PERFORM pg_advisory_xact_lock(91006945); END IF; RETURN NEW; END $$; CREATE TRIGGER hold_legacy_decision BEFORE UPDATE ON space_join_operations FOR EACH ROW EXECUTE FUNCTION hold_legacy_decision()`)
				require.NoError(t, err)
				blocker, e := st.Pool.Acquire(ctx)
				require.NoError(t, e)
				defer blocker.Release()
				_, e = blocker.Exec(ctx, `SELECT pg_advisory_lock(91006945)`)
				require.NoError(t, e)
				done := make(chan error, 1)
				go func() { _, e := invoke(); done <- e }()
				require.Eventually(t, func() bool {
					var waiting bool
					e := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=91006945 AND NOT granted)`).Scan(&waiting)
					return e == nil && waiting
				}, 3*time.Second, 10*time.Millisecond)
				elapsed.Store(int64(5 * time.Second))
				_, e = blocker.Exec(ctx, `SELECT pg_advisory_unlock(91006945)`)
				require.NoError(t, e)
				err = <-done
				require.Error(t, err)
				var state string
				var basis []byte
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT a.state,o.decision_basis_sha256 FROM space_join_operations o JOIN space_join_attempts a ON a.id=o.selected_attempt_id WHERE o.operation_id=$1`, op).Scan(&state, &basis))
				require.Equal(t, "COLLECTING", state)
				require.Nil(t, basis)
				var groups int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_request_groups`).Scan(&groups))
				require.Zero(t, groups, "expired decision rolls back Q and every decision effect")
				return
			}
			result, err = invoke()
			if mode == "questions_require_input" || mode == "captcha_require_input" || mode == "phone_unavailable" {
				if mode == "phone_unavailable" {
					require.ErrorIs(t, err, store.ErrJoinAttemptUnavailable)
				} else {
					require.ErrorIs(t, err, store.ErrLegacyJoinInputRequired)
				}
				require.Equal(t, []authv1.SpaceAdmissionPhase{authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START}, phases)
				var count int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations WHERE decision_basis IS NOT NULL`).Scan(&count))
				require.Zero(t, count)
				return
			}
			if mode == "becomes_guest" {
				require.Error(t, err)
				var guest bool
				var state string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT guest,state FROM space_join_attempts WHERE account_id=$1`, account).Scan(&guest, &state))
				require.False(t, guest, "immutable A class cannot be rewritten")
				require.Equal(t, "COLLECTING", state)
				var count int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations WHERE decision_basis IS NOT NULL`).Scan(&count))
				require.Zero(t, count)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []authv1.SpaceAdmissionPhase{authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_START, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_SUBMIT}, phases)
			record := result.(*store.JoinSubmissionRecord)
			loaded, err := st.LoadVersionedJoinDecisionBasis(ctx, record.DecisionBasisSHA256)
			require.NoError(t, err)
			require.EqualValues(t, 2, loaded.Version())
			basis, err := loaded.LegacyV2()
			require.NoError(t, err)
			require.Equal(t, spacev1.SpaceService_JoinSpace_FullMethodName, basis.Context.RequestMethod)
			require.Equal(t, op, basis.Context.OperationID.String())
			ctxSHA, err := basis.Context.SHA256()
			require.NoError(t, err)
			require.Equal(t, finalBinding, ctxSHA[:])
			if mode == "manual" {
				require.Equal(t, "PENDING", record.Attempt.State)
				require.NotNil(t, record.RequestID)
				require.Equal(t, store.JoinLegacyPending, basis.Context.Kind)
				var savedState string
				var savedCode int32
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,outcome_code FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, op).Scan(&savedState, &savedCode))
				require.Equal(t, "COMPLETED", savedState, "original keyed pending is a final safe legacy outcome")
				require.EqualValues(t, 9, savedCode)
				_, err = st.Pool.Exec(ctx, `UPDATE space_entry_policies SET version=version+1 WHERE space_id=$1`, space)
				require.NoError(t, err)
				retryID := uuid.NewString()
				retryToken, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_JoinSpace_FullMethodName, RequestID: retryID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
				require.NoError(t, e)
				retryCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+retryToken, "x-request-id", retryID))
				_, e = verifier.Unary()(retryCtx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_JoinSpace_FullMethodName}, func(callctx context.Context, _ any) (any, error) {
					p, e := st.PrepareLegacyJoinStart(callctx, request)
					if e != nil {
						return nil, e
					}
					member, done, e := st.ReplayLegacyJoinOutcome(callctx, p)
					require.True(t, done)
					require.Nil(t, member)
					return nil, e
				})
				require.ErrorIs(t, e, store.ErrJoinAttemptPending)
				require.Len(t, phases, 2, "completed exact pending replay has no new Subject or Role observation")
				_, e = st.Pool.Exec(ctx, `UPDATE spaces SET visibility='private' WHERE id=$1`, space)
				require.NoError(t, e)
				_, e = verifier.Unary()(retryCtx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_JoinSpace_FullMethodName}, func(callctx context.Context, _ any) (any, error) { return st.PrepareLegacyJoinStart(callctx, request) })
				require.ErrorIs(t, e, store.ErrJoinAttemptDenied, "a legacy key is not a current target discovery/access bypass")
				require.Len(t, phases, 2)
			} else {
				require.Equal(t, "READY", record.Attempt.State)
				require.Nil(t, record.RequestID)
				require.Equal(t, store.JoinLegacyReady, basis.Context.Kind)
			}
			var members int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE profile_id=$1`, profile).Scan(&members))
			require.Zero(t, members, "decision is not native publication")
		})
	}
}
