package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeFileAttachmentRestartProof_live is deliberately split into two
// externally orchestrated phases. The compose runner executes "prepare",
// restarts File only, then executes "verify" using the state file
// left by prepare. This test must remain an HTTP client and must never invoke
// Docker itself.
//
// Required runner environment:
//
//	VOICE_FILE_ATTACHMENT_RESTART_PHASE=prepare|verify
//	VOICE_FILE_ATTACHMENT_RESTART_STATE_PATH=/absolute/path/to/state.json
//	VOICE_FILE_ATTACHMENT_RESTART_PROOF_ID=<generated runner proof id>
//	VOICE_FILE_ATTACHMENT_RESTART_RUN_STARTED_UNIX_NANO=<generated runner timestamp>
func TestComposeFileAttachmentRestartProof_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}

	phase := strings.TrimSpace(os.Getenv("VOICE_FILE_ATTACHMENT_RESTART_PHASE"))
	statePath := strings.TrimSpace(os.Getenv("VOICE_FILE_ATTACHMENT_RESTART_STATE_PATH"))
	proofID := strings.TrimSpace(os.Getenv("VOICE_FILE_ATTACHMENT_RESTART_PROOF_ID"))
	runStarted := strings.TrimSpace(os.Getenv("VOICE_FILE_ATTACHMENT_RESTART_RUN_STARTED_UNIX_NANO"))
	if phase == "" || statePath == "" {
		t.Skip("restart proof requires external runner phase and state path")
	}
	require.Contains(t, []string{"prepare", "verify"}, phase, "unknown restart proof phase")
	require.True(t, filepath.IsAbs(statePath), "restart proof state path must be absolute")
	require.NotEmpty(t, proofID, "restart proof requires a generated proof id")
	_, err := strconv.ParseInt(runStarted, 10, 64)
	require.NoError(t, err, "restart proof requires runner start time in Unix nanoseconds")

	client := &http.Client{Timeout: 60 * time.Second}
	base := liveGatewayBaseURL()
	switch phase {
	case "prepare":
		prepareComposeFileAttachmentRestartProof(t, client, base, statePath, proofID, runStarted)
	case "verify":
		verifyComposeFileAttachmentRestartProof(t, client, base, statePath, proofID, runStarted)
	}
}

type composeFileAttachmentRestartState struct {
	ChatID               string `json:"chat_id"`
	FileID               string `json:"file_id"`
	ImageFileID          string `json:"image_file_id"`
	SingletonFileID      string `json:"singleton_file_id"`
	MessageID            string `json:"message_id"`
	RecipientAccessToken string `json:"recipient_access_token"`
	UploaderAccessToken  string `json:"uploader_access_token"`
	SHA256               string `json:"sha256"`
	ImageSHA256          string `json:"image_sha256"`
	ImageSize            int    `json:"image_size"`
	Content              string `json:"content"`
	ProofID              string `json:"proof_id"`
	CreatedAtUnixNano    int64  `json:"created_at_unix_nano"`
}

