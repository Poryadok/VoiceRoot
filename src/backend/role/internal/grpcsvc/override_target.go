package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/role/internal/authctx"
)

// OverrideTargetValidator performs fresh protected owner reads outside the Role
// transaction. The final fenced setter independently repeats local authority.
type OverrideTargetValidator interface {
	ValidateChat(context.Context, string, string) error
	ValidateVoice(context.Context, string, string) error
}

func (s *RoleGRPC) preflightOverrideTarget(ctx context.Context, rawSpace, rawRole, rawTarget string) (uuid.UUID, uuid.UUID, error) {
	spaceID, err := parseUUIDField("space_id", rawSpace)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	targetID, err := parseUUIDField("target_id", rawTarget)
	if err != nil || targetID == uuid.Nil {
		if err == nil {
			err = status.Error(codes.InvalidArgument, "invalid target_id")
		}
		return uuid.Nil, uuid.Nil, err
	}
	_, err = ordinarySpaceRPC(ctx, s, rawSpace, func(scoped *RoleGRPC) (struct{}, error) {
		actor, ok := authctx.ProfileID(ctx)
		if !ok {
			return struct{}{}, status.Error(codes.Unauthenticated, "missing profile")
		}
		if err := scoped.requireManageRoles(ctx, spaceID, actor); err != nil {
			return struct{}{}, err
		}
		roleID, err := parseUUIDField("role_id", rawRole)
		if err != nil {
			return struct{}{}, err
		}
		row, err := scoped.Store.GetRoleByIDInSpace(ctx, spaceID, roleID)
		if err != nil {
			return struct{}{}, ordinaryStoreError(err)
		}
		if row == nil {
			return struct{}{}, status.Error(codes.NotFound, "role not found")
		}
		return struct{}{}, nil
	})
	return spaceID, targetID, err
}

func targetValidationError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return status.FromContextError(ctx.Err()).Err()
	}
	switch status.Code(err) {
	case codes.NotFound, codes.InvalidArgument:
		return err
	}
	return status.Error(codes.Unavailable, "target validation unavailable")
}
