package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/grpcsvc"

	rolev1 "voice.app/voice/role/v1"
)

const (
	permVoiceJoin        = "VOICE_JOIN"
	permVoiceScreenShare = "VOICE_SCREEN_SHARE"
	permVoiceMuteOthers  = "VOICE_MUTE_OTHERS"
)

// GRPCRolePermissions validates voice-room permissions via RoleService.
type GRPCRolePermissions struct {
	Client rolev1.RoleServiceClient
}

func NewGRPCRolePermissions(c rolev1.RoleServiceClient) *GRPCRolePermissions {
	return &GRPCRolePermissions{Client: c}
}

func (g *GRPCRolePermissions) EnsureScreenShare(ctx context.Context, spaceID, profileID, voiceRoomID string) error {
	return g.ensureVoicePermission(ctx, spaceID, profileID, voiceRoomID, permVoiceScreenShare, grpcsvc.ErrScreenShareDenied)
}

func (g *GRPCRolePermissions) EnsureVoiceJoin(ctx context.Context, spaceID, profileID, voiceRoomID string) error {
	return g.ensureVoicePermission(ctx, spaceID, profileID, voiceRoomID, permVoiceJoin, grpcsvc.ErrVoiceJoinDenied)
}

func (g *GRPCRolePermissions) EnsureMuteOthers(ctx context.Context, spaceID, profileID, voiceRoomID string) error {
	return g.ensureVoicePermission(ctx, spaceID, profileID, voiceRoomID, permVoiceMuteOthers, grpcsvc.ErrMuteOthersDenied)
}

func (g *GRPCRolePermissions) ensureVoicePermission(ctx context.Context, spaceID, profileID, voiceRoomID, permission string, denied error) error {
	if g == nil || g.Client == nil {
		return status.Error(codes.FailedPrecondition, "role service not configured")
	}
	sid, err := uuid.Parse(strings.TrimSpace(spaceID))
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid space id")
	}
	pid, err := uuid.Parse(strings.TrimSpace(profileID))
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid profile id")
	}
	ctx = ForwardIncomingMetadata(ctx)
	req := &rolev1.CheckPermissionRequest{
		SpaceId:        sid.String(),
		ProfileId:      pid.String(),
		PermissionName: permission,
	}
	if vr := strings.TrimSpace(voiceRoomID); vr != "" {
		req.VoiceRoomId = &vr
	}
	resp, err := g.Client.CheckPermission(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return status.Error(codes.Unavailable, "role service unavailable")
		}
		return err
	}
	if !resp.GetAllowed() {
		return denied
	}
	return nil
}
