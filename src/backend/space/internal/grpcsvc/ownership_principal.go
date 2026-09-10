package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func (s *SpaceGRPC) ownershipRoleRuntimeReady() error {
	if s == nil || s.Roles == nil && s.OwnershipRoles == nil {
		return nil
	}
	if s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "ownership role principal runtime unavailable")
	}
	return nil
}

func (s *SpaceGRPC) ownershipRoleContext(ctx context.Context, request proto.Message, method string, operationID uuid.UUID) (context.Context, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid ownership request")
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "role", RPC: method, RequestID: operationID.String(), RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "ownership principal signing unavailable")
	}
	// Do not inherit client credentials, duplicated request IDs, or raw actor authority.
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operationID.String())), nil
}

func (s *SpaceGRPC) applyOwnerRole(ctx context.Context, spaceID, previousOwner, newOwner, operationID uuid.UUID) error {
	if err := s.ownershipRoleRuntimeReady(); err != nil {
		return err
	}
	if s == nil || s.OwnershipRoles == nil {
		return nil
	}
	req := &rolev1.ApplyOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: previousOwner.String(), NewOwnerProfileId: newOwner.String(), OperationId: operationID.String()}
	signed, err := s.ownershipRoleContext(ctx, req, rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName, operationID)
	if err != nil {
		return err
	}
	resp, err := s.OwnershipRoles.ApplyOwnershipTransfer(signed, req)
	if err != nil {
		return err
	}
	if resp.GetCurrentOwnerProfileId() != newOwner.String() {
		return status.Error(codes.Internal, "ownership apply receipt mismatch")
	}
	return nil
}

// Compensation retains the original tuple and operation, and gets a fresh JWT
// and bounded detached context even when the original RPC was canceled.
func (s *SpaceGRPC) compensateOwnerRole(ctx context.Context, spaceID, previousOwner, newOwner, operationID uuid.UUID) error {
	if err := s.ownershipRoleRuntimeReady(); err != nil {
		return err
	}
	if s == nil || s.OwnershipRoles == nil {
		return nil
	}
	cleanup, cancel := ownershipTransferCleanupContext(ctx)
	defer cancel()
	req := &rolev1.CompensateOwnershipTransferRequest{SpaceId: spaceID.String(), OldOwnerProfileId: previousOwner.String(), NewOwnerProfileId: newOwner.String(), OperationId: operationID.String()}
	signed, err := s.ownershipRoleContext(cleanup, req, rolev1.RoleService_CompensateOwnershipTransfer_FullMethodName, operationID)
	if err != nil {
		return err
	}
	resp, err := s.OwnershipRoles.CompensateOwnershipTransfer(signed, req)
	if err != nil {
		return err
	}
	if resp.GetCurrentOwnerProfileId() != previousOwner.String() {
		return status.Error(codes.Internal, "ownership compensation receipt mismatch")
	}
	return nil
}
