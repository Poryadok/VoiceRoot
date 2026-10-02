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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Admission and the seven-day public deadline use production code. Only this
// newly created aggregate's mutable deadline is advanced by the fixture; the
// original schedule event and all admitted request/receipt bytes stay intact.
// This proves the real expiry worker and purge barrier, not a seven-day wait.
func TestComposeSpaceLifecycleExpiredPurge_live(t *testing.T) {
	if !liveComposeEnabled() || os.Getenv("VOICE_RUN_SPACE_LIFECYCLE_COMPOSE") != "true" {
		t.Skip("set both live Compose and Space lifecycle opt-ins")
	}
	project := os.Getenv("COMPOSE_PROJECT_NAME")
	require.True(t, strings.HasPrefix(project, "voice-game-lifecycle-"), "explicit owned lifecycle project required")
	base := os.Getenv("VOICE_API_BASE_URL")
	for _, endpoint := range []string{base, os.Getenv("VOICE_AUTH_MAIL_STUB_URL")} {
		u, err := url.Parse(endpoint)
		require.NoError(t, err)
		require.Equal(t, "http", u.Scheme)
		require.Equal(t, "127.0.0.1", u.Hostname())
		require.NotEmpty(t, u.Port())
		require.Nil(t, u.User)
	}
	client := composeLiveObjectClient(90 * time.Second)
	const password = "VoiceQaTest1!"
	owner := registerComposeUser(t, client, base, formatComposeEmail("lifecycle-purge-owner", time.Now().UnixNano()), password)
	const name = "Lifecycle expired purge QA"
	spaceID := createComposeSpace(t, client, base, owner.AccessToken, name, "purge fixture")
	chatID, _ := lifecycleComposePopulatedChat(t, client, base, owner.AccessToken, spaceID)
	controlSpace := createComposeSpace(t, client, base, owner.AccessToken, "Lifecycle purge control", "must survive")
	controlChat, controlMessage := lifecycleComposePopulatedChat(t, client, base, owner.AccessToken, controlSpace)
	// These are public uploads, actual object PUTs and production Messaging
	// reference acquisition. A foreign Space must retain both its reference and
	// readable object when the target's saved nonempty manifest is purged.
	targetContent := []byte("exclusive lifecycle attachment " + uuid.NewString())
	controlContent := []byte("control lifecycle attachment " + uuid.NewString())
	targetFile, _ := lifecycleComposeAttachment(t, client, base, owner.AccessToken, chatID, targetContent)
	controlFile, _ := lifecycleComposeAttachment(t, client, base, owner.AccessToken, controlChat, controlContent)
	lifecycleComposeDownload(t, client, base, owner.AccessToken, targetFile, targetContent)
	lifecycleComposeDownload(t, client, base, owner.AccessToken, controlFile, controlContent)
	postgres := lifecycleComposePostgres(t, project)
	targetReference := "SELECT count(*) FROM file_references WHERE file_id='" + targetFile + "'::uuid AND scope_space_id='" + spaceID + "'::uuid AND released_at IS NULL;"
	controlReference := "SELECT count(*) FROM file_references WHERE file_id='" + controlFile + "'::uuid AND scope_space_id='" + controlSpace + "'::uuid AND released_at IS NULL;"
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "file_db", targetReference), "public send must acquire a Space-scoped reference")
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "file_db", controlReference))
	sharedContent := []byte("shared lifecycle attachment " + uuid.NewString())
	sharedFile, sharedMessage := lifecycleComposeAttachment(t, client, base, owner.AccessToken, chatID, sharedContent)
	forwardStatus, _ := forwardComposeMessageStatus(t, client, base, owner.AccessToken, sharedMessage, controlChat, "")
	require.Equal(t, http.StatusOK, forwardStatus, "public forward must acquire the same blob under a new exact owner tuple")
	sharedRefs := "SELECT count(*) FROM file_references WHERE file_id='" + sharedFile + "'::uuid AND released_at IS NULL;"
	require.Equal(t, "2", lifecycleComposeDatabaseSQL(t, postgres, "file_db", sharedRefs))
	operation := uuid.NewString()
	path := "/api/v1/spaces/" + spaceID
	status, body := lifecycleComposeRequest(t, client, base, owner.AccessToken, http.MethodPost, "/api/v1/auth/space-deletion-proof", map[string]string{
		"space_id": spaceID, "confirmation_name": name, "operation_id": operation, "password": password,
	})
	require.Equal(t, http.StatusOK, status)
	var proof struct {
		Proof string `json:"proof"`
	}
	require.NoError(t, json.Unmarshal(body, &proof))
	require.NotEmpty(t, proof.Proof)
	lifecycleComposeRetry(t, client, base, owner.AccessToken, http.MethodDelete, path, map[string]string{
		"confirmation_name": name, "operation_id": operation, "proof": proof.Proof,
	}, http.StatusNoContent)
	status, body = lifecycleComposeRequest(t, client, base, owner.AccessToken, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, status)
	var frozen struct {
		Space map[string]any `json:"space"`
	}
	require.NoError(t, json.Unmarshal(body, &frozen))
	scheduled, err := time.Parse(time.RFC3339Nano, frozen.Space["deletion_scheduled_at"].(string))
	require.NoError(t, err)
	purgeAfter, err := time.Parse(time.RFC3339Nano, frozen.Space["purge_after"].(string))
	require.NoError(t, err)
	require.Equal(t, 7*24*time.Hour, purgeAfter.Sub(scheduled))
	fileStatus, _ := composeGetFileURL(t, client, base, owner.AccessToken, targetFile)
	require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound, http.StatusGone, http.StatusPreconditionFailed}, fileStatus, "frozen target cannot refresh its download")
	lifecycleComposeDownload(t, client, base, owner.AccessToken, controlFile, controlContent)

	spaceUUID, err := uuid.Parse(spaceID)
	require.NoError(t, err)
	require.Equal(t, spaceUUID.String(), spaceID)
	// Both values come from one database instant. The exact operation and phase
	// guard prevents touching any old, foreign or already progressing fixture.
	expired := lifecycleComposeSQL(t, postgres, "WITH deadline AS (SELECT clock_timestamp() AS now), updated AS ("+
		"UPDATE space_lifecycle_aggregates SET scheduled_at=deadline.now-interval '8 days', purge_after=deadline.now-interval '1 day' FROM deadline "+
		"WHERE space_id='"+spaceID+"'::uuid AND deletion_operation_id='"+operation+"'::uuid AND phase='SCHEDULED' AND generation=1 RETURNING space_id) SELECT count(*) FROM updated;")
	require.Equal(t, "1", expired, "exact newly scheduled fixture must be advanced once")

	var state struct {
		Phase           string `json:"phase"`
		Generation      int    `json:"generation"`
		FenceReceipts   int    `json:"fence_receipts"`
		PurgeReceipts   int    `json:"purge_receipts"`
		SpaceExists     bool   `json:"space_exists"`
		TombstoneExists bool   `json:"tombstone_exists"`
	}
	query := "SELECT json_build_object('phase',a.phase,'generation',a.generation," +
		"'fence_receipts',(SELECT count(*) FROM space_lifecycle_participants p WHERE p.space_id=a.space_id AND p.generation=2 AND p.request_kind='FENCE' AND p.completed_at IS NOT NULL)," +
		"'purge_receipts',(SELECT count(*) FROM space_lifecycle_participants p WHERE p.space_id=a.space_id AND p.generation=2 AND p.request_kind IN ('PURGE','ROLE_RETIREMENT') AND p.completed_at IS NOT NULL)," +
		"'space_exists',EXISTS(SELECT 1 FROM spaces s WHERE s.id=a.space_id),'tombstone_exists',EXISTS(SELECT 1 FROM space_deletion_tombstones t WHERE t.space_id=a.space_id)) " +
		"FROM space_lifecycle_aggregates a WHERE a.space_id='" + spaceID + "'::uuid AND a.deletion_operation_id='" + operation + "'::uuid;"
	deadline := time.Now().Add(2 * time.Minute)
	for {
		raw := lifecycleComposeSQL(t, postgres, query)
		require.NoError(t, json.Unmarshal([]byte(raw), &state))
		if state.Phase == "PURGED" || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	t.Logf("fixture space=%s operation=%s phase=%s generation=%d fence_receipts=%d purge_receipts=%d", spaceID, operation, state.Phase, state.Generation, state.FenceReceipts, state.PurgeReceipts)
	require.Equal(t, "PURGED", state.Phase, "production worker must finish the complete purge barrier")
	require.Equal(t, 2, state.Generation)
	require.Equal(t, 10, state.FenceReceipts)
	require.Equal(t, 10, state.PurgeReceipts)
	require.False(t, state.SpaceExists, "Space row must be physically removed")
	require.True(t, state.TombstoneExists, "minimal terminal evidence must remain")
	require.Equal(t, "0", lifecycleComposeDatabaseSQL(t, postgres, "chat_db", "SELECT count(*) FROM chats WHERE id='"+chatID+"'::uuid;"), "saved nonempty Chat manifest must be purged")
	require.Equal(t, "0", lifecycleComposeDatabaseSQL(t, postgres, "messaging_db", "SELECT count(*) FROM messages WHERE chat_id='"+chatID+"'::uuid;"), "all target message content must be removed")
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "chat_db", "SELECT count(*) FROM chats WHERE id='"+controlChat+"'::uuid;"), "another Space's Chat must survive")
	require.Equal(t, "0", lifecycleComposeDatabaseSQL(t, postgres, "file_db", targetReference), "target references must be durably released")
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "file_db", controlReference), "foreign reference must remain live")
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "file_db", "SELECT count(*) FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id='"+targetFile+"'::uuid AND b.gc_operation_id IS NOT NULL AND b.state IN ('GC_PENDING','GC_COMPLETE');"), "unreferenced target blob must have durable GC handoff")
	fileStatus, _ = composeGetFileURL(t, client, base, owner.AccessToken, targetFile)
	require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound, http.StatusGone}, fileStatus, "purged target cannot mint a download")
	lifecycleComposeDownload(t, client, base, owner.AccessToken, controlFile, controlContent)
	getComposeMessagesContains(t, client, base, owner.AccessToken, controlChat, controlMessage, "lifecycle message must retain its identity")
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "file_db", sharedRefs), "only the foreign Space's forwarded reference survives")
	require.Equal(t, "1", lifecycleComposeDatabaseSQL(t, postgres, "file_db", "SELECT count(*) FROM file_references WHERE file_id='"+sharedFile+"'::uuid AND scope_space_id='"+controlSpace+"'::uuid AND released_at IS NULL;"))
	require.Equal(t, "LIVE", lifecycleComposeDatabaseSQL(t, postgres, "file_db", "SELECT b.state FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id='"+sharedFile+"'::uuid;"), "shared binary must not enter GC")
	lifecycleComposeDownload(t, client, base, owner.AccessToken, sharedFile, sharedContent)
	status, _ = lifecycleComposeRequest(t, client, base, owner.AccessToken, http.MethodGet, path, nil)
	require.Equal(t, http.StatusNotFound, status, "purged Space cannot expose a frozen projection")
	status, _ = lifecycleComposeRequest(t, client, base, owner.AccessToken, http.MethodPost, path+"/restore", map[string]string{"operation_id": uuid.NewString()})
	require.Equal(t, http.StatusNotFound, status, "purged Space must not be restored")
}

