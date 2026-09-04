package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
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
// restarts only the File container, then executes "verify" using the state
// file left by prepare. This test must remain an HTTP client and must never
// invoke Docker itself.
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
	MessageID            string `json:"message_id"`
	RecipientAccessToken string `json:"recipient_access_token"`
	SHA256               string `json:"sha256"`
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

	chatID := createComposeDMBetween(t, client, base, sessA, sessB)
	// A second message makes the following page's cursor meaningful; verify must
	// consume that cursor after finding the attachment reference in fresh history.
	sendComposeMessage(t, client, base, sessA.AccessToken, chatID, "attachment-restart-baseline")

	content := []byte("attachment-restart-proof: deterministic text bytes\n")
	fileID, fileType := composeUploadTextFile(t, client, base, sessA.AccessToken, chatID, "restart-proof.txt", content)
	attachments, err := json.Marshal([]map[string]string{{"file_id": fileID, "type": fileType}})
	require.NoError(t, err)
	messageID := sendComposeMessageWithAttachmentsJSON(t, client, base, sessA.AccessToken, chatID, string(attachments))

	// File ACL remains fail-closed even before the restart proof resumes.
	status, _ := composeGetFileURL(t, client, base, stranger.AccessToken, fileID)
	require.Equal(t, http.StatusForbidden, status, "unrelated profile must not receive a file URL")

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
		MessageID:            messageID,
		RecipientAccessToken: sessB.AccessToken,
		SHA256:               hex.EncodeToString(sum[:]),
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
	require.NotEmpty(t, state.MessageID)
	require.NotEmpty(t, state.RecipientAccessToken)
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
	status, downloadURL := composeGetFileURL(t, client, base, state.RecipientAccessToken, state.FileID)
	require.Equal(t, http.StatusOK, status, "DM recipient must receive a fresh URL after File restart")
	require.NotEmpty(t, downloadURL)

	resp, err := composeRestartProofDownloadClient().Get(downloadURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	downloaded, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "download status=%d", resp.StatusCode)
	sum := sha256.Sum256(downloaded)
	require.Equal(t, state.SHA256, hex.EncodeToString(sum[:]), "download must retain original SHA-256")
	require.Equal(t, state.Content, string(downloaded), "download must retain exact deterministic bytes")
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
