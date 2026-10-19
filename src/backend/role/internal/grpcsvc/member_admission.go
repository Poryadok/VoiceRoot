package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/internal/store"
)

type MemberAuthorityObserver interface {
	ObserveMember(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (memberauthority.Observation, error)
}

// These entrypoints deliberately bypass ordinarySpaceRPC: owner RPCs must
// finish before any local operation/Space lock. The protected interceptor must
// still authenticate exact service:space before this handler boundary check.
func (s *RoleGRPC) EnsureDefaultMemberRole(ctx context.Context, req *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	if err := principalgrpc.RequireMemberAdmission(ctx, req, rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil || s.Store.Pool == nil || s.MemberAuthority == nil {
		return nil, status.Error(codes.Unavailable, "member admission dependencies unavailable")
	}
	generation, operation := req.MembershipGeneration, req.AdmissionOperationId
	observation, err := s.MemberAuthority.ObserveMember(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: req.ProfileId, ExpectedGeneration: &generation, ExpectedAdmissionOperationId: &operation})
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	receipt, err := s.Store.PrepareQualifiedDefaultMemberRole(ctx, memberadmission.EnsureBinding{ProtocolVersion: req.ProtocolVersion, SpaceID: req.SpaceId, ProfileID: req.ProfileId, MembershipGeneration: generation, AdmissionOperationID: operation, AdmissionBindingSHA256: append([]byte(nil), req.AdmissionBindingSha256...)}, observation)
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	return &rolev1.EnsureDefaultMemberRoleResponse{Receipt: &rolev1.DefaultMemberRoleReceipt{ProtocolVersion: receipt.ProtocolVersion, SpaceId: receipt.SpaceID, ProfileId: receipt.ProfileID, MembershipGeneration: receipt.Generation, AdmissionOperationId: receipt.OperationID, AdmissionBindingSha256: receipt.BindingSHA256, RoleId: receipt.RoleID, ReceiptSha256: receipt.ReceiptSHA256}}, nil
}

func (s *RoleGRPC) RemoveMemberRoles(ctx context.Context, req *rolev1.RemoveMemberRolesRequest) (*rolev1.RemoveMemberRolesResponse, error) {
	if err := principalgrpc.RequireMemberAdmission(ctx, req, rolev1.RoleService_RemoveMemberRoles_FullMethodName); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil || s.Store.Pool == nil || s.MemberAuthority == nil {
		return nil, status.Error(codes.Unavailable, "member admission dependencies unavailable")
	}
	observation, err := s.MemberAuthority.ObserveMember(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: req.ProfileId})
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	receipt, err := s.Store.RemoveQualifiedOrdinaryMemberRoles(ctx, memberadmission.RemoveBinding{ProtocolVersion: req.ProtocolVersion, SpaceID: req.SpaceId, ProfileID: req.ProfileId, RemovalGenerationFloor: req.RemovalGenerationFloor, RemovalOperationID: req.RemovalOperationId, RemovalBindingSHA256: append([]byte(nil), req.RemovalBindingSha256...)}, observation)
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	return &rolev1.RemoveMemberRolesResponse{Receipt: &rolev1.MemberRolesRemovalReceipt{ProtocolVersion: receipt.ProtocolVersion, SpaceId: receipt.SpaceID, ProfileId: receipt.ProfileID, RemovalGenerationFloor: receipt.Generation, RemovalOperationId: receipt.OperationID, RemovalBindingSha256: receipt.BindingSHA256, ReceiptSha256: receipt.ReceiptSHA256}}, nil
}

func memberAdmissionError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return status.FromContextError(ctx.Err()).Err()
	}
	if errors.Is(err, store.ErrMemberAdmissionConflict) {
		return status.Error(codes.AlreadyExists, "member_admission_conflict")
	}
	if errors.Is(err, store.ErrMemberAdmissionStale) || errors.Is(err, store.ErrMemberAdmissionDefaultRole) {
		return status.Error(codes.FailedPrecondition, "member_admission_unavailable")
	}
	if errors.Is(err, store.ErrRoleNotFound) {
		return status.Error(codes.NotFound, "member admission default role missing")
	}
	return status.Error(codes.Unavailable, "member admission authority or persistence unavailable")
}
