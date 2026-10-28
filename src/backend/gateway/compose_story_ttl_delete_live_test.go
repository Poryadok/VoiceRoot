package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestComposeStoryTTLDeleteFile_live is intentionally limited to a disposable
// Compose project. The fixture advances only its own archived_until after the
// real expiry worker has archived the story; product retention stays 30 days.
func TestComposeStoryTTLDeleteFile_live(t *testing.T) {
	if !liveComposeEnabled() || os.Getenv("VOICE_STORY_TTL_DELETE_LIVE_TEST") != "true" {
		t.Skip("requires private Story TTL Compose proof opt-in")
	}
	storyTTLRequireTestBinary(t)
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
	fileID := composeUploadUnscopedStoryImage(t, composeLiveObjectClient(30*time.Second), base, sess.AccessToken)
	composeWaitForProcessedImage(t, client, base, sess.AccessToken, fileID)
	require.Equal(t, http.StatusOK, composeGetFileMetadataStatus(t, client, base, sess.AccessToken, fileID))
	t.Logf("private File fixture project=%s file_id=%s", project, fileID)
	story := composeCreateStory(t, client, base, sess.AccessToken, map[string]any{
		"type": "photo", "media_file_id": fileID, "visibility": "everyone",
	})
	storyID := composeStoryID(t, story)
	require.NoError(t, uuid.Validate(storyID))
	require.NoError(t, uuid.Validate(fileID))
	postgres := storyTTLComposeContainer(t, project, "postgres")
	storyContainer := storyTTLComposeContainer(t, project, "story")
	fileContainer := storyTTLComposeContainer(t, project, "file")
	gatewayContainer := storyTTLComposeContainer(t, project, "gateway")
	minioContainer := storyTTLComposeContainer(t, project, "minio")
	storyTTLRequireOwnedContainer(t, postgres, project, "postgres", startedNano)
	storyTTLRequireOwnedContainer(t, storyContainer, project, "story", startedNano)
	storyTTLRequireOwnedContainer(t, fileContainer, project, "file", startedNano)
	storyTTLRequireOwnedContainer(t, gatewayContainer, project, "gateway", startedNano)
	storyTTLRequireOwnedContainer(t, minioContainer, project, "minio", startedNano)
	for service, container := range map[string]string{"story": storyContainer, "file": fileContainer, "gateway": gatewayContainer} {
		storyTTLRequireImage(t, container, os.Getenv("VOICE_STORY_TTL_DELETE_"+strings.ToUpper(service)+"_IMAGE_ID"))
	}
	t.Logf("private fixture project=%s source=%s test_sha=%s postgres=%s story=%s file=%s gateway=%s story_id=%s file_id=%s",
		project, os.Getenv("VOICE_STORY_TTL_DELETE_SOURCE_HEAD"), os.Getenv("VOICE_STORY_TTL_DELETE_TEST_SHA256"),
		postgres, storyContainer, fileContainer, gatewayContainer, storyID, fileID)
	require.Equal(t, "1", storyTTLComposeSQL(t, postgres,
		"SELECT count(*) FROM stories WHERE id='"+storyID+"'::uuid AND media_file_id='"+fileID+"'::uuid AND expired_at IS NULL AND archived_until > now();"))
	var blob struct {
		ID        string `json:"id"`
		Original  string `json:"original"`
		Converted string `json:"converted"`
		Thumbnail string `json:"thumbnail"`
	}
	raw := storyTTLDatabaseSQL(t, postgres, "file_db", "SELECT json_build_object('id', b.blob_id, 'original', b.original_r2_key, 'converted', b.converted_r2_key, 'thumbnail', b.thumbnail_r2_key) FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id='"+fileID+"'::uuid AND b.state='LIVE';")
	require.NoError(t, json.Unmarshal([]byte(raw), &blob))
	require.NoError(t, uuid.Validate(blob.ID))
	keys := []string{blob.Original, blob.Converted, blob.Thumbnail}
	for _, key := range keys {
		require.NotEmpty(t, key)
	}
	require.Len(t, map[string]bool{keys[0]: true, keys[1]: true, keys[2]: true}, 3)
	require.True(t, storyTTLObjectsMatch(t, project, proofID, minioContainer, keys, true), "all three captured objects must exist before purge")

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
		return composeGetFileMetadataStatus(t, client, base, sess.AccessToken, fileID) == http.StatusForbidden
	}, 2*time.Minute, 2*time.Second, "released reference must deny legacy metadata access")
	require.Equal(t, "0", storyTTLComposeSQL(t, postgres,
		"SELECT count(*) FROM story_media_deletion_outbox WHERE operation_id='"+storyID+"'::uuid AND media_file_id='"+fileID+"'::uuid;"),
		"Story must acknowledge its File deletion operation")
	require.Eventually(t, func() bool {
		return storyTTLDatabaseSQL(t, postgres, "file_db", "SELECT state FROM file_blobs WHERE blob_id='"+blob.ID+"'::uuid;") == "GC_COMPLETE"
	}, 2*time.Minute, 2*time.Second, "File GC must complete")
	require.True(t, storyTTLObjectsMatch(t, project, proofID, minioContainer, keys, false), "all captured objects must be absent after GC")
}

func storyTTLRequireTestBinary(t *testing.T) {
	t.Helper()
	expected := os.Getenv("VOICE_STORY_TTL_DELETE_TEST_SHA256")
	if expected == "" {
		return // The Bash runner compiles the test through the local Go toolchain.
	}
	require.Len(t, expected, 64, "bound test executable SHA-256 required")
	path, err := os.Executable()
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	actual := sha256.Sum256(content)
	require.Equal(t, expected, hex.EncodeToString(actual[:]), "test executable changed after preparation")
}

