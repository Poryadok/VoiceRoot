package grpcsvc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type captureAnalytics struct {
	subject     string
	eventType   string
	accountID   string
	propsJSON   string
}

func (c *captureAnalytics) Publish(context.Context, string, string, string, map[string]any) error {
	return nil
}

func (c *captureAnalytics) PublishWithAccount(_ context.Context, subject, _ string, eventType, accountID string, props map[string]any) error {
	c.subject = subject
	c.eventType = eventType
	c.accountID = accountID
	b, _ := json.Marshal(props)
	c.propsJSON = string(b)
	return nil
}

func TestPublishPaymentEventOmitsAccountIDFromProps(t *testing.T) {
	cap := &captureAnalytics{}
	svc := &SubscriptionGRPC{Analytics: cap}
	accountID := "11111111-1111-1111-1111-111111111111"
	svc.publishPaymentEvent(context.Background(), "analytics.subscription.payment_success", "payment_success", accountID, "premium", "evt-1")
	require.Equal(t, accountID, cap.accountID)
	require.NotContains(t, cap.propsJSON, accountID)
	require.NotContains(t, cap.propsJSON, "account_id")
	require.Contains(t, cap.propsJSON, "provider_event_id")
}
