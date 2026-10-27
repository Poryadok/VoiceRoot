package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeStoryTTLDeleteFile_live is intentionally limited to a disposable
// Compose project. The fixture advances only its own archived_until after the
// real expiry worker has archived the story; product retention stays 30 days.
func TestComposeStoryTTLDeleteFile_live(t *testing.T) {
	if !liveComposeEnabled() || os.Getenv("VOICE_STORY_TTL_DELETE_LIVE_TEST") != "true" {
		t.Skip("requires private Story TTL Compose proof opt-in")
	}
	project := os.Getenv("COMPOSE_PROJECT_NAME")
	require.True(t, strings.HasPrefix(project, "voice-story-ttl-delete-"), "private Compose project required")
	proofID := os.Getenv("VOICE_STORY_TTL_DELETE_PROOF_ID")
	require.NotEmpty(t, proofID)
	require.Equal(t, "voice-story-ttl-delete-"+proofID, project)
	startedNano, err := strconv.ParseInt(os.Getenv("VOICE_STORY_TTL_DELETE_STARTED_UNIX_NANO"), 10, 64)
	require.NoError(t, err)
	require.Greater(t, startedNano, int64(0))
	require.Equal(t, "30s", os.Getenv("STORY_TTL_DEV"))
	base := os.Getenv("VOICE_API_BASE_URL")
	for _, endpoint := range []string{base, os.Getenv("VOICE_AUTH_MAIL_STUB_URL")} {
		u, err := url.Parse(endpoint)
		require.NoError(t, err)
		require.Equal(t, "http", u.Scheme)
		require.Equal(t, "127.0.0.1", u.Hostname())
		require.NotEmpty(t, u.Port())
	}

	client := &http.Client{Timeout: 30 * time.Second}
	sess := registerComposeUser(t, client, base, formatComposeEmail("story-ttl-delete", time.Now().UnixNano()), "VoiceQaTest1!")
	fileID := composeUploadUnscopedStoryImage(t, client, base, sess.AccessToken)
	composeWaitForProcessedImage(t, client, base, sess.AccessToken, fileID)
	require.Equal(t, http.StatusOK, composeGetFileMetadataStatus(t, client, base, sess.AccessToken, fileID))
	story := composeCreateStory(t, client, base, sess.AccessToken, map[string]any{
		"type": "photo", "media_file_id": fileID, "visibility": "everyone",
	})
	storyID := composeStoryID(t, story)
	require.NotEmpty(t, storyID)
	postgres := storyTTLComposeContainer(t, project, "postgres")
	storyContainer := storyTTLComposeContainer(t, project, "story")
	storyTTLRequireOwnedContainer(t, postgres, project, "postgres", startedNano)
	storyTTLRequireOwnedContainer(t, storyContainer, project, "story", startedNano)
	t.Logf("private fixture project=%s postgres=%s story=%s story_id=%s file_id=%s", project, postgres, storyContainer, storyID, fileID)
	require.Equal(t, "1", storyTTLComposeSQL(t, postgres,
		"SELECT count(*) FROM stories WHERE id='"+storyID+"'::uuid AND media_file_id='"+fileID+"'::uuid AND expired_at IS NULL AND archived_until > now();"))

	// The expiry marker, not merely feed filtering by expires_at, proves the worker ran.
	require.Eventually(t, func() bool {
		return storyTTLComposeSQL(t, postgres,
			"SELECT count(*) FROM stories WHERE id='"+storyID+"'::uuid AND expired_at IS NOT NULL AND archived_until > now();") == "1"
	}, 2*time.Minute, 2*time.Second)
	require.Contains(t, composeGetStoryArchive(t, client, base, sess.AccessToken), storyID)
	require.Equal(t, http.StatusOK, composeGetFileMetadataStatus(t, client, base, sess.AccessToken, fileID))

	// Only this test row crosses the retention boundary. Restarting the owned
	// Story container causes its actual startup purge/dispatch without waiting
	// for the production 24-hour purge ticker.
	require.Equal(t, "1", storyTTLComposeSQL(t, postgres,
		"WITH changed AS (UPDATE stories SET archived_until=now()-interval '1 second' WHERE id='"+storyID+"'::uuid AND media_file_id='"+fileID+"'::uuid AND expired_at IS NOT NULL AND archived_until > now() RETURNING id) SELECT count(*) FROM changed;"))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, exec.CommandContext(ctx, "docker", "restart", storyContainer).Run())

	require.Eventually(t, func() bool {
		return storyTTLComposeSQL(t, postgres,
			"SELECT count(*) FROM stories WHERE id='"+storyID+"'::uuid;") == "0"
	}, 90*time.Second, 2*time.Second, "Story startup purge must remove the archived row")
	require.Eventually(t, func() bool {
		return composeGetFileMetadataStatus(t, client, base, sess.AccessToken, fileID) == http.StatusNotFound
	}, 2*time.Minute, 2*time.Second, "actual File DeleteFile must make uploaded media unavailable")
	require.Equal(t, "0", storyTTLComposeSQL(t, postgres,
		"SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id='"+storyID+"'::uuid AND media_file_id='"+fileID+"'::uuid;"),
		"Story must acknowledge its File deletion operation")
}

