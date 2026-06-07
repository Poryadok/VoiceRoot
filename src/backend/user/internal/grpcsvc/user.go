package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/r2avatar"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// UserGRPC implements profile RPCs backed by user_db v1 profiles DDL.
type UserGRPC struct {
	userv1.UnimplementedUserServiceServer
	Profiles *store.ProfileStore
	Presence *store.PresenceStore
	// Blocks optional Social S2S checker; nil skips block filtering (dev / tests).
	Blocks AccountBlockChecker
	// AvatarPresigner optional; nil → CreateAvatarPresignedUpload returns FailedPrecondition.
	AvatarPresigner AvatarPresigner
	// AvatarPublicBaseURL is used to build public_url and to validate UpdateProfile.avatar_url (empty skips validation).
	AvatarPublicBaseURL string
}

func (s *UserGRPC) GetProfile(ctx context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	switch b := req.GetBy().(type) {
	case *userv1.GetProfileRequest_ProfileId:
		id, err := uuid.Parse(strings.TrimSpace(b.ProfileId))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
		}
		row, err := s.Profiles.GetByID(ctx, id)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if row == nil {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return &userv1.GetProfileResponse{Profile: rowToProto(row)}, nil

	case *userv1.GetProfileRequest_Username:
		u, d, err := parseHandle(b.Username)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		row, err := s.Profiles.GetByUsernameDiscriminator(ctx, u, d)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if row == nil {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return &userv1.GetProfileResponse{Profile: rowToProto(row)}, nil

	default:
		return nil, status.Error(codes.InvalidArgument, "profile_id or username required")
	}
}

func (s *UserGRPC) GetProfiles(ctx context.Context, req *userv1.GetProfilesRequest) (*userv1.GetProfilesResponse, error) {
	raw := req.GetProfileIds()
	if len(raw) == 0 {
		return &userv1.GetProfilesResponse{ProfileList: &userv1.ProfileList{Profiles: []*userv1.Profile{}}}, nil
	}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, sid := range raw {
		id, err := uuid.Parse(strings.TrimSpace(sid))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid profile_id in batch")
		}
		ids = append(ids, id)
	}
	rows, err := s.Profiles.GetByIDs(ctx, ids)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*userv1.Profile, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToProto(r))
	}
	return &userv1.GetProfilesResponse{ProfileList: &userv1.ProfileList{Profiles: out}}, nil
}

func (s *UserGRPC) UpdateProfile(ctx context.Context, req *userv1.UpdateProfileRequest) (*userv1.UpdateProfileResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}

	in := store.UpdateProfileInput{}
	if req.DisplayName != nil {
		dn := strings.TrimSpace(*req.DisplayName)
		if dn == "" || len(dn) > store.MaxDisplayNameRunes {
			return nil, status.Errorf(codes.InvalidArgument, "display_name must be 1..%d characters", store.MaxDisplayNameRunes)
		}
		in.DisplayName = &dn
	}
	if req.AvatarUrl != nil {
		u := strings.TrimSpace(*req.AvatarUrl)
		if u == "" {
			empty := ""
			in.AvatarURL = &empty
		} else {
			base := strings.TrimSpace(s.AvatarPublicBaseURL)
			if base != "" {
				switch {
				case r2avatar.ValidateAvatarPublicURLForProfile(base, profileID, u) == nil:
					in.AvatarURL = proto.String(u)
				case r2avatar.ValidateAvatarObjectKeyForProfile(profileID, u) == nil:
					nk := strings.TrimLeft(strings.TrimSpace(u), "/")
					in.AvatarURL = proto.String(r2avatar.JoinPublicURL(base, nk))
				default:
					return nil, status.Error(codes.InvalidArgument, "avatar_url must be the R2 public URL or object key for this profile's avatar")
				}
			} else if r2avatar.ValidateAvatarObjectKeyForProfile(profileID, u) == nil {
				nk := strings.TrimLeft(strings.TrimSpace(u), "/")
				in.AvatarURL = proto.String(nk)
			} else {
				in.AvatarURL = proto.String(u)
			}
		}
	}
	if req.Bio != nil {
		if len(*req.Bio) > 500 {
			return nil, status.Error(codes.InvalidArgument, "bio exceeds 500 characters")
		}
		in.Bio = req.Bio
	}
	if req.Locale != nil {
		l := strings.TrimSpace(*req.Locale)
		if l != "ru" && l != "en" {
			return nil, status.Error(codes.InvalidArgument, "invalid locale")
		}
		in.Locale = &l
	}
	if req.Theme != nil {
		t := strings.TrimSpace(*req.Theme)
		if t != "light" && t != "dark" && t != "high_contrast" {
			return nil, status.Error(codes.InvalidArgument, "invalid theme")
		}
		in.Theme = &t
	}
	// v1 DDL has no banner_url / custom_status columns — ignore for persistence.

	row, err := s.Profiles.UpdateOwnedProfile(ctx, accountID, profileID, in)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found or not owned")
	}
	return &userv1.UpdateProfileResponse{Profile: rowToProto(row)}, nil
}

