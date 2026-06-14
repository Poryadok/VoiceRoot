package grpcsvc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/integrationtest"
	"voice/backend/subscription/internal/billing"
	"voice/backend/subscription/internal/store"

	subscriptionv1 "voice.app/voice/subscription/v1"
)

const subscriptionSchemaSQL = `
CREATE TABLE IF NOT EXISTS subscriptions (
	id UUID PRIMARY KEY,
	account_id UUID NOT NULL,
	plan TEXT NOT NULL,
	billing_period TEXT NOT NULL,
	status TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_subscription_id TEXT NOT NULL,
	current_period_start TIMESTAMPTZ NOT NULL,
	current_period_end TIMESTAMPTZ NOT NULL,
	grace_period_end TIMESTAMPTZ,
	cancelled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS space_subscriptions (
	id UUID PRIMARY KEY,
	space_id UUID NOT NULL,
	purchaser_account_id UUID NOT NULL,
	plan TEXT NOT NULL,
	billing_period TEXT NOT NULL,
	status TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_subscription_id TEXT NOT NULL,
	current_period_start TIMESTAMPTZ NOT NULL,
	current_period_end TIMESTAMPTZ NOT NULL,
	grace_period_end TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS billing_events (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	subscription_id UUID REFERENCES subscriptions(id),
	space_subscription_id UUID REFERENCES space_subscriptions(id),
	type TEXT NOT NULL,
	amount NUMERIC(12,2),
	currency TEXT,
	provider TEXT NOT NULL,
	provider_event_id TEXT NOT NULL,
	details JSONB NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (provider, provider_event_id)
);
`

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startSubscriptionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "subscriptiondb", "")
	_, err := pool.Exec(ctx, subscriptionSchemaSQL)
	require.NoError(t, err)
	return pool
}

func startSubscriptionGRPCTestServer(t *testing.T, pool *pgxpool.Pool) (subscriptionv1.SubscriptionServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	st := &store.SubscriptionStore{Pool: pool}
	subscriptionv1.RegisterSubscriptionServiceServer(srv, NewSubscriptionGRPC(st))
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
	}
	return subscriptionv1.NewSubscriptionServiceClient(conn), cleanup
}

func premiumActivatedWebhookBody(t *testing.T, accountID uuid.UUID) (string, string) {
	t.Helper()
	eventID := "evt_paddle_" + uuid.New().String()
	body, err := json.Marshal(map[string]any{
		"event_id":   eventID,
		"event_type": "subscription.activated",
		"data": map[string]any{
			"custom_data": map[string]string{
				"account_id": accountID.String(),
				"plan":       "premium",
			},
			"status": "active",
		},
	})
	require.NoError(t, err)
	return string(body), eventID
}

func paymentFailedWebhookBody(t *testing.T, accountID uuid.UUID) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"event_id":   "evt_failed_" + uuid.New().String(),
		"event_type": "subscription.payment_failed",
		"data": map[string]any{
			"custom_data": map[string]string{"account_id": accountID.String()},
		},
	})
	require.NoError(t, err)
	return string(body)
}

func spaceProActivatedWebhookBody(t *testing.T, spaceID, purchaserID uuid.UUID) (string, string) {
	t.Helper()
	eventID := "evt_space_" + uuid.New().String()
	body, err := json.Marshal(map[string]any{
		"event_id":   eventID,
		"event_type": "subscription.activated",
		"data": map[string]any{
			"custom_data": map[string]string{
				"space_id":     spaceID.String(),
				"purchaser_id": purchaserID.String(),
				"plan":         "space_pro",
			},
			"status": "active",
		},
	})
	require.NoError(t, err)
	return string(body), eventID
}

func signedWebhook(t *testing.T, body string) string {
	t.Helper()
	secret := os.Getenv("PADDLE_WEBHOOK_SECRET")
	if secret == "" {
		secret = billing.WebhookSecret()
	}
	ts := strconv.FormatInt(time.Unix(1700000000, 0).Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(ts + ":" + body))
	require.NoError(t, err)
	return "ts=" + ts + ",h1=" + hex.EncodeToString(mac.Sum(nil))
}

func limitsJSONInt(t *testing.T, limitsJSON, key string) int64 {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(limitsJSON), &m))
	v, ok := m[key]
	require.True(t, ok, "missing limit %q in %s", key, limitsJSON)
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		t.Fatalf("limit %q has unexpected type %T", key, v)
		return 0
	}
}
