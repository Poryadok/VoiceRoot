package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func formatComposeEmail(prefix string, n int64) string {
	return fmt.Sprintf("%s-%d@voice-qa.test", prefix, n)
}

func composeClientMessageID() string {
	return uuid.NewString()
}

type composeWSFrame struct {
	Op string          `json:"op"`
	S  int64           `json:"s"`
	D  json.RawMessage `json:"d"`
}

func composeWebSocketURL(t *testing.T, apiBase string) string {
	t.Helper()
	u, err := url.Parse(strings.TrimRight(apiBase, "/"))
	require.NoError(t, err)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		t.Fatalf("unsupported API base scheme %q", u.Scheme)
	}
	u.Path = "/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func dialComposeRealtimeWS(t *testing.T, apiBase, accessToken string) *websocket.Conn {
	t.Helper()
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+accessToken)
	conn, resp, err := websocket.DefaultDialer.Dial(composeWebSocketURL(t, apiBase), hdr)
	if resp != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err, "WS /ws upgrade via gateway")
	require.NotNil(t, conn)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func waitComposeWSHello(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	frame := waitComposeWSOp(t, conn, "hello", 15*time.Second, nil)
	require.Greater(t, frame.S, int64(0))
}

func waitComposeWSOp(
	t *testing.T,
	conn *websocket.Conn,
	wantOp string,
	timeout time.Duration,
	match func(map[string]any) bool,
) composeWSFrame {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(remaining))
		var frame composeWSFrame
		err := conn.ReadJSON(&frame)
		if err != nil {
			t.Fatalf("read WS while waiting for op=%s: %v", wantOp, err)
		}
		if frame.Op != wantOp {
			continue
		}
		if match == nil {
			return frame
		}
		var data map[string]any
		if len(frame.D) > 0 {
			require.NoError(t, json.Unmarshal(frame.D, &data))
		}
		if match(data) {
			return frame
		}
	}
	t.Fatalf("timeout waiting for WS op=%s", wantOp)
	return composeWSFrame{}
}

type composeSpaceItem struct {
	ID          string
	Name        string
	Description string
	IconURL     string
}

func createComposeSpace(t *testing.T, client *http.Client, base, accessToken, name, description string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"name":        name,
		"description": description,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /api/v1/spaces body=%s", string(body))

	var parsed struct {
		Space struct {
			ID string `json:"id"`
		} `json:"space"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Space.ID)
	return parsed.Space.ID
}

func listComposeSpaces(t *testing.T, client *http.Client, base, accessToken string) []composeSpaceItem {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/spaces", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET /api/v1/spaces body=%s", string(body))

	var parsed struct {
		SpaceList struct {
			Spaces []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				IconURL     string `json:"icon_url"`
			} `json:"spaces"`
		} `json:"space_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	var out []composeSpaceItem
	for _, s := range parsed.SpaceList.Spaces {
		out = append(out, composeSpaceItem{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			IconURL:     s.IconURL,
		})
	}
	return out
}

func getComposeSpace(t *testing.T, client *http.Client, base, accessToken, spaceID string) composeSpaceItem {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/spaces/"+spaceID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET space body=%s", string(body))

	var parsed struct {
		Space struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			IconURL     string `json:"icon_url"`
		} `json:"space"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return composeSpaceItem{
		ID:          parsed.Space.ID,
		Name:        parsed.Space.Name,
		Description: parsed.Space.Description,
		IconURL:     parsed.Space.IconURL,
	}
}

type composeSpaceTree struct {
	Categories []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
	Nodes []struct {
		ID           string `json:"id"`
		Kind         string `json:"kind"`
		SortOrder    int32  `json:"sort_order"`
		VoiceRoomID  string `json:"voice_room_id"`
		LinkedChatID string `json:"linked_chat_id"`
	} `json:"nodes"`
}

func getComposeSpaceTree(t *testing.T, client *http.Client, base, accessToken, spaceID string) composeSpaceTree {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/spaces/"+spaceID+"/tree", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET tree body=%s", string(body))
	var parsed composeSpaceTree
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed
}

func createComposeSpaceCategory(t *testing.T, client *http.Client, base, accessToken, spaceID, name string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"name": name, "sort_order": 0})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/categories", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST category body=%s", string(body))
	var parsed struct {
		Category struct {
			ID string `json:"id"`
		} `json:"category"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Category.ID)
	return parsed.Category.ID
}

