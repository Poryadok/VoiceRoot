package outboxdelivery

import (
	"context"
	"errors"
	"fmt"
)

type alertingCounter interface {
	CountAlertingOwnershipOutbox(context.Context) (int64, error)
}

type alertGauge interface {
	Set(float64)
}

// AlertMonitor projects the durable alerting-row count and decorates a
// dispatcher so each completed pass refreshes the projection, including passes
// that return a publish or acknowledgement error.
type AlertMonitor struct {
	counter    alertingCounter
	gauge      alertGauge
	dispatcher dispatcher
}

func NewAlertMonitor(counter alertingCounter, gauge alertGauge, next dispatcher) *AlertMonitor {
	return &AlertMonitor{counter: counter, gauge: gauge, dispatcher: next}
}

// Refresh synchronously queries PostgreSQL-backed state before changing the
// gauge. Query or cancellation failures preserve the last observed value.
func (m *AlertMonitor) Refresh(ctx context.Context) error {
	if m == nil || m.counter == nil || m.gauge == nil {
		return errors.New("ownership outbox alert monitor not configured")
	}
	if ctx == nil {
		return errors.New("ownership outbox alert monitor context is nil")
	}
	count, err := m.counter.CountAlertingOwnershipOutbox(ctx)
	if err != nil {
		return fmt.Errorf("refresh ownership outbox alert count: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.gauge.Set(float64(count))
	return nil
}

// DispatchOnce refreshes durable alert state after the wrapped delivery pass.
// errors.Join retains both failures when delivery and observation fail.
func (m *AlertMonitor) DispatchOnce(ctx context.Context) error {
	if m == nil || m.dispatcher == nil {
		return errors.New("ownership outbox alert monitor dispatcher not configured")
	}
	dispatchErr := m.dispatcher.DispatchOnce(ctx)
	refreshErr := m.Refresh(ctx)
	return errors.Join(dispatchErr, refreshErr)
}
