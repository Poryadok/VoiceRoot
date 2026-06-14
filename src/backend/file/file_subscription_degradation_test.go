package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	filev1 "voice.app/voice/file/v1"
)

const (
	upload51MiBNoTier = 51 << 20
)

// TestRequestUpload_NoSubscriptionTierUsesFreeLimit documents defensive free-tier cap when tier metadata is absent.
func TestRequestUpload_NoSubscriptionTierUsesFreeLimit(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	profileID := uuid.New()
	authed := withFileProfile(ctx, uuid.New(), profileID)

	_, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "over-free-limit.bin",
		MimeType:     "application/octet-stream",
		SizeBytes:    upload51MiBNoTier,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