func createComposeSpaceVoiceRoom(t *testing.T, client *http.Client, base, accessToken, spaceID, name string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"name": name})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/voice-rooms", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST voice room body=%s", string(body))
	var parsed struct {
		VoiceRoom struct {
			ID string `json:"id"`
		} `json:"voice_room"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.VoiceRoom.ID)
	return parsed.VoiceRoom.ID
}

func createComposeSpaceChat(t *testing.T, client *http.Client, base, accessToken, spaceID, name string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"type": "CHAT_TYPE_GROUP",
		"name": name,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/chats", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST space chat body=%s", string(body))
	var parsed struct {
		SpaceTreeNode struct {
			ID string `json:"id"`
		} `json:"space_tree_node"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.SpaceTreeNode.ID)
	return parsed.SpaceTreeNode.ID
}

func reorderComposeSpaceTree(t *testing.T, client *http.Client, base, accessToken, spaceID string, nodeIDs []string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"ordered_node_ids": nodeIDs})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/tree/reorder", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "POST reorder body=%s", string(body))
}

type composeSpaceInvite struct {
	ID   string
	Code string
}

func createComposeSpaceInvite(t *testing.T, client *http.Client, base, accessToken, spaceID string) composeSpaceInvite {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/spaces/"+spaceID+"/invites", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST invite body=%s", string(body))

	var parsed struct {
		Invite struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"invite"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Invite.Code)
	return composeSpaceInvite{ID: parsed.Invite.ID, Code: parsed.Invite.Code}
}

func listComposeSpaceInvites(t *testing.T, client *http.Client, base, accessToken, spaceID string) []composeSpaceInvite {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/spaces/"+spaceID+"/invites", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET invites body=%s", string(body))

	var parsed struct {
		InviteList struct {
			Invites []struct {
				ID   string `json:"id"`
				Code string `json:"code"`
			} `json:"invites"`
		} `json:"invite_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	var out []composeSpaceInvite
	for _, inv := range parsed.InviteList.Invites {
		out = append(out, composeSpaceInvite{ID: inv.ID, Code: inv.Code})
	}
	return out
}

func joinComposeSpaceByInvite(t *testing.T, client *http.Client, base, accessToken, code string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/invites/"+code+"/join", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST join body=%s", string(body))
}

func revokeComposeSpaceInvite(t *testing.T, client *http.Client, base, accessToken, spaceID, inviteID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/spaces/"+spaceID+"/invites/"+inviteID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "DELETE invite body=%s", string(body))
}

func updateComposeSpace(t *testing.T, client *http.Client, base, accessToken, spaceID, iconURL, description string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"icon_url":    iconURL,
		"description": description,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/spaces/"+spaceID, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH space body=%s", string(body))
}

func createComposeGroup(t *testing.T, client *http.Client, base, accessToken, name string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"type": "CHAT_TYPE_GROUP",
		"name": name,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /api/v1/chats body=%s", string(body))

	var parsed struct {
		Chat struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Chat.ID)
	require.Equal(t, "CHAT_TYPE_GROUP", parsed.Chat.Type)
	require.Equal(t, name, parsed.Chat.Name)
	return parsed.Chat.ID
}

func addComposeGroupMembers(t *testing.T, client *http.Client, base, accessToken, chatID string, profileIDs ...string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"profile_ids": profileIDs})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/members", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "POST members body=%s", string(body))
}

func removeComposeGroupMember(t *testing.T, client *http.Client, base, accessToken, chatID, profileID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/chats/"+chatID+"/members/"+profileID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "DELETE member body=%s", string(body))
}

func updateComposeGroupAvatar(t *testing.T, client *http.Client, base, accessToken, chatID, avatarURL string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"avatar_url": avatarURL})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/chats/"+chatID, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH chat body=%s", string(body))

	var parsed struct {
		Chat struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Equal(t, avatarURL, parsed.Chat.AvatarURL)
}

type composeGroupMember struct {
	ProfileID string `json:"profile_id"`
	Role      string `json:"role"`
}

func listComposeGroupMembers(t *testing.T, client *http.Client, base, accessToken, chatID string) []composeGroupMember {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+chatID+"/members", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET members body=%s", string(body))

	var parsed struct {
		MemberList struct {
			Members []composeGroupMember `json:"members"`
		} `json:"member_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.MemberList.Members
}

