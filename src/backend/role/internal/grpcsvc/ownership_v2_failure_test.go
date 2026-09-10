package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/store"
)

func TestOwnershipTransferV2_FinalPersistenceFailuresUnavailableWithoutMutationOrFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	for _, failure := range []string{"closed pool", "query failure"} {
		t.Run(failure, func(t *testing.T) {
			s, cleanup := startRoleStoreTest(t)
			defer cleanup()
			ctx := context.Background()
			intent := v2HandlerIntent()
			spaceID := uuid.MustParse(intent.SpaceId)
			ownerID := uuid.MustParse(intent.OldOwnerProfileId)
			require.NoError(t, s.BootstrapSpaceRoles(ctx, spaceID, ownerID))
			prepare := &rolev1.PrepareOwnershipTransferRequest{Intent: intent}
			healthy := &RoleGRPC{Store: s}
			_, err := healthy.PrepareOwnershipTransfer(terminalOwnershipContext(t, rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, prepare), prepare)
			require.NoError(t, err)
			var before string
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT row_to_json(ownership_transfer_v2)::text FROM ownership_transfer_v2 WHERE operation_id=$1`, intent.OperationId).Scan(&before))
			broken := healthy
			if failure == "closed pool" {
				pool, err := pgxpool.NewWithConfig(ctx, s.Pool.Config())
				require.NoError(t, err)
				pool.Close()
				broken = &RoleGRPC{Store: &store.RoleStore{Pool: pool}}
			} else {
				_, err = s.Pool.Exec(ctx, `ALTER TABLE ownership_transfer_v2 RENAME TO unavailable_ownership_v2`)
				require.NoError(t, err)
				defer func() {
					_, _ = s.Pool.Exec(context.Background(), `ALTER TABLE IF EXISTS unavailable_ownership_v2 RENAME TO ownership_transfer_v2`)
				}()
			}
			// Every request has a verified exact principal; failure must be persistence
			// Unavailable, not auth failure, empty success, or a legacy owner mutation.
			for _, action := range v2HandlerCases(broken) {
				req := action.wrap(intent)
				err := action.invoke(terminalOwnershipContext(t, action.rpc, req), req)
				require.Equal(t, codes.Unavailable, status.Code(err), action.rpc)
			}
			if failure == "query failure" {
				_, err = s.Pool.Exec(ctx, `ALTER TABLE unavailable_ownership_v2 RENAME TO ownership_transfer_v2`)
				require.NoError(t, err)
			}
			var after string
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT row_to_json(ownership_transfer_v2)::text FROM ownership_transfer_v2 WHERE operation_id=$1`, intent.OperationId).Scan(&after))
			require.Equal(t, before, after)
			var legacy int
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM ownership_transfer_role_receipts WHERE operation_id=$1`, intent.OperationId).Scan(&legacy))
			require.Zero(t, legacy)
			requireSoleTerminalOwner(t, s, spaceID, ownerID)
		})
	}
}
