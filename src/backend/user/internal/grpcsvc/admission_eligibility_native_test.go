package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"testing"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/store"
)

func TestAdmissionEligibility_NativeExactPairAndRevisionWriters(t *testing.T) {
	dsn := os.Getenv("USER_ADMISSION_TEST_DSN")
	if dsn == "" {
		t.Skip("explicit isolated native fixture required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
	a, p := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary,is_guest_account) VALUES($1,$2,$3,'0001','Admission guest',true,true)`, p, a, "admission-"+uuid.NewString()[:8])
	require.NoError(t, err)
	service := &AdmissionEligibilityGRPC{Profiles: store.NewProfileStore(pool)}
	call := func(account uuid.UUID) *userv1.GetSpaceAdmissionProfileEligibilityResponse {
		req := &userv1.GetSpaceAdmissionProfileEligibilityRequest{AccountId: account.String(), ProfileId: p.String(), ObservationBindingSha256: make([]byte, 32)}
		result, e := service.GetSpaceAdmissionProfileEligibility(sdkVerifiedAuth(t, ctx, userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName, req), req)
		if account != a {
			require.Equal(t, codes.PermissionDenied, status.Code(e))
			return nil
		}
		require.NoError(t, e)
		return result
	}
	call(uuid.New())
	initial := call(a)
	require.False(t, initial.Frozen)
	require.False(t, initial.Deleted)
	proveAdmissionTransport(t, pool, a, p)
	err = store.NewProfileStore(pool).MarkAccountRegular(ctx, a)
	require.NoError(t, err)
	converted := call(a)
	require.GreaterOrEqual(t, converted.ProfileRevision, initial.ProfileRevision)
	_, err = pool.Exec(ctx, `UPDATE profiles SET frozen_at=now() WHERE id=$1`, p)
	require.NoError(t, err)
	frozen := call(a)
	require.True(t, frozen.Frozen)
	require.Greater(t, frozen.ProfileRevision, converted.ProfileRevision)
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at=now() WHERE id=$1`, p)
	require.NoError(t, err)
	deleted := call(a)
	require.True(t, deleted.Deleted)
	require.Greater(t, deleted.ProfileRevision, frozen.ProfileRevision)
}