func leaveComposeGroupStatus(t *testing.T, client *http.Client, base, accessToken, chatID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/"+chatID+"/leave", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func removeComposeGroupMemberStatus(t *testing.T, client *http.Client, base, accessToken, chatID, profileID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/v1/chats/"+chatID+"/members/"+profileID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func getComposeChatStatus(t *testing.T, client *http.Client, base, accessToken, chatID string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats/"+chatID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func createComposeDM(t *testing.T, client *http.Client, base, accessToken, otherProfileID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"other_profile_id": otherProfileID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /api/v1/chats/dm body=%s", string(body))

	var parsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Chat.ID)
	return parsed.Chat.ID
}

type composeCallSession struct {
	RoomID             string `json:"room_id"`
	LivekitRoomName    string `json:"livekit_room_name"`
	InitiatorProfileID string `json:"initiator_profile_id"`
	CalleeProfileID    string `json:"callee_profile_id"`
	MediaKind          string `json:"media_kind"`
	Status             string `json:"status"`
	LinkedChat         struct {
		ID string `json:"id"`
	} `json:"linked_chat"`
}

type composeJoinToken struct {
	JWT        string `json:"jwt"`
	LivekitURL string `json:"livekit_url"`
}

func startComposeCall(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, calleeProfileID string,
) composeCallSession {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"linked_chat":        map[string]string{"id": chatID},
		"callee_profile_id":  calleeProfileID,
		"media_kind":         "audio",
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/calls", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /api/v1/voice/calls body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.CallSession.RoomID)
	return parsed.CallSession
}

func acceptComposeCall(t *testing.T, client *http.Client, base, accessToken, roomID string) composeCallSession {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/accept", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST accept body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.CallSession
}

func getComposeJoinToken(t *testing.T, client *http.Client, base, accessToken, roomID string) composeJoinToken {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/token", base, roomID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET token body=%s", string(body))

	var parsed composeJoinToken
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.JWT)
	return parsed
}

func getComposeActiveCall(t *testing.T, client *http.Client, base, accessToken string) *composeCallSession {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/voice/calls/active", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET active body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	if parsed.CallSession.RoomID == "" {
		return nil
	}
	s := parsed.CallSession
	return &s
}

func endComposeCall(t *testing.T, client *http.Client, base, accessToken, roomID string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/end", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func composeWSSend(t *testing.T, conn *websocket.Conn, payload map[string]any) {
	t.Helper()
	require.NoError(t, conn.WriteJSON(payload))
}

func connectComposeWSSubscribed(t *testing.T, base, accessToken, chatID string) *websocket.Conn {
	t.Helper()
	conn := dialComposeRealtimeWS(t, base, accessToken)
	waitComposeWSHello(t, conn)
	composeWSSend(t, conn, map[string]any{
		"op": "subscribe",
		"d":  map[string]string{"chat_id": chatID},
	})
	waitComposeWSOp(t, conn, "subscribe_ack", 15*time.Second, func(d map[string]any) bool {
		return d["chat_id"] == chatID
	})
	return conn
}

func sendComposeMessage(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, content string,
) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":              map[string]string{"id": chatID},
		"content":           content,
		"client_message_id": composeClientMessageID(),
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST send body=%s", string(body))

	var parsed struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Message.ID)
	return parsed.Message.ID
}

func getComposeMessagesContains(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, messageID, content string,
) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages?chat_id="+chatID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET messages body=%s", string(body))

	var hist struct {
		MessageList struct {
			Messages []struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(body, &hist))
	for _, m := range hist.MessageList.Messages {
		if m.ID == messageID && m.Content == content {
			return
		}
	}
	t.Fatalf("message %s with content %q not in history: %s", messageID, content, string(body))
}

