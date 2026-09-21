package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/space/internal/store"
)

type ownershipAuthClientStub struct {
	authv1.AuthServiceClient
	consume func(context.Context, *authv1.ConsumeOwnershipTransferProofRequest) (*authv1.ConsumeOwnershipTransferProofResponse, error)
	lookup  func(context.Context, *authv1.GetOwnershipTransferReceiptRequest) (*authv1.GetOwnershipTransferReceiptResponse, error)
}

func (c *ownershipAuthClientStub) ConsumeOwnershipTransferProof(ctx context.Context, request *authv1.ConsumeOwnershipTransferProofRequest, _ ...grpc.CallOption) (*authv1.ConsumeOwnershipTransferProofResponse, error) {
	return c.consume(ctx, request)
}

func (c *ownershipAuthClientStub) GetOwnershipTransferReceipt(ctx context.Context, request *authv1.GetOwnershipTransferReceiptRequest, _ ...grpc.CallOption) (*authv1.GetOwnershipTransferReceiptResponse, error) {
	return c.lookup(ctx, request)
}

func ownershipAuthBinding() store.OwnershipBinding {
	return store.OwnershipBinding{ProtocolVersion: 2, OperationID: uuid.New(), SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New(), NewOwnerProfileID: uuid.New(), SessionEpoch: 7, ProofDigest: strings.Repeat("a", 64)}
}

func ownershipAuthReceipt(binding store.OwnershipBinding) *authv1.ConsumeOwnershipTransferProofResponse {
	return &authv1.ConsumeOwnershipTransferProofResponse{ReceiptId: uuid.NewString(), AccountId: binding.AccountID.String(), ProfileId: binding.ActorProfileID.String(), SpaceId: binding.SpaceID.String(), NewOwnerProfileId: binding.NewOwnerProfileID.String(), OperationId: binding.OperationID.String(), SessionEpoch: binding.SessionEpoch, ConsumedAt: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond)), VerifiedFactors: []string{"password"}}
}

func TestOwnershipAuth_ConsumeUsesExactServicePrincipalAndRecoversLostResponse(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	binding, proof := ownershipAuthBinding(), "opaque-proof"
	digest := sha256.Sum256([]byte(proof))
	binding.ProofDigest = hex.EncodeToString(digest[:])
	valid := ownershipAuthReceipt(binding)
	client := &ownershipAuthClientStub{}
	client.consume = func(ctx context.Context, request *authv1.ConsumeOwnershipTransferProofRequest) (*authv1.ConsumeOwnershipTransferProofResponse, error) {
		require.Equal(t, binding.AccountID.String(), request.GetAccountId())
		require.Equal(t, binding.ActorProfileID.String(), request.GetProfileId())
		require.Equal(t, binding.SpaceID.String(), request.GetSpaceId())
		require.Equal(t, binding.NewOwnerProfileID.String(), request.GetNewOwnerProfileId())
		require.Equal(t, binding.OperationID.String(), request.GetOperationId())
		require.Equal(t, binding.SessionEpoch, request.GetSessionEpoch())
		require.Equal(t, proof, request.GetProof())
		verifyOwnershipCredential(t, ctx, request, authv1.AuthService_ConsumeOwnershipTransferProof_FullMethodName, key)
		return nil, context.DeadlineExceeded
	}
	client.lookup = func(ctx context.Context, request *authv1.GetOwnershipTransferReceiptRequest) (*authv1.GetOwnershipTransferReceiptResponse, error) {
		require.Equal(t, hex.EncodeToString(digest[:]), request.GetProofDigest())
		verifyOwnershipCredential(t, ctx, request, authv1.AuthService_GetOwnershipTransferReceipt_FullMethodName, key)
		return &authv1.GetOwnershipTransferReceiptResponse{ReceiptId: valid.GetReceiptId(), AccountId: valid.GetAccountId(), ProfileId: valid.GetProfileId(), SpaceId: valid.GetSpaceId(), NewOwnerProfileId: valid.GetNewOwnerProfileId(), OperationId: valid.GetOperationId(), SessionEpoch: valid.GetSessionEpoch(), ConsumedAt: valid.GetConsumedAt(), VerifiedFactors: valid.GetVerifiedFactors()}, nil
	}
	s := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: client}
	receipt, err := s.consumeOwnershipAuthProof(context.Background(), binding, proof)
	require.NoError(t, err)
	require.Equal(t, uuid.MustParse(valid.GetReceiptId()), receipt.ReceiptID)
}

func TestOwnershipAuth_RejectsMalformedOrMismatchedReceipts(t *testing.T) {
	issuer, _ := newOwnershipTestIssuer(t)
	binding := ownershipAuthBinding()
	digest := sha256.Sum256([]byte("opaque-proof"))
	binding.ProofDigest = hex.EncodeToString(digest[:])
	for _, mutate := range []func(*authv1.ConsumeOwnershipTransferProofResponse){
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.ReceiptId = "not-a-uuid" },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.AccountId = uuid.NewString() },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.ProfileId = uuid.NewString() },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.SpaceId = uuid.NewString() },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.NewOwnerProfileId = uuid.NewString() },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.OperationId = uuid.NewString() },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.SessionEpoch++ },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.ConsumedAt = nil },
		func(r *authv1.ConsumeOwnershipTransferProofResponse) { r.VerifiedFactors = []string{"totp"} },
	} {
		response := ownershipAuthReceipt(binding)
		mutate(response)
		s := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: &ownershipAuthClientStub{consume: func(context.Context, *authv1.ConsumeOwnershipTransferProofRequest) (*authv1.ConsumeOwnershipTransferProofResponse, error) {
			return response, nil
		}}}
		_, err := s.consumeOwnershipAuthProof(context.Background(), binding, "opaque-proof")
		require.Error(t, err)
	}
}

func TestOwnershipAuth_ContextReplacesPoisonedAuthorityMetadata(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	binding := ownershipAuthBinding()
	digest := sha256.Sum256([]byte("opaque-proof"))
	binding.ProofDigest = hex.EncodeToString(digest[:])
	client := &ownershipAuthClientStub{consume: func(ctx context.Context, request *authv1.ConsumeOwnershipTransferProofRequest) (*authv1.ConsumeOwnershipTransferProofResponse, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		for _, name := range []string{"x-profile-id", "x-account-id", "x-user-id", "x-actor-id", "x-internal-caller", "x-voice-profile-id"} {
			require.Empty(t, md.Get(name))
		}
		verifyOwnershipCredential(t, ctx, request, authv1.AuthService_ConsumeOwnershipTransferProof_FullMethodName, key)
		return ownershipAuthReceipt(binding), nil
	}}
	poisoned := metadata.Pairs("authorization", "Bearer user", "authorization", "Bearer duplicate", "x-request-id", "old", "x-profile-id", uuid.NewString(), "x-voice-profile-id", "forged")
	ctx := metadata.NewOutgoingContext(metadata.NewIncomingContext(context.Background(), poisoned), poisoned)
	s := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: client}
	_, err := s.consumeOwnershipAuthProof(ctx, binding, "opaque-proof")
	require.NoError(t, err)
}
