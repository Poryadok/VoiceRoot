package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	moderationv1 "voice.app/voice/moderation/v1"
)

func TestProfileIDFromMetadata_Missing(t *testing.T) {
	_, err := profileIDFromMetadata(context.Background())
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestProfileIDFromMetadata_Invalid(t *testing.T) {
	md := metadata.Pairs("x-voice-profile-id", "not-uuid")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := profileIDFromMetadata(ctx)
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestProfileIDFromMetadata_OK(t *testing.T) {
	id := uuid.New()
	md := metadata.Pairs("x-voice-profile-id", id.String())
	ctx := metadata.NewIncomingContext(context.Background(), md)
	got, err := profileIDFromMetadata(ctx)
	require.NoError(t, err)
	require.Equal(t, id, got)
}

func TestIsInternalRequest(t *testing.T) {
	require.False(t, isInternalRequest(context.Background()))
	md := metadata.Pairs("x-voice-internal", "true")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	require.True(t, isInternalRequest(ctx))
}

func TestCreateReport_StoreNotConfigured(t *testing.T) {
	svc := &ModerationGRPC{}
	_, err := svc.CreateReport(context.Background(), &moderationv1.CreateReportRequest{
		TargetType: "user",
		TargetId:   uuid.New().String(),
		Category:   "spam",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestListReports_StoreNotConfigured(t *testing.T) {
	svc := &ModerationGRPC{}
	md := metadata.Pairs("x-voice-internal", "true")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := svc.ListReports(ctx, &moderationv1.ListReportsRequest{})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestReportRowToProto_Nil(t *testing.T) {
	require.Nil(t, reportRowToProto(nil))
}
