//go:build spacebanintegration

package store

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
)

func TestAccountBanLateCaptureCannotCommitOrSplice50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	serving := projectionServingLogin(t, ctx, owner)
	space, operation := nativeBanSpace(12), uuid.New()
	var first *userv1.CaptureSpaceMemberAccountBindingsRequest
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		first = proto.Clone(r).(*userv1.CaptureSpaceMemberAccountBindingsRequest)
		// A misbehaving remote client can return after context cancellation;
		// elapsed owner/phase budgets must still reject that complete response.
		time.Sleep(2100 * time.Millisecond)
		return nativeBanOwnerFixture(r), nil
	})}
	allow := func(context.Context) error { return nil }
	err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, allow)
	require.ErrorIs(t, err, ErrAccountBanUnavailable)
	require.NotNil(t, first)
	nativeBanAssertNoEffect(t, serving, space)
	var qualifications int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_qualifications WHERE space_id=$1`, space).Scan(&qualifications))
	require.Zero(t, qualifications, "late response supplies no accepted identity facts")
	st.AccountCaptures = nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		require.True(t, proto.Equal(first, r), "same source cohort reuses its immutable child request")
		fresh := nativeBanOwnerFixture(r)
		fresh.OwnerSourceRevision = 902
		fresh.IdentityCut.ThroughRevision = 4
		fresh.IdentityCut.Digest = bytes.Repeat([]byte{2}, 32)
		return fresh, nil
	})
	require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, allow), "later authenticated phase can consume a fresh coherent whole response")
	var children int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_captures WHERE actor_profile_id=$1 AND client_operation_id=$2`, nativeBanProfile(1), operation).Scan(&children))
	require.Equal(t, 1, children)
	var raw []byte
	require.NoError(t, serving.QueryRow(ctx, `SELECT response_bytes FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2 AND state='COMMITTED'`, nativeBanProfile(1), operation).Scan(&raw))
	var saved userv1.CaptureSpaceMemberAccountBindingsResponse
	require.NoError(t, proto.Unmarshal(raw, &saved))
	require.EqualValues(t, 902, saved.OwnerSourceRevision)
	require.EqualValues(t, 4, saved.IdentityCut.ThroughRevision)
	require.Equal(t, bytes.Repeat([]byte{2}, 32), saved.IdentityCut.Digest)
	var audit int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&audit))
	require.Equal(t, 1, audit)
}