func prepareComposeFileAttachmentRestartProof(t *testing.T, client *http.Client, base, statePath, proofID, runStarted string) {
	t.Helper()
	clearLiveComposeAuthRateLimit(t)

	n := time.Now().UnixNano()
	sessA := registerComposeUser(t, client, base, formatComposeEmail("file-restart-a", n), "VoiceQaTest1!")
	sessB := registerComposeUser(t, client, base, formatComposeEmail("file-restart-b", n), "VoiceQaTest1!")
	stranger := registerComposeUser(t, client, base, formatComposeEmail("file-restart-stranger", n), "VoiceQaTest1!")
	if !composeFileUploadAvailable(t, client, base, sessA.AccessToken) {
		t.Skip("object storage not configured (MinIO/R2); set FILE_R2_* in .env for compose app profile")
	}

	sendComposeFriendInvitation(t, client, base, sessA.AccessToken, sessB.ProfileID)
	acceptComposeFriendInvitation(t, client, base, sessB.AccessToken, sessA.ProfileID)
	chatID := createComposeDMBetween(t, client, base, sessA, sessB)
	// A second message makes the following page's cursor meaningful; verify must
	// consume that cursor after finding the attachment reference in fresh history.
	sendComposeMessage(t, client, base, sessA.AccessToken, chatID, "attachment-restart-baseline")

	content := []byte("attachment-restart-proof: deterministic text bytes\n")
	objectClient := composeRestartProofDownloadClient()
	t.Cleanup(objectClient.CloseIdleConnections)
	fileID, fileType := composeUploadTextFile(t, objectClient, base, sessA.AccessToken, chatID, "restart-proof.txt", content)
	imageInput := restartProofPNG()
	imageFileID, imageType := composeUploadImageFile(t, objectClient, base, sessA.AccessToken, chatID, "restart-proof.png", imageInput)
	imageMetadata := composeWaitForProcessedImage(t, client, base, sessA.AccessToken, imageFileID)
	require.Equal(t, "image", imageType)
	require.Equal(t, "ready", imageMetadata.Status)
	require.NotEmpty(t, imageMetadata.ConvertedR2Key)
	require.NotEmpty(t, imageMetadata.ThumbnailR2Key)
	attachments, err := json.Marshal([]map[string]string{
		{"file_id": fileID, "type": fileType},
		{"file_id": imageFileID, "type": imageType},
	})
	require.NoError(t, err)
	messageID := sendComposeMessageWithAttachmentsJSON(t, client, base, sessA.AccessToken, chatID, string(attachments))
	composeRequireAttachmentInCursorHistory(t, client, base, sessB.AccessToken, chatID, messageID, imageFileID)
	status, imageURL := composeGetFileURL(t, client, base, sessB.AccessToken, imageFileID)
	require.Equal(t, http.StatusOK, status, "DM recipient must receive an image URL after processing")
	preRestartImage, imageContentType := composeDownloadProofObject(t, objectClient, imageURL)
	mediaType, _, err := mime.ParseMediaType(imageContentType)
	require.NoError(t, err)
	require.Equal(t, "image/webp", mediaType)
	require.True(t, len(preRestartImage) >= 12 && string(preRestartImage[:4]) == "RIFF" && string(preRestartImage[8:12]) == "WEBP", "download must contain a WebP object")
	imageDigest := sha256.Sum256(preRestartImage)
	// A distinct, unattached upload exercises zero-reference terminal deletion
	// after restart without attempting to revoke a surviving message reference.
	singletonID, _ := composeUploadTextFile(t, objectClient, base, sessA.AccessToken, chatID, "restart-singleton.txt", []byte("unattached-restart-proof: distinct singleton bytes\n"))
	require.NotEqual(t, fileID, singletonID)

	// File ACL remains fail-closed even before the restart proof resumes.
	status, _ = composeGetFileURL(t, client, base, stranger.AccessToken, fileID)
	require.Equal(t, http.StatusForbidden, status, "unrelated profile must not receive a file URL")
	require.Equal(t, http.StatusForbidden,
		composeDeleteFileStatus(t, client, base, stranger.AccessToken, fileID),
		"unrelated profile must not delete an attachment")

	pendingID, pendingType := composeRequestPendingTextFile(t, client, base, sessA.AccessToken, chatID)
	pendingAttachments, err := json.Marshal([]map[string]string{{"file_id": pendingID, "type": pendingType}})
	require.NoError(t, err)
	require.Equal(t, http.StatusPreconditionFailed,
		sendComposeMessageWithAttachmentsStatus(t, client, base, sessA.AccessToken, chatID, string(pendingAttachments)),
		"pending upload must not be attachable")
	status, _ = composeGetFileURL(t, client, base, sessA.AccessToken, pendingID)
	require.Equal(t, http.StatusPreconditionFailed, status, "pending upload must not receive a download URL")

	sum := sha256.Sum256(content)
	state := composeFileAttachmentRestartState{
		ChatID:               chatID,
		FileID:               fileID,
		ImageFileID:          imageFileID,
		SingletonFileID:      singletonID,
		MessageID:            messageID,
		RecipientAccessToken: sessB.AccessToken,
		UploaderAccessToken:  sessA.AccessToken,
		SHA256:               hex.EncodeToString(sum[:]),
		ImageSHA256:          hex.EncodeToString(imageDigest[:]),
		ImageSize:            len(preRestartImage),
		Content:              string(content),
		ProofID:              proofID,
		CreatedAtUnixNano:    time.Now().UnixNano(),
	}
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	writeComposeFileAttachmentRestartState(t, statePath, encoded)
}

