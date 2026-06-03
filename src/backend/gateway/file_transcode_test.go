package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	filev1 "voice.app/voice/file/v1"
)

type recordingFileGRPC struct {
	filev1.UnimplementedFileServiceServer
	lastMD metadata.MD
	upload *filev1.RequestUploadRequest
	getURL *filev1.GetFileURLRequest
}

func (s *recordingFileGRPC) RequestUpload(ctx context.Context, req *filev1.RequestUploadRequest) (*filev1.RequestUploadResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	s.upload = req
	return &filev1.RequestUploadResponse{
		UploadResponse: &filev1.UploadResponse{
			FileId:          "11111111-1111-4111-8111-111111111111",
			R2Key:           "attachments/11111111-1111-4111-8111-111111111111/cat.png",
			PresignedPutUrl: "https://r2.example/upload/cat.png",
		},
	}, nil
}

func (s *recordingFileGRPC) GetFileURL(ctx context.Context, req *filev1.GetFileURLRequest) (*filev1.GetFileURLResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	s.lastMD = md
	s.getURL = req
	return &filev1.GetFileURLResponse{
		PresignedGetUrl: "https://r2.example/download/" + req.GetFileId(),
		ExpiresAt:       timestamppb.New(time.Unix(1790000000, 0)),
	}, nil
}

func TestTranscodeFilesUploadPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingFileGRPC{}
	conn, cleanup := startBufconnFileConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{file: filev1.NewFileServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"files": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	body := `{"original_name":"cat.png","mime_type":"image/png","size_bytes":2048}`
	resp := performRequest(h, http.MethodPost, "/api/v1/files/upload", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled, "REST proxy must not run when gRPC transcoder handles POST /api/v1/files/upload")
	require.NotNil(t, grpcRec.upload)
	require.Equal(t, "cat.png", grpcRec.upload.GetOriginalName())
	require.Equal(t, "image/png", grpcRec.upload.GetMimeType())
	require.Equal(t, int64(2048), grpcRec.upload.GetSizeBytes())
	require.Equal(t, []string{"account-1"}, grpcRec.lastMD.Get("x-voice-user-id"))
	require.Equal(t, []string{"profile-1"}, grpcRec.lastMD.Get("x-voice-profile-id"))

	var out struct {
		UploadResponse struct {
			FileID          string `json:"file_id"`
			R2Key           string `json:"r2_key"`
			PresignedPutURL string `json:"presigned_put_url"`
		} `json:"upload_response"`
	}
	decodeJSON(t, resp.Body, &out)
	require.Equal(t, "11111111-1111-4111-8111-111111111111", out.UploadResponse.FileID)
	require.Contains(t, out.UploadResponse.R2Key, "attachments/")
	require.Equal(t, "https://r2.example/upload/cat.png", out.UploadResponse.PresignedPutURL)
}

func TestTranscodeFilesGetURLPrecedenceOverRESTProxy(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingFileGRPC{}
	conn, cleanup := startBufconnFileConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{file: filev1.NewFileServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"files": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	fileID := "22222222-2222-4222-8222-222222222222"
	resp := performRequest(h, http.MethodGet, "/api/v1/files/"+fileID+"/url", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	require.Equal(t, http.StatusOK, resp.Code, "body=%s", resp.Body.String())
	require.False(t, proxyCalled, "REST proxy must not run when gRPC transcoder handles GET /api/v1/files/{id}/url")
	require.NotNil(t, grpcRec.getURL)
	require.Equal(t, fileID, grpcRec.getURL.GetFileId())
	require.Equal(t, []string{"account-1"}, grpcRec.lastMD.Get("x-voice-user-id"))
	require.Equal(t, []string{"profile-1"}, grpcRec.lastMD.Get("x-voice-profile-id"))

	var out struct {
		PresignedGetURL string `json:"presigned_get_url"`
		ExpiresAt       string `json:"expires_at"`
	}
	decodeJSON(t, resp.Body, &out)
	require.Equal(t, "https://r2.example/download/"+fileID, out.PresignedGetURL)
	require.NotEmpty(t, out.ExpiresAt)
}

func startBufconnFileConn(t *testing.T, impl filev1.FileServiceServer) (grpc.ClientConnInterface, func()) {
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
