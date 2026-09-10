package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

type v2HandlerCase struct {
	rpc    string
	wrap   func(*rolev1.OwnershipTransferIntent) proto.Message
	invoke func(context.Context, proto.Message) error
}

func v2HandlerCases(svc *RoleGRPC) []v2HandlerCase {
	return []v2HandlerCase{
		{"/voice.role.v1.RoleService/PrepareOwnershipTransfer", func(i *rolev1.OwnershipTransferIntent) proto.Message {
			return &rolev1.PrepareOwnershipTransferRequest{Intent: i}
		}, func(c context.Context, m proto.Message) error {
			_, e := svc.PrepareOwnershipTransfer(c, m.(*rolev1.PrepareOwnershipTransferRequest))
			return e
		}},
		{"/voice.role.v1.RoleService/FinalizeOwnershipTransfer", func(i *rolev1.OwnershipTransferIntent) proto.Message {
			return &rolev1.FinalizeOwnershipTransferRequest{Intent: i}
		}, func(c context.Context, m proto.Message) error {
			_, e := svc.FinalizeOwnershipTransfer(c, m.(*rolev1.FinalizeOwnershipTransferRequest))
			return e
		}},
		{"/voice.role.v1.RoleService/AbortOwnershipTransfer", func(i *rolev1.OwnershipTransferIntent) proto.Message {
			return &rolev1.AbortOwnershipTransferRequest{Intent: i}
		}, func(c context.Context, m proto.Message) error {
			_, e := svc.AbortOwnershipTransfer(c, m.(*rolev1.AbortOwnershipTransferRequest))
			return e
		}},
	}
}

func v2HandlerIntent() *rolev1.OwnershipTransferIntent {
	return &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: uuid.NewString(), OldOwnerProfileId: uuid.NewString(), NewOwnerProfileId: uuid.NewString(), OperationId: uuid.NewString()}
}

func TestOwnershipTransferV2_HandlersRequireExactVerifiedSpaceBinding(t *testing.T) {
	for _, action := range v2HandlerCases(&RoleGRPC{}) {
		t.Run(action.rpc, func(t *testing.T) {
			intent := v2HandlerIntent()
			intent.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			req := action.wrap(intent)
			req.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x01})
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			valid := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: action.rpc, RequestID: "v2-request", RequestHash: hash}
			require.Equal(t, codes.PermissionDenied, status.Code(action.invoke(context.Background(), req)))
			for _, name := range []string{"kind", "issuer", "subject", "audience", "rpc", "hash"} {
				t.Run(name, func(t *testing.T) {
					p := valid
					switch name {
					case "kind":
						p.Kind = "delegated_user"
					case "issuer":
						p.Issuer = "gateway"
					case "subject":
						p.Subject = "service:gateway"
					case "audience":
						p.Audience = "auth"
					case "rpc":
						p.RPC = "/voice.role.v1.RoleService/Other"
					case "hash":
						p.RequestHash = "sha256:invalid"
					}
					require.Equal(t, codes.PermissionDenied, status.Code(action.invoke(principal.WithVerified(context.Background(), p), req)))
				})
			}
			ctx := principal.WithVerified(context.Background(), valid)
			changed := proto.Clone(req)
			changed.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x02})
			require.Equal(t, codes.PermissionDenied, status.Code(action.invoke(ctx, changed)), "unknown wrapper fields are signed")
			changedIntent := proto.Clone(intent).(*rolev1.OwnershipTransferIntent)
			changedIntent.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x02})
			changed = action.wrap(changedIntent)
			changed.ProtoReflect().SetUnknown(req.ProtoReflect().GetUnknown())
			require.Equal(t, codes.PermissionDenied, status.Code(action.invoke(ctx, changed)), "unknown shared intent fields are signed")
			// Nil storage distinguishes successful validation from binding rejection.
			require.Equal(t, codes.Unavailable, status.Code(action.invoke(ctx, req)), "valid unknown fields reach the fail-closed dependency boundary")
		})
	}
}

func TestOwnershipTransferV2_HandlersRejectMalformedIntentBeforeDependency(t *testing.T) {
	for _, action := range v2HandlerCases(&RoleGRPC{}) {
		for _, name := range []string{"missing", "space uuid", "old uuid", "new uuid", "operation uuid"} {
			t.Run(action.rpc+"/"+name, func(t *testing.T) {
				intent := v2HandlerIntent()
				switch name {
				case "missing":
					intent = nil
				case "space uuid":
					intent.SpaceId = "bad"
				case "old uuid":
					intent.OldOwnerProfileId = "bad"
				case "new uuid":
					intent.NewOwnerProfileId = "bad"
				case "operation uuid":
					intent.OperationId = "bad"
				}
				req := action.wrap(intent)
				ctx := terminalOwnershipContext(t, action.rpc, req)
				require.Equal(t, codes.InvalidArgument, status.Code(action.invoke(ctx, req)))
			})
		}
	}
}

func TestOwnershipTransferV2_CapabilityAdvertisesOnlyCompleteActiveProtocol(t *testing.T) {
	req := &rolev1.GetOwnershipTransferCapabilitiesRequest{}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	rpc := rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName
	verified := principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role",
		RPC: rpc, RequestID: "capability-request", RequestHash: hash,
	}
	svc := &RoleGRPC{}

	_, err = svc.GetOwnershipTransferCapabilities(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = svc.GetOwnershipTransferCapabilities(principal.WithVerified(context.Background(), verified), req)
	require.Equal(t, codes.Unavailable, status.Code(err), "verified transport alone must not bypass the activation hold")

	active := principalgrpc.WithOwnershipV2CapabilitiesActive(context.Background())
	for _, name := range []string{"kind", "issuer", "subject", "audience", "rpc", "request_id", "hash"} {
		t.Run(name, func(t *testing.T) {
			p := verified
			switch name {
			case "kind":
				p.Kind = "delegated_user"
			case "issuer":
				p.Issuer = "gateway"
			case "subject":
				p.Subject = "service:gateway"
			case "audience":
				p.Audience = "space"
			case "rpc":
				p.RPC = rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
			case "request_id":
				p.RequestID = ""
			case "hash":
				p.RequestHash = runtimeHashForCapabilityTest
			}
			_, err := svc.GetOwnershipTransferCapabilities(principal.WithVerified(active, p), req)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
	changed := proto.Clone(req).(*rolev1.GetOwnershipTransferCapabilitiesRequest)
	changed.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	_, err = svc.GetOwnershipTransferCapabilities(principal.WithVerified(active, verified), changed)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "capability request must retain exact unknown-field binding")

	response, err := svc.GetOwnershipTransferCapabilities(principal.WithVerified(active, verified), req)
	require.NoError(t, err)
	require.Equal(t, uint32(2), response.ProtocolVersion)
	require.Equal(t, []string{
		rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName,
		rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName,
		rolev1.RoleService_AbortOwnershipTransfer_FullMethodName,
	}, response.SupportedMethods)
	require.NotContains(t, response.SupportedMethods, rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName)
	require.NotContains(t, response.SupportedMethods, rolev1.RoleService_CompensateOwnershipTransfer_FullMethodName)
}

const runtimeHashForCapabilityTest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