func refreshComposeSession(t *testing.T, client *http.Client, base, refreshToken string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"refresh_token":    refreshToken,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	resp, err := client.Post(base+"/api/v1/auth/refresh", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "refresh body=%s", string(raw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	sess := envelope.Session
	require.NotEmpty(t, sess.AccessToken)
	require.NotEmpty(t, sess.RefreshToken)
	return sess
}

func logoutComposeSession(t *testing.T, client *http.Client, base, accessToken, refreshToken string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"refresh_token": refreshToken})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/logout", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func composeProtectedRouteStatus(t *testing.T, client *http.Client, base, accessToken string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func markReadComposeMessage(t *testing.T, client *http.Client, base, accessToken, chatID, messageID string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":                  map[string]string{"id": chatID},
		"last_read_message_id":  messageID,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/read", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "mark read body=%s", string(body))
}

func getComposeReadState(t *testing.T, client *http.Client, base, accessToken, chatID string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages/read-state?chat_id="+chatID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "read-state body=%s", string(body))

	var parsed struct {
		ReadState struct {
			LastReadMessageID string `json:"last_read_message_id"`
			ProfileID         string `json:"profile_id"`
			Chat              struct {
				ID string `json:"id"`
			} `json:"chat"`
		} `json:"read_state"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.ReadState.LastReadMessageID
}

func sendComposeFriendInvitation(t *testing.T, client *http.Client, base, accessToken, targetProfileID string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"target_profile_id": targetProfileID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/invitations", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "friend invitation body=%s", string(body))
}

func acceptComposeFriendInvitation(t *testing.T, client *http.Client, base, accessToken, requesterProfileID string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/friends/invitations/%s/accept", base, requesterProfileID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "accept friend body=%s", string(body))
}

func composeFriendIDs(t *testing.T, client *http.Client, base, accessToken string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/friends", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list friends body=%s", string(body))

	var parsed struct {
		FriendList struct {
			Friends []struct {
				ProfileID string `json:"profile_id"`
			} `json:"friends"`
		} `json:"friend_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	var ids []string
	for _, f := range parsed.FriendList.Friends {
		ids = append(ids, f.ProfileID)
	}
	return ids
}

func declineComposeCall(t *testing.T, client *http.Client, base, accessToken, roomID string) composeCallSession {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/decline", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST decline body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.CallSession
}

func editComposeMessage(
	t *testing.T,
	client *http.Client,
	base, accessToken, messageID, content string,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"content": content})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPatch, base+"/api/v1/messages/"+messageID, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "PATCH message body=%s", string(body))
}

