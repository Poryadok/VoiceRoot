package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

// This is a retained-publication storage fixture, not a Role receipt or native
// authorization proof. It tests the safe outcome hook against actual migrated
// tables. Production must reach this hook through the private native publisher.
func TestLegacyPublishedOutcomeRequiresExactSameTransactionHistory(t *testing.T) {
	for _, name := range []string{"fresh", "expired", "sql_expiry"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			st, space := joinAdmissionNativeFixture(t)
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			cap, elapsed := signedNativeInitiatingCall(t, MemberAdmissionInput{SpaceID: in.SpaceID, AccountID: in.AccountID, ProfileID: in.ProfileID})
			request := cap.request.(*spacev1.JoinSpaceRequest)
			rawRequest, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
			require.NoError(t, err)
			requestSHA := sha256.Sum256(rawRequest)
			op := JoinOperationInput{SpaceID: in.SpaceID, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.MustParse(request.GetOperationId()), Method: spacev1.SpaceService_JoinSpace_FullMethodName, RequestSHA256: requestSHA[:]}
			selection, err := st.ReserveLegacyJoinSelection(ctx, op, JoinOperationClientKey, nil)
			require.NoError(t, err)
			p := &JoinLegacyStartProposal{request: request, call: cap.call, selection: selection}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
			require.NoError(t, err)
			var binding, submission []byte
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
			basis := JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyReady, OperationOrigin: JoinOperationClientKey, SpaceID: in.SpaceID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ReadyEvidenceSHA256: a.EvidenceSHA256, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: op.OperationID, RequestMethod: op.Method, RequestSHA256: op.RequestSHA256}, ApplicantObservationSHA256: bytes.Repeat([]byte{1}, 32)}
			basisSHA, err := basis.SHA256()
			require.NoError(t, err)
			rawBasis, err := json.Marshal(basis)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `UPDATE space_join_operations SET selected_attempt_id=$3,state='DECIDED',decision_basis=$4,decision_basis_sha256=$5 WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID, a.ID, rawBasis, basisSHA[:])
			require.NoError(t, err)
			native := &MemberAdmissionIntent{Input: MemberAdmissionInput{OperationID: uuid.New(), SpaceID: in.SpaceID, AccountID: in.AccountID, ProfileID: in.ProfileID, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: basisSHA[:]}, State: "PUBLISHED", Generation: 1, PolicyVersion: a.PolicyVersion}
			require.ErrorIs(t, st.MarkLegacyJoinPublishedOutcome(ctx, p, native), ErrJoinAttemptUnavailable, "hook is scoped-TX only")
			joined := time.Now().UTC().Truncate(time.Microsecond)
			// A marker before native/35/40 publication must fail without completing op.
			require.Error(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkLegacyJoinPublishedOutcome(ctx, p, native) }))
			require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error {
				if err := local.VerifyVersionedJoinDecisionBasis(ctx, in, *native.Input.JoinAttempt, basisSHA[:], a.PolicyVersion, native.Input.OperationID); err != nil {
					return err
				}
				if err := local.MarkJoinAttemptPublished(ctx, *native.Input.JoinAttempt, native.Input.OperationID); err != nil {
					return err
				}
				if err := local.MarkVersionedJoinDecisionBasisPublished(ctx, *native.Input.JoinAttempt, basisSHA[:], native.Input.OperationID); err != nil {
					return err
				}
				hash := admissionRequestHash(native.Input)
				_, err := local.db().Exec(ctx, `INSERT INTO space_member_admission_intents(operation_id,space_id,profile_id,account_id,guest,generation,request_sha256,binding_sha256,policy_version,state,role_id,role_receipt_sha256,published_at,join_attempt_id,join_attempt_evidence_sha256,decision_basis_sha256) VALUES($1,$2,$3,$4,false,1,$5,$6,$7,'PUBLISHED',$8,$6,$9,$10,$11,$12)`, native.Input.OperationID, in.SpaceID, in.ProfileID, in.AccountID, hash[:], bytes.Repeat([]byte{2}, 32), a.PolicyVersion, uuid.New(), joined, a.ID, a.EvidenceSHA256, basisSHA[:])
				return err
			}))
			for _, change := range []func(*MemberAdmissionIntent){
				func(n *MemberAdmissionIntent) { n.Input.OperationID = uuid.New() },
				func(n *MemberAdmissionIntent) { n.Input.AccountID = uuid.New() },
				func(n *MemberAdmissionIntent) { n.Input.Guest = true },
				func(n *MemberAdmissionIntent) { n.Input.DecisionBasisSHA256 = bytes.Repeat([]byte{3}, 32) },
			} {
				wrong := *native
				change(&wrong)
				require.Error(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkLegacyJoinPublishedOutcome(ctx, p, &wrong) }))
			}
			// Expired initiating calls cannot complete the safe outcome, even with valid
			// retained publication history. Each control has its own monotonic clock.
			if name == "expired" {
				elapsed.Store(int64(5 * time.Second))
				require.Error(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkLegacyJoinPublishedOutcome(ctx, p, native) }))
			}
			if name == "sql_expiry" {
				holder, e := st.Pool.Begin(ctx)
				require.NoError(t, e)
				defer holder.Rollback(ctx)
				var locked uuid.UUID
				require.NoError(t, holder.QueryRow(ctx, `SELECT operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, op.ActorProfileID, op.OperationID).Scan(&locked))
				var pid int32
				require.NoError(t, holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
				result := make(chan error, 1)
				go func() {
					result <- st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkLegacyJoinPublishedOutcome(ctx, p, native) })
				}()
				require.Eventually(t, func() bool {
					var blocked bool
					return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked) == nil && blocked
				}, 3*time.Second, 5*time.Millisecond)
				elapsed.Store(int64(5 * time.Second))
				require.NoError(t, holder.Commit(ctx))
				require.Error(t, <-result, "post-SQL ingress check rolls back the outcome")
			}
			if name != "fresh" {
				var state string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, op.ActorProfileID, op.OperationID).Scan(&state))
				require.Equal(t, "DECIDED", state)
				return
			}
			require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { return local.MarkLegacyJoinPublishedOutcome(ctx, p, native) }))
			member, done, err := st.ReplayLegacyJoinOutcome(ctx, p)
			require.NoError(t, err)
			require.True(t, done)
			require.Equal(t, in.SpaceID.String(), member.SpaceId)
			require.Equal(t, in.ProfileID.String(), member.ProfileId)
			require.True(t, member.JoinedAt.AsTime().Equal(joined))
			var count int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID).Scan(&count))
			require.Zero(t, count, "the journal hook never inserts membership")
		})
	}
}
