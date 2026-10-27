package main

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	filev1 "voice.app/voice/file/v1"
)

func TestGIFProcessingIntentUsesAuthenticatedFileTranscoder(t *testing.T) {
	rec := &recordingFileGRPC{}
	conn, cleanup := startBufconnFileConn(t, rec)
	t.Cleanup(cleanup)
	handler := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"gif-token": {UserID: "account-1", ProfileID: "profile-1"}}, transcoder: &transcoder{clients: grpcClients{file: filev1.NewFileServiceClient(conn)}}})
	body := `{"original_name":"animation.gif","mime_type":"image/gif","size_bytes":1024,"intent":"UPLOAD_INTENT_GIF"}`
	denied := performRequest(handler, http.MethodPost, "/api/v1/files/upload", body, nil)
	require.Equal(t, http.StatusUnauthorized, denied.Code)
	require.Nil(t, rec.upload)
	response := performRequest(handler, http.MethodPost, "/api/v1/files/upload", body, map[string]string{"Authorization": "Bearer gif-token"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, filev1.UploadIntent_UPLOAD_INTENT_GIF, rec.upload.GetIntent())
	require.Equal(t, []string{"profile-1"}, rec.lastMD.Get("x-voice-profile-id"))
	rec.confirmResponse = &filev1.ConfirmUploadResponse{FileMetadata: &filev1.FileMetadata{Id: "11111111-1111-4111-8111-111111111111", MimeType: "image/gif", SizeBytes: 1024, Sha256Hash: "source-hash", ObjectRevision: 1, Intent: filev1.UploadIntent_UPLOAD_INTENT_GIF, Status: "ready", FileType: "video", DeliveredFull: &filev1.FileRepresentation{MimeType: "video/mp4", SizeBytes: 2048, Sha256Hash: "delivered-hash", Filename: "animation.mp4", ProcessorVersion: "gif-mp4-v1"}}}
	response = performRequest(handler, http.MethodPost, "/api/v1/files/11111111-1111-4111-8111-111111111111/confirm", `{"sha256_hash":"source-hash"}`, map[string]string{"Authorization": "Bearer gif-token"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"delivered_full"`)
	require.Contains(t, response.Body.String(), `"video/mp4"`)
	require.Contains(t, response.Body.String(), `"image/gif"`)
}
