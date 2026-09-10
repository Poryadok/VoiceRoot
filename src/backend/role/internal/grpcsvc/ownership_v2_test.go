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
