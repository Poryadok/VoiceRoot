package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/entrypolicy"
)

func TestJoinAttempt_ReadOwnRequiresExactAccountProfileAndReturnsSafeSnapshot(t *testing.T) {
	f := newJoinGroupFixture(t)
	question := uuid.New()
	_, err := f.store.SetEntryPolicy(f.ctx, SetEntryPolicyInput{SpaceID: f.space, ActorProfileID: f.owner, OperationID: uuid.New(), ExpectedVersion: 2, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{{ID: question, Prompt: "What is 2x2?", Kind: "exact_text", ExpectedAnswer: "configured-secret-4"}}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: f.space, AccountID: f.account, ProfileID: f.profile}
	a, err := f.store.StartJoinAttempt(f.ctx, in)
	require.NoError(t, err)
	checks := 0
	guard := func() error { checks++; return nil }
	for _, other := range []struct{ account, profile uuid.UUID }{{uuid.New(), f.profile}, {f.account, uuid.New()}} {
		_, err = f.store.ReadOwnJoinAttempt(f.ctx, f.space, a.ID, other.account, other.profile, guard)
		require.ErrorIs(t, err, ErrJoinReadNotFound)
	}
	require.Zero(t, checks, "foreign and missing lookup exposes no snapshot")
	_, err = f.store.ReadOwnJoinAttempt(f.ctx, f.space, uuid.New(), f.account, f.profile, guard)
	require.ErrorIs(t, err, ErrJoinReadNotFound)
	view, err := f.store.ReadOwnJoinAttempt(f.ctx, f.space, a.ID, f.account, f.profile, guard)
	require.NoError(t, err)
	require.Equal(t, "COLLECTING", view.State)
	require.Equal(t, []JoinQuestionRead{{ID: question.String(), Prompt: "What is 2x2?"}}, view.Requirements.Questions)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "configured-secret")
	require.NotContains(t, string(raw), "SHA256")
	expired := errors.New("private owner capability expired")
	_, err = f.store.ReadOwnJoinAttempt(f.ctx, f.space, a.ID, f.account, f.profile, func() error { return expired })
	require.ErrorIs(t, err, expired, "a final authority failure returns no projection")
	_, err = f.store.ReadOwnJoinAttempt(f.ctx, f.space, a.ID, f.account, f.profile, nil)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	_, err = f.store.Pool.Exec(f.ctx, `UPDATE space_join_attempts SET collection_deadline=clock_timestamp() WHERE id=$1`, a.ID)
	require.NoError(t, err)
	view, err = f.store.ReadOwnJoinAttempt(f.ctx, f.space, a.ID, f.account, f.profile, guard)
	require.NoError(t, err)
	require.Equal(t, "EXPIRED", view.State)
	var stored string
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT state FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&stored))
	require.Equal(t, "COLLECTING", stored, "status reads do not mutate admission journals")
}

func TestJoinAttempt_ReadPendingReturnsActualQWithoutAdmissionSideEffects(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	view, err := f.store.ReadOwnJoinAttempt(f.ctx, f.space, a.ID, f.account, f.profile, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, "PENDING", view.State)
	require.NotNil(t, view.RequestID)
	require.Equal(t, q.ID, *view.RequestID)
	require.NotEqual(t, a.ID, *view.RequestID)
	require.Nil(t, view.PublicationID)
	var count int
	require.NoError(t, f.store.Pool.QueryRow(f.ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, f.space, f.profile).Scan(&count))
	require.Zero(t, count)
}
