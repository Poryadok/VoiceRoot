package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/user/internal/authctx"
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
		ProfileID:           profileID,
		Preset:              strings.TrimSpace(in.GetPreset()),
		ShowOnline:          strings.TrimSpace(in.GetShowOnline()),
		ShowGameStatus:      strings.TrimSpace(in.GetShowGameStatus()),
		ShowMmRating:        strings.TrimSpace(in.GetShowMmRating()),
		ShowPhone:           strings.TrimSpace(in.GetShowPhone()),
		ShowStories:         strings.TrimSpace(in.GetShowStories()),
		AllowDM:             strings.TrimSpace(in.GetAllowDm()),
		AllowFriendRequests: strings.TrimSpace(in.GetAllowFriendRequests()),
		AllowGuestDM:        in.GetAllowGuestDm(),
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
	oneOf := func(name, v string, allowed ...string) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return status.Errorf(codes.InvalidArgument, "%s is invalid", name)
	}
	if err := oneOf("preset", strings.TrimSpace(ps.GetPreset()), "personal", "gaming", "work"); err != nil {
		return err
	}
	if err := oneOf("show_online", strings.TrimSpace(ps.GetShowOnline()), "everyone", "friends", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	if err := oneOf("show_game_status", strings.TrimSpace(ps.GetShowGameStatus()), "everyone", "friends", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	if err := oneOf("show_mm_rating", strings.TrimSpace(ps.GetShowMmRating()), "everyone", "friends", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	if err := oneOf("show_phone", strings.TrimSpace(ps.GetShowPhone()), "friends", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	if err := oneOf("show_stories", strings.TrimSpace(ps.GetShowStories()), "everyone", "friends", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	if err := oneOf("allow_dm", strings.TrimSpace(ps.GetAllowDm()), "everyone", "friends", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	if err := oneOf("allow_friend_requests", strings.TrimSpace(ps.GetAllowFriendRequests()), "everyone", "friends_of_friends", "nobody"); err != nil {
		return err
	}
	return nil
}

func privacyRowToProto(row *store.PrivacyRow) *userv1.PrivacySettings {
	if row == nil {
		return nil
	}
	return &userv1.PrivacySettings{
		ProfileId:           row.ProfileID.String(),
		Preset:              row.Preset,
		ShowOnline:          row.ShowOnline,
		ShowGameStatus:      row.ShowGameStatus,
		ShowMmRating:        row.ShowMmRating,
		ShowPhone:           row.ShowPhone,
		ShowStories:         row.ShowStories,
		AllowDm:             row.AllowDM,
		AllowFriendRequests: row.AllowFriendRequests,
		AllowGuestDm:        row.AllowGuestDM,
		UpdatedAt:           timestamppb.New(row.UpdatedAt.UTC()),
	}
}

func parseUUIDField(name, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is invalid", name)
	}
	return id, nil
}
