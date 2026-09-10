package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// The embedded nil client makes any generic Role RPC an immediate test failure.
type ownershipPrincipalRoleClient struct {
	rolev1.RoleServiceClient
	apply      func(context.Context, *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error)
	compensate func(context.Context, *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error)
}

func (f *ownershipPrincipalRoleClient) ApplyOwnershipTransfer(ctx context.Context, req *rolev1.ApplyOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.ApplyOwnershipTransferResponse, error) {
	return f.apply(ctx, req)
}
func (f *ownershipPrincipalRoleClient) CompensateOwnershipTransfer(ctx context.Context, req *rolev1.CompensateOwnershipTransferRequest, _ ...grpc.CallOption) (*rolev1.CompensateOwnershipTransferResponse, error) {
	return f.compensate(ctx, req)
}

func newOwnershipTestIssuer(t *testing.T) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}

func verifyOwnershipCredential(t *testing.T, ctx context.Context, req proto.Message, rpc string, key *rsa.PublicKey) principal.Principal {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Len(t, md.Get("authorization"), 1)
	require.Len(t, md.Get("x-request-id"), 1)
	require.NotEmpty(t, strings.TrimSpace(md.Get("x-request-id")[0]))
	for name := range md {
		require.False(t, strings.HasPrefix(name, "x-voice-"), "raw identity key %s", name)
	}
	for _, name := range []string{"x-profile-id", "x-account-id", "x-user-id", "x-actor-id", "x-internal-caller"} {
		require.Empty(t, md.Get(name), "raw identity key %s", name)
	}
	auth := md.Get("authorization")[0]
	require.True(t, strings.HasPrefix(auth, "Bearer "))
	// Compute independently from the implementation's principal.RequestHash helper.
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	require.NoError(t, err)
	digest := sha256.Sum256(payload)
	claims, err := principal.VerifyService(ctx, strings.TrimPrefix(auth, "Bearer "), principal.VerifyConfig{
		ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rpc,
		ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: "sha256:" + hex.EncodeToString(digest[:]),
		KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			require.Equal(t, "space", issuer)
			require.Equal(t, "current", kid)
			return key, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "service:space", claims.Subject)
	require.Empty(t, claims.AccountID)
	require.Empty(t, claims.ProfileID)
	require.NotEmpty(t, claims.JWTID)
	require.LessOrEqual(t, claims.ExpiresAt.Sub(claims.IssuedAt), 30*time.Second)
	return claims
}

func TestOwnershipPrincipal_ApplyAndCompensateBindSameOperationWithFreshCredentials(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	spaceID, oldOwner, newOwner, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	var credentials []principal.Principal
	applyCalls, compensateCalls := 0, 0
	client := &ownershipPrincipalRoleClient{}
	client.apply = func(ctx context.Context, req *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error) {
		applyCalls++
		require.Equal(t, spaceID.String(), req.GetSpaceId())
		require.Equal(t, oldOwner.String(), req.GetOldOwnerProfileId())
		require.Equal(t, newOwner.String(), req.GetNewOwnerProfileId())
		require.Equal(t, operationID.String(), req.GetOperationId())
		credentials = append(credentials, verifyOwnershipCredential(t, ctx, req, rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName, key))
		return &rolev1.ApplyOwnershipTransferResponse{CurrentOwnerProfileId: newOwner.String()}, nil
	}
	client.compensate = func(ctx context.Context, req *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error) {
		compensateCalls++
		require.NoError(t, ctx.Err(), "cleanup must detach original cancellation")
		deadline, ok := ctx.Deadline()
		require.True(t, ok, "cleanup must have a bounded deadline")
		require.Positive(t, time.Until(deadline))
		require.LessOrEqual(t, time.Until(deadline), 30*time.Second)
		require.Equal(t, spaceID.String(), req.GetSpaceId())
		require.Equal(t, oldOwner.String(), req.GetOldOwnerProfileId())
		require.Equal(t, newOwner.String(), req.GetNewOwnerProfileId())
		require.Equal(t, operationID.String(), req.GetOperationId())
		credentials = append(credentials, verifyOwnershipCredential(t, ctx, req, rolev1.RoleService_CompensateOwnershipTransfer_FullMethodName, key))
		return &rolev1.CompensateOwnershipTransferResponse{CurrentOwnerProfileId: oldOwner.String()}, nil
	}
	// Poison both incoming and outgoing metadata to catch either forwarding path.
	poisoned := metadata.Pairs("authorization", "Bearer client-token", "authorization", "Bearer duplicate", "x-request-id", "old-1", "x-request-id", "old-2",
		"x-profile-id", oldOwner.String(), "x-account-id", uuid.NewString(), "x-user-id", "user", "x-actor-id", "actor", "x-internal-caller", "gateway", "x-voice-profile-id", "forged", "x-voice-custom", "forged")
	ctx := metadata.NewIncomingContext(context.Background(), poisoned)
	ctx = metadata.NewOutgoingContext(ctx, poisoned.Copy())
	// Legacy Roles is intentionally a separate unusable client; only OwnershipRoles may be called.
	s := &SpaceGRPC{Roles: &ownershipPrincipalRoleClient{}, OwnershipRoles: client, PrincipalIssuer: issuer}
	require.NoError(t, s.applyOwnerRole(ctx, spaceID, oldOwner, newOwner, operationID))
	require.NoError(t, s.applyOwnerRole(ctx, spaceID, oldOwner, newOwner, operationID))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.NoError(t, s.compensateOwnerRole(canceled, spaceID, oldOwner, newOwner, operationID))
	require.Equal(t, 2, applyCalls)
	require.Equal(t, 1, compensateCalls)
	require.Len(t, credentials, 3)
	require.NotEqual(t, credentials[0].JWTID, credentials[1].JWTID, "idempotent retry needs a fresh replay credential")
	require.NotEqual(t, credentials[0].JWTID, credentials[2].JWTID)
	require.NotEqual(t, credentials[1].JWTID, credentials[2].JWTID)
}

func TestOwnershipPrincipal_MissingRuntimeDependenciesFailsClosed(t *testing.T) {
	issuer, _ := newOwnershipTestIssuer(t)
	for _, tc := range []struct {
		name      string
		issuer    *principal.Issuer
		dedicated bool
	}{
		{name: "missing issuer", dedicated: true},
		{name: "missing dedicated client", issuer: issuer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &ownershipPrincipalRoleClient{
				apply: func(context.Context, *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error) {
					calls++
					return &rolev1.ApplyOwnershipTransferResponse{}, nil
				},
				compensate: func(context.Context, *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error) {
					calls++
					return &rolev1.CompensateOwnershipTransferResponse{}, nil
				},
			}
			s := &SpaceGRPC{Roles: client, PrincipalIssuer: tc.issuer}
			if tc.dedicated {
				s.OwnershipRoles = client
			}
			a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			require.Error(t, s.applyOwnerRole(context.Background(), a, b, c, d))
			require.Error(t, s.compensateOwnerRole(context.Background(), a, b, c, d))
			require.Zero(t, calls)
		})
	}
}

func TestOwnershipPrincipal_PropagatesLifecycleFailure(t *testing.T) {
	issuer, _ := newOwnershipTestIssuer(t)
	client := &ownershipPrincipalRoleClient{
		apply: func(context.Context, *rolev1.ApplyOwnershipTransferRequest) (*rolev1.ApplyOwnershipTransferResponse, error) {
			return nil, status.Error(codes.Unavailable, "ambiguous apply")
		},
		compensate: func(context.Context, *rolev1.CompensateOwnershipTransferRequest) (*rolev1.CompensateOwnershipTransferResponse, error) {
			return nil, status.Error(codes.Unavailable, "receipt unavailable")
		},
	}
	s := &SpaceGRPC{Roles: client, OwnershipRoles: client, PrincipalIssuer: issuer}
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.Error(t, s.applyOwnerRole(context.Background(), a, b, c, d))
	require.Error(t, s.compensateOwnerRole(context.Background(), a, b, c, d))
}
