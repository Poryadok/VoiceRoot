package s2s

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	filev1 "voice.app/voice/file/v1"
)

// FileGRPCMetadata validates attachment metadata via FileService (forwards caller metadata).
type FileGRPCMetadata struct {
	Client filev1.FileServiceClient
}

func NewFileGRPCMetadata(c filev1.FileServiceClient) *FileGRPCMetadata {
	return &FileGRPCMetadata{Client: c}
}

func (f *FileGRPCMetadata) GetBulkMetadata(ctx context.Context, req *filev1.GetBulkMetadataRequest, opts ...grpc.CallOption) (*filev1.GetBulkMetadataResponse, error) {
	if f == nil || f.Client == nil {
		return nil, status.Error(codes.FailedPrecondition, "file service not configured")
	}
	ctx = ForwardIncomingMetadata(ctx)
	return f.Client.GetBulkMetadata(ctx, req, opts...)
}
