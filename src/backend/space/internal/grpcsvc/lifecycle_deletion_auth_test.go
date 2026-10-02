package grpcsvc

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

type deletionAuthClientStub struct {
	authv1.AuthServiceClient
	consume func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error)
	lookup  func(context.Context, *authv1.GetSpaceDeletionProofReceiptRequest) (*authv1.GetSpaceDeletionProofReceiptResponse, error)
}

func (c *deletionAuthClientStub) ConsumeSpaceDeletionProof(ctx context.Context, req *authv1.ConsumeSpaceDeletionProofRequest, _ ...grpc.CallOption) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
	return c.consume(ctx, req)
}

func (c *deletionAuthClientStub) GetSpaceDeletionProofReceipt(ctx context.Context, req *authv1.GetSpaceDeletionProofReceiptRequest, _ ...grpc.CallOption) (*authv1.GetSpaceDeletionProofReceiptResponse, error) {
	return c.lookup(ctx, req)
}

func TestSpaceDeletionProofConsumeRecoversReceiptAndStoresDeterministicResponse(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	accountID, profileID, spaceID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	proof, name, epoch := "opaque-deletion-proof", "Exact Space Name", int64(8)
	request := &spacev1.DeleteSpaceRequest{SpaceId: spaceID.String(), OperationId: operationID.String(), ConfirmationName: name, Proof: proof}
	response := validDeletionProofResponse(accountID, profileID, epoch, spaceID, operationID, name, proof)
	client := &deletionAuthClientStub{}
	client.consume = func(ctx context.Context, req *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
		require.Equal(t, uint32(1), req.GetProtocolVersion())
		require.Equal(t, accountID.String(), req.GetAccountId())
		require.Equal(t, profileID.String(), req.GetProfileId())
		require.Equal(t, epoch, req.GetSessionEpoch())
		require.Equal(t, spaceID.String(), req.GetSpaceId())
		require.Equal(t, operationID.String(), req.GetOperationId())
		require.Equal(t, name, req.GetConfirmationName())
		require.Equal(t, proof, req.GetProof())
		verifyOwnershipCredential(t, ctx, req, authv1.AuthService_ConsumeSpaceDeletionProof_FullMethodName, key)
		return nil, context.DeadlineExceeded
	}
	client.lookup = func(ctx context.Context, req *authv1.GetSpaceDeletionProofReceiptRequest) (*authv1.GetSpaceDeletionProofReceiptResponse, error) {
		nameDigest, proofDigest := sha256.Sum256([]byte(name)), sha256.Sum256([]byte(proof))
		require.Equal(t, uint32(1), req.GetProtocolVersion())
		require.Equal(t, accountID.String(), req.GetAccountId())
		require.Equal(t, profileID.String(), req.GetProfileId())
		require.Equal(t, epoch, req.GetSessionEpoch())
		require.Equal(t, spaceID.String(), req.GetSpaceId())
		require.Equal(t, operationID.String(), req.GetOperationId())
		require.Equal(t, nameDigest[:], req.GetConfirmationNameSha256())
		require.Equal(t, proofDigest[:], req.GetProofDigestSha256())
		verifyOwnershipCredential(t, ctx, req, authv1.AuthService_GetSpaceDeletionProofReceipt_FullMethodName, key)
		return &authv1.GetSpaceDeletionProofReceiptResponse{Receipt: response.GetReceipt()}, nil
	}
	service := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: client}
	wrapped, digest, err := service.consumeSpaceDeletionProof(context.Background(), accountID, profileID, epoch, request)
	require.NoError(t, err)
	var stored authv1.ConsumeSpaceDeletionProofResponse
	require.NoError(t, proto.Unmarshal(wrapped, &stored))
	require.True(t, proto.Equal(response, &stored))
	expected, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	require.Equal(t, expected, wrapped, "saved evidence is the exact typed Auth response")
	require.Equal(t, lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", wrapped), digest)
}

func TestSpaceDeletionProofConsumedResponsePersistsInRealStore(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
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
	row, err := st.CreateSpace(ctx, owner, "typed proof persistence", "", "private")
	require.NoError(t, err)
	const epoch = int64(7)
	const proof = "test-owned opaque proof"
	req := &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: operation.String(), ConfirmationName: row.Name, Proof: proof}
	_, err = st.ReserveLifecycleSchedule(ctx, account, owner, epoch, req)
	require.NoError(t, err)
	issuer, _ := newOwnershipTestIssuer(t)
	response := validDeletionProofResponse(account, owner, epoch, row.ID, operation, row.Name, proof)
	service := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: &deletionAuthClientStub{consume: func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
		return response, nil
	}}}
	raw, digest, err := service.consumeSpaceDeletionProof(ctx, account, owner, epoch, req)
	require.NoError(t, err)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, raw, digest)
	require.NoError(t, err, "real store must accept the validated typed Auth response")
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, raw, digest)
	require.NoError(t, err, "exact response retry must be durable and idempotent")
}

func TestSpaceDeletionProofConsumeRejectsReceiptWithChangedBinding(t *testing.T) {
	issuer, _ := newOwnershipTestIssuer(t)
	accountID, profileID, spaceID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	proof, name, epoch := "opaque-deletion-proof", "Exact Space Name", int64(8)
	response := validDeletionProofResponse(accountID, profileID, epoch, spaceID, operationID, name, proof)
	response.Receipt.BindingSha256 = append([]byte(nil), response.Receipt.BindingSha256...)
	response.Receipt.BindingSha256[0] ^= 0xff
	service := &SpaceGRPC{
		PrincipalIssuer: issuer,
		OwnershipAuth: &deletionAuthClientStub{consume: func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
			return response, nil
		}},
	}
	_, _, err := service.consumeSpaceDeletionProof(context.Background(), accountID, profileID, epoch, &spacev1.DeleteSpaceRequest{
		SpaceId: spaceID.String(), OperationId: operationID.String(), ConfirmationName: name, Proof: proof,
	})
	require.Error(t, err)
}

func validDeletionProofResponse(accountID, profileID uuid.UUID, epoch int64, spaceID, operationID uuid.UUID, name, proof string) *authv1.ConsumeSpaceDeletionProofResponse {
	nameDigest, proofDigest := sha256.Sum256([]byte(name)), sha256.Sum256([]byte(proof))
	factors := []authv1.VerifiedFactor{authv1.VerifiedFactor_VERIFIED_FACTOR_PASSWORD, authv1.VerifiedFactor_VERIFIED_FACTOR_TOTP}
	binding := &authv1.SpaceDeletionProofBinding{
		ProtocolVersion: 1, AccountId: accountID.String(), ProfileId: profileID.String(), SessionEpoch: epoch,
		SpaceId: spaceID.String(), OperationId: operationID.String(), ConfirmationNameSha256: nameDigest[:],
		ProofDigestSha256: proofDigest[:], VerifiedFactors: factors, Purpose: authv1.ProofPurpose_PROOF_PURPOSE_SPACE_DELETE,
	}
	bindingBytes, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(binding)
	bindingDigest := lifecycleEvidenceHash("voice.auth.v1.SpaceDeletionProofBinding", bindingBytes)
	return &authv1.ConsumeSpaceDeletionProofResponse{Receipt: &authv1.SpaceDeletionProofReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: operationID.String(), BindingSha256: bindingDigest[:],
		ConfirmationNameSha256: nameDigest[:], ConsumedAt: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond)), VerifiedFactors: factors,
	}}
}
