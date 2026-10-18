package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	lkmedia "github.com/livekit/server-sdk-go/v2/pkg/media"
	"github.com/pion/webrtc/v4"
	"google.golang.org/protobuf/encoding/protojson"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// TestMatchSquadRemoteMediaAndChat_live is a single-shot DEV acceptance harness.
// It is intentionally separate from the Compose live tests: it accepts no
// ordinary call token, does not clear server state, and never prints response
// bodies or credentials. Run only with the explicit DEV opt-in and private
// actor/session file described by the PR667 acceptance report.
func TestMatchSquadRemoteMediaAndChat_live(t *testing.T) {
	if os.Getenv("VOICE_RUN_MATCHSQUAD_REMOTE_PROOF") != "true" {
		t.Skip("set VOICE_RUN_MATCHSQUAD_REMOTE_PROOF=true for the bounded DEV acceptance run")
	}

	base := strings.TrimRight(strings.TrimSpace(os.Getenv("VOICE_API_BASE_URL")), "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Hostname() != "api-dev.comrade.click" || u.User != nil || u.RawQuery != "" || u.Path != "" {
		t.Fatal("remote MatchSquad proof requires the exact public DEV Gateway URL")
	}
	gameID := strings.TrimSpace(os.Getenv("VOICE_MATCHSQUAD_GAME_ID"))
	if gameID == "" {
		t.Fatal("VOICE_MATCHSQUAD_GAME_ID is required")
	}
	actorPath := strings.TrimSpace(os.Getenv("VOICE_MATCHSQUAD_ACTORS_FILE"))
	stateRoot := strings.TrimSpace(os.Getenv("VOICE_MATCHSQUAD_PRIVATE_STATE"))
	if actorPath == "" || stateRoot == "" {
		t.Fatal("private actor file and private state directory are required")
	}
	actorDir, err := validateRemoteProofPrivateDirectory(filepath.Dir(actorPath))
	if err != nil {
		t.Fatal("private actor directory is unavailable or invalid")
	}
	stateDir, err := validateRemoteProofPrivateDirectory(stateRoot)
	if err != nil || actorDir != stateDir {
		t.Fatal("actor file and proof state must share one canonical private directory")
	}
	actors, err := readRemoteProofActors(actorPath)
	if err != nil {
		t.Fatal("private actor session file is unavailable or invalid")
	}
	attemptDir, intentPath, ledgerPath, err := createRemoteProofIntent(stateRoot)
	if err != nil {
		t.Fatal("private intent could not be persisted")
	}
	_ = attemptDir
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	appendRemoteProofEvent(t, ledgerPath, "refresh_actor_a_intent")
	if err := appendRemoteProofOperation(intentPath, "refresh_actor_a", composeClientMessageID(), nil); err != nil {
		t.Fatal("actor A refresh intent could not be persisted")
	}
	actors.a = refreshRemoteProofActor(t, client, base, actors.a)
	persistRemoteProofActor(t, attemptDir, "actor-a-session.json", actors.a)
	if err := appendRemoteProofReceipt(intentPath, "refresh_actor_a", map[string]any{"profile_id": actors.a.ProfileID, "session_refreshed": true}); err != nil {
		t.Fatal("actor A refresh receipt could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, "refresh_actor_b_intent")
	if err := appendRemoteProofOperation(intentPath, "refresh_actor_b", composeClientMessageID(), nil); err != nil {
		t.Fatal("actor B refresh intent could not be persisted")
	}
	actors.b = refreshRemoteProofActor(t, client, base, actors.b)
	persistRemoteProofActor(t, attemptDir, "actor-b-session.json", actors.b)
	if err := appendRemoteProofReceipt(intentPath, "refresh_actor_b", map[string]any{"profile_id": actors.b.ProfileID, "session_refreshed": true}); err != nil {
		t.Fatal("actor B refresh receipt could not be persisted")
	}

	var games map[string]any
	remoteProofRequest(t, client, http.MethodGet, base+"/api/v1/matchmaking/games", actors.a.AccessToken, nil, http.StatusOK, &games, "list_matchmaking_games")
	if !remoteProofContainsGame(games, gameID) {
		t.Fatal("configured matchmaking game is not in the public DEV catalog")
	}

	criteria := `{"region":"eu"}`
	appendRemoteProofEvent(t, ledgerPath, "start_actor_a_search_intent")
	searchA := startRemoteProofSearch(t, client, base, actors.a.AccessToken, gameID, criteria, intentPath, ledgerPath, "actor_a_search")
	appendRemoteProofEvent(t, ledgerPath, "start_actor_b_search_intent")
	searchB := startRemoteProofSearch(t, client, base, actors.b.AccessToken, gameID, criteria, intentPath, ledgerPath, "actor_b_search")

	var matchID string
	for poll := 0; poll < 12; poll++ {
		statusBody := remoteProofRequest(t, client, http.MethodGet, base+"/api/v1/matchmaking/search/"+url.PathEscape(searchA), actors.a.AccessToken, nil, http.StatusOK, nil, "poll_actor_a_search")
		var parseErr error
		matchID, parseErr = remoteProofSearchMatchID(statusBody)
		if parseErr != nil {
			t.Fatal("actor A search status response was malformed; stop without retry")
		}
		if matchID != "" {
			break
		}
		if poll < 11 {
			time.Sleep(2 * time.Second)
		}
	}
	if matchID == "" {
		t.Fatal("matcher did not produce a match within the bounded polling window; stop without retry")
	}
	if err := appendRemoteProofReceipt(intentPath, "actor_a_match", map[string]any{"match_id": matchID}); err != nil {
		t.Fatal("actor A match receipt could not be persisted")
	}
	statusBodyB := remoteProofRequest(t, client, http.MethodGet, base+"/api/v1/matchmaking/search/"+url.PathEscape(searchB), actors.b.AccessToken, nil, http.StatusOK, nil, "confirm_actor_b_search")
	matchIDB, err := remoteProofSearchMatchID(statusBodyB)
	if err != nil {
		t.Fatal("actor B search status response was malformed; stop without retry")
	}
	if matchIDB != matchID {
		t.Fatal("actor search sessions did not converge on the same match")
	}
	if err := appendRemoteProofReceipt(intentPath, "actor_b_match", map[string]any{"match_id": matchID}); err != nil {
		t.Fatal("actor B match receipt could not be persisted")
	}

	var current map[string]any
	remoteProofMatchGet(t, client, base, actors.a.AccessToken, matchID, &current)
	if remoteProofString(current, "status") != "pending_accept" {
		t.Fatal("fresh match was not pending acceptance")
	}
	appendRemoteProofEvent(t, ledgerPath, "accept_actor_a_intent")
	remoteProofAccept(t, client, base, actors.a.AccessToken, matchID, intentPath, ledgerPath, "actor_a_accept")
	appendRemoteProofEvent(t, ledgerPath, "accept_actor_b_intent")
	remoteProofAccept(t, client, base, actors.b.AccessToken, matchID, intentPath, ledgerPath, "actor_b_accept")
	remoteProofMatchGet(t, client, base, actors.a.AccessToken, matchID, &current)
	if remoteProofString(current, "status") != "active" {
		t.Fatal("two accepted participants did not produce an active match")
	}
	chatID := remoteProofString(current, "chatId", "chat_id")
	roomID := remoteProofString(current, "voiceRoomId", "voice_room_id")
	if chatID == "" || roomID == "" || !remoteProofHasRoster(current, actors.a.ProfileID, actors.b.ProfileID) {
		t.Fatal("active match is missing its expected roster or provisioned Chat/Voice IDs")
	}

	joinA := remoteProofJoin(t, client, base, actors.a.AccessToken, matchID, roomID, intentPath, ledgerPath, "actor_a")
	joinB := remoteProofJoin(t, client, base, actors.b.AccessToken, matchID, roomID, intentPath, ledgerPath, "actor_b")
	tokenA := remoteProofToken(t, client, base, actors.a.AccessToken, matchID, roomID, joinA, ledgerPath, "actor_a")
	tokenB := remoteProofToken(t, client, base, actors.b.AccessToken, matchID, roomID, joinB, ledgerPath, "actor_b")
	if tokenA.jwt == "" || tokenB.jwt == "" || tokenA.url == "" || tokenB.url == "" {
		t.Fatal("MatchSquad token response omitted LiveKit connection fields")
	}
	livekitURL := resolveComposeLivekitURL(tokenA.url, tokenB.url)
	if livekitURL == "" {
		t.Fatal("MatchSquad token response did not provide a public LiveKit URL")
	}

	mediaCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	peerA, err := newRemoteProofLivekitPeer(mediaCtx, livekitURL, tokenA.jwt, actors.b.ProfileID)
	if err != nil {
		cancel()
		t.Fatal("MatchSquad actor A could not connect or publish synthetic audio")
	}
	peerB, err := newRemoteProofLivekitPeer(mediaCtx, livekitURL, tokenB.jwt, actors.a.ProfileID)
	if err != nil {
		peerA.close()
		cancel()
		t.Fatal("MatchSquad actor B could not connect or publish synthetic audio")
	}
	if peerA.waitRemoteAudioRTP(45*time.Second) != nil {
		peerA.close()
		peerB.close()
		cancel()
		t.Fatal("actor A did not receive the peer's LiveKit RTP audio")
	}
	if peerB.waitRemoteAudioRTP(45*time.Second) != nil {
		peerA.close()
		peerB.close()
		cancel()
		t.Fatal("actor B did not receive the peer's LiveKit RTP audio")
	}
	peerA.close()
	peerB.close()
	cancel()

	marker := "pr667-matchsquad-" + composeClientMessageID()
	appendRemoteProofEvent(t, ledgerPath, "send_match_chat_message_intent")
	messageID := remoteProofSendMessage(t, client, base, actors.a.AccessToken, chatID, marker, intentPath, ledgerPath)
	if messageID == "" {
		t.Fatal("Chat send did not return a message ID")
	}
	if !remoteProofReceiverHasMessage(t, client, base, actors.b.AccessToken, chatID, messageID, marker) {
		t.Fatal("recipient Chat history did not contain the exact sent message")
	}

	appendRemoteProofEvent(t, ledgerPath, "leave_actor_a_intent")
	remoteProofLeave(t, client, base, actors.a.AccessToken, matchID, roomID, joinA, intentPath, ledgerPath, "actor_a")
	appendRemoteProofEvent(t, ledgerPath, "leave_actor_b_intent")
	remoteProofLeave(t, client, base, actors.b.AccessToken, matchID, roomID, joinB, intentPath, ledgerPath, "actor_b")
	appendRemoteProofEvent(t, ledgerPath, "complete_actor_a_intent")
	current = remoteProofComplete(t, client, base, actors.a.AccessToken, matchID, ledgerPath, intentPath, "actor_a", "active", chatID, roomID, actors.a.ProfileID, actors.b.ProfileID)
	if !remoteProofMayCompleteNext(current, chatID, roomID, actors.a.ProfileID, actors.b.ProfileID) {
		t.Fatal("first completion did not leave the same match active; stop before actor B mutation")
	}
	appendRemoteProofEvent(t, ledgerPath, "complete_actor_b_intent")
	current = remoteProofComplete(t, client, base, actors.b.AccessToken, matchID, ledgerPath, intentPath, "actor_b", "completed", chatID, roomID, actors.a.ProfileID, actors.b.ProfileID)
	remoteProofMatchGet(t, client, base, actors.b.AccessToken, matchID, &current)
	if !remoteProofHasBoundMatch(current, "completed", chatID, roomID, actors.a.ProfileID, actors.b.ProfileID) {
		t.Fatal("public Match GET did not confirm completed aggregate with unchanged roster")
	}
	if err := appendRemoteProofReceipt(intentPath, "final_match", map[string]any{"status": "completed", "chat_id": chatID, "voice_room_id": roomID}); err != nil {
		t.Fatal("final public match receipt could not be persisted")
	}
	t.Log("MatchSquad remote proof passed: bidirectional RTP packets, exact recipient Chat history match, aggregate completed")
}

type remoteProofActor struct {
	Label        string `json:"label"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ProfileID    string `json:"profile_id"`
	AccountID    string `json:"account_id"`
	AccountType  string `json:"account_type"`
}

type remoteProofActors struct {
	Actors []remoteProofActor `json:"actors"`
	a      remoteProofActor
	b      remoteProofActor
}

func readRemoteProofActors(path string) (remoteProofActors, error) {
	parent, err := validateRemoteProofPrivateDirectory(filepath.Dir(path))
	if err != nil || filepath.Clean(path) != filepath.Join(parent, filepath.Base(path)) {
		return remoteProofActors{}, os.ErrPermission
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return remoteProofActors{}, os.ErrPermission
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return remoteProofActors{}, err
	}
	var file remoteProofActors
	if err := json.Unmarshal(raw, &file); err != nil {
		return remoteProofActors{}, err
	}
	for _, actor := range file.Actors {
		switch actor.Label {
		case "A":
			file.a = actor
		case "B":
			file.b = actor
		}
	}
	if file.a.AccessToken == "" || file.a.RefreshToken == "" || file.a.ProfileID == "" || file.a.AccountType != "regular" ||
		file.b.AccessToken == "" || file.b.RefreshToken == "" || file.b.ProfileID == "" || file.b.AccountType != "regular" || file.a.ProfileID == file.b.ProfileID {
		return remoteProofActors{}, os.ErrInvalid
	}
	return file, nil
}

func createRemoteProofIntent(root string) (string, string, string, error) {
	root, err := validateRemoteProofPrivateDirectory(root)
	if err != nil {
		return "", "", "", err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", "", "", err
	}
	dir := filepath.Join(root, "attempt-"+hex.EncodeToString(nonce[:]))
	guard := map[string]string{"schema": "voice-pr667-matchsquad-remote-proof-once-v1", "attempt_dir": filepath.Base(dir)}
	guardRaw, err := json.Marshal(guard)
	if err != nil || writeRemoteProofPrivate(filepath.Join(root, "remote-proof.once"), guardRaw, false) != nil {
		return "", "", "", os.ErrExist
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", "", "", err
	}
	if err := syncRemoteProofDirectory(root); err != nil {
		return "", "", "", err
	}
	intentPath := filepath.Join(dir, "intent.json")
	ledgerPath := filepath.Join(dir, "events.jsonl")
	intent := map[string]any{
		"schema":         "voice-pr667-matchsquad-remote-proof-intent-v1",
		"attempt_dir":    filepath.Base(dir),
		"source":         "public DEV Gateway",
		"actors":         []string{"A", "B"},
		"planned":        []string{"refresh", "search", "accept", "join", "token", "bidirectional-rtp", "chat-send-and-recipient-read", "leave", "complete", "final-read"},
		"writes":         []string{"search-start x2", "accept x2", "join x2", "chat-send x1", "leave x2", "complete x2"},
		"operation_ids":  map[string]any{},
		"request_sha256": map[string]any{},
		"receipts":       map[string]any{},
	}
	raw, err := json.Marshal(intent)
	if err != nil {
		return "", "", "", err
	}
	if err := writeRemoteProofPrivate(intentPath, raw, false); err != nil {
		return "", "", "", err
	}
	if err := writeRemoteProofPrivate(ledgerPath, nil, false); err != nil {
		return "", "", "", err
	}
	return dir, intentPath, ledgerPath, nil
}

func validateRemoteProofPrivateDirectory(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", os.ErrInvalid
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil || realPath != path {
		return "", os.ErrPermission
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return "", os.ErrPermission
	}
	return realPath, nil
}

func writeRemoteProofPrivate(path string, data []byte, replace bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if replace {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	if n, err := f.Write(data); err != nil || n != len(data) {
		_ = f.Close()
		if err != nil {
			return err
		}
		return io.ErrShortWrite
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return syncRemoteProofDirectory(filepath.Dir(path))
}

func syncRemoteProofDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func appendRemoteProofEvent(t *testing.T, path, event string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal("private intent ledger unavailable")
	}
	defer f.Close()
	row, err := json.Marshal(map[string]string{"event": event, "at": time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal("private intent ledger serialization failed")
	}
	if _, err := f.Write(append(row, '\n')); err != nil || f.Sync() != nil {
		t.Fatal("private intent ledger write failed")
	}
}

func persistRemoteProofActor(t *testing.T, dir, name string, actor remoteProofActor) {
	t.Helper()
	raw, err := json.Marshal(actor)
	if err != nil || writeRemoteProofPrivate(filepath.Join(dir, name), raw, false) != nil {
		t.Fatal("refreshed private actor session could not be persisted")
	}
}

func refreshRemoteProofActor(t *testing.T, client *http.Client, base string, actor remoteProofActor) remoteProofActor {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"refresh_token":    actor.RefreshToken,
		"device_info_json": `{"platform":"go-matchsquad-live-test"}`,
	})
	if err != nil {
		t.Fatal("refresh request serialization failed")
	}
	var envelope struct {
		Session remoteProofActor `json:"session"`
	}
	remoteProofRequest(t, client, http.MethodPost, base+"/api/v1/auth/refresh", "", payload, http.StatusOK, &envelope, "refresh_actor_session")
	if envelope.Session.AccessToken == "" || envelope.Session.RefreshToken == "" || envelope.Session.ProfileID != actor.ProfileID || envelope.Session.AccountID != actor.AccountID {
		t.Fatal("refresh response did not preserve the owned actor identity")
	}
	envelope.Session.Label = actor.Label
	envelope.Session.AccountType = actor.AccountType
	return envelope.Session
}

func remoteProofRequest(t *testing.T, client *http.Client, method, target, accessToken string, payload []byte, want int, out any, operation string) []byte {
	t.Helper()
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatalf("%s request could not be constructed", operation)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s transport failed; stop without retry", operation)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatalf("%s response read failed; stop without retry", operation)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s returned HTTP %d; stop without retry", operation, resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s response did not match the expected JSON shape", operation)
		}
	}
	return raw
}

func remoteProofSearchMatchID(body []byte) (string, error) {
	var response matchmakingv1.GetSearchStatusResponse
	if err := protojson.Unmarshal(body, &response); err != nil {
		return "", err
	}
	return response.GetSearchSession().GetMatchId(), nil
}

func startRemoteProofSearch(t *testing.T, client *http.Client, base, token, gameID, criteria, intentPath, ledgerPath, actor string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"gameId": gameID, "mode": "Duo", "criteriaJson": criteria})
	operationID := composeClientMessageID()
	if err := appendRemoteProofOperation(intentPath, actor, operationID, body); err != nil {
		t.Fatal("search operation intent could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, actor+"_request_intent")
	var response map[string]any
	remoteProofRequest(t, client, http.MethodPost, base+"/api/v1/matchmaking/search", token, body, http.StatusOK, &response, "start_matchmaking_search")
	session := remoteProofMap(response, "searchSession", "search_session")
	if session == nil || remoteProofString(session, "id") == "" {
		t.Fatal("search response omitted its session ID")
	}
	searchID := remoteProofString(session, "id")
	if err := appendRemoteProofReceipt(intentPath, actor, map[string]any{"search_session_id": searchID}); err != nil {
		t.Fatal("search receipt could not be persisted")
	}
	return searchID
}

func remoteProofMatchGet(t *testing.T, client *http.Client, base, token, matchID string, out *map[string]any) {
	t.Helper()
	var envelope map[string]any
	remoteProofRequest(t, client, http.MethodGet, base+"/api/v1/matchmaking/matches/"+url.PathEscape(matchID), token, nil, http.StatusOK, &envelope, "read_match")
	match := remoteProofMap(envelope, "match", "Match")
	if match == nil {
		t.Fatal("match response omitted its public Match object")
	}
	*out = match
}

func remoteProofAccept(t *testing.T, client *http.Client, base, token, matchID, intentPath, ledgerPath, actor string) {
	t.Helper()
	body, _ := json.Marshal(map[string]bool{"accept": true})
	operationID := composeClientMessageID()
	if err := appendRemoteProofOperation(intentPath, actor, operationID, body); err != nil {
		t.Fatal("accept operation intent could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, actor+"_request_intent")
	var response map[string]any
	remoteProofRequest(t, client, http.MethodPost, base+"/api/v1/matchmaking/matches/"+url.PathEscape(matchID)+"/respond", token, body, http.StatusOK, &response, "accept_match")
	receipt := map[string]any{"match_id": matchID, "http_status": http.StatusOK}
	if match := remoteProofMap(response, "match", "Match"); match != nil {
		receipt["response_match_id"] = remoteProofString(match, "id", "matchId", "match_id")
		receipt["response_status"] = remoteProofString(match, "status")
	}
	if err := appendRemoteProofReceipt(intentPath, actor, receipt); err != nil {
		t.Fatal("accept receipt could not be persisted")
	}
}

func remoteProofJoin(t *testing.T, client *http.Client, base, token, matchID, roomID, intentPath, ledgerPath, actor string) string {
	t.Helper()
	operationID := composeClientMessageID()
	body, _ := json.Marshal(map[string]any{"protocolVersion": 1, "operationId": operationID, "matchId": matchID, "roomId": roomID})
	appendRemoteProofEvent(t, ledgerPath, actor+"_join_operation_persist_intent")
	if err := appendRemoteProofOperation(intentPath, actor+"_join", operationID, body); err != nil {
		t.Fatal("join operation intent could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, actor+"_join_request_intent")
	var envelope map[string]any
	remoteProofRequest(t, client, http.MethodPost, remoteProofMatchSquadURL(base, matchID, "join"), token, body, http.StatusOK, &envelope, actor+"_join")
	if remoteProofString(envelope, "membershipState", "membership_state") != "MATCH_SQUAD_MEMBERSHIP_STATE_JOINED" || remoteProofString(envelope, "mediaEpoch", "media_epoch") == "" {
		t.Fatal("MatchSquad Join response was not JOINED with a media epoch")
	}
	epoch := remoteProofString(envelope, "mediaEpoch", "media_epoch")
	if err := appendRemoteProofReceipt(intentPath, actor+"_join", map[string]any{"match_id": matchID, "media_epoch": epoch, "membership_state": "JOINED"}); err != nil {
		t.Fatal("join receipt could not be persisted")
	}
	return epoch
}

type remoteProofJoinCredentials struct {
	jwt string
	url string
}

func remoteProofToken(t *testing.T, client *http.Client, base, token, matchID, roomID, mediaEpoch, ledgerPath, actor string) remoteProofJoinCredentials {
	t.Helper()
	appendRemoteProofEvent(t, ledgerPath, actor+"_token_request_intent")
	body, _ := json.Marshal(map[string]any{"protocolVersion": 1, "matchId": matchID, "roomId": roomID, "mediaEpoch": mediaEpoch})
	var envelope map[string]any
	remoteProofRequest(t, client, http.MethodPost, remoteProofMatchSquadURL(base, matchID, "token"), token, body, http.StatusOK, &envelope, actor+"_matchsquad_token")
	tokenMap := remoteProofMap(envelope, "token")
	if tokenMap == nil {
		t.Fatal("MatchSquad token response omitted token payload")
	}
	return remoteProofJoinCredentials{jwt: remoteProofString(tokenMap, "jwt"), url: remoteProofString(tokenMap, "livekitUrl", "livekit_url")}
}

func newRemoteProofLivekitPeer(ctx context.Context, livekitURL, jwt, expectedRemoteProfileID string) (*livekitCallPeer, error) {
	peer := &livekitCallPeer{
		ctx:            ctx,
		expectedRemote: expectedRemoteProfileID,
		remoteTrack:    make(chan *webrtc.TrackRemote, 1),
	}
	trackSubscribed := make(chan struct{}, 4)
	callback := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, _ *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
				if track == nil || track.Kind() != webrtc.RTPCodecTypeAudio || (expectedRemoteProfileID != "" && participant.Identity() != expectedRemoteProfileID) {
					return
				}
				select {
				case peer.remoteTrack <- track:
				default:
				}
				select {
				case trackSubscribed <- struct{}{}:
				default:
				}
			},
		},
	}
	room, err := lksdk.ConnectToRoomWithToken(livekitURL, jwt, callback, lksdk.WithAutoSubscribe(true))
	if err != nil {
		return nil, os.ErrInvalid
	}
	peer.room = room
	pcmTrack, err := lkmedia.NewPCMLocalTrack(48000, 1, nil)
	if err != nil {
		peer.close()
		return nil, os.ErrInvalid
	}
	_, err = room.LocalParticipant.PublishTrack(pcmTrack, &lksdk.TrackPublicationOptions{Name: "qa-mic", Source: livekit.TrackSource_MICROPHONE})
	if err != nil {
		_ = pcmTrack.Close()
		peer.close()
		return nil, os.ErrInvalid
	}
	peer.pcmTrack = pcmTrack
	go peer.publishTone(ctx, pcmTrack, 440.0)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-trackSubscribed:
			case <-time.After(200 * time.Millisecond):
				return
			}
		}
	}()
	return peer, nil
}

func remoteProofSendMessage(t *testing.T, client *http.Client, base, token, chatID, marker, intentPath, ledgerPath string) string {
	t.Helper()
	clientMessageID := composeClientMessageID()
	body, _ := json.Marshal(map[string]any{"chat": map[string]string{"id": chatID}, "content": marker, "client_message_id": clientMessageID})
	if err := appendRemoteProofOperation(intentPath, "chat_message", clientMessageID, body); err != nil {
		t.Fatal("chat message intent could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, "chat_message_request_intent")
	var response map[string]any
	remoteProofRequest(t, client, http.MethodPost, base+"/api/v1/messages/send", token, body, http.StatusOK, &response, "send_match_chat_message")
	message := remoteProofMap(response, "message")
	messageID := remoteProofString(message, "id")
	if messageID == "" {
		t.Fatal("chat send response omitted message ID")
	}
	if err := appendRemoteProofReceipt(intentPath, "chat_message", map[string]any{"chat_id": chatID, "message_id": messageID, "client_message_id": clientMessageID}); err != nil {
		t.Fatal("chat message receipt could not be persisted")
	}
	return messageID
}

func remoteProofReceiverHasMessage(t *testing.T, client *http.Client, base, token, chatID, messageID, marker string) bool {
	t.Helper()
	var response map[string]any
	remoteProofRequest(t, client, http.MethodGet, base+"/api/v1/messages?chat_id="+url.QueryEscape(chatID), token, nil, http.StatusOK, &response, "read_recipient_chat_history")
	messageList := remoteProofMap(response, "messageList", "message_list")
	if messageList == nil {
		return false
	}
	messages, ok := messageList["messages"].([]any)
	if !ok {
		return false
	}
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if ok && remoteProofString(message, "id") == messageID && remoteProofString(message, "content") == marker {
			return true
		}
	}
	return false
}

func remoteProofLeave(t *testing.T, client *http.Client, base, token, matchID, roomID, mediaEpoch, intentPath, ledgerPath, actor string) {
	t.Helper()
	operationID := composeClientMessageID()
	body, _ := json.Marshal(map[string]any{"protocolVersion": 1, "operationId": operationID, "matchId": matchID, "roomId": roomID, "expectedMediaEpoch": mediaEpoch})
	if err := appendRemoteProofOperation(intentPath, actor+"_leave", operationID, body); err != nil {
		t.Fatal("leave operation intent could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, actor+"_leave_request_intent")
	var response map[string]any
	remoteProofRequest(t, client, http.MethodPost, remoteProofMatchSquadURL(base, matchID, "leave"), token, body, http.StatusOK, &response, actor+"_leave")
	if remoteProofString(response, "mediaEpoch", "media_epoch") != mediaEpoch ||
		(remoteProofString(response, "membershipState", "membership_state") != "MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING" &&
			remoteProofString(response, "membershipState", "membership_state") != "MATCH_SQUAD_MEMBERSHIP_STATE_LEFT") {
		t.Fatal("MatchSquad leave response did not bind the requested epoch and leave state")
	}
	if err := appendRemoteProofReceipt(intentPath, actor+"_leave", map[string]any{"match_id": matchID, "media_epoch": mediaEpoch, "membership_state": remoteProofString(response, "membershipState", "membership_state")}); err != nil {
		t.Fatal("leave receipt could not be persisted")
	}
}

func remoteProofComplete(t *testing.T, client *http.Client, base, token, matchID, ledgerPath, intentPath, actor, expectedStatus, chatID, roomID, profileA, profileB string) map[string]any {
	t.Helper()
	operationID := composeClientMessageID()
	body, _ := json.Marshal(map[string]string{"operationId": operationID})
	if err := appendRemoteProofOperation(intentPath, actor+"_complete", operationID, body); err != nil {
		t.Fatal("completion operation intent could not be persisted")
	}
	appendRemoteProofEvent(t, ledgerPath, actor+"_complete_request_intent")
	var response map[string]any
	remoteProofRequest(t, client, http.MethodPost, base+"/api/v1/matchmaking/matches/"+url.PathEscape(matchID)+"/complete", token, body, http.StatusOK, &response, actor+"_complete")
	match := remoteProofMap(response, "match", "Match")
	if match == nil {
		t.Fatal("CompleteMatch response omitted Match")
	}
	if !remoteProofHasBoundMatch(match, expectedStatus, chatID, roomID, profileA, profileB) {
		t.Fatalf("CompleteMatch response did not confirm %s with the expected roster and resource binding; stop without retry", expectedStatus)
	}
	if err := appendRemoteProofReceipt(intentPath, actor+"_complete", map[string]any{"match_id": matchID, "status": expectedStatus}); err != nil {
		t.Fatal("completion receipt could not be persisted")
	}
	return match
}

func remoteProofMatchSquadURL(base, matchID, operation string) string {
	if operation != "join" && operation != "token" && operation != "leave" {
		return ""
	}
	return base + "/api/v1/matchmaking/matches/" + url.PathEscape(matchID) + "/voice/" + operation
}

func remoteProofMayCompleteNext(match map[string]any, chatID, roomID, profileA, profileB string) bool {
	return remoteProofHasBoundMatch(match, "active", chatID, roomID, profileA, profileB)
}

func appendRemoteProofOperation(path, action, operationID string, request []byte) error {
	var intent map[string]any
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &intent) != nil {
		return os.ErrInvalid
	}
	operations, _ := intent["operation_ids"].(map[string]any)
	if operations == nil {
		operations = map[string]any{}
		intent["operation_ids"] = operations
	}
	if _, exists := operations[action]; exists {
		return os.ErrExist
	}
	operations[action] = operationID
	if len(request) > 0 {
		digest := sha256.Sum256(request)
		requestHashes, _ := intent["request_sha256"].(map[string]any)
		if requestHashes == nil {
			requestHashes = map[string]any{}
			intent["request_sha256"] = requestHashes
		}
		requestHashes[action] = hex.EncodeToString(digest[:])
	}
	updated, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	return replaceRemoteProofPrivate(path, updated)
}

func appendRemoteProofReceipt(path, name string, value any) error {
	var intent map[string]any
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &intent) != nil {
		return os.ErrInvalid
	}
	receipts, _ := intent["receipts"].(map[string]any)
	if receipts == nil {
		receipts = map[string]any{}
		intent["receipts"] = receipts
	}
	if _, exists := receipts[name]; exists {
		return os.ErrExist
	}
	receipts[name] = value
	updated, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	return replaceRemoteProofPrivate(path, updated)
}

func replaceRemoteProofPrivate(path string, data []byte) error {
	tmp := path + ".new"
	if err := writeRemoteProofPrivate(tmp, data, false); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncRemoteProofDirectory(filepath.Dir(path))
}

func remoteProofHasBoundMatch(match map[string]any, status, chatID, roomID, profileA, profileB string) bool {
	return remoteProofString(match, "status") == status &&
		remoteProofString(match, "chatId", "chat_id") == chatID &&
		remoteProofString(match, "voiceRoomId", "voice_room_id") == roomID &&
		remoteProofHasRoster(match, profileA, profileB)
}

func remoteProofContainsGame(response map[string]any, gameID string) bool {
	list := remoteProofMap(response, "gameList", "game_list")
	if list == nil {
		return false
	}
	games, ok := list["games"].([]any)
	if !ok {
		return false
	}
	for _, value := range games {
		game, ok := value.(map[string]any)
		if ok && remoteProofString(game, "id") == gameID {
			return true
		}
	}
	return false
}

func remoteProofMap(value map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if out, ok := value[key].(map[string]any); ok {
			return out
		}
	}
	return nil
}

func remoteProofString(value map[string]any, keys ...string) string {
	if value == nil {
		return ""
	}
	for _, key := range keys {
		if out, ok := value[key].(string); ok && out != "" {
			return out
		}
	}
	return ""
}

func remoteProofHasRoster(match map[string]any, profileA, profileB string) bool {
	profiles, ok := match["profileIds"].([]any)
	if !ok {
		profiles, ok = match["profile_ids"].([]any)
	}
	if !ok || len(profiles) != 2 {
		return false
	}
	seen := map[string]bool{}
	for _, profile := range profiles {
		if value, ok := profile.(string); ok {
			seen[value] = true
		}
	}
	return seen[profileA] && seen[profileB]
}

func TestRemoteProofMatchSquadRoutesAndCompletionPhases(t *testing.T) {
	const base, matchID = "https://api-dev.comrade.click", "match-123"
	for operation, want := range map[string]string{
		"join":  base + "/api/v1/matchmaking/matches/match-123/voice/join",
		"token": base + "/api/v1/matchmaking/matches/match-123/voice/token",
		"leave": base + "/api/v1/matchmaking/matches/match-123/voice/leave",
	} {
		if got := remoteProofMatchSquadURL(base, matchID, operation); got != want {
			t.Errorf("%s route = %q, want %q", operation, got, want)
		}
	}
	if got := remoteProofMatchSquadURL(base, matchID, "complete"); got != "" {
		t.Fatalf("unsupported member route = %q, want empty", got)
	}

	match := map[string]any{
		"status": "active", "chatId": "chat-1", "voiceRoomId": "room-1",
		"profileIds": []any{"profile-a", "profile-b"},
	}
	if !remoteProofMayCompleteNext(match, "chat-1", "room-1", "profile-a", "profile-b") {
		t.Fatal("first completion response should authorize the next completion")
	}
	match["status"] = "completed"
	if remoteProofMayCompleteNext(match, "chat-1", "room-1", "profile-a", "profile-b") {
		t.Fatal("unexpected terminal response should stop before the next completion write")
	}
	match["status"] = "active"
	match["profileIds"] = []any{"profile-a", "profile-c"}
	if remoteProofMayCompleteNext(match, "chat-1", "room-1", "profile-a", "profile-b") {
		t.Fatal("changed roster should stop before the next completion write")
	}
}

func TestRemoteProofSearchStatusUsesCanonicalProtoResponse(t *testing.T) {
	fixtures := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "protojson camelCase",
			body: `{"searchSession":{"id":"search-1","profileId":"profile-a","gameId":"game-1","mode":"duo","status":"matched","matchId":"match-camel"}}`,
			want: "match-camel",
		},
		{
			name: "protojson snake_case",
			body: `{"search_session":{"id":"search-2","profile_id":"profile-b","game_id":"game-1","mode":"duo","status":"matched","match_id":"match-snake"}}`,
			want: "match-snake",
		},
		{
			name:    "top-level match ID is outside the protobuf response schema",
			body:    `{"matchId":"not-the-search-session-match"}`,
			wantErr: true,
		},
		{
			name: "unmatched search session",
			body: `{"searchSession":{"id":"search-3","profileId":"profile-c","gameId":"game-1","mode":"duo","status":"searching"}}`,
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			got, err := remoteProofSearchMatchID([]byte(fixture.body))
			if fixture.wantErr {
				if err == nil {
					t.Fatal("response outside the protobuf schema should be rejected")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse source-shaped status response: %v", err)
			}
			if got != fixture.want {
				t.Fatalf("match ID = %q, want %q", got, fixture.want)
			}
		})
	}
	if _, err := remoteProofSearchMatchID([]byte(`{"searchSession":`)); err == nil {
		t.Fatal("malformed status response should be rejected")
	}
}

func TestRemoteProofPrivateIntentIsOneShot(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal("private test state directory could not be secured")
	}
	if _, _, _, err := createRemoteProofIntent(root); err != nil {
		t.Fatal("first private remote-proof intent was rejected")
	}
	if _, _, _, err := createRemoteProofIntent(root); err == nil {
		t.Fatal("second invocation reused a private actor state root")
	}
}
