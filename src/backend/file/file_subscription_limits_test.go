package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	filev1 "voice.app/voice/file/v1"
)

const (
	upload100MiB = 100 << 20
	upload250MiB = 250 << 20
	upload51MiB  = 51 << 20
)

func TestRequestUpload_Premium100MBAccepted(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	profileID := uuid.New()
	authed := withFileProfileAndTier(ctx, uuid.New(), profileID, "premium")

	_, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "large-archive.zip",
		MimeType:     "application/zip",
		SizeBytes:    upload100MiB,
	})
	require.NoError(t, err)
}

func TestRequestUpload_Premium250MBRejected(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	profileID := uuid.New()
	authed := withFileProfileAndTier(ctx, uuid.New(), profileID, "premium")

	_, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "too-large.iso",
		MimeType:     "application/octet-stream",
		SizeBytes:    upload250MiB,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRequestUpload_Free51MBRejected(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	profileID := uuid.New()
	authed := withFileProfileAndTier(ctx, uuid.New(), profileID, "free")

	_, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "over-free-limit.bin",
		MimeType:     "application/octet-stream",
		SizeBytes:    upload51MiB,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func withFileProfileAndTier(ctx context.Context, accountID, profileID uuid.UUID, tier string) context.Context {
	ctx = withFileProfile(ctx, accountID, profileID)
	return metadata.AppendToOutgoingContext(ctx, "x-voice-subscription-tier", tier)
}
