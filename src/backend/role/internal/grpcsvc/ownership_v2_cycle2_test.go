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
)

func TestOwnershipTransferV2_Cycle2HandlersPreserveReceiptIntentAndActionBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	for _, action := range []string{"finalize", "abort"} {
		t.Run(action, func(t *testing.T) {
			s, cleanup := startRoleStoreTest(t)
			defer cleanup()
			intent := v2HandlerIntent()
			intent.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			require.NoError(t, s.BootstrapSpaceRoles(context.Background(), uuid.MustParse(intent.SpaceId), uuid.MustParse(intent.OldOwnerProfileId)))
			svc := &RoleGRPC{Store: s}
			prepare := &rolev1.PrepareOwnershipTransferRequest{Intent: intent}
			prepare.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x01})
			prepared, err := svc.PrepareOwnershipTransfer(terminalOwnershipContext(t, "/voice.role.v1.RoleService/PrepareOwnershipTransfer", prepare), prepare)
			require.NoError(t, err)
			require.Equal(t, rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED, prepared.Receipt.State)
			require.Empty(t, prepared.Receipt.CurrentOwnerProfileId)
			require.True(t, proto.Equal(intent, prepared.Receipt.Intent))
			var request proto.Message
			var rpc string
			var invoke func(context.Context, proto.Message) (*rolev1.OwnershipTransferReceipt, error)
			wantState := rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED
			wantOwner := intent.NewOwnerProfileId
			if action == "finalize" {
				request = &rolev1.FinalizeOwnershipTransferRequest{Intent: intent}
				rpc = "/voice.role.v1.RoleService/FinalizeOwnershipTransfer"
				invoke = func(ctx context.Context, m proto.Message) (*rolev1.OwnershipTransferReceipt, error) {
					r, e := svc.FinalizeOwnershipTransfer(ctx, m.(*rolev1.FinalizeOwnershipTransferRequest))
					if e != nil {
						return nil, e
					}
					return r.Receipt, nil
				}
			} else {
				request = &rolev1.AbortOwnershipTransferRequest{Intent: intent}
				rpc = "/voice.role.v1.RoleService/AbortOwnershipTransfer"
				wantState = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_ABORTED
				wantOwner = intent.OldOwnerProfileId
				invoke = func(ctx context.Context, m proto.Message) (*rolev1.OwnershipTransferReceipt, error) {
					r, e := svc.AbortOwnershipTransfer(ctx, m.(*rolev1.AbortOwnershipTransferRequest))
					if e != nil {
						return nil, e
					}
					return r.Receipt, nil
				}
			}
			request.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x02})
			first, err := invoke(terminalOwnershipContext(t, rpc, request), request)
			require.NoError(t, err)
			require.Equal(t, wantState, first.State)
			require.Equal(t, wantOwner, first.CurrentOwnerProfileId)
			require.True(t, proto.Equal(intent, first.Intent))
			replay, err := invoke(terminalOwnershipContext(t, rpc, request), request)
			require.NoError(t, err)
			require.True(t, proto.Equal(first, replay))
			changed := proto.Clone(request)
			changed.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x03})
			_, err = invoke(terminalOwnershipContext(t, rpc, changed), changed)
			require.Equal(t, codes.AlreadyExists, status.Code(err), "fresh valid signature cannot change accepted action body")
			requireSoleTerminalOwner(t, s, uuid.MustParse(intent.SpaceId), uuid.MustParse(wantOwner))
		})
	}
}