func verifyComposeFileAttachmentRestartProof(t *testing.T, client *http.Client, base, statePath, proofID, runStarted string) {
	t.Helper()
	info, err := os.Stat(statePath)
	require.NoError(t, err, "stat restart-proof state written before File restart")
	require.True(t, info.Mode().IsRegular(), "restart-proof state must be a regular file")
	encoded, err := os.ReadFile(statePath)
	require.NoError(t, err, "read restart-proof state written before File restart")
	var state composeFileAttachmentRestartState
	require.NoError(t, json.Unmarshal(encoded, &state))
	require.NotEmpty(t, state.ChatID)
	require.NotEmpty(t, state.FileID)
	require.NotEmpty(t, state.ImageFileID)
	require.NotEmpty(t, state.SingletonFileID)
	require.NotEqual(t, state.FileID, state.SingletonFileID)
	require.NotEmpty(t, state.MessageID)
	require.NotEmpty(t, state.RecipientAccessToken)
	require.NotEmpty(t, state.UploaderAccessToken)
	require.Equal(t, proofID, state.ProofID, "state must belong to this runner invocation")
	startedAt, err := strconv.ParseInt(runStarted, 10, 64)
	require.NoError(t, err)
	require.GreaterOrEqual(t, state.CreatedAtUnixNano, startedAt, "state predates this runner invocation")
	require.LessOrEqual(t, state.CreatedAtUnixNano, time.Now().UnixNano(), "state timestamp is in the future")
	require.Less(t, time.Since(time.Unix(0, state.CreatedAtUnixNano)), 15*time.Minute, "state is stale")
	decodedHash, err := hex.DecodeString(state.SHA256)
	require.NoError(t, err, "state SHA-256 must be hexadecimal")
	require.Len(t, decodedHash, sha256.Size, "state SHA-256 must have exactly 32 bytes")

	composeRequireAttachmentInCursorHistory(t, client, base, state.RecipientAccessToken, state.ChatID, state.MessageID, state.FileID)
	composeRequireAttachmentInCursorHistory(t, client, base, state.RecipientAccessToken, state.ChatID, state.MessageID, state.ImageFileID)
	status, downloadURL := composeGetFileURL(t, client, base, state.RecipientAccessToken, state.FileID)
	require.Equal(t, http.StatusOK, status, "DM recipient must receive a fresh URL after File restart")
	require.NotEmpty(t, downloadURL)

	resp, err := composeRestartProofDownloadClient().Get(downloadURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	downloaded, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "download status=%d", resp.StatusCode)
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"), "MinIO must honor the signed response override so edge caches cannot outlive access")
	sum := sha256.Sum256(downloaded)
	require.Equal(t, state.SHA256, hex.EncodeToString(sum[:]), "download must retain original SHA-256")
	require.Equal(t, state.Content, string(downloaded), "download must retain exact deterministic bytes")
	status, imageURL := composeGetFileURL(t, client, base, state.RecipientAccessToken, state.ImageFileID)
	require.Equal(t, http.StatusOK, status, "DM recipient must receive a processed-image URL after File restart")
	imageClient := composeRestartProofDownloadClient()
	t.Cleanup(imageClient.CloseIdleConnections)
	imageBytes, imageContentType := composeDownloadProofObject(t, imageClient, imageURL)
	mediaType, _, err := mime.ParseMediaType(imageContentType)
	require.NoError(t, err)
	require.Equal(t, "image/webp", mediaType)
	require.True(t, len(imageBytes) >= 12 && string(imageBytes[:4]) == "RIFF" && string(imageBytes[8:12]) == "WEBP", "download must contain WebP bytes after restart")
	require.Len(t, imageBytes, state.ImageSize, "processed image size must survive restart")
	imageDigest := sha256.Sum256(imageBytes)
	require.Equal(t, state.ImageSHA256, hex.EncodeToString(imageDigest[:]), "processed image bytes must retain SHA-256 across restart")

	// A live message reference remains authoritative across File restart. Legacy
	// uploader deletion must not invalidate the recipient's attachment.
	require.Equal(t, http.StatusPreconditionFailed,
		composeDeleteFileStatus(t, client, base, state.UploaderAccessToken, state.FileID),
		"surviving message reference must prevent legacy uploader deletion")
	composeRequireAttachmentInCursorHistory(t, client, base, state.RecipientAccessToken, state.ChatID, state.MessageID, state.FileID)
	require.Equal(t, http.StatusOK,
		composeGetFileMetadataStatus(t, client, base, state.RecipientAccessToken, state.FileID))
	status, retainedURL := composeGetFileURL(t, client, base, state.RecipientAccessToken, state.FileID)
	require.Equal(t, http.StatusOK, status, "retained attachment must still receive a fresh URL")
	retained, err := composeRestartProofDownloadClient().Get(retainedURL)
	require.NoError(t, err)
	defer retained.Body.Close()
	retainedBytes, err := io.ReadAll(retained.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, retained.StatusCode)
	require.Equal(t, "no-store", retained.Header.Get("Cache-Control"))
	require.Equal(t, downloaded, retainedBytes, "denied deletion must retain exact attachment bytes")

	// Zero-reference singleton deletion is still terminal after restart.
	require.Equal(t, http.StatusOK,
		composeGetFileMetadataStatus(t, client, base, state.UploaderAccessToken, state.SingletonFileID))
	require.Equal(t, http.StatusNoContent,
		composeDeleteFileStatus(t, client, base, state.UploaderAccessToken, state.SingletonFileID))
	require.Equal(t, http.StatusPreconditionFailed,
		composeGetFileMetadataStatus(t, client, base, state.UploaderAccessToken, state.SingletonFileID))
	status, _ = composeGetFileURL(t, client, base, state.UploaderAccessToken, state.SingletonFileID)
	require.Equal(t, http.StatusPreconditionFailed, status, "deleted singleton must not receive a fresh URL")
}

