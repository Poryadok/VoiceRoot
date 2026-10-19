package store

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinAttempt_SubmissionProposalUsesExactServerSnapshot(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic", true: "manual"}[manual], func(t *testing.T) {
			ctx := context.Background()
			pool := startJoinAttemptPostgres(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			owner := uuid.New()
			space, err := st.CreateSpace(ctx, owner, "Submission plan", "", "public")
			require.NoError(t, err)
			applyEntryPolicyMigration(t, st)
			applyJoinAttemptMigration(t, st)
			applyJoinOperationMigration(t, st)
			question := uuid.New()
			policy := entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: manual, Questions: []entrypolicy.Question{{ID: question, Kind: "exact_text", Prompt: "2x2", ExpectedAnswer: "4"}}}
			_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 1, Policy: policy})
			require.NoError(t, err)
			in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			a, err := st.StartJoinAttempt(ctx, in)
			require.NoError(t, err)
			req := &spacev1.SubmitJoinAttemptRequest{SpaceId: space.ID.String(), AttemptId: a.ID.String(), OperationId: uuid.NewString(), Answers: []*spacev1.JoinAnswer{{QuestionId: question.String(), Value: " 4 "}}}
			raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
			require.NoError(t, err)
			requestSHA := sha256.Sum256(raw)
			op := JoinOperationInput{SpaceID: space.ID, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.MustParse(req.OperationId), Method: JoinOperationSubmit, RequestSHA256: requestSHA[:]}
			_, err = st.ReserveJoinOperation(ctx, op, &a.ID)
			require.NoError(t, err)
			// Internal storage fixture only: a proposal with no private ingress
			// capability must never yield an owner request or sealed decision.
			proposal, err := st.prepareJoinSubmissionProposal(ctx, op, req, nil)
			require.NoError(t, err)
			c := proposal.Context()
			require.Equal(t, op.SpaceID, c.SpaceID)
			require.Equal(t, a.ID, c.AttemptID)
			require.Equal(t, op.ActorAccountID, c.ActorAccountID)
			require.Equal(t, op.ActorProfileID, c.ActorProfileID)
			require.Equal(t, op.OperationID, c.OperationID)
			require.Equal(t, op.RequestSHA256, c.RequestSHA256)
			if manual {
				require.Equal(t, JoinDecisionSubmitPending, c.Kind)
				require.Nil(t, c.ReadyEvidenceSHA256)
			} else {
				require.Equal(t, JoinDecisionSubmitReady, c.Kind)
				require.Len(t, c.ReadyEvidenceSHA256, 32)
			}
			_, err = proposal.OwnerRequest()
			require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
			_, err = proposal.Seal(&admissionauthority.VerifiedSubject{})
			require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
			// Compare against the actual frozen35 transition, then roll it back:
			// prospective data must not reinterpret its immutable evidence codec.
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space.ID}))
			scoped := *st
			scoped.tx = tx
			submitted, err := scoped.SubmitJoinAttempt(ctx, in, a.ID, []entrypolicy.Answer{{QuestionID: question, Value: " 4 "}})
			require.NoError(t, err)
			require.Equal(t, c.ReadyEvidenceSHA256, submitted.EvidenceSHA256)
			var actualSubmission []byte
			require.NoError(t, tx.QueryRow(ctx, `SELECT submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&actualSubmission))
			require.Equal(t, c.SubmissionSHA256, actualSubmission)
			require.NoError(t, tx.Rollback(ctx))
			original := append([]byte(nil), c.AttemptBindingSHA256...)
			c.AttemptBindingSHA256[0] ^= 1
			require.Equal(t, original, proposal.Context().AttemptBindingSHA256, "data projection cannot mutate the prospective owner context")
			req.Answers[0].Value = "different"
			require.Equal(t, " 4 ", proposal.request.Answers[0].Value)
			// Policy change invalidates this stored attempt for a new decision.
			policy.RequireManualApproval = !manual
			_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: 2, Policy: policy})
			require.NoError(t, err)
			_, err = st.prepareJoinSubmissionProposal(ctx, op, proposal.request, nil)
			require.ErrorIs(t, err, ErrJoinAttemptDenied)
			var state string
			var submission []byte
			require.NoError(t, pool.QueryRow(ctx, `SELECT state,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state, &submission))
			require.Equal(t, "COLLECTING", state)
			require.Nil(t, submission)
		})
	}
}

func TestJoinAttempt_SubmissionProposalRejectsUnverifiedRequestBeforeReservation(t *testing.T) {
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyEntryPolicyMigration(t, st)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	request := &spacev1.SubmitJoinAttemptRequest{SpaceId: uuid.NewString(), AttemptId: uuid.NewString(), OperationId: uuid.NewString()}
	_, err := st.PrepareJoinSubmissionDecision(ctx, request)
	require.Error(t, err)
	account := uuid.NewString()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	generic := principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "space", Subject: account, AccountID: account, ProfileID: uuid.NewString(), RequestID: uuid.NewString(), RPC: JoinOperationSubmit, RequestHash: hash, SessionEpoch: 7, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)})
	_, err = st.PrepareJoinSubmissionDecision(generic, request)
	require.Error(t, err, "generic principal alone is never a private admission call")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations`).Scan(&count))
	require.Zero(t, count)
}
