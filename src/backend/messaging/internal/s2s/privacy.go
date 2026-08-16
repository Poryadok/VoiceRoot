package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/privacy"

	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
}

func privacyS2SContext(ctx context.Context) context.Context {
	// Internal caller only — forwarding end-user account MD would hit ownership denial
	// when reading another profile's audience for enforcement.
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "messaging"))
}

func (u *GRPCUserPrivacy) AllowDMAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	resp, err := u.Client.GetPrivacySettings(privacyS2SContext(ctx), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowDm()), nil
}

func (u *GRPCUserPrivacy) AllowGuestDM(ctx context.Context, profileID uuid.UUID) (bool, error) {
	if u == nil || u.Client == nil {
		return true, nil
	}
	resp, err := u.Client.GetPrivacySettings(privacyS2SContext(ctx), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetPrivacySettings().GetAllowGuestDm(), nil
}

func (u *GRPCUserPrivacy) AllowFilesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.EveryoneWithGuests(), nil
	}
	resp, err := u.Client.GetPrivacySettings(privacyS2SContext(ctx), &userv1.GetPrivacySettingsRequest{
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
	resp, err := u.Client.GetPrivacySettings(privacyS2SContext(ctx), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowVoiceMessages()), nil
}

// AllowForward returns whether profileID allows others to forward their messages (privacy.md).
// Missing / unset field defaults to true. Nil client fails open (same as other gates).
func (u *GRPCUserPrivacy) AllowForward(ctx context.Context, profileID uuid.UUID) (bool, error) {
	if u == nil || u.Client == nil {
		return true, nil
	}
	resp, err := u.Client.GetPrivacySettings(privacyS2SContext(ctx), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	if err != nil {
		return false, err
	}
	ps := resp.GetPrivacySettings()
	if ps == nil || ps.AllowForward == nil {
		return true, nil
	}
	return *ps.AllowForward, nil
}