func lifecycleComposeAttachment(t *testing.T, client *http.Client, base, bearer, chatID string, content []byte) (string, string) {
	t.Helper()
	fileID, fileType := composeUploadTextFileInChat(t, client, base, bearer, chatID, "CHAT_TYPE_GROUP", "lifecycle.txt", content)
	parsed, err := uuid.Parse(fileID)
	require.NoError(t, err)
	require.Equal(t, parsed.String(), fileID)
	attachments, err := json.Marshal([]map[string]string{{"file_id": fileID, "type": fileType}})
	require.NoError(t, err)
	messageID := sendComposeMessageWithAttachmentsJSON(t, client, base, bearer, chatID, string(attachments))
	return fileID, messageID
}

func lifecycleComposeDownload(t *testing.T, client *http.Client, base, bearer, fileID string, expected []byte) {
	t.Helper()
	status, downloadURL := composeGetFileURL(t, client, base, bearer, fileID)
	require.Equal(t, http.StatusOK, status, "live reference must mint a download")
	require.NotEmpty(t, downloadURL)
	response, err := client.Get(downloadURL)
	require.True(t, err == nil, "object download failed; signed URL withheld")
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	content, err := io.ReadAll(io.LimitReader(response.Body, int64(len(expected)+1)))
	require.NoError(t, err)
	require.Equal(t, expected, content, "actual uploaded object bytes must survive")
}

