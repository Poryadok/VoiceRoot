package controlledgame

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxPublicClientResponseBytes = 1 << 20

// PublicSessionClient is an internal acceptance client for the frozen GIS
// public session routes. It deliberately has no provider or engine SDK code.
type PublicSessionClient struct {
	baseURL string
	client  *http.Client
}

type PublicSessionRequest struct {
	OperationID    string   `json:"operation_id"`
	Kind           string   `json:"kind"`
	ExternalKey    string   `json:"external_key"`
	ParentPartyKey string   `json:"parent_party_key,omitempty"`
	DisplayName    string   `json:"display_name,omitempty"`
	RosterRevision int64    `json:"roster_revision"`
	RosterComplete bool     `json:"roster_complete"`
	Members        []string `json:"members"`
}

type PublicRosterRequest struct {
	OperationID    string   `json:"operation_id"`
	RosterRevision int64    `json:"roster_revision"`
	RosterComplete bool     `json:"roster_complete"`
	Members        []string `json:"members"`
}

type PublicConsentRequest struct {
	OperationID    string `json:"operation_id"`
	RosterRevision int64  `json:"roster_revision"`
}

type PublicKeepGroupReceipt struct {
	OperationID    string   `json:"operation_id"`
	ReceiptID      string   `json:"receipt_id"`
	SessionID      string   `json:"session_id"`
	ChatID         string   `json:"chat_id"`
	DestinationKey string   `json:"destination_key"`
	RosterRevision int64    `json:"roster_revision"`
	AddedProfiles  []string `json:"added_profiles"`
	Replayed       bool     `json:"replayed"`
}

type PublicSessionOperation struct {
	OperationID             string `json:"operation_id"`
	SessionID               string `json:"session_id"`
	Status                  string `json:"status"`
	SessionStatus           string `json:"session_status"`
	ChatID                  string `json:"chat_id"`
	VoiceRoomID             string `json:"voice_room_id"`
	ChatOwnerSessionID      string `json:"chat_owner_session_id"`
	ChatCreateReceiptID     string `json:"chat_create_receipt_id"`
	ChatRosterReceiptID     string `json:"chat_roster_receipt_id"`
	VoiceProvisionReceiptID string `json:"voice_provision_receipt_id"`
	RoleGrantReceiptID      string `json:"role_grant_receipt_id"`
}

type PublicCommunityRosterRequest struct {
	OperationID     string                      `json:"operation_id"`
	SourceRevision  int64                       `json:"source_revision"`
	OwnerGeneration int64                       `json:"owner_generation"`
	SnapshotSHA256  string                      `json:"snapshot_sha256"`
	Pages           []PublicCommunityRosterPage `json:"pages"`
}

type PublicCommunityRosterPage struct {
	Index   int32                                 `json:"index"`
	Count   int32                                 `json:"count"`
	SHA256  string                                `json:"sha256"`
	Members []PublicCommunityRosterSnapshotMember `json:"members"`
}

type PublicCommunityRosterSnapshotMember struct {
	BindingID                string   `json:"binding_id"`
	CharacterKey             string   `json:"character_key"`
	SourceMembershipRevision int64    `json:"source_membership_revision"`
	SourceRank               string   `json:"source_rank"`
	GrantReasons             []string `json:"grant_reasons"`
}

type PublicCommunityRosterResult struct {
	OperationID     string `json:"operation_id"`
	CorporationKey  string `json:"corporation_key"`
	SourceRevision  int64  `json:"source_revision"`
	OwnerGeneration int64  `json:"owner_generation"`
	Outcome         string `json:"outcome"`
	MemberCount     int    `json:"member_count"`
	Replayed        bool   `json:"replayed"`
}

type PublicCommunityRosterMember struct {
	BindingID    string   `json:"binding_id"`
	CharacterKey string   `json:"character_key"`
	SourceRank   string   `json:"source_rank"`
	GrantReasons []string `json:"grant_reasons"`
}

type PublicCommunityRosterView struct {
	CorporationKey  string                        `json:"corporation_key"`
	SourceRevision  int64                         `json:"source_revision"`
	OwnerGeneration int64                         `json:"owner_generation"`
	Members         []PublicCommunityRosterMember `json:"members"`
}

func NewPublicSessionClient(baseURL string, client *http.Client) (*PublicSessionClient, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("public session client: invalid base URL")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &PublicSessionClient{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), client: client}, nil
}

