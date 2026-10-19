package memberadmissionrole

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
)

type ensureRPC func(context.Context, *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error)

func (f ensureRPC) EnsureDefaultMemberRole(ctx context.Context, r *rolev1.EnsureDefaultMemberRoleRequest, _ ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	return f(ctx, r)
}

func TestEnsureExactSignedBindingReceiptAndUncertainty(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	for _, mode := range []string{"valid", "wrong space", "wrong profile", "wrong P", "wrong generation", "wrong binding", "wrong version", "wrong digest", "bad role", "unknown receipt", "unknown response", "nil", "request mutation", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			binding := memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: uuid.NewString(), ProfileID: uuid.NewString(), AdmissionOperationID: uuid.NewString(), MembershipGeneration: 4, AdmissionBindingSHA256: bytes.Repeat([]byte{7}, 32)}
			selected := uuid.New()
			var response *rolev1.EnsureDefaultMemberRoleResponse
			ids := map[string]bool{}
			client := &Client{Issuer: issuer, Clock: func() time.Time { return now }, Role: ensureRPC(func(ctx context.Context, request *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Empty(t, md.Get("x-voice-profile-id"))
				transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
				require.NoError(t, err)
				hash, err := principal.RequestHash(request)
				require.NoError(t, err)
				actor, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
				require.NoError(t, err)
				require.Equal(t, "service:space", actor.Subject)
				require.False(t, ids[transport.RequestID], "retry uses fresh transport identity, same logical P")
				ids[transport.RequestID] = true
				receipt, err := memberadmission.EnsureReceiptFingerprint(binding, selected.String())
				require.NoError(t, err)
				response = &rolev1.EnsureDefaultMemberRoleResponse{Receipt: &rolev1.DefaultMemberRoleReceipt{ProtocolVersion: 1, SpaceId: binding.SpaceID, ProfileId: binding.ProfileID, AdmissionOperationId: binding.AdmissionOperationID, MembershipGeneration: binding.MembershipGeneration, AdmissionBindingSha256: append([]byte(nil), binding.AdmissionBindingSHA256...), RoleId: selected.String(), ReceiptSha256: receipt[:]}}
				switch mode {
				case "wrong space":
					response.Receipt.SpaceId = uuid.NewString()
				case "wrong profile":
					response.Receipt.ProfileId = uuid.NewString()
				case "wrong P":
					response.Receipt.AdmissionOperationId = uuid.NewString()
				case "wrong generation":
					response.Receipt.MembershipGeneration++
				case "wrong binding":
					response.Receipt.AdmissionBindingSha256[0] ^= 1
				case "wrong version":
					response.Receipt.ProtocolVersion = 2
				case "wrong digest":
					response.Receipt.ReceiptSha256[0] ^= 1
				case "bad role":
					response.Receipt.RoleId = "guessed-default"
				case "unknown receipt":
					response.Receipt.ProtoReflect().SetUnknown([]byte{0x78, 1})
				case "unknown response":
					response.ProtoReflect().SetUnknown([]byte{0x78, 1})
				case "nil":
					return nil, nil
				case "request mutation":
					request.ProfileId = uuid.NewString()
				case "deadline":
					now = now.Add(2 * time.Second)
				}
				return response, nil
			})}
			ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", uuid.NewString()))
			verified, err := client.Ensure(ctx, binding)
			if mode != "valid" {
				require.ErrorIs(t, err, ErrUnavailable)
				return
			}
			require.NoError(t, err)
			role, digest, err := verified.For(binding)
			require.NoError(t, err)
			require.Equal(t, selected, role)
			expected := append([]byte(nil), digest...)
			response.Receipt.RoleId = uuid.NewString()
			response.Receipt.ReceiptSha256[0] ^= 1
			digest[0] ^= 1
			role, digest, err = verified.For(binding)
			require.NoError(t, err)
			require.Equal(t, selected, role)
			require.Equal(t, expected, digest)
			changed := binding
			changed.MembershipGeneration++
			_, _, err = verified.For(changed)
			require.ErrorIs(t, err, ErrUnavailable)
			_, err = client.Ensure(ctx, binding)
			require.NoError(t, err)
			require.Len(t, ids, 2)
			require.False(t, proto.Equal(response.Receipt, &rolev1.DefaultMemberRoleReceipt{}))
		})
	}
}