func storyTTLRequireImage(t *testing.T, container, expected string) {
	t.Helper()
	require.True(t, strings.HasPrefix(expected, "sha256:"), "bound image ID required")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Image}}", container).Output()
	require.NoError(t, err)
	require.Equal(t, expected, strings.TrimSpace(string(raw)), "container image changed before private fixture mutation")
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
	raw, err := exec.CommandContext(ctx, "docker", "ps", "--quiet", "--no-trunc",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "label=com.docker.compose.service="+service).Output()
	require.NoError(t, err)
	ids := strings.Fields(string(raw))
	require.Len(t, ids, 1)
	require.Len(t, ids[0], 64)
	return ids[0]
}

func storyTTLComposeSQL(t *testing.T, postgres, query string) string {
	t.Helper()
	return storyTTLDatabaseSQL(t, postgres, "story_db", query)
}

func storyTTLDatabaseSQL(t *testing.T, postgres, database, query string) string {
	t.Helper()
	require.Contains(t, []string{"story_db", "file_db"}, database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", postgres,
		"psql", "-U", "voice", "-d", database, "-v", "ON_ERROR_STOP=1", "-Atq")
	cmd.Stdin = strings.NewReader(query)
	raw, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(raw))
}

// A successful authenticated recursive bucket listing distinguishes absence from
// a missing bucket, access denial or transport error. It never deletes objects.
func storyTTLObjectsMatch(t *testing.T, project, proofID, minioID string, keys []string, present bool) bool {
	t.Helper()
	networkID := storyTTLNetworkID(t, project, minioID)
	name := project + "-object-check-" + uuid.NewString()
	t.Logf("private object-check container name=%s", name)
	containerID := ""
	defer func() {
		// The unique name and matching label guard cleanup even if create timed out.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ids, err := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "name=^/"+name+"$").Output()
		require.NoError(t, err, "cannot reconcile private object-check container")
		matches := strings.Fields(string(ids))
		if len(matches) == 0 {
			require.Empty(t, containerID, "created object-check container disappeared")
			return
		}
		require.Len(t, matches, 1)
		if containerID != "" {
			require.True(t, strings.HasPrefix(containerID, matches[0]))
		}
		raw, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{index .Config.Labels \"voice.story-ttl-proof\"}}", matches[0]).Output()
		require.NoError(t, err)
		require.Equal(t, proofID, strings.TrimSpace(string(raw)))
		require.NoError(t, exec.CommandContext(ctx, "docker", "rm", "-f", matches[0]).Run(), "remove owned object-check container")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "create", "--name", name, "--label", "voice.story-ttl-proof="+proofID, "--network", networkID,
		"-e", "MC_HOST_proof=http://voice-minio:voice-minio-dev@minio:9000",
		"quay.io/minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727",
		"ls", "--json", "--recursive", "proof/voice-story-ttl-"+proofID)
	id, err := cmd.Output()
	require.NoError(t, err, "create private object-check container; timeout requires later daemon reconciliation for %s", name)
	containerID = strings.TrimSpace(string(id))
	require.Len(t, containerID, 64)
	raw, err := exec.CommandContext(ctx, "docker", "start", "-a", containerID).Output()
	require.NoError(t, err, "authenticated private bucket listing failed")
	require.Equal(t, networkID, storyTTLNetworkID(t, project, minioID))
	ok, err := storyTTLInventoryMatches(bytes.NewReader(raw), keys, present)
	require.NoError(t, err)
	return ok
}

func storyTTLNetworkID(t *testing.T, project, minioID string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "docker", "network", "inspect", project+"_default").Output()
	require.NoError(t, err)
	var networks []struct {
		ID         string                     `json:"Id"`
		Labels     map[string]string          `json:"Labels"`
		Containers map[string]json.RawMessage `json:"Containers"`
	}
	require.NoError(t, json.Unmarshal(raw, &networks))
	require.Len(t, networks, 1)
	network := networks[0]
	require.Equal(t, project, network.Labels["com.docker.compose.project"])
	require.NotEmpty(t, network.ID)
	require.Contains(t, network.Containers, minioID)
	aliases := 0
	for id := range network.Containers {
		raw, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .NetworkSettings.Networks}}", id).Output()
		require.NoError(t, err)
		var attached map[string]struct {
			NetworkID string   `json:"NetworkID"`
			Aliases   []string `json:"Aliases"`
		}
		require.NoError(t, json.Unmarshal(raw, &attached))
		entry, ok := attached[project+"_default"]
		require.True(t, ok)
		require.Equal(t, network.ID, entry.NetworkID)
		for _, alias := range entry.Aliases {
			if alias == "minio" {
				aliases++
				require.Equal(t, minioID, id)
			}
		}
	}
	require.Equal(t, 1, aliases, "MinIO alias must resolve only to owned container")
	return network.ID
}

func storyTTLInventoryMatches(input io.Reader, keys []string, present bool) (bool, error) {
	found := make(map[string]bool)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var row struct {
			Status string `json:"status"`
			Key    string `json:"key"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return false, err
		}
		if row.Status != "success" || row.Key == "" {
			return false, fmt.Errorf("invalid object listing")
		}
		found[row.Key] = true
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	for _, key := range keys {
		if key == "" || found[key] != present {
			return false, nil
		}
	}
	return true, nil
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