func writeComposeFileAttachmentRestartState(t *testing.T, statePath string, encoded []byte) {
	t.Helper()
	parent := filepath.Dir(statePath)
	info, err := os.Stat(parent)
	require.NoError(t, err, "restart-proof state directory must exist")
	require.True(t, info.IsDir(), "restart-proof state parent must be a directory")
	_, err = os.Lstat(statePath)
	require.True(t, os.IsNotExist(err), "restart-proof state path must be fresh")

	temporary, err := os.CreateTemp(parent, ".restart-proof-*")
	require.NoError(t, err)
	temporaryName := temporary.Name()
	t.Cleanup(func() { _ = os.Remove(temporaryName) })
	require.NoError(t, temporary.Chmod(0o600))
	_, err = temporary.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, temporary.Sync())
	require.NoError(t, temporary.Close())
	require.NoError(t, os.Rename(temporaryName, statePath), "atomically publish restart-proof state")
}

func restartProofPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
}

func composeUploadImageFile(t *testing.T, client *http.Client, base, accessToken, chatID, originalName string, content []byte) (fileID, fileType string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"original_name": originalName,
		"mime_type":     "image/png",
		"size_bytes":    len(content),
		"context_chat":  map[string]string{"id": chatID, "type": "CHAT_TYPE_DM"},
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/files/upload", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "image upload request body=%s", string(body))
	var ticket struct {
		UploadResponse struct {
			FileID          string `json:"file_id"`
			PresignedPutURL string `json:"presigned_put_url"`
		} `json:"upload_response"`
	}
	require.NoError(t, json.Unmarshal(body, &ticket))
	require.NotEmpty(t, ticket.UploadResponse.FileID)
	require.NotEmpty(t, ticket.UploadResponse.PresignedPutURL)
	putReq, err := http.NewRequest(http.MethodPut, ticket.UploadResponse.PresignedPutURL, bytes.NewReader(content))
	require.NoError(t, err)
	putReq.Header.Set("Content-Type", "image/png")
	putResp, err := client.Do(putReq)
	require.NoError(t, err)
	putBody, err := io.ReadAll(putResp.Body)
	putResp.Body.Close()
	require.NoError(t, err)
	require.True(t, putResp.StatusCode >= 200 && putResp.StatusCode < 300, "image PUT status=%d body=%s", putResp.StatusCode, string(putBody))
	confirmPayload, err := json.Marshal(map[string]string{"sha256_hash": sha256Hex(content)})
	require.NoError(t, err)
	confirmReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/files/"+url.PathEscape(ticket.UploadResponse.FileID)+"/confirm", bytes.NewReader(confirmPayload))
	require.NoError(t, err)
	confirmReq.Header.Set("Authorization", "Bearer "+accessToken)
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmResp, err := client.Do(confirmReq)
	require.NoError(t, err)
	confirmBody, err := io.ReadAll(confirmResp.Body)
	confirmResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, confirmResp.StatusCode, "image confirm body=%s", string(confirmBody))
	var confirmed struct {
		FileMetadata struct {
			FileType string `json:"file_type"`
		} `json:"file_metadata"`
	}
	require.NoError(t, json.Unmarshal(confirmBody, &confirmed))
	return ticket.UploadResponse.FileID, strings.TrimSpace(confirmed.FileMetadata.FileType)
}

type composeProcessedImageMetadata struct {
	Status         string `json:"status"`
	ConvertedR2Key string `json:"converted_r2_key"`
	ThumbnailR2Key string `json:"thumbnail_r2_key"`
}