func (c *PublicSessionClient) CreateSession(ctx context.Context, gameCredential string, request PublicSessionRequest) (PublicSessionOperation, error) {
	var operation PublicSessionOperation
	err := c.do(ctx, http.MethodPost, "/api/v1/sessions", gameCredential, request, http.StatusAccepted, &operation)
	return operation, err
}

func (c *PublicSessionClient) GetOperation(ctx context.Context, gameCredential, operationID string) (PublicSessionOperation, error) {
	var operation PublicSessionOperation
	err := c.do(ctx, http.MethodGet, "/api/v1/operations/"+url.PathEscape(operationID), gameCredential, nil, http.StatusOK, &operation)
	return operation, err
}

func (c *PublicSessionClient) UpdateRoster(ctx context.Context, gameCredential, sessionID string, request PublicRosterRequest) (PublicSessionOperation, error) {
	var operation PublicSessionOperation
	err := c.do(ctx, http.MethodPut, "/api/v1/sessions/"+url.PathEscape(sessionID)+"/roster", gameCredential, request, http.StatusAccepted, &operation)
	return operation, err
}

func (c *PublicSessionClient) ConsentToKeepGroup(ctx context.Context, playerVoiceToken, sessionID string, request PublicConsentRequest) error {
	return c.do(ctx, http.MethodPost, "/api/v1/sessions/"+url.PathEscape(sessionID)+"/consent", playerVoiceToken, request, http.StatusOK, nil)
}

func (c *PublicSessionClient) KeepGroup(ctx context.Context, playerVoiceToken, sessionID string, request PublicConsentRequest) (PublicKeepGroupReceipt, error) {
	var receipt PublicKeepGroupReceipt
	err := c.do(ctx, http.MethodPost, "/api/v1/sessions/"+url.PathEscape(sessionID)+"/keep-group", playerVoiceToken, request, http.StatusOK, &receipt)
	return receipt, err
}

func (c *PublicSessionClient) CloseSession(ctx context.Context, gameCredential, sessionID, operationID string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/sessions/"+url.PathEscape(sessionID)+"/close", gameCredential, map[string]string{"operation_id": operationID}, http.StatusAccepted, nil)
}

func (c *PublicSessionClient) ApplyCommunityRosterSnapshot(ctx context.Context, rosterCredential, corporationKey string, request PublicCommunityRosterRequest) (PublicCommunityRosterResult, error) {
	var result PublicCommunityRosterResult
	path := "/api/v1/community-bindings/" + url.PathEscape(corporationKey) + "/roster"
	err := c.doStatuses(ctx, http.MethodPut, path, rosterCredential, request, []int{http.StatusAccepted, http.StatusOK}, &result)
	return result, err
}

func (c *PublicSessionClient) GetCommunityRosterSnapshot(ctx context.Context, rosterCredential, corporationKey string) (PublicCommunityRosterView, error) {
	var view PublicCommunityRosterView
	path := "/api/v1/community-bindings/" + url.PathEscape(corporationKey) + "/roster"
	err := c.do(ctx, http.MethodGet, path, rosterCredential, nil, http.StatusOK, &view)
	return view, err
}

func (c *PublicSessionClient) do(ctx context.Context, method, path, bearer string, input any, wantStatus int, output any) error {
	return c.doStatuses(ctx, method, path, bearer, input, []int{wantStatus}, output)
}

func (c *PublicSessionClient) doStatuses(ctx context.Context, method, path, bearer string, input any, wantStatuses []int, output any) error {
	if c == nil || c.client == nil || bearer == "" || !strings.HasPrefix(path, "/api/v1/") {
		return errors.New("public session client: invalid request")
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("public session client: encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("public session client: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := *c.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("public session client: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	wanted := false
	for _, status := range wantStatuses {
		if resp.StatusCode == status {
			wanted = true
			break
		}
	}
	if !wanted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var detail struct {
			ErrorCode string `json:"error_code"`
		}
		if json.Unmarshal(body, &detail) == nil && validPublicErrorCode(detail.ErrorCode) {
			return fmt.Errorf("public session client: %s %s returned unexpected status %d error_code=%s", method, path, resp.StatusCode, detail.ErrorCode)
		}
		return fmt.Errorf("public session client: %s %s returned unexpected status %d", method, path, resp.StatusCode)
	}
	if output == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxPublicClientResponseBytes+1))
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("public session client: decode response: %w", err)
	}
	return nil
}

func validPublicErrorCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}
