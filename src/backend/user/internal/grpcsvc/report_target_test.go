package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"strings"
	"testing"
	commonv1 "voice.app/voice/common/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/reporttargetprincipal"
	"voice/backend/user/internal/store"
)

type reportTargetDeletionFunc func(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error)

func (f reportTargetDeletionFunc) DeletedAmong(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return f(ctx, ids)
}
func TestReportTargetCurrentAuth(t *testing.T) {
	dsn := os.Getenv("REPORT_TARGET_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := "target_auth_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TABLE profiles(id uuid PRIMARY KEY,account_id uuid,deleted_at timestamptz);CREATE TABLE user_account_lifecycle(account_id uuid,state text)`)
	require.NoError(t, err)
	profile, account := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id) VALUES($1,$2)`, profile, account)
	require.NoError(t, err)
	req := &userv1.ResolveReportTargetRequest{ReportId: uuid.NewString(), ProfileId: profile.String()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx = principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "moderation", Subject: "service:moderation", Audience: "user", RPC: reporttargetprincipal.Method("user"), RequestID: uuid.NewString(), RequestHash: hash})
	svc := &UserGRPC{Profiles: store.NewProfileStore(pool)}
	_, err = svc.ResolveReportTarget(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err), "missing current Auth authority must not certify LIVE")
	for name, fixture := range map[string]struct {
		deleted map[uuid.UUID]struct{}
		failure error
		state   commonv1.ReportTargetState
		code    codes.Code
	}{
		"live":    {deleted: map[uuid.UUID]struct{}{}, state: commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE},
		"deleted": {deleted: map[uuid.UUID]struct{}{account: {}}, state: commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED},
		"outage":  {failure: errors.New("unavailable"), code: codes.Unavailable},
		"foreign": {deleted: map[uuid.UUID]struct{}{uuid.New(): {}}, code: codes.Unavailable},
	} {
		t.Run(name, func(t *testing.T) {
			svc.DeletedAccounts = reportTargetDeletionFunc(func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
				require.Equal(t, []uuid.UUID{account}, ids)
				return fixture.deleted, fixture.failure
			})
			out, err := svc.ResolveReportTarget(ctx, req)
			require.Equal(t, fixture.code, status.Code(err))
			if err == nil {
				require.Equal(t, fixture.state, out.State)
				if fixture.state == commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE {
					require.Equal(t, account.String(), out.AccountId)
				} else {
					require.Empty(t, out.AccountId)
				}
			}
		})
	}
}
