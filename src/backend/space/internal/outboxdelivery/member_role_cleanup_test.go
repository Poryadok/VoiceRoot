package outboxdelivery

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
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

type removalSource struct {
	binding  memberadmission.RemoveBinding
	marks    int
	failMark bool
}

func (s *removalSource) ListPendingMemberRoleRemovals(context.Context, int) ([]memberadmission.RemoveBinding, error) {
	return []memberadmission.RemoveBinding{s.binding}, nil
}
func (s *removalSource) MarkMemberRoleRemovalCompleted(_ context.Context, b memberadmission.RemoveBinding, receipt []byte) error {
	s.marks++
	if s.failMark {
		return errors.New("commit outcome unknown")
	}
	return nil
}

type removalClientFunc func(context.Context, *rolev1.RemoveMemberRolesRequest) (*rolev1.RemoveMemberRolesResponse, error)

func (f removalClientFunc) RemoveMemberRoles(ctx context.Context, r *rolev1.RemoveMemberRolesRequest, _ ...grpc.CallOption) (*rolev1.RemoveMemberRolesResponse, error) {
	return f(ctx, r)
}

func TestMemberRoleRemovalDispatcherFreshSignedTransportAndExactReceipt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	source := &removalSource{binding: memberadmission.RemoveBinding{ProtocolVersion: 1, SpaceID: uuid.NewString(), ProfileID: uuid.NewString(), RemovalOperationID: uuid.NewString(), RemovalGenerationFloor: 3, RemovalBindingSHA256: make([]byte, 32)}}
	var requests []*rolev1.RemoveMemberRolesRequest
	seenIDs, seenJWTs := map[string]bool{}, map[string]bool{}
	fault := "unknown"
	client := removalClientFunc(func(ctx context.Context, r *rolev1.RemoveMemberRolesRequest) (*rolev1.RemoveMemberRolesResponse, error) {
		requests = append(requests, proto.Clone(r).(*rolev1.RemoveMemberRolesRequest))
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2, "no inherited client or raw actor metadata")
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		id := md.Get("x-request-id")[0]
		require.NotEqual(t, r.RemovalOperationId, id)
		require.False(t, seenIDs[id])
		seenIDs[id] = true
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		verified, err := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_RemoveMemberRoles_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seenJWTs[jti] {
				return errors.New("replay")
			}
			seenJWTs[jti] = true
			return nil
		}})
		require.NoError(t, err)
		require.Equal(t, "service:space", verified.Subject)
		if fault == "unknown" {
			return nil, context.DeadlineExceeded
		}
		fingerprint, err := memberadmission.RemoveReceiptFingerprint(source.binding)
		require.NoError(t, err)
		response := &rolev1.RemoveMemberRolesResponse{Receipt: &rolev1.MemberRolesRemovalReceipt{ProtocolVersion: r.ProtocolVersion, SpaceId: r.SpaceId, ProfileId: r.ProfileId, RemovalOperationId: r.RemovalOperationId, RemovalGenerationFloor: r.RemovalGenerationFloor, RemovalBindingSha256: r.RemovalBindingSha256, ReceiptSha256: fingerprint[:]}}
		switch fault {
		case "floor":
			response.Receipt.RemovalGenerationFloor++
		case "hash":
			response.Receipt.ReceiptSha256 = make([]byte, 32)
		case "nestedUnknown":
			response.Receipt.ProtoReflect().SetUnknown([]byte{0x78, 1})
		case "outerUnknown":
			response.ProtoReflect().SetUnknown([]byte{0x78, 1})
		}
		return response, nil
	})
	d := &MemberRoleRemovalDispatcher{Store: source, Client: client, Issuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "client", "x-voice-profile-id", uuid.NewString()))
	for _, fault = range []string{"unknown", "floor", "hash", "nestedUnknown", "outerUnknown"} {
		require.Error(t, d.DispatchOnce(ctx))
		require.Zero(t, source.marks)
	}
	fault = ""
	source.failMark = true
	require.Error(t, d.DispatchOnce(ctx))
	require.Equal(t, 1, source.marks)
	source.failMark = false
	require.NoError(t, d.DispatchOnce(ctx))
	require.Equal(t, 2, source.marks)
	for _, request := range requests[1:] {
		require.True(t, proto.Equal(requests[0], request), "retry preserves exact logical removal despite fresh transport")
	}
}
