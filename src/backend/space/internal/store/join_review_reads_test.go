package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJoinReviewCursorBoundToSpaceAndStableWithoutBoundaryRow(t *testing.T) {
	space, id := uuid.New(), uuid.New()
	created := time.Date(2026, 10, 19, 10, 20, 30, 123456000, time.UTC)
	token := encodeJoinReviewCursor(space, created, id)
	gotTime, gotID, err := decodeJoinReviewCursor(token, space)
	require.NoError(t, err)
	require.Equal(t, created, gotTime)
	require.Equal(t, id, gotID)
	_, _, err = decodeJoinReviewCursor(token, uuid.New())
	require.Error(t, err)
	for _, malformed := range []string{"garbage", token + "=", token[:len(token)-1]} {
		_, _, err = decodeJoinReviewCursor(malformed, space)
		require.Error(t, err)
	}
}

// This tests only the data projection. It supplies no visibility capability and
// cannot qualify a public reviewer or the native readiness producer.
func TestJoinReviewStoredProjectionPreservesDistinctAttemptsAndHidesConfiguredData(t *testing.T) {
	f := newJoinGroupFixture(t)
	a, q := f.child(t)
	b, other := f.child(t)
	require.Equal(t, q.ID, other.ID)
	var view *joinReviewGroupRead
	err := f.store.withOwnershipScope(f.ctx, []uuid.UUID{f.space}, func(local *SpaceStore) error {
		var e error
		view, e = local.readJoinReviewGroup(f.ctx, f.space, q.ID)
		return e
	})
	require.NoError(t, err)
	require.Equal(t, q.ID.String(), view.summary.RequestId)
	require.Equal(t, f.profile.String(), view.summary.ApplicantProfileId)
	require.EqualValues(t, 2, view.summary.AttemptCount)
	require.Len(t, view.attempts, 2)
	require.ElementsMatch(t, []string{a.ID.String(), b.ID.String()}, []string{view.attempts[0].AttemptId, view.attempts[1].AttemptId})
	raw, err := json.Marshal(view.attempts)
	require.NoError(t, err)
	for _, private := range []string{"expected_answer", "policy_sha256", "invite", "account_id", "captcha_token", "binding"} {
		require.NotContains(t, string(raw), private)
	}
	err = f.store.withOwnershipScope(f.ctx, []uuid.UUID{f.space}, func(local *SpaceStore) error {
		_, e := local.readJoinReviewGroup(f.ctx, f.space, uuid.New())
		return e
	})
	require.ErrorIs(t, err, ErrJoinReadNotFound)
}
