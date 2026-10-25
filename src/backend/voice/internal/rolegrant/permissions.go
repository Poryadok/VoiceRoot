package rolegrant

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/grpcsvc"
)

var _ grpcsvc.RolePermissionChecker = (*Checker)(nil)

func (c *Checker) EnsureVoiceJoin(ctx context.Context, space, profile, room string) error {
	return c.ensurePermission(ctx, space, profile, room, "VOICE_JOIN", grpcsvc.ErrVoiceJoinDenied)
}

func (c *Checker) EnsureVoiceSpeak(ctx context.Context, space, profile, room string) error {
	return c.ensurePermission(ctx, space, profile, room, "VOICE_SPEAK", grpcsvc.ErrVoiceSpeakDenied)
}

func (c *Checker) EnsureScreenShare(ctx context.Context, space, profile, room string) error {
	return c.ensurePermission(ctx, space, profile, room, "VOICE_SCREEN_SHARE", grpcsvc.ErrScreenShareDenied)
}

func (c *Checker) EnsureMuteOthers(ctx context.Context, space, profile, room string) error {
	return c.ensurePermission(ctx, space, profile, room, "VOICE_MUTE_OTHERS", grpcsvc.ErrMuteOthersDenied)
}

func (c *Checker) EnsureVoiceMoveOthers(ctx context.Context, space, profile, room string) error {
	return c.ensurePermission(ctx, space, profile, room, "VOICE_MOVE_OTHERS", grpcsvc.ErrVoiceMoveOthersDenied)
}

func (c *Checker) ensurePermission(ctx context.Context, space, profile, room, permission string, denied error) error {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}
	if c == nil || c.client == nil || c.issuer == nil {
		return status.Error(codes.Unavailable, "Role permission decision unavailable")
	}
	for _, value := range []string{space, profile, room} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return status.Error(codes.InvalidArgument, "canonical Voice permission identity required")
		}
	}
	request := &rolev1.CheckPermissionRequest{SpaceId: space, ProfileId: profile, VoiceRoomId: &room, PermissionName: permission}
	requestID := uuid.NewString()
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.Internal, "Role permission request could not be bound")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	token, err := c.issuer.IssueService(principal.ServiceInput{
		Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName,
		RequestID: requestID, RequestHash: hash,
	})
	if err != nil {
		return status.Error(codes.Unavailable, "Voice permission principal unavailable")
	}
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := c.client.CheckPermission(callCtx, request)
	if err != nil {
		return err
	}
	if err := callCtx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}
	if response == nil {
		return status.Error(codes.Unavailable, "Role permission decision missing")
	}
	if !response.GetAllowed() {
		return denied
	}
	return nil
}
