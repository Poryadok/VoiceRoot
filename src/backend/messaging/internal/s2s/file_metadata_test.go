package s2s

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	filev1 "voice.app/voice/file/v1"
)

type recordingFileBulkMetadata struct {
	filev1.UnimplementedFileServiceServer
	lastMD metadata.MD
}

func (s *recordingFileBulkMetadata) GetBulkMetadata(ctx context.Context, req *filev1.GetBulkMetadataRequest) (*filev1.GetBulkMetadataResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	return &filev1.GetBulkMetadataResponse{
		BulkFileMetadata: &filev1.BulkFileMetadata{ByFileId: map[string]*filev1.FileMetadata{}},
	}, nil
}

func TestFileGRPCMetadataForwardsCallerProfile(t *testing.T) {
	t.Parallel()

	rec := &recordingFileBulkMetadata{}
	conn, cleanup := startBufconnFileService(t, rec)
	t.Cleanup(cleanup)

	client := NewFileGRPCMetadata(filev1.NewFileServiceClient(conn))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-user-id", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"x-voice-profile-id", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	))
	_, err := client.GetBulkMetadata(ctx, &filev1.GetBulkMetadataRequest{FileIds: []string{"cccccccc-cccc-4ccc-8ccc-cccccccccccc"}})
	require.NoError(t, err)
	require.Equal(t, []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}, rec.lastMD.Get("x-voice-user-id"))
	require.Equal(t, []string{"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}, rec.lastMD.Get("x-voice-profile-id"))
}

func startBufconnFileService(t *testing.T, impl filev1.FileServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	filev1.RegisterFileServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}
