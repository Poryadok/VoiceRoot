// Package mmadmission checks a separately installed MM native admission binding.
// It cannot establish installation custody or issue/mutate admission receipts.
package mmadmission

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrUnavailable = errors.New("matchmaking native admission unavailable")

type Binding struct {
	ProtocolVersion                                                                                                                      uint32
	EnvironmentID, InstallationID, AuthorityIncarnation, TransitionID                                                                    uuid.UUID
	AdmissionGeneration                                                                                                                  uint64
	CredentialCohortID                                                                                                                   uuid.UUID
	DatabaseBindingHash, SchemaContractHash, SourceFactoryManifestHash, FreshRouteManifestHash, AdmissionReceiptHash, LaunchManifestHash [32]byte
	ExpectedLoginRoleOID                                                                                                                 uint32
	ExpectedLoginRoleName                                                                                                                string
}

// Validate checks binding shape, not external installation authority.
func (b *Binding) Validate() error {
	if b == nil || b.ProtocolVersion != 1 || b.EnvironmentID == uuid.Nil || b.InstallationID == uuid.Nil || b.AuthorityIncarnation == uuid.Nil || b.TransitionID == uuid.Nil || b.AdmissionGeneration == 0 || b.AdmissionGeneration > math.MaxInt64 || b.CredentialCohortID == uuid.Nil || b.ExpectedLoginRoleOID == 0 || strings.TrimSpace(b.ExpectedLoginRoleName) == "" {
		return ErrUnavailable
	}
	for _, digest := range [][32]byte{b.DatabaseBindingHash, b.SchemaContractHash, b.SourceFactoryManifestHash, b.FreshRouteManifestHash, b.AdmissionReceiptHash, b.LaunchManifestHash} {
		if digest == [32]byte{} {
			return ErrUnavailable
		}
	}
	return nil
}

// Acquire is the FIRST cooperating transition gate, before account and row
// locks. It retains a shared transaction lock until commit/rollback. The
// installation closer must acquire AcquireCloseGate before changing the head.
// The fresh READ COMMITTED reads follow any actual wait for that closer.
func Acquire(ctx context.Context, tx pgx.Tx, expected *Binding) error {
	var want Binding
	if expected != nil {
		want = *expected
		if want.Validate() != nil {
			return ErrUnavailable
		}
	}
	if acquireGate(ctx, tx, true) != nil {
		return ErrUnavailable
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT pg_catalog.to_regclass('public.mm_native_admission_head') IS NOT NULL`).Scan(&exists); err != nil {
		return ErrUnavailable
	}
	if !exists {
		if expected == nil {
			return nil
		}
		return ErrUnavailable
	}
	var state string
	err := tx.QueryRow(ctx, `SELECT state FROM public.mm_native_admission_head WHERE singleton`).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) && expected == nil {
		return nil
	}
	if err != nil || state != "ADMITTED" || expected == nil {
		return ErrUnavailable
	}
	var actual Binding
	hashes := make([][]byte, 6)
	err = tx.QueryRow(ctx, `SELECT protocol_version,environment_id,installation_id,authority_incarnation,transition_id,admission_generation,credential_cohort_id,
database_binding_sha256,schema_contract_sha256,source_factory_manifest_sha256,fresh_route_manifest_sha256,admission_receipt_sha256,launch_manifest_sha256,
expected_login_role_oid,expected_login_role_name FROM public.mm_native_admission_head WHERE singleton AND state='ADMITTED'`).Scan(
		&actual.ProtocolVersion, &actual.EnvironmentID, &actual.InstallationID, &actual.AuthorityIncarnation, &actual.TransitionID, &actual.AdmissionGeneration, &actual.CredentialCohortID,
		&hashes[0], &hashes[1], &hashes[2], &hashes[3], &hashes[4], &hashes[5], &actual.ExpectedLoginRoleOID, &actual.ExpectedLoginRoleName)
	if err != nil {
		return ErrUnavailable
	}
	destinations := []*[32]byte{&actual.DatabaseBindingHash, &actual.SchemaContractHash, &actual.SourceFactoryManifestHash, &actual.FreshRouteManifestHash, &actual.AdmissionReceiptHash, &actual.LaunchManifestHash}
	for i, raw := range hashes {
		if len(raw) != 32 {
			return ErrUnavailable
		}
		copy(destinations[i][:], raw)
	}
	if actual.Validate() != nil || actual != want {
		return ErrUnavailable
	}
	return nil
}

// AcquireCloseGate serializes a real installation closer against admitted
// transactions. It performs no head mutation, privilege grant or admission.
// Merely acquiring this lock is never predecessor-process retirement evidence.
func AcquireCloseGate(ctx context.Context, tx pgx.Tx) error {
	return acquireGate(ctx, tx, false)
}

// AcquireTerminal is only for exact terminal decisions retaining the current
// binding. It never qualifies a forward transition: CLOSED remains refused by
// Acquire. The shared FIRST gate is retained through the terminal COMMIT.
func AcquireTerminal(ctx context.Context, tx pgx.Tx, expected *Binding) error {
	if expected == nil || expected.Validate() != nil {
		return ErrUnavailable
	}
	want := *expected
	if err := acquireGate(ctx, tx, true); err != nil {
		return err
	}
	_, err := ReadTerminalHistory(ctx, tx, WireBinding(want))
	return err
}

func acquireGate(ctx context.Context, tx pgx.Tx, shared bool) error {
	if tx == nil {
		return ErrUnavailable
	}
	var isolation string
	if err := tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation); err != nil || isolation != "read committed" {
		return ErrUnavailable
	}
	// One stable key across incarnations: a new binding cannot miss an old
	// in-flight transaction. PostgreSQL advisory locks are database-local.
	key := nativeGateKey()
	query := `SELECT pg_catalog.pg_advisory_xact_lock($1)`
	if shared {
		query = `SELECT pg_catalog.pg_advisory_xact_lock_shared($1)`
	}
	if _, err := tx.Exec(ctx, query, key); err != nil {
		return ErrUnavailable
	}
	return nil
}

func nativeGateKey() int64 {
	digest := sha256.Sum256([]byte("voice.mm.native-admission.gate.v1\x00"))
	return int64(binary.BigEndian.Uint64(digest[:8]))
}
