package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// EnsurePrimaryProfile bootstraps the primary profile for a new account (Auth S2S).
func (s *UserGRPC) EnsurePrimaryProfile(ctx context.Context, req *userv1.EnsurePrimaryProfileRequest) (*userv1.EnsurePrimaryProfileResponse, error) {
	if !authctx.IsInternalService(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal only")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	var profileID *uuid.UUID
	if req.ProfileId != nil && strings.TrimSpace(*req.ProfileId) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.ProfileId))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
		}
		profileID = &id
	}
	row, err := s.Profiles.EnsurePrimaryProfile(
		ctx, accountID, profileID, strings.TrimSpace(req.GetDisplayHint()), req.GetIsGuestAccount())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row.DeletedAt != nil {
		return nil, status.Error(codes.FailedPrecondition, "primary profile is deleted")
	}
	if privacyStore := s.privacyStore(); privacyStore != nil {
		priv, err := privacyStore.GetByProfileID(ctx, row.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if priv == nil {
			if req.GetIsGuestAccount() {
				if _, err := privacyStore.CreateDefaultGamingForGuest(ctx, row.ID); err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}
			} else if _, err := privacyStore.CreateDefaultGaming(ctx, row.ID); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	if s.Events != nil {
		_ = s.Events.PublishProfileCreated(ctx, row.ID.String(), row.AccountID.String())
	}
	return &userv1.EnsurePrimaryProfileResponse{Profile: rowToProto(row)}, nil
}

// ResolveAccountIDForProfile resolves profile ownership for Messaging and Chat DM lifecycle checks.
// It intentionally bypasses public profile visibility because it returns no Profile data.
func (s *UserGRPC) ResolveAccountIDForProfile(ctx context.Context, req *userv1.ResolveAccountIDForProfileRequest) (*userv1.ResolveAccountIDForProfileResponse, error) {
	if !isLifecycleOwnerInternalCaller(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal lifecycle caller only")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	row, err := s.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	return &userv1.ResolveAccountIDForProfileResponse{AccountId: row.AccountID.String()}, nil
}

func isLifecycleOwnerInternalCaller(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	callers := md.Get(authctx.HeaderInternalCaller)
	return len(callers) == 1 && (callers[0] == "messaging" || callers[0] == "chat")
}

// ListProfileIDsForAccount returns profile ids for an account (Social block cascade S2S).
func (s *UserGRPC) ListProfileIDsForAccount(ctx context.Context, req *userv1.ListProfileIDsForAccountRequest) (*userv1.ListProfileIDsForAccountResponse, error) {
	if !authctx.IsInternalService(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal only")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	rows, err := s.Profiles.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID.String())
	}
	return &userv1.ListProfileIDsForAccountResponse{ProfileIds: out}, nil
}

// ResolvePrimaryProfileIDs is an internal read seam for Auth-owned account identifiers.
// It never provisions profiles and omits accounts without an existing, non-deleted primary profile.
func (s *UserGRPC) ResolvePrimaryProfileIDs(ctx context.Context, req *userv1.ResolvePrimaryProfileIDsRequest) (*userv1.ResolvePrimaryProfileIDsResponse, error) {
	if !authctx.IsInternalService(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal only")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	accountIDs := make([]uuid.UUID, 0, len(req.GetAccountIds()))
	for _, raw := range req.GetAccountIds() {
		accountID, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid account_id")
		}
		accountIDs = append(accountIDs, accountID)
	}
	resolved, err := s.Profiles.ResolvePrimaryProfileIDs(ctx, accountIDs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make(map[string]string, len(resolved))
	for accountID, profileID := range resolved {
		out[accountID.String()] = profileID.String()
	}
	return &userv1.ResolvePrimaryProfileIDsResponse{PrimaryProfileIds: out}, nil
}

// MarkAccountRegular clears the guest marker after Auth has converted the account.
// The operation is internal-only and idempotent, including for accounts without profiles.
func (s *UserGRPC) MarkAccountRegular(ctx context.Context, req *userv1.MarkAccountRegularRequest) (*userv1.MarkAccountRegularResponse, error) {
	if !authctx.IsInternalService(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal only")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	if err := s.Profiles.MarkAccountRegular(ctx, accountID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.MarkAccountRegularResponse{}, nil
}

// GetSettings returns per-profile UI settings stored on profiles row.
func (s *UserGRPC) GetSettings(ctx context.Context, req *userv1.GetSettingsRequest) (*userv1.GetSettingsResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	row, err := s.Profiles.GetOwnedProfile(ctx, accountID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	prefs, err := s.Profiles.GetNotificationPrefsJSON(ctx, profileID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.GetSettingsResponse{
		UserSettings: settingsRowToProto(row, prefs),
	}, nil
}

// UpdateSettings updates locale/theme/notification prefs for an owned profile.
func (s *UserGRPC) UpdateSettings(ctx context.Context, req *userv1.UpdateSettingsRequest) (*userv1.UpdateSettingsResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	in := req.GetSettings()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}
	if pid := strings.TrimSpace(in.GetProfileId()); pid != "" && pid != profileID.String() {
		return nil, status.Error(codes.InvalidArgument, "settings.profile_id mismatch")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	update := store.UpdateProfileInput{}
	if lang := strings.TrimSpace(in.GetLanguage()); lang != "" {
		if lang != "ru" && lang != "en" {
			return nil, status.Error(codes.InvalidArgument, "invalid language")
		}
		update.Locale = &lang
	}
	if theme := strings.TrimSpace(in.GetTheme()); theme != "" {
		if theme != "light" && theme != "dark" && theme != "high_contrast" {
			return nil, status.Error(codes.InvalidArgument, "invalid theme")
		}
		update.Theme = &theme
	}
	row, err := s.Profiles.UpdateOwnedProfile(ctx, accountID, profileID, update)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	prefs := strings.TrimSpace(in.GetNotificationPrefsJson())
	if prefs != "" {
		if err := s.Profiles.SetNotificationPrefsJSON(ctx, profileID, prefs); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		prefs, _ = s.Profiles.GetNotificationPrefsJSON(ctx, profileID)
	}
	if s.Events != nil {
		_ = s.Events.PublishProfileUpdated(ctx, profileID.String(), accountID.String(), `["settings"]`)
	}
	return &userv1.UpdateSettingsResponse{UserSettings: settingsRowToProto(row, prefs)}, nil
}

func settingsRowToProto(row *store.ProfileRow, notificationPrefsJSON string) *userv1.UserSettings {
	if notificationPrefsJSON == "" {
		notificationPrefsJSON = "{}"
	}
	return &userv1.UserSettings{
		ProfileId:             row.ID.String(),
		Language:              row.Locale,
		Theme:                 row.Theme,
		NotificationPrefsJson: notificationPrefsJSON,
	}
}