func deleteComposeMessage(
	t *testing.T,
	client *http.Client,
	base, accessToken, messageID, scope string,
) {
	t.Helper()
	url := base + "/api/v1/messages/" + messageID + "?scope=" + scope
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func composeWSSendResume(t *testing.T, conn *websocket.Conn, lastS int64) {
	t.Helper()
	composeWSSend(t, conn, map[string]any{
		"op": "resume",
		"d":  map[string]any{"last_s": lastS},
	})
}

func declineComposeFriendInvitation(t *testing.T, client *http.Client, base, accessToken, requesterProfileID string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/friends/invitations/%s/decline", base, requesterProfileID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "decline friend body=%s", string(body))
}

func blockComposeAccount(t *testing.T, client *http.Client, base, accessToken, blockedAccountID string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"blocked_account_id": blockedAccountID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/blocks", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "block account body=%s", string(body))
}

type composeChatListItem struct {
	ChatID           string
	LastPreview      string
	UnreadCount      int
	Inbox            string
	IsStranger       bool
	DMPeerProfileID  string
}

func listComposeChats(t *testing.T, client *http.Client, base, accessToken, inbox string) []composeChatListItem {
	t.Helper()
	url := base + "/api/v1/chats"
	if inbox != "" {
		url += "?inbox=" + inbox
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list chats body=%s", string(body))

	var parsed struct {
		ChatList struct {
			Items []struct {
				Chat struct {
					ID string `json:"id"`
				} `json:"chat"`
				LastMessagePreview  string `json:"last_message_preview"`
				UnreadCount         int    `json:"unread_count"`
				Inbox               string `json:"inbox"`
				IsStranger          bool   `json:"is_stranger"`
				DMPeerProfileID     string `json:"dm_peer_profile_id"`
			} `json:"items"`
		} `json:"chat_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	var out []composeChatListItem
	for _, item := range parsed.ChatList.Items {
		out = append(out, composeChatListItem{
			ChatID:          item.Chat.ID,
			LastPreview:     item.LastMessagePreview,
			UnreadCount:     item.UnreadCount,
			Inbox:           item.Inbox,
			IsStranger:      item.IsStranger,
			DMPeerProfileID: item.DMPeerProfileID,
		})
	}
	return out
}

func composePostStatus(
	t *testing.T,
	client *http.Client,
	base, accessToken, path string,
	payload []byte,
) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func composeFileUploadAvailable(t *testing.T, client *http.Client, base, accessToken string) bool {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"original_name": "probe.txt",
		"mime_type":     "text/plain",
		"size_bytes":    4,
	})
	require.NoError(t, err)
	status := composePostStatus(t, client, base, accessToken, "/api/v1/files/upload", payload)
	return status == http.StatusOK
}

func composeUploadSmallTextFile(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID string,
) (fileID, fileType string) {
	t.Helper()
	const content = "e2e"
	payload, err := json.Marshal(map[string]any{
		"original_name": "e2e.txt",
		"mime_type":     "text/plain",
		"size_bytes":    len(content),
		"context_chat": map[string]string{
			"id":   chatID,
			"type": "CHAT_TYPE_DM",
		},
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/files/upload", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "upload request body=%s", string(body))

	var parsed struct {
		UploadResponse struct {
			FileID          string `json:"file_id"`
			PresignedPutURL string `json:"presigned_put_url"`
		} `json:"upload_response"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	fileID = parsed.UploadResponse.FileID
	putURL := parsed.UploadResponse.PresignedPutURL
	require.NotEmpty(t, fileID)
	require.NotEmpty(t, putURL)

	putReq, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(content))
	require.NoError(t, err)
	putReq.Header.Set("Content-Type", "text/plain")
	putResp, err := client.Do(putReq)
	require.NoError(t, err)
	defer putResp.Body.Close()
	require.True(t, putResp.StatusCode >= 200 && putResp.StatusCode < 300, "PUT presigned status=%d", putResp.StatusCode)

	hash := sha256Hex([]byte(content))
	confirmPayload, err := json.Marshal(map[string]string{"sha256_hash": hash})
	require.NoError(t, err)
	confirmURL := base + "/api/v1/files/" + fileID + "/confirm"
	confirmReq, err := http.NewRequest(http.MethodPost, confirmURL, bytes.NewReader(confirmPayload))
	require.NoError(t, err)
	confirmReq.Header.Set("Authorization", "Bearer "+accessToken)
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmResp, err := client.Do(confirmReq)
	require.NoError(t, err)
	defer confirmResp.Body.Close()
	confirmBody, _ := io.ReadAll(confirmResp.Body)
	require.Equal(t, http.StatusOK, confirmResp.StatusCode, "confirm body=%s", string(confirmBody))
	var confirmed struct {
		FileMetadata struct {
			FileType string `json:"file_type"`
		} `json:"file_metadata"`
	}
	require.NoError(t, json.Unmarshal(confirmBody, &confirmed))
	fileType = strings.TrimSpace(confirmed.FileMetadata.FileType)
	if fileType == "" {
		fileType = "file"
	}
	return fileID, fileType
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func sendComposeMessageWithAttachmentsJSON(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, attachmentsJSON string,
) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":              map[string]string{"id": chatID},
		"content":           "",
		"attachments_json":  attachmentsJSON,
		"client_message_id": composeClientMessageID(),
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST send attachment body=%s", string(body))

	var parsed struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Message.ID)
	return parsed.Message.ID
}
