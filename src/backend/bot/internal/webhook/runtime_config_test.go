package webhook_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/webhook"
)

type configTransport func(*http.Request) (*http.Response, error)

func (f configTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWebhookRuntimeAttemptBudget(t *testing.T) {
	for _, autocomplete := range []bool{false, true} {
		for _, tc := range []struct {
			name, value string
			want        int
		}{
			{"single", "1", 1}, {"configured", "2", 2}, {"trimmed", " 2 ", 2},
			{"default", "", 3}, {"invalid", "invalid", 3}, {"zero", "0", 3}, {"negative", "-1", 3},
		} {
			t.Run(tc.name+endpointName(autocomplete), func(t *testing.T) {
				t.Setenv("BOT_WEBHOOK_DELIVERY_ATTEMPTS", tc.value)
				t.Setenv("BOT_WEBHOOK_RETRY_BACKOFF_MS", "1")
				hits := 0
				client := &http.Client{Transport: configTransport(func(*http.Request) (*http.Response, error) {
					hits++
					return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(""))}, nil
				})}
				err := deliverConfigured(context.Background(), client, autocomplete)
				require.Error(t, err)
				require.Equal(t, tc.want, hits, "attempt budget includes the initial request")
			})
		}
	}
}

func TestWebhookRuntimeBackoffHonorsCancellation(t *testing.T) {
	for _, autocomplete := range []bool{false, true} {
		for _, value := range []string{"1000", "", "invalid", "0", "-1"} {
			t.Run(value+endpointName(autocomplete), func(t *testing.T) {
				t.Setenv("BOT_WEBHOOK_DELIVERY_ATTEMPTS", "3")
				t.Setenv("BOT_WEBHOOK_RETRY_BACKOFF_MS", value)
				hits := 0
				// Cancel from the first completed attempt. A canceled backoff must
				// never start another HTTP request, including for invalid config.
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client := &http.Client{Transport: configTransport(func(*http.Request) (*http.Response, error) {
					hits++
					cancel()
					return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(""))}, nil
				})}
				require.ErrorIs(t, deliverConfigured(ctx, client, autocomplete), context.Canceled)
				require.Equal(t, 1, hits)
			})
		}
	}
}

func TestWebhookRuntimeConfiguredBackoff(t *testing.T) {
	for _, autocomplete := range []bool{false, true} {
		t.Run(endpointName(autocomplete), func(t *testing.T) {
			t.Setenv("BOT_WEBHOOK_DELIVERY_ATTEMPTS", "2")
			t.Setenv("BOT_WEBHOOK_RETRY_BACKOFF_MS", "200")
			hits := 0
			var first time.Time
			client := &http.Client{Transport: configTransport(func(*http.Request) (*http.Response, error) {
				hits++
				if hits == 1 {
					first = time.Now()
				} else {
					require.GreaterOrEqual(t, time.Since(first), 200*time.Millisecond)
				}
				return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}
			require.Error(t, deliverConfigured(context.Background(), client, autocomplete))
			require.Equal(t, 2, hits)
		})
	}
}

func endpointName(autocomplete bool) string {
	if autocomplete {
		return "/autocomplete"
	}
	return "/interaction"
}

func deliverConfigured(ctx context.Context, client *http.Client, autocomplete bool) error {
	if autocomplete {
		_, err := webhook.DeliverAutocompletePOST(ctx, client, "https://bot.invalid/hook", "secret", webhook.InteractionPayload{}, time.Second)
		return err
	}
	_, err := webhook.DeliverPOST(ctx, client, "https://bot.invalid/hook", "secret", webhook.InteractionPayload{}, time.Second)
	return err
}
