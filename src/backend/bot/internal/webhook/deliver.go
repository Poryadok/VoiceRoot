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
	InteractionToken string         `json:"interaction_token,omitempty"`
	CommandName      string         `json:"command_name"`
	OptionName       string         `json:"option_name,omitempty"`
	FocusedValue     string         `json:"focused_value,omitempty"`
	Options          map[string]any `json:"options,omitempty"`
	ChatID           string         `json:"chat_id"`
	ChatType         string         `json:"chat_type"`
	InvokerProfileID string         `json:"invoker_profile_id"`
}

// AutocompleteChoice is one autocomplete suggestion from the bot.
type AutocompleteChoice struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AutocompleteResponse is expected from bot webhook for autocomplete.
type AutocompleteResponse struct {
	Choices []AutocompleteChoice `json:"choices"`
}

// InteractionResponse is expected from bot webhook handler.
type InteractionResponse struct {
	Content   string `json:"content"`
	Ephemeral bool   `json:"ephemeral"`
	Deferred  bool   `json:"deferred"`
}

const maxDeliveryAttempts = 3

// DeliverPOST sends signed interaction to webhook and parses sync response.
// Retries up to maxDeliveryAttempts with exponential backoff on transport errors and 5xx.
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

	var lastErr error
	for attempt := 0; attempt < maxDeliveryAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return InteractionResponse{}, ctx.Err()
			case <-time.After(backoff):
			}
		}
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		resp, err := deliverOnce(reqCtx, client, url, secret, ts, body)
		cancel()
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableWebhookError(err) {
			break
		}
	}
	return InteractionResponse{}, lastErr
}

// DeliverAutocompletePOST sends an autocomplete request and parses choices (max 25).
func DeliverAutocompletePOST(ctx context.Context, client *http.Client, url, secret string, payload InteractionPayload, timeout time.Duration) ([]AutocompleteChoice, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	payload.Type = "autocomplete"
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	ts := time.Now().Unix()

	var lastErr error
	for attempt := 0; attempt < maxDeliveryAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		choices, err := deliverAutocompleteOnce(reqCtx, client, url, secret, ts, body)
		cancel()
		if err == nil {
			if len(choices) > 25 {
				choices = choices[:25]
			}
			return choices, nil
		}
		lastErr = err
		if !isRetryableWebhookError(err) {
			break
		}
	}
	return nil, lastErr
}

func deliverAutocompleteOnce(ctx context.Context, client *http.Client, url, secret string, ts int64, body []byte) ([]AutocompleteChoice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(url), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", ts))
	req.Header.Set(HeaderSignature, Sign(secret, ts, body))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out AutocompleteResponse
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Choices, nil
}

func deliverOnce(ctx context.Context, client *http.Client, url, secret string, ts int64, body []byte) (InteractionResponse, error) {
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
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return InteractionResponse{}, err
	}
	if resp.StatusCode >= 500 {
		return InteractionResponse{}, fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return InteractionResponse{}, fmt.Errorf("webhook status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out InteractionResponse
	if len(strings.TrimSpace(string(raw))) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return InteractionResponse{}, err
	}
	return out, nil
}

func isRetryableWebhookError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "webhook status 5")
}
