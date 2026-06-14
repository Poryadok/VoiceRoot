package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// DNSResolver looks up TXT records for org verification.
type DNSResolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

type systemDNSResolver struct{}

func (systemDNSResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	return netDefaultResolver.LookupTXT(ctx, domain)
}

// GetVerificationStatus returns badge state for a profile owned by the caller.
func (s *UserGRPC) GetVerificationStatus(ctx context.Context, req *userv1.GetVerificationStatusRequest) (*userv1.GetVerificationStatusResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	row, err := s.Profiles.GetOwnedProfile(ctx, accountID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	return &userv1.GetVerificationStatusResponse{VerificationStatus: verificationStatusFromRow(row)}, nil
}

// SetVerification is S2S from Auth after OAuth checks.
func (s *UserGRPC) SetVerification(ctx context.Context, req *userv1.SetVerificationRequest) (*userv1.SetVerificationResponse, error) {
	if !authctx.IsInternalService(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal only")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	vType := strings.TrimSpace(req.GetVerificationType())
	if vType != "personal" && vType != "organization" {
		return nil, status.Error(codes.InvalidArgument, "invalid verification_type")
	}
	badge := "verified"
	if req.Badge != nil && strings.TrimSpace(*req.Badge) != "" {
		badge = strings.TrimSpace(*req.Badge)
	}
	row, err := s.Profiles.SetProfileVerification(ctx, profileID, vType, badge)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	if s.Events != nil {
		_ = s.Events.PublishVerified(ctx, row.ID.String(), row.AccountID.String(), vType)
	}
	return &userv1.SetVerificationResponse{VerificationStatus: verificationStatusFromRow(row)}, nil
}

// ClearVerification is S2S from Auth when unlinking linked accounts.
func (s *UserGRPC) ClearVerification(ctx context.Context, req *userv1.ClearVerificationRequest) (*userv1.ClearVerificationResponse, error) {
	if !authctx.IsInternalService(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal only")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	row, err := s.Profiles.ClearProfileVerification(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	return &userv1.ClearVerificationResponse{VerificationStatus: verificationStatusFromRow(row)}, nil
}

// StartOrganizationVerification begins DNS TXT org verification.
func (s *UserGRPC) StartOrganizationVerification(ctx context.Context, req *userv1.StartOrganizationVerificationRequest) (*userv1.StartOrganizationVerificationResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	row, err := s.Profiles.GetOwnedProfile(ctx, accountID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	txt, err := s.Profiles.StartOrgVerification(ctx, profileID, req.GetDomain())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &userv1.StartOrganizationVerificationResponse{
		Domain:    strings.ToLower(strings.TrimSpace(req.GetDomain())),
		TxtRecord: txt,
	}, nil
}

// CheckOrganizationVerification polls DNS for org verification TXT record.
func (s *UserGRPC) CheckOrganizationVerification(ctx context.Context, req *userv1.CheckOrganizationVerificationRequest) (*userv1.CheckOrganizationVerificationResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	row, err := s.Profiles.GetOwnedProfile(ctx, accountID, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "profile not found")
	}
	domain, token, err := s.Profiles.LatestOrgVerification(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.FailedPrecondition, "no pending org verification")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	resolver := s.DNSResolver
	if resolver == nil {
		resolver = systemDNSResolver{}
	}
	records, err := resolver.LookupTXT(ctx, domain)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "dns lookup failed")
	}
	want := "voice-verify=" + token
	found := false
	for _, rec := range records {
		if strings.TrimSpace(rec) == want {
			found = true
			break
		}
	}
	if !found {
		return &userv1.CheckOrganizationVerificationResponse{
			VerificationStatus: &userv1.VerificationStatus{
				ProfileId:        profileID.String(),
				VerificationType: row.VerificationType,
			},
		}, nil
	}
	updated, err := s.Profiles.MarkOrgVerificationVerified(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.Events != nil {
		_ = s.Events.PublishVerified(ctx, updated.ID.String(), updated.AccountID.String(), "organization")
	}
	return &userv1.CheckOrganizationVerificationResponse{
		VerificationStatus: verificationStatusFromRow(updated),
	}, nil
}

// ApplyDowngradeProfiles freezes all profiles except kept set (subscription downgrade).
func (s *UserGRPC) ApplyDowngradeProfiles(ctx context.Context, req *userv1.ApplyDowngradeProfilesRequest) (*userv1.ApplyDowngradeProfilesResponse, error) {
	if !authctx.IsInternalService(ctx) {
		accountID, ok := authctx.AccountID(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}
		if strings.TrimSpace(req.GetAccountId()) != "" && req.GetAccountId() != accountID.String() {
			return nil, status.Error(codes.PermissionDenied, "account mismatch")
		}
		req.AccountId = accountID.String()
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	kept := make([]uuid.UUID, 0, len(req.GetKeptProfileIds()))
	for _, raw := range req.GetKeptProfileIds() {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid kept_profile_id")
		}
		kept = append(kept, id)
	}
	if len(kept) != 2 {
		return nil, status.Error(codes.InvalidArgument, "exactly two profiles must be kept on downgrade")
	}
	if err := s.Profiles.ApplyDowngradeProfileSelection(ctx, accountID, kept); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.ApplyDowngradeProfilesResponse{KeptProfileIds: req.GetKeptProfileIds()}, nil
}

// DeleteProfile soft-archives a secondary profile.
func (s *UserGRPC) DeleteProfile(ctx context.Context, req *userv1.DeleteProfileRequest) (*userv1.DeleteProfileResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	if err := s.Profiles.SoftDeleteProfile(ctx, accountID, profileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.FailedPrecondition, "cannot delete profile")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.DeleteProfileResponse{}, nil
}

func verificationStatusFromRow(row *store.ProfileRow) *userv1.VerificationStatus {
	out := &userv1.VerificationStatus{
		ProfileId:        row.ID.String(),
		VerificationType: row.VerificationType,
	}
	if row.VerificationBadge != nil {
		out.Badge = row.VerificationBadge
	}
	return out
}
