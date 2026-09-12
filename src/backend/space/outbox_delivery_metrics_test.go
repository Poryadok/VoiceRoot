package main

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"voice/backend/space/internal/outboxdelivery"
)

type fixedOwnershipOutboxAlertCount int64

func (count fixedOwnershipOutboxAlertCount) CountAlertingOwnershipOutbox(context.Context) (int64, error) {
	return int64(count), nil
}

type successfulOwnershipOutboxDispatch struct{}

func (successfulOwnershipOutboxDispatch) DispatchOnce(context.Context) error { return nil }

func TestOwnershipOutboxDeliveryMetrics_RegistersExactGaugeAndProjectsDurableValue(t *testing.T) {
	registry := prometheus.NewRegistry()
	gauge := newOwnershipOutboxDeliveryAlertGauge(registry)
	monitor := outboxdelivery.NewAlertMonitor(
		fixedOwnershipOutboxAlertCount(13),
		gauge,
		successfulOwnershipOutboxDispatch{},
	)
	require.NoError(t, monitor.Refresh(context.Background()))

	families, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, families, 1, "the Space registry must contain one ownership outbox alert metric family")
	family := families[0]
	require.Equal(t, "space_ownership_outbox_delivery_alerting_events", family.GetName())
	require.Equal(t, "GAUGE", family.GetType().String())
	require.Len(t, family.GetMetric(), 1, "the unlabeled alert gauge must be registered exactly once")
	require.Empty(t, family.GetMetric()[0].GetLabel(), "the aggregate durable count must not create label cardinality")
	require.Equal(t, float64(13), family.GetMetric()[0].GetGauge().GetValue(),
		"AlertMonitor must expose the exact durable PostgreSQL count")
}
