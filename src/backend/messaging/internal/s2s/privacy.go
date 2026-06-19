package s2s

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/pkg/privacy"

	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
}

func (u *GRPCUserPrivacy) AllowDMAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowDm()), nil
}

func (u *GRPCUserPrivacy) AllowFilesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowFiles()), nil
}

func (u *GRPCUserPrivacy) AllowVoiceMessagesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := u.Client.GetPrivacySettings(ctx, &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowVoiceMessages()), nil
}