func composeWaitForProcessedImage(t *testing.T, client *http.Client, base, accessToken, fileID string) composeProcessedImageMetadata {
	t.Helper()
	type response struct {
		FileMetadata composeProcessedImageMetadata `json:"file_metadata"`
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, base+"/api/v1/files/"+url.PathEscape(fileID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, readErr)
		require.Equal(t, http.StatusOK, resp.StatusCode, "image metadata body=%s", string(body))
		var current response
		require.NoError(t, json.Unmarshal(body, &current))
		if current.FileMetadata.Status == "failed" {
			t.Fatalf("image processing failed: %s", string(body))
		}
		if current.FileMetadata.Status == "ready" && current.FileMetadata.ConvertedR2Key != "" && current.FileMetadata.ThumbnailR2Key != "" {
			return current.FileMetadata
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("image processing did not reach ready with full and thumbnail WebP keys before timeout")
	return composeProcessedImageMetadata{}
}

func composeDownloadProofObject(t *testing.T, client *http.Client, downloadURL string) ([]byte, string) {
	t.Helper()
	resp, err := client.Get(downloadURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "download status=%d body=%s", resp.StatusCode, string(body))
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	return body, resp.Header.Get("Content-Type")
}

// File reaches the isolated MinIO from its container through
// host.docker.internal. Keep that signed host intact, but connect it to the
// host-loopback listener when this live test runs outside Docker (notably CI).
func composeRestartProofDownloadClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err == nil && strings.EqualFold(host, "host.docker.internal") {
			address = net.JoinHostPort("127.0.0.1", port)
		}
		return dialer.DialContext(ctx, network, address)
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: transport}
}

func composeRequireAttachmentInCursorHistory(t *testing.T, client *http.Client, base, accessToken, chatID, messageID, fileID string) {
	t.Helper()
	cursor := ""
	usedCursor := false
	found := false
	for page := 0; page < 4; page++ {
		endpoint := base + "/api/v1/messages?chat_id=" + url.QueryEscape(chatID) + "&page_size=1"
		if cursor != "" {
			endpoint += "&cursor=" + url.QueryEscape(cursor)
			usedCursor = true
		}
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, readErr)
		require.Equal(t, http.StatusOK, resp.StatusCode, "GET messages body=%s", string(body))

		var history struct {
			MessageList struct {
				Messages []struct {
					ID              string `json:"id"`
					AttachmentsJSON string `json:"attachments_json"`
				} `json:"messages"`
				NextCursor string `json:"next_cursor"`
			} `json:"message_list"`
		}
		require.NoError(t, json.Unmarshal(body, &history))
		for _, message := range history.MessageList.Messages {
			if message.ID != messageID {
				continue
			}
			var attachments []struct {
				FileID string `json:"file_id"`
			}
			require.NoError(t, json.Unmarshal([]byte(message.AttachmentsJSON), &attachments), "history attachment JSON")
			for _, attachment := range attachments {
				if attachment.FileID == fileID {
					found = true
					break
				}
			}
		}
		if found && (usedCursor || history.MessageList.NextCursor == "") {
			break
		}
		cursor = history.MessageList.NextCursor
		if cursor == "" {
			break
		}
	}
	require.True(t, found, "fresh history must retain attachment file_id")
	require.True(t, usedCursor, "history proof must consume a per-chat cursor")
}

func composeGetFileURL(t *testing.T, client *http.Client, base, accessToken, fileID string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/files/"+url.PathEscape(fileID)+"/url", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, ""
	}
	var parsed struct {
		PresignedGetURL string `json:"presigned_get_url"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return resp.StatusCode, parsed.PresignedGetURL
}

func composeGetFileMetadataStatus(t *testing.T, client *http.Client, base, accessToken, fileID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/files/"+url.PathEscape(fileID), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode
}

func composeDeleteFileStatus(t *testing.T, client *http.Client, base, accessToken, fileID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/files/"+url.PathEscape(fileID), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode
}

func composeRequestPendingTextFile(t *testing.T, client *http.Client, base, accessToken, chatID string) (fileID, fileType string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"original_name": "pending-restart-proof.txt",
		"mime_type":     "text/plain",
		"size_bytes":    7,
		"context_chat":  map[string]string{"id": chatID, "type": "CHAT_TYPE_DM"},
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/files/upload", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "pending upload request body=%s", string(body))
	var parsed struct {
		UploadResponse struct {
			FileID string `json:"file_id"`
		} `json:"upload_response"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.UploadResponse.FileID)
	return parsed.UploadResponse.FileID, "file"
}
