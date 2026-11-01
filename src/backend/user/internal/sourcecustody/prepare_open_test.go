package sourcecustody

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrepareOpenHoldsPhysicalPinAcrossSourceOpening(t *testing.T) {
	ctx, control, admin, _, _, controller, claim, public, _, _ := openSourceFixture(t)
	entered := make(chan uint32, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	controller.afterPrepareIntent = func(pid uint32) { entered <- pid; <-release }
	result := make(chan error, 1)
	go func() { result <- controller.PrepareOpenSource(ctx, admin, claim, public) }()
	select {
	case <-entered:
	case err := <-result:
		t.Fatalf("preparation ended before the source opening: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("preparation did not record intent")
	}
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "PREPARING_OPEN", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed, "the intent must commit before opening the source")
	bounded, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, err := control.Exec(bounded, `UPDATE user_custody_physical_source SET database_name='changed' WHERE source_id=$1 AND epoch=$2`, claim.Binding.SourceID, claim.Binding.Epoch)
	cancel()
	require.Error(t, err, "the physical pin must remain locked through source opening")
	unblock()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("preparation did not finish")
	}
	require.NoError(t, controller.Shutdown(ctx))
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	require.NoError(t, RecoverInterruptedActivation(ctx, control, admin, request, claim.Physical, claim.WriterRole))
}

func TestPrepareOpenFencesIfPhysicalPinSessionDies(t *testing.T) {
	ctx, control, admin, _, _, controller, claim, public, _, _ := openSourceFixture(t)
	entered := make(chan uint32, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	controller.afterPrepareIntent = func(pid uint32) { entered <- pid; <-release }
	result := make(chan error, 1)
	go func() { result <- controller.PrepareOpenSource(ctx, admin, claim, public) }()
	var pid uint32
	select {
	case pid = <-entered:
	case err := <-result:
		t.Fatalf("preparation ended before the source opening: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("preparation did not record intent")
	}
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated)
	unblock()
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("preparation did not fail after losing its pin")
	}
	var phase, state string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, login)
}
