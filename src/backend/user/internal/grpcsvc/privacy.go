package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/user/internal/authctx"
	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func (s *UserGRPC) GetPrivacySettings(ctx context.Context, req *userv1.GetPrivacySettingsRequest) (*userv1.GetPrivacySettingsResponse, error) {
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	privacyStore := s.privacyStore()
	if privacyStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "privacy store not configured")
	}

	row, err := privacyStore.GetByProfileID(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		row, err = privacyStore.CreateDefaultGaming(ctx, profileID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &userv1.GetPrivacySettingsResponse{PrivacySettings: privacyRowToProto(row)}, nil
}

func (s *UserGRPC) UpdatePrivacySettings(ctx context.Context, req *userv1.UpdatePrivacySettingsRequest) (*userv1.UpdatePrivacySettingsResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	callerProfile, hasCallerProfile := authctx.ProfileID(ctx)
	if hasCallerProfile && callerProfile != profileID {
		return nil, status.Error(codes.PermissionDenied, "cannot update another profile")
	}
	if s.Profiles != nil {
		row, err := s.Profiles.GetByID(ctx, profileID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if row == nil {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		if row.AccountID != accountID {
			return nil, status.Error(codes.PermissionDenied, "cannot update another profile")
		}
	}
	in := req.GetSettings()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}
	if pid := strings.TrimSpace(in.GetProfileId()); pid != "" && pid != profileID.String() {
		return nil, status.Error(codes.InvalidArgument, "settings.profile_id mismatch")
	}
	if err := validatePrivacySettings(in); err != nil {
		return nil, err
	}
	privacyStore := s.privacyStore()
	if privacyStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "privacy store not configured")
	}
	saved, err := privacyStore.Upsert(ctx, store.PrivacyRow{
		ProfileID:             profileID,
		Preset:                strings.TrimSpace(in.GetPreset()),
		ShowOnline:            privacy.FromProto(in.GetShowOnline()),
		ShowGameStatus:        privacy.FromProto(in.GetShowGameStatus()),
		ShowMmRating:          privacy.FromProto(in.GetShowMmRating()),
		ShowPhone:             privacy.FromProto(in.GetShowPhone()),
		ShowStories:           privacy.FromProto(in.GetShowStories()),
		AllowPhoneSearch:      privacy.FromProto(in.GetAllowPhoneSearch()),
		AllowDM:               privacy.FromProto(in.GetAllowDm()),
		AllowCalls:            privacy.FromProto(in.GetAllowCalls()),
		AllowChatSpaceInvites: privacy.FromProto(in.GetAllowChatSpaceInvites()),
		AllowFiles:            privacy.FromProto(in.GetAllowFiles()),
		AllowVoiceMessages:    privacy.FromProto(in.GetAllowVoiceMessages()),
		AllowFriendRequests:   privacy.FromProto(in.GetAllowFriendRequests()),
		AllowGuestDM:          in.GetAllowGuestDm(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.UpdatePrivacySettingsResponse{PrivacySettings: privacyRowToProto(saved)}, nil
}

func (s *UserGRPC) privacyStore() *store.PrivacyStore {
	if s == nil {
		return nil
	}
	if s.Privacy != nil {
		return s.Privacy
	}
	if s.Profiles != nil && s.Profiles.Pool() != nil {
		s.Privacy = store.NewPrivacyStore(s.Profiles.Pool())
	}
	return s.Privacy
}

func validatePrivacySettings(ps *userv1.PrivacySettings) error {
	if err := oneOfPreset(strings.TrimSpace(ps.GetPreset())); err != nil {
		return err
	}
	fields := []struct {
		name string
		a    privacy.Audience
	}{
		{"show_online", privacy.FromProto(ps.GetShowOnline())},
		{"show_game_status", privacy.FromProto(ps.GetShowGameStatus())},
		{"show_mm_rating", privacy.FromProto(ps.GetShowMmRating())},
		{"show_phone", privacy.FromProto(ps.GetShowPhone())},
		{"show_stories", privacy.FromProto(ps.GetShowStories())},
		{"allow_phone_search", privacy.FromProto(ps.GetAllowPhoneSearch())},
		{"allow_dm", privacy.FromProto(ps.GetAllowDm())},
		{"allow_calls", privacy.FromProto(ps.GetAllowCalls())},
		{"allow_chat_space_invites", privacy.FromProto(ps.GetAllowChatSpaceInvites())},
		{"allow_files", privacy.FromProto(ps.GetAllowFiles())},
		{"allow_voice_messages", privacy.FromProto(ps.GetAllowVoiceMessages())},
		{"allow_friend_requests", privacy.FromProto(ps.GetAllowFriendRequests())},
	}
	for _, f := range fields {
		if err := privacy.ValidateAudience(f.name, f.a); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
	}
	return nil
}

func oneOfPreset(v string) error {
	for _, a := range []string{"personal", "gaming", "work"} {
		if v == a {
			return nil
		}
	}
	return status.Errorf(codes.InvalidArgument, "preset is invalid")
}

func privacyRowToProto(row *store.PrivacyRow) *userv1.PrivacySettings {
	if row == nil {
		return nil
	}
	return &userv1.PrivacySettings{
		ProfileId:              row.ProfileID.String(),
		Preset:                 row.Preset,
		ShowOnline:             privacy.ToProto(row.ShowOnline),
		ShowGameStatus:         privacy.ToProto(row.ShowGameStatus),
		ShowMmRating:           privacy.ToProto(row.ShowMmRating),
		ShowPhone:              privacy.ToProto(row.ShowPhone),
		ShowStories:            privacy.ToProto(row.ShowStories),
		AllowPhoneSearch:       privacy.ToProto(row.AllowPhoneSearch),
		AllowDm:                privacy.ToProto(row.AllowDM),
		AllowCalls:             privacy.ToProto(row.AllowCalls),
		AllowChatSpaceInvites:  privacy.ToProto(row.AllowChatSpaceInvites),
		AllowFiles:             privacy.ToProto(row.AllowFiles),
		AllowVoiceMessages:     privacy.ToProto(row.AllowVoiceMessages),
		AllowFriendRequests:    privacy.ToProto(row.AllowFriendRequests),
		AllowGuestDm:           row.AllowGuestDM,
		UpdatedAt:              timestamppb.New(row.UpdatedAt.UTC()),
	}
}

func (s *UserGRPC) audienceMatcher() privacy.Matcher {
	return privacy.Matcher{
		Social: s.SocialGraph,
		Space:  s.SpaceCoMembership,
	}
}

func parseUUIDField(name, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is invalid", name)
	}
	return id, nil
}