func lifecycleComposePostgres(t *testing.T, project string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "rtk", "proxy", "docker", "ps", "--quiet",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "label=com.docker.compose.service=postgres").Output()
	require.NoError(t, err)
	ids := strings.Fields(string(raw))
	require.Len(t, ids, 1, "owned project must contain exactly one PostgreSQL container")
	return ids[0]
}

func lifecycleComposeSQL(t *testing.T, postgres, query string) string {
	t.Helper()
	return lifecycleComposeDatabaseSQL(t, postgres, "space_db", query)
}

func lifecycleComposeDatabaseSQL(t *testing.T, postgres, database, query string) string {
	t.Helper()
	require.Contains(t, []string{"space_db", "chat_db", "messaging_db", "file_db"}, database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "rtk", "proxy", "docker", "exec", "-i", postgres,
		"psql", "-U", "voice", "-d", database, "-v", "ON_ERROR_STOP=1", "-Atq")
	cmd.Stdin = strings.NewReader(query)
	raw, err := cmd.Output()
	require.NoError(t, err, "owned fixture SQL; response bodies and credentials withheld")
	return strings.TrimSpace(string(raw))
}

// This test mutates disposable accounts and Spaces only in an explicitly owned,
// loopback lifecycle stack. It must never use the default Compose/A1 project.
func TestComposeSpaceLifecycleFreezeRestore_live(t *testing.T) {
	if !liveComposeEnabled() || os.Getenv("VOICE_RUN_SPACE_LIFECYCLE_COMPOSE") != "true" {
		t.Skip("set both live Compose and Space lifecycle opt-ins")
	}
	project := os.Getenv("COMPOSE_PROJECT_NAME")
	require.True(t, strings.HasPrefix(project, "voice-game-lifecycle-"), "explicit owned lifecycle project required")
	base := os.Getenv("VOICE_API_BASE_URL")
	for _, endpoint := range []string{base, os.Getenv("VOICE_AUTH_MAIL_STUB_URL")} {
		u, err := url.Parse(endpoint)
		require.NoError(t, err)
		require.Equal(t, "http", u.Scheme)
		require.Equal(t, "127.0.0.1", u.Hostname())
		require.NotEmpty(t, u.Port())
		require.Nil(t, u.User)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"
	owner := registerComposeUser(t, client, base, formatComposeEmail("lifecycle-owner", n), password)
	member := registerComposeUser(t, client, base, formatComposeEmail("lifecycle-member", n), password)
	outsider := registerComposeUser(t, client, base, formatComposeEmail("lifecycle-outsider", n), password)
	const name = "Lifecycle recovery QA"
	spaceID := createComposeSpace(t, client, base, owner.AccessToken, name, "private full-view field")
	allowComposeChatSpaceInvitesEveryone(t, client, base, member.AccessToken)
	invite := createComposeSpaceInvite(t, client, base, owner.AccessToken, spaceID)
	joinComposeSpaceByInvite(t, client, base, member.AccessToken, invite.Code)
	chatID, messageID := lifecycleComposePopulatedChat(t, client, base, owner.AccessToken, spaceID)
	path := "/api/v1/spaces/" + spaceID
	ordinaryReads := []string{
		"/api/v1/chats/" + chatID,
		"/api/v1/messages?chat_id=" + chatID,
		"/api/v1/search/in-chat?chat_id=" + chatID + "&q=lifecycle",
	}
	for _, actor := range []authSessionResponse{owner, member} {
		for _, read := range ordinaryReads {
			status, _ := lifecycleComposeOrdinaryRequest(t, client, base, actor.AccessToken, http.MethodGet, read)
			require.Equal(t, http.StatusOK, status, "live ordinary read route=%s", read)
		}
	}
	for _, actor := range []authSessionResponse{owner, member} {
		status, body := lifecycleComposeRequest(t, client, base, actor.AccessToken, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, status)
		var parsed struct {
			Space map[string]any `json:"space"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))
		require.Equal(t, "private full-view field", parsed.Space["description"])
	}
	status, _ := lifecycleComposeRequest(t, client, base, outsider.AccessToken, http.MethodGet, path, nil)
	require.Equal(t, http.StatusNotFound, status)
	operation := uuid.NewString()
	status, body := lifecycleComposeRequest(t, client, base, owner.AccessToken, http.MethodPost, "/api/v1/auth/space-deletion-proof", map[string]string{
		"space_id": spaceID, "confirmation_name": name, "operation_id": operation, "password": password,
	})
	require.Equal(t, http.StatusOK, status, "Auth proof issue status")
	var proof struct {
		Proof string `json:"proof"`
	}
	require.NoError(t, json.Unmarshal(body, &proof))
	require.True(t, proof.Proof != "", "Auth must issue opaque proof")
	intent := map[string]string{"confirmation_name": name, "operation_id": operation, "proof": proof.Proof}
	lifecycleComposeRetry(t, client, base, owner.AccessToken, http.MethodDelete, path, intent, http.StatusNoContent)
	// A fresh signed transport token must replay the exact durable business result.
	lifecycleComposeRetry(t, client, base, owner.AccessToken, http.MethodDelete, path, intent, http.StatusNoContent)
	status, body = lifecycleComposeRequest(t, client, base, owner.AccessToken, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, status)
	var frozen struct {
		Space map[string]any `json:"space"`
	}
	require.NoError(t, json.Unmarshal(body, &frozen))
	require.Len(t, frozen.Space, 4, "frozen owner read must contain only the documented projection")
	require.Equal(t, spaceID, frozen.Space["id"])
	require.Equal(t, name, frozen.Space["name"])
	scheduled, err := time.Parse(time.RFC3339Nano, frozen.Space["deletion_scheduled_at"].(string))
	require.NoError(t, err)
	purgeAfter, err := time.Parse(time.RFC3339Nano, frozen.Space["purge_after"].(string))
	require.NoError(t, err)
	require.Equal(t, 7*24*time.Hour, purgeAfter.Sub(scheduled))
	for _, actor := range []authSessionResponse{member, outsider} {
		status, _ = lifecycleComposeRequest(t, client, base, actor.AccessToken, http.MethodGet, path, nil)
		require.Equal(t, http.StatusNotFound, status, "frozen Space must be hidden from nonowner")
	}
	// The minimal owner Space projection never grants access to its content.
	// Exercise actual ordinary owner/member transports while all ten fences are
	// durably acknowledged, and require a policy denial rather than a server error.
	for _, actor := range []authSessionResponse{owner, member} {
		for _, read := range ordinaryReads {
			status, body = lifecycleComposeOrdinaryRequest(t, client, base, actor.AccessToken, http.MethodGet, read)
			require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound, http.StatusPreconditionFailed}, status, "frozen ordinary read route=%s", read)
			require.NotContains(t, string(body), messageID, "frozen response must not expose message identity")
			require.NotContains(t, string(body), "lifecycle message must retain its identity", "frozen response must not expose content")
		}
		status = sendComposeMessageStatus(t, client, base, actor.AccessToken, chatID, "frozen write must not persist", "")
		require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound, http.StatusPreconditionFailed}, status, "frozen send must be denied")
	}
	restore := map[string]string{"operation_id": uuid.NewString()}
	lifecycleComposeRetry(t, client, base, owner.AccessToken, http.MethodPost, path+"/restore", restore, http.StatusOK)
	lifecycleComposeRetry(t, client, base, owner.AccessToken, http.MethodPost, path+"/restore", restore, http.StatusOK)
	for _, actor := range []authSessionResponse{owner, member} {
		status, body = lifecycleComposeRequest(t, client, base, actor.AccessToken, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, status)
		var restored struct {
			Space map[string]any `json:"space"`
		}
		require.NoError(t, json.Unmarshal(body, &restored))
		require.Equal(t, "private full-view field", restored.Space["description"])
		require.NotContains(t, restored.Space, "deletion_scheduled_at")
		require.NotContains(t, restored.Space, "purge_after")
	}
	getComposeMessagesContains(t, client, base, owner.AccessToken, chatID, messageID, "lifecycle message must retain its identity")
	getComposeMessagesContains(t, client, base, member.AccessToken, chatID, messageID, "lifecycle message must retain its identity")
	for _, read := range ordinaryReads {
		status, _ = lifecycleComposeOrdinaryRequest(t, client, base, member.AccessToken, http.MethodGet, read)
		require.Equal(t, http.StatusOK, status, "restore must reopen ordinary read route=%s", read)
	}
}

func lifecycleComposeOrdinaryRequest(t *testing.T, client *http.Client, base, bearer, method, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	require.NoError(t, err)
	return resp.StatusCode, body
}

func lifecycleComposePopulatedChat(t *testing.T, client *http.Client, base, bearer, spaceID string) (string, string) {
	t.Helper()
	nodeID := createComposeSpaceChat(t, client, base, bearer, spaceID, "Lifecycle chat")
	tree := getComposeSpaceTree(t, client, base, bearer, spaceID)
	var chatID string
	for _, node := range tree.Nodes {
		if node.ID == nodeID {
			chatID = node.LinkedChat.ID
		}
	}
	require.NotEmpty(t, chatID, "Space chat tree node must bind a concrete Chat")
	parsed, err := uuid.Parse(chatID)
	require.NoError(t, err)
	require.Equal(t, parsed.String(), chatID)
	messageID := sendComposeMessage(t, client, base, bearer, chatID, "lifecycle message must retain its identity")
	getComposeMessagesContains(t, client, base, bearer, chatID, messageID, "lifecycle message must retain its identity")
	return chatID, messageID
}

func lifecycleComposeRequest(t *testing.T, client *http.Client, base, bearer, method, path string, payload any) (int, []byte) {
	t.Helper()
	var raw []byte
	if payload != nil {
		var err error
		raw, err = json.Marshal(payload)
		require.NoError(t, err)
	}
	req, err := http.NewRequest(method, base+path, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+bearer)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	require.NoError(t, err)
	require.Contains(t, resp.Header.Get("Cache-Control"), "no-store")
	return resp.StatusCode, body
}

func lifecycleComposeRetry(t *testing.T, client *http.Client, base, bearer, method, path string, payload any, expected int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		code, _ := lifecycleComposeRequest(t, client, base, bearer, method, path, payload)
		if code == expected {
			return
		}
		if code != http.StatusServiceUnavailable || time.Now().After(deadline) {
			require.Equal(t, expected, code, "lifecycle operation status; credentials and response body withheld")
			return
		}
		time.Sleep(time.Second)
	}
}
