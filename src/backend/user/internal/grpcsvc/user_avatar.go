package grpcsvc

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/r2avatar"

	userv1 "voice.app/voice/user/v1"
)

// AvatarPresigner issues presigned PUT URLs for profile avatars (e.g. Cloudflare R2).
type AvatarPresigner interface {
	PresignPut(ctx context.Context, objectKey, contentType string, contentLength int64) (uploadURL string, signedHeaders map[string]string, expiresAt time.Time, err error)
}

// CreateAvatarPresignedUpload returns a presigned PUT per PLAN § R2 (whitelist MIME, max size).
func (s *UserGRPC) CreateAvatarPresignedUpload(ctx context.Context, req *userv1.CreateAvatarPresignedUploadRequest) (*userv1.CreateAvatarPresignedUploadResponse, error) {
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.AvatarPresigner == nil {
		return nil, status.Error(codes.FailedPrecondition, "avatar upload is not configured")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	ct := strings.TrimSpace(req.GetContentType())
	clen := req.GetContentLength()
	if err := r2avatar.ValidateUploadParams(ct, clen); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	ct = strings.ToLower(ct)
	ext := r2avatar.FileExtForContentType(ct)
	if ext == "" {
		return nil, status.Error(codes.InvalidArgument, "unsupported content_type")
	}
	row, err := s.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil || row.AccountID != accountID {
		return nil, status.Error(codes.NotFound, "profile not found or not owned")
	}
	objectKey := r2avatar.ObjectKey(profileID, ext)
	uploadURL, hdrs, exp, err := s.AvatarPresigner.PresignPut(ctx, objectKey, ct, clen)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	publicURL := r2avatar.JoinPublicURL(s.AvatarPublicBaseURL, objectKey)
	return &userv1.CreateAvatarPresignedUploadResponse{
		HttpMethod:       "PUT",
		UploadUrl:        uploadURL,
		RequiredHeaders:  hdrs,
		MaxBytes:         r2avatar.MaxAvatarBytes,
		ExpiresAt:        timestamppb.New(exp),
		PublicUrl:        publicURL,
		ObjectKey:        objectKey,
	}, nil
}
