package s2s

import (
	"context"
	"strings"

	"github.com/google/uuid"

	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
}

func (u *GRPCUserPrivacy) AllowDM(ctx context.Context, profileID uuid.UUID) (string, error) {
	if u == nil || u.Client == nil {
		return "everyone", nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(resp.GetPrivacySettings().GetAllowDm())), nil
}