func storyTTLRequireOwnedContainer(t *testing.T, id, project, service string, startedNano int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "docker", "inspect", "--format",
		"{{index .Config.Labels \"com.docker.compose.project\"}}|{{index .Config.Labels \"com.docker.compose.service\"}}|{{.Created}}", id).Output()
	require.NoError(t, err)
	parts := strings.Split(strings.TrimSpace(string(raw)), "|")
	require.Len(t, parts, 3)
	require.Equal(t, project, parts[0])
	require.Equal(t, service, parts[1])
	created, err := time.Parse(time.RFC3339Nano, parts[2])
	require.NoError(t, err)
	require.GreaterOrEqual(t, created.UnixNano(), startedNano, "preexisting container is not owned by this proof")
}

func storyTTLComposeContainer(t *testing.T, project, service string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "docker", "ps", "--quiet",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "label=com.docker.compose.service="+service).Output()
	require.NoError(t, err)
	ids := strings.Fields(string(raw))
	require.Len(t, ids, 1)
	return ids[0]
}

func storyTTLComposeSQL(t *testing.T, postgres, query string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", postgres,
		"psql", "-U", "voice", "-d", "story_db", "-v", "ON_ERROR_STOP=1", "-Atq")
	cmd.Stdin = strings.NewReader(query)
	raw, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(raw))
}

func composeUploadUnscopedStoryImage(t *testing.T, client *http.Client, base, token string) string {
	t.Helper()
	content := restartProofPNG()
	payload, err := json.Marshal(map[string]any{
		"original_name": "story-ttl-proof.png", "mime_type": "image/png", "size_bytes": len(content),
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/files/upload", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var ticket struct {
		UploadResponse struct {
			FileID string `json:"file_id"`
			PutURL string `json:"presigned_put_url"`
		} `json:"upload_response"`
	}
	require.NoError(t, json.Unmarshal(body, &ticket))
	require.NotEmpty(t, ticket.UploadResponse.FileID)
	require.NotEmpty(t, ticket.UploadResponse.PutURL)
	put, err := http.NewRequest(http.MethodPut, ticket.UploadResponse.PutURL, bytes.NewReader(content))
	require.NoError(t, err)
	put.Header.Set("Content-Type", "image/png")
	putResp, err := client.Do(put)
	require.NoError(t, err)
	putResp.Body.Close()
	require.True(t, putResp.StatusCode >= 200 && putResp.StatusCode < 300)
	confirm, err := json.Marshal(map[string]string{"sha256_hash": sha256Hex(content)})
	require.NoError(t, err)
	confirmReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/files/"+ticket.UploadResponse.FileID+"/confirm", bytes.NewReader(confirm))
	require.NoError(t, err)
	confirmReq.Header.Set("Authorization", "Bearer "+token)
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmed, err := client.Do(confirmReq)
	require.NoError(t, err)
	confirmed.Body.Close()
	require.Equal(t, http.StatusOK, confirmed.StatusCode)
	return ticket.UploadResponse.FileID
}
