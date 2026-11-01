package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestRecoverInterruptedActivationFencesPreparedSource(t *testing.T) {
	ctx, control, admin, source, _, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	config := source.Config().Copy()
	other, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer other.Close()
	require.NoError(t, other.Ping(ctx))
	require.NoError(t, controller.Shutdown(ctx))
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	require.Error(t, RecoverInterruptedActivation(ctx, control, admin, request, claim.Physical, "wrong_role"))
	wrong := claim.Physical
	wrong.DatabaseOID++
	require.Error(t, RecoverInterruptedActivation(ctx, control, admin, request, wrong, claim.WriterRole))
	require.NoError(t, RecoverInterruptedActivation(ctx, control, admin, request, claim.Physical, claim.WriterRole))
	var state, phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, login)
	require.Eventually(t, func() bool {
		bounded, stop := context.WithTimeout(ctx, 250*time.Millisecond)
		defer stop()
		return other.Ping(bounded) != nil
	}, 5*time.Second, 50*time.Millisecond)
}

func TestRecoverInterruptedActivationAfterGuardCutover(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixtureOptions(t, false)
	require.True(t, claim.GuardCutover)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.ActivateQualifiedGuards(ctx, admin, source, keys, claim, public))
	var state string
	var active, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, source.QueryRow(ctx, `SELECT guards_active FROM identity_meta WHERE singleton`).Scan(&active))
	require.True(t, active)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, login)
	require.NoError(t, controller.Shutdown(ctx))
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	require.NoError(t, RecoverInterruptedActivation(ctx, control, admin, request, claim.Physical, claim.WriterRole))
	var phase string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	// A new leader must use a fresh challenge and a new signed decision. The
	// recovered guard is retained; reactivation would incorrectly repeat the
	// baseline cutover.
	config := source.Config().Copy()
	source.Close()
	source, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer source.Close()
	var restarted *Controller
	require.Eventually(t, func() bool { restarted, err = Start(ctx, control, claim.Binding); return err == nil }, 10*time.Second, 50*time.Millisecond)
	defer func() { _ = restarted.Shutdown(ctx) }()
	require.Error(t, restarted.PrepareOpenSource(ctx, admin, claim, public), "the old leader's decision cannot be replayed")
	challenge, err := restarted.ActivationChallenge()
	require.NoError(t, err)
	newPublic, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	claim.Challenge = challenge
	claim.GuardCutover = false
	claim.Signature = nil
	claim.IssuedAtUnix = time.Now().Unix()
	claim.ExpiresAtUnix = time.Now().Add(time.Minute).Unix()
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	require.NoError(t, restarted.verifyOpenAttestation(claim, newPublic, time.Now()))
	var sessions, prepared int
	require.NoError(t, admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1 OR usename=$3),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2 OR owner=$3)`, claim.Physical.DatabaseOID, claim.Physical.Database, claim.WriterRole).Scan(&sessions, &prepared))
	require.Zero(t, sessions)
	require.Zero(t, prepared)
	require.NoError(t, restarted.PrepareOpenSource(ctx, admin, claim, newPublic))
	require.NoError(t, restarted.OpenQualified(ctx, admin, source, keys, claim, newPublic))
	permit, physical, err := restarted.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	require.Equal(t, claim.Physical, physical)
	require.NoError(t, permit.Release(ctx))
	var count int
	require.NoError(t, control.QueryRow(ctx, `SELECT count(*) FROM user_custody_open_evidence WHERE source_id=$1 AND epoch=$2`, claim.Binding.SourceID, claim.Binding.Epoch).Scan(&count))
	require.Equal(t, 2, count)
}

func TestRecoverInterruptedActivationFencesActiveWriter(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	config := source.Config().Copy()
	config.ConnConfig.User = "custody_app"
	config.ConnConfig.Password = "fixture_only"
	writer, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer writer.Close()
	require.NoError(t, writer.Ping(ctx))
	require.NoError(t, controller.Shutdown(ctx))
	request := CreationRequest{SourceID: claim.Binding.SourceID, Epoch: claim.Binding.Epoch, ControlSystemID: claim.ControlSystem, SourceSystemID: claim.Physical.SystemIdentifier, Database: claim.Physical.Database}
	require.NoError(t, RecoverInterruptedActivation(ctx, control, admin, request, claim.Physical, claim.WriterRole))
	var state, phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.False(t, login)
	require.Eventually(t, func() bool {
		bounded, stop := context.WithTimeout(ctx, 250*time.Millisecond)
		defer stop()
		return writer.Ping(bounded) != nil
	}, 5*time.Second, 50*time.Millisecond)
}

func TestRecoveryRequiredCannotReopenBeforeVerifiedFence(t *testing.T) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	other, err := pgxpool.NewWithConfig(ctx, source.Config().Copy())
	require.NoError(t, err)
	defer other.Close()
	require.NoError(t, other.Ping(ctx))
	require.Error(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
	var allowed, login bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE rolname=$2) FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID, claim.WriterRole).Scan(&allowed, &login))
	require.False(t, allowed)
	require.False(t, login)
	require.Eventually(t, func() bool {
		var count int
		return admin.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datid=$1 OR usename=$2`, claim.Physical.DatabaseOID, claim.WriterRole).Scan(&count) == nil && count == 0
	}, 5*time.Second, 25*time.Millisecond)
	var restarted *Controller
	require.Eventually(t, func() bool { restarted, err = Start(ctx, control, claim.Binding); return err == nil }, 10*time.Second, 50*time.Millisecond)
	defer func() { _ = restarted.Shutdown(ctx) }()
	challenge, err := restarted.ActivationChallenge()
	require.NoError(t, err)
	newPublic, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	claim.Challenge = challenge
	claim.Signature = nil
	claim.IssuedAtUnix = time.Now().Unix()
	claim.ExpiresAtUnix = time.Now().Add(time.Minute).Unix()
	claim, err = SignOpenAttestation(claim, private)
	require.NoError(t, err)
	require.NoError(t, restarted.verifyOpenAttestation(claim, newPublic, time.Now()))
	require.Error(t, restarted.PrepareOpenSource(ctx, admin, claim, newPublic), "even a fresh signature cannot replace a verified source fence")
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERY_REQUIRED", phase)
}
