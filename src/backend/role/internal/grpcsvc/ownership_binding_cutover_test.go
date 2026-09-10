package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"testing"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestOwnershipTransferInput_AcceptsCanonicalExactProtobufBinding(t *testing.T) {
	for _, rpc := range []string{applyOwnershipTransferRPC, compensateOwnershipTransferRPC} {
		t.Run(rpc, func(t *testing.T) {
			var req ownershipTransferRequest = &rolev1.ApplyOwnershipTransferRequest{SpaceId: uuid.NewString(), OldOwnerProfileId: uuid.NewString(), NewOwnerProfileId: uuid.NewString(), OperationId: uuid.NewString()}
			if rpc == compensateOwnershipTransferRPC {
				req = &rolev1.CompensateOwnershipTransferRequest{SpaceId: req.GetSpaceId(), OldOwnerProfileId: req.GetOldOwnerProfileId(), NewOwnerProfileId: req.GetNewOwnerProfileId(), OperationId: req.GetOperationId()}
			}
			// Field 100, varint 1: a valid unknown field must remain part of the signed request.
			req.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: rpc, RequestID: "canonical-request", RequestHash: hash})
			in, err := ownershipTransferInput(ctx, rpc, req)
			require.NoError(t, err)
			require.Equal(t, hash, in.RequestHash)
			for _, field := range []protoreflect.Name{"space_id", "old_owner_profile_id", "new_owner_profile_id", "operation_id"} {
				t.Run(string(field), func(t *testing.T) {
					changed := proto.Clone(req).(ownershipTransferRequest)
					changed.ProtoReflect().Set(changed.ProtoReflect().Descriptor().Fields().ByName(field), protoreflect.ValueOfString(uuid.NewString()))
					_, err := ownershipTransferInput(ctx, rpc, changed)
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				})
			}
			changed := proto.Clone(req).(ownershipTransferRequest)
			changed.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x02})
			_, err = ownershipTransferInput(ctx, rpc, changed)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			changed.ProtoReflect().SetUnknown(nil)
			_, err = ownershipTransferInput(ctx, rpc, changed)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
}