func (s *UserGRPC) CreateProfile(ctx context.Context, req *userv1.CreateProfileRequest) (*userv1.CreateProfileResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	dn := strings.TrimSpace(req.GetDisplayName())
	if dn == "" || len(dn) > store.MaxDisplayNameRunes {
		return nil, status.Errorf(codes.InvalidArgument, "display_name must be 1..%d characters", store.MaxDisplayNameRunes)
	}
	var usernameHint *string
	if req.Username != nil {
		u := strings.TrimSpace(*req.Username)
		if u == "" {
			return nil, status.Error(codes.InvalidArgument, "username must not be empty when set")
		}
		usernameHint = &u
	}
	row, err := s.Profiles.CreateSecondaryProfile(ctx, accountID, dn, usernameHint)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.CreateProfileResponse{Profile: rowToProto(row)}, nil
}

func (s *UserGRPC) ListMyProfiles(ctx context.Context, _ *userv1.ListMyProfilesRequest) (*userv1.ListMyProfilesResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "profile store not configured")
	}
	rows, err := s.Profiles.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*userv1.Profile, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToProto(r))
	}
	return &userv1.ListMyProfilesResponse{
		ProfileList: &userv1.ProfileList{Profiles: out},
	}, nil
}

func parseHandle(s string) (username, discriminator string, err error) {
	s = strings.TrimSpace(s)
	i := strings.LastIndex(s, "#")
	if i <= 0 || i >= len(s)-1 {
		return "", "", fmt.Errorf("username lookup requires handle format username#1234")
	}
	u, d := s[:i], s[i+1:]
	if len(d) != 4 {
		return "", "", fmt.Errorf("discriminator must be exactly 4 digits")
	}
	for _, c := range d {
		if c < '0' || c > '9' {
			return "", "", fmt.Errorf("discriminator must be decimal digits")
		}
	}
	if strings.TrimSpace(u) == "" {
		return "", "", fmt.Errorf("username part must not be empty")
	}
	return u, d, nil
}

func rowToProto(p *store.ProfileRow) *userv1.Profile {
	out := &userv1.Profile{
		Id:               p.ID.String(),
		AccountId:        p.AccountID.String(),
		Username:         p.Username,
		Discriminator:    p.Discriminator,
		DisplayName:      p.DisplayName,
		Locale:           p.Locale,
		Theme:            p.Theme,
		IsPrimary:        p.IsPrimary,
		VerificationType: p.VerificationType,
		CreatedAt:        timestamppb.New(p.CreatedAt),
		UpdatedAt:        timestamppb.New(p.UpdatedAt),
	}
	if p.AvatarURL != nil {
		out.AvatarUrl = proto.String(*p.AvatarURL)
	}
	if p.Bio != nil {
		out.Bio = proto.String(*p.Bio)
	}
	if p.VerificationBadge != nil {
		out.VerificationBadge = proto.String(*p.VerificationBadge)
	}
	return out
}

func onboardingRowToProto(r *store.OnboardingStateRow) *userv1.OnboardingState {
	out := &userv1.OnboardingState{
		ProfileId:      r.ProfileID.String(),
		CompletedSteps: append([]string(nil), r.CompletedSteps...),
		Completed:      r.Completed,
	}
	if r.CompletedAt != nil {
		out.CompletedAt = timestamppb.New(*r.CompletedAt)
	}
	return out
}

func activeProfilePtr(ctx context.Context) *uuid.UUID {
	if id, ok := authctx.ProfileID(ctx); ok {
		return &id
	}
	return nil
}

func (s *UserGRPC) GetOnboardingState(ctx context.Context, _ *userv1.GetOnboardingStateRequest) (*userv1.GetOnboardingStateResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	row, err := s.Profiles.GetOrCreateOnboardingState(ctx, accountID, activeProfilePtr(ctx))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.GetOnboardingStateResponse{OnboardingState: onboardingRowToProto(row)}, nil
}

func (s *UserGRPC) CompleteOnboardingStep(ctx context.Context, req *userv1.CompleteOnboardingStepRequest) (*userv1.CompleteOnboardingStepResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	row, err := s.Profiles.CompleteOnboardingStep(ctx, accountID, activeProfilePtr(ctx), req.GetStepId())
	if err != nil {
		if errors.Is(err, store.ErrInvalidOnboardingStep) {
			return nil, status.Error(codes.InvalidArgument, "invalid step_id")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.CompleteOnboardingStepResponse{OnboardingState: onboardingRowToProto(row)}, nil
}
