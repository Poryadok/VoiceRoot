package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// InteractionPayload is POSTed to bot webhook_url.
type InteractionPayload struct {
	Type             string         `json:"type"`
	InteractionToken string         `json:"interaction_token"`
	CommandName      string         `json:"command_name"`
	Options          map[string]any `json:"options"`
	ChatID           string         `json:"chat_id"`
	ChatType         string         `json:"chat_type"`
	InvokerProfileID string         `json:"invoker_profile_id"`
}

// InteractionResponse is expected from bot webhook handler.
type InteractionResponse struct {
	Content   string `json:"content"`
	Ephemeral bool   `json:"ephemeral"`
	Deferred  bool   `json:"deferred"`
}

// DeliverPOST sends signed interaction to webhook and parses sync response.
func DeliverPOST(ctx context.Context, client *http.Client, url, secret string, payload InteractionPayload, timeout time.Duration) (InteractionResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return InteractionResponse{}, err
	}
	ts := time.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(url), bytes.NewReader(body))
	if err != nil {
		return InteractionResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", ts))
	req.Header.Set(HeaderSignature, Sign(secret, ts, body))
	resp, err := client.Do(req)
	if err != nil {
		return InteractionResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return InteractionResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return InteractionResponse{}, fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out InteractionResponse
	if len(bytes.TrimSpace(raw)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return InteractionResponse{}, err
	}
	return out, nil
}
