package grpcsvc

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

func TestRecoverLifecycleProofPersistsExactLookupReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	for _, changedBinding := range []bool{false, true} {
		name := "exact receipt survives restart"
		if changedBinding {
			name = "changed binding leaves proof pending"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpacePostgresForTest(t, ctx)
			dir := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db")
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".up.sql") {
					raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
					require.NoError(t, err)
					_, err = pool.Exec(ctx, string(raw))
					require.NoError(t, err, entry.Name())
				}
			}
			st := &store.SpaceStore{Pool: pool}
			account, owner, operation := uuid.New(), uuid.New(), uuid.New()
			row, err := st.CreateSpace(ctx, owner, "recover consumed deletion proof", "", "private")
			require.NoError(t, err)
			const epoch = int64(7)
			const proof = "test-owned opaque consumed proof"
			req := &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: operation.String(), ConfirmationName: row.Name, Proof: proof}
			_, err = st.ReserveLifecycleSchedule(ctx, account, owner, epoch, req)
			require.NoError(t, err)
			response := validDeletionProofResponse(account, owner, epoch, row.ID, operation, row.Name, proof)
			if changedBinding {
				response.Receipt.BindingSha256[0] ^= 0xff
			}
			issuer, key := newOwnershipTestIssuer(t)
			lookups := 0
			client := &deletionAuthClientStub{lookup: func(call context.Context, request *authv1.GetSpaceDeletionProofReceiptRequest) (*authv1.GetSpaceDeletionProofReceiptResponse, error) {
				lookups++
				verifyOwnershipCredential(t, call, request, authv1.AuthService_GetSpaceDeletionProofReceipt_FullMethodName, key)
				require.Equal(t, account.String(), request.GetAccountId())
				require.Equal(t, owner.String(), request.GetProfileId())
				require.Equal(t, epoch, request.GetSessionEpoch())
				require.Equal(t, row.ID.String(), request.GetSpaceId())
				require.Equal(t, operation.String(), request.GetOperationId())
				nameHash, proofHash := sha256.Sum256([]byte(row.Name)), sha256.Sum256([]byte(proof))
				require.Equal(t, nameHash[:], request.GetConfirmationNameSha256())
				require.Equal(t, proofHash[:], request.GetProofDigestSha256())
				return &authv1.GetSpaceDeletionProofReceiptResponse{Receipt: response.GetReceipt()}, nil
			}}
			service := &SpaceGRPC{Store: st, PrincipalIssuer: issuer, OwnershipAuth: client}
			err = service.RecoverLifecycleProof(ctx, row.ID)
			if changedBinding {
				require.Error(t, err)
				saved, lookupErr := st.LifecycleDeletionProofRecorded(ctx, row.ID)
				require.NoError(t, lookupErr)
				require.False(t, saved)
				return
			}
			require.NoError(t, err, "a valid consumed receipt must unblock background recovery")
			saved, err := st.LifecycleDeletionProofRecorded(ctx, row.ID)
			require.NoError(t, err)
			require.True(t, saved)
			// The foreground path must recognize the same exact typed bytes.
			client.consume = func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
				return response, nil
			}
			raw, digest, err := service.consumeSpaceDeletionProof(ctx, account, owner, epoch, req)
			require.NoError(t, err)
			_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, raw, digest)
			require.NoError(t, err, "background and foreground persistence must share immutable bytes")
			require.NoError(t, service.RecoverLifecycleProof(ctx, row.ID))
			require.Equal(t, 1, lookups, "restart after persistence cannot consume or look up another receipt")
		})
	}
}
