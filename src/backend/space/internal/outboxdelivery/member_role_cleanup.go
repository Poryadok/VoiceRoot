package outboxdelivery

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
)

type MemberRoleRemovalStore interface {
	ListPendingMemberRoleRemovals(context.Context, int) ([]memberadmission.RemoveBinding, error)
	MarkMemberRoleRemovalCompleted(context.Context, memberadmission.RemoveBinding, []byte) error
}

// Client is the protected Role connection. Logical operations survive retries;
// their transport request ID and JTI are fresh on each attempt.
type MemberRoleRemovalClient interface {
	RemoveMemberRoles(context.Context, *rolev1.RemoveMemberRolesRequest, ...grpc.CallOption) (*rolev1.RemoveMemberRolesResponse, error)
}

type MemberRoleRemovalDispatcher struct {
	Store  MemberRoleRemovalStore
	Client MemberRoleRemovalClient
	Issuer *principal.Issuer
}

func (d *MemberRoleRemovalDispatcher) DispatchOnce(ctx context.Context) error {
	if d == nil || d.Store == nil || d.Client == nil || d.Issuer == nil {
		return errors.New("member role removal dispatcher unavailable")
	}
	rows, err := d.Store.ListPendingMemberRoleRemovals(ctx, 100)
	if err != nil {
		return err
	}
	for _, b := range rows {
		fingerprint, err := memberadmission.RemoveReceiptFingerprint(b)
		if err != nil {
			return err
		}
		request := &rolev1.RemoveMemberRolesRequest{ProtocolVersion: b.ProtocolVersion, SpaceId: b.SpaceID, ProfileId: b.ProfileID, RemovalGenerationFloor: b.RemovalGenerationFloor, RemovalOperationId: b.RemovalOperationID, RemovalBindingSha256: append([]byte(nil), b.RemovalBindingSHA256...)}
		hash, err := principal.RequestHash(request)
		if err != nil {
			return err
		}
		requestID := uuid.NewString()
		token, err := d.Issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_RemoveMemberRoles_FullMethodName, RequestID: requestID, RequestHash: hash})
		if err != nil {
			return err
		}
		signed := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
		callCtx, cancel := context.WithTimeout(signed, 5*time.Second)
		response, err := d.Client.RemoveMemberRoles(callCtx, request)
		cancel()
		if err != nil {
			return err
		}
		if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
			return errors.New("member role removal receipt missing")
		}
		r := response.Receipt
		if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != b.ProtocolVersion || r.SpaceId != b.SpaceID || r.ProfileId != b.ProfileID || r.RemovalOperationId != b.RemovalOperationID || r.RemovalGenerationFloor != b.RemovalGenerationFloor || !bytes.Equal(r.RemovalBindingSha256, b.RemovalBindingSHA256) || !bytes.Equal(r.ReceiptSha256, fingerprint[:]) {
			return errors.New("member role removal receipt mismatch")
		}
		if err := d.Store.MarkMemberRoleRemovalCompleted(ctx, b, r.ReceiptSha256); err != nil {
			return err
		}
	}
	return nil
}
