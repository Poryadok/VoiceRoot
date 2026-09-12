package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
)

const (
	r23RetirementUp               = "000012_space_retirement.up.sql"
	r23RetirementDown             = "000012_space_retirement.down.sql"
	r23RetirementDownRefusalState = "55000"
)

type r23RetirementEvidence struct {
	spaceID, operationID, manifestID, receiptID uuid.UUID
	generation                                  int64
	manifestItemCount                           uint64
	purgeDecidedAt, retiredAt                   time.Time
	requestHash, manifestHash                   []byte
	requestBytes, receiptBytes                  []byte
}

func r23RetirementMigrationSQL(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db", name))
	require.NoError(t, err, "R23 permanent-retirement migration must exist")
	return string(b)
}

func applyR23RetirementBase(t *testing.T, ctx context.Context) *RoleStore {
	t.Helper()
	pool := StartRoleDBForStoreTest(t, ctx)
	t.Cleanup(pool.Close)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	applyR22RoleEpochMigration(t, ctx, &RoleStore{Pool: pool})
	return &RoleStore{Pool: pool}
}

func r23DownError(t *testing.T, err error) *pgconn.PgError {
	t.Helper()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "DOWN refusal must be a named PostgreSQL error")
	require.Equal(t, r23RetirementDownRefusalState, pgErr.Code)
	return pgErr
}

func r23EvidenceAt(t *testing.T, retiredAt time.Time) r23RetirementEvidence {
	t.Helper()
	e := r23RetirementEvidence{
		spaceID: uuid.New(), operationID: uuid.New(), manifestID: uuid.New(), receiptID: uuid.New(), generation: 7, manifestItemCount: 23,
		purgeDecidedAt: retiredAt.Add(-time.Minute).UTC().Truncate(time.Microsecond),
		retiredAt:      retiredAt.UTC().Truncate(time.Microsecond),
		manifestHash:   bytes.Repeat([]byte{0x5a}, sha256.Size),
	}
	request := &rolev1.RetireSpaceRequest{
		ProtocolVersion: 1, SpaceId: e.spaceID.String(), DeletionOperationId: e.operationID.String(), Generation: uint64(e.generation),
		PurgeDecidedAt: timestamppb.New(e.purgeDecidedAt),
		Manifest:       &commonv1.ManifestBinding{ManifestId: e.manifestID.String(), ManifestSha256: e.manifestHash, ItemCount: e.manifestItemCount},
	}
	var err error
	e.requestBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	e.requestHash = r23DomainHash("voice.role.v1.RetireSpaceRequest", e.requestBytes)
	receipt := &rolev1.RetireSpaceResponse{Receipt: &rolev1.RetireSpaceReceipt{
		ProtocolVersion: 1, ReceiptId: e.receiptID.String(), SpaceId: e.spaceID.String(), DeletionOperationId: e.operationID.String(),
		Generation: uint64(e.generation), State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED,
		RequestSha256: e.requestHash, ManifestSha256: e.manifestHash, RetiredAt: timestamppb.New(e.retiredAt),
	}}
	e.receiptBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	require.NoError(t, err)
	return e
}

func r23DomainHash(name string, wire []byte) []byte {
	digest := sha256.Sum256(append(append([]byte(name), 0), wire...))
	return digest[:]
}

func insertR23Evidence(t *testing.T, ctx context.Context, st *RoleStore, e r23RetirementEvidence) {
	t.Helper()
	_, err := st.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,$2)`, e.spaceID, e.retiredAt)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO role_space_retirement_receipts
 (space_id,deletion_operation_id,protocol_version,generation,purge_decided_at,manifest_id,manifest_item_count,receipt_id,
  request_sha256,manifest_sha256,request_bytes,receipt_bytes,retired_at)
 VALUES($1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		e.spaceID, e.operationID, e.generation, e.purgeDecidedAt, e.manifestID, e.manifestItemCount, e.receiptID,
		e.requestHash, e.manifestHash, e.requestBytes, e.receiptBytes, e.retiredAt)
	require.NoError(t, err)
}

func TestSpaceRetirementMigration_UpPreservesLifecycleAndPersistsExactBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st := applyR23RetirementBase(t, ctx)
	preexisting := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,NULL)`, preexisting)
	require.NoError(t, err)
	var beforeOID uint32
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT 'role_space_lifecycle'::regclass::oid`).Scan(&beforeOID))
	_, err = st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
	require.NoError(t, err)
	var afterOID uint32
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT 'role_space_lifecycle'::regclass::oid`).Scan(&afterOID))
	require.Equal(t, beforeOID, afterOID, "000012 must extend, never replace, the 000010 lifecycle table")
	var preexistingRows int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NULL`, preexisting).Scan(&preexistingRows))
	require.Equal(t, 1, preexistingRows)

	var databaseNow time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow))
	e := r23EvidenceAt(t, databaseNow)
	insertR23Evidence(t, ctx, st, e)
	var requestBytes, receiptBytes, requestHash, manifestHash []byte
	var manifestID uuid.UUID
	var manifestItemCount uint64
	var retiredAt, retainUntil time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT request_bytes,receipt_bytes,request_sha256,manifest_sha256,manifest_id,manifest_item_count,retired_at,full_bytes_retain_until
 FROM role_space_retirement_receipts WHERE space_id=$1`, e.spaceID).Scan(&requestBytes, &receiptBytes, &requestHash, &manifestHash, &manifestID, &manifestItemCount, &retiredAt, &retainUntil))
	require.Equal(t, e.requestBytes, requestBytes)
	require.Equal(t, e.receiptBytes, receiptBytes)
	require.Equal(t, e.requestHash, requestHash)
	require.Equal(t, e.manifestHash, manifestHash)
	require.Equal(t, e.manifestID, manifestID)
	require.Equal(t, e.manifestItemCount, manifestItemCount)
	require.Equal(t, e.retiredAt, retiredAt.UTC())
	require.Equal(t, e.retiredAt.Add(30*24*time.Hour), retainUntil.UTC(), "full-byte retention boundary derives exactly from database retired_at")
	_, err = st.Pool.Exec(ctx, `UPDATE role_space_retirement_receipts SET request_bytes=NULL,receipt_bytes=NULL WHERE space_id=$1`, e.spaceID)
	require.Error(t, err, "full replay bytes cannot compact before the database-derived boundary")
}

func TestSpaceRetirementMigration_ExpiredFullBytesCompactButPermanentEvidenceCannotChange(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st := applyR23RetirementBase(t, ctx)
	_, err := st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
	require.NoError(t, err)
	var expiredRetiredAt time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()-interval '31 days'`).Scan(&expiredRetiredAt))
	e := r23EvidenceAt(t, expiredRetiredAt)
	insertR23Evidence(t, ctx, st, e)
	_, err = st.Pool.Exec(ctx, `UPDATE role_space_retirement_receipts SET request_bytes=NULL,receipt_bytes=NULL WHERE space_id=$1`, e.spaceID)
	require.NoError(t, err)
	var requestBytes, receiptBytes []byte
	var operationID, manifestID, receiptID uuid.UUID
	var generation int64
	var manifestItemCount uint64
	var requestHash, manifestHash []byte
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT request_bytes,receipt_bytes,deletion_operation_id,manifest_id,manifest_item_count,receipt_id,generation,request_sha256,manifest_sha256
 FROM role_space_retirement_receipts WHERE space_id=$1`, e.spaceID).Scan(&requestBytes, &receiptBytes, &operationID, &manifestID, &manifestItemCount, &receiptID, &generation, &requestHash, &manifestHash))
	require.Nil(t, requestBytes)
	require.Nil(t, receiptBytes)
	require.Equal(t, e.operationID, operationID)
	require.Equal(t, e.manifestID, manifestID)
	require.Equal(t, e.manifestItemCount, manifestItemCount)
	require.Equal(t, e.receiptID, receiptID)
	require.Equal(t, e.generation, generation)
	require.Equal(t, e.requestHash, requestHash)
	require.Equal(t, e.manifestHash, manifestHash)
	for _, statement := range []string{
		`UPDATE role_space_retirement_receipts SET generation=8 WHERE space_id=$1`,
		`UPDATE role_space_retirement_receipts SET manifest_id=gen_random_uuid() WHERE space_id=$1`,
		`UPDATE role_space_retirement_receipts SET manifest_item_count=24 WHERE space_id=$1`,
		`DELETE FROM role_space_retirement_receipts WHERE space_id=$1`,
		`UPDATE role_space_lifecycle SET retired_at=NULL WHERE space_id=$1`,
		`DELETE FROM role_space_lifecycle WHERE space_id=$1`,
	} {
		_, err = st.Pool.Exec(ctx, statement, e.spaceID)
		require.Error(t, err, "compact receipt and retirement fence are permanent")
	}
}

func TestSpaceRetirementMigration_RetiredLifecycleIdentityCannotMovePastAuthorityFence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st := applyR23RetirementBase(t, ctx)
	_, err := st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
	require.NoError(t, err)
	var databaseNow time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow))
	e := r23EvidenceAt(t, databaseNow)
	insertR23Evidence(t, ctx, st, e)
	movedSpaceID := uuid.New()

	_, err = st.Pool.Exec(ctx, `UPDATE role_space_lifecycle SET space_id=$2 WHERE space_id=$1`, e.spaceID, movedSpaceID)
	require.Error(t, err, "a permanent retirement fence cannot move to another Space identity")
	var originalRows, movedRows int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL),
 (SELECT count(*) FROM role_space_lifecycle WHERE space_id=$2)`, e.spaceID, movedSpaceID).Scan(&originalRows, &movedRows))
	require.Equal(t, 1, originalRows)
	require.Zero(t, movedRows)

	callbackReached := false
	err = st.WithinSpaces(ctx, []uuid.UUID{e.spaceID}, func(*RoleStore) error {
		callbackReached = true
		return nil
	})
	require.ErrorIs(t, err, ErrSpaceRetired)
	require.False(t, callbackReached, "the original retired identity must remain fenced before authority work")
}

func TestSpaceRetirementMigration_DownPreservesPreexistingLifecycleWhenEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st := applyR23RetirementBase(t, ctx)
	preexisting := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,NULL)`, preexisting)
	require.NoError(t, err)
	var beforeOID uint32
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT 'role_space_lifecycle'::regclass::oid`).Scan(&beforeOID))
	_, err = st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementDown))
	require.NoError(t, err)
	var afterOID uint32
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT 'role_space_lifecycle'::regclass::oid`).Scan(&afterOID))
	require.Equal(t, beforeOID, afterOID)
	var rows int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NULL`, preexisting).Scan(&rows))
	require.Equal(t, 1, rows)
	var receiptTable *string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regclass('role_space_retirement_receipts')::text`).Scan(&receiptTable))
	require.Nil(t, receiptTable)
}

func TestSpaceRetirementMigration_DownRefusesNamedPermanentEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	for _, evidence := range []string{"fence", "receipt"} {
		t.Run(evidence, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			st := applyR23RetirementBase(t, ctx)
			_, err := st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
			require.NoError(t, err)
			var databaseNow time.Time
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow))
			e := r23EvidenceAt(t, databaseNow)
			if evidence == "fence" {
				_, err = st.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,$2)`, e.spaceID, e.retiredAt)
				require.NoError(t, err)
			} else {
				insertR23Evidence(t, ctx, st, e)
			}
			_, err = st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementDown))
			pgErr := r23DownError(t, err)
			require.Contains(t, pgErr.Message, "permanent Role retirement evidence")
			var fenceTable, receiptTable string
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regclass('role_space_lifecycle')::text,to_regclass('role_space_retirement_receipts')::text`).Scan(&fenceTable, &receiptTable))
			require.Equal(t, "role_space_lifecycle", fenceTable)
			require.Equal(t, "role_space_retirement_receipts", receiptTable)
		})
	}
}

func TestSpaceRetirementMigration_DownWaitsThenRefusesConcurrentRetirement(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st := applyR23RetirementBase(t, ctx)
	_, err := st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
	require.NoError(t, err)
	writer, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	spaceID := uuid.New()
	_, err = writer.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,clock_timestamp())`, spaceID)
	require.NoError(t, err)
	downConn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer downConn.Release()
	pid := downConn.Conn().PgConn().PID()
	downSQL := r23RetirementMigrationSQL(t, r23RetirementDown)
	done := make(chan error, 1)
	go func() { _, downErr := downConn.Exec(ctx, downSQL); done <- downErr }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT wait_event_type='Lock' FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	r23DownError(t, <-done)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL`, spaceID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestSpaceRetirementMigration_DownWaitsForActualReceiptFirstStoreRetirementThenRefuses(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st := applyR23RetirementBase(t, ctx)
	_, err := st.Pool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementUp))
	require.NoError(t, err)

	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `LOCK TABLE role_space_lifecycle IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)

	e := r23EvidenceAt(t, time.Now().UTC())
	retirementPool := r22SecondRolePool(t, ctx, st.Pool, "r23-actual-retirement")
	retirementDone := make(chan error, 1)
	go func() {
		_, retireErr := (&RoleStore{Pool: retirementPool}).RetireSpace(ctx, SpaceRetirementInput{
			ProtocolVersion: 1, SpaceID: e.spaceID, DeletionOperationID: e.operationID,
			Generation: uint64(e.generation), PurgeDecidedAt: e.purgeDecidedAt,
			ManifestID: e.manifestID, ManifestItemCount: e.manifestItemCount,
			RequestSHA256: e.requestHash, ManifestSHA256: e.manifestHash, RequestBytes: e.requestBytes,
		}, func(fields SpaceRetirementReceipt) ([]byte, error) {
			return (proto.MarshalOptions{Deterministic: true}).Marshal(&rolev1.RetireSpaceResponse{Receipt: &rolev1.RetireSpaceReceipt{
				ProtocolVersion: fields.ProtocolVersion, ReceiptId: fields.ReceiptID.String(), SpaceId: fields.SpaceID.String(),
				DeletionOperationId: fields.DeletionOperationID.String(), Generation: fields.Generation,
				State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: fields.RequestSHA256,
				ManifestSha256: fields.ManifestSHA256, RetiredAt: timestamppb.New(fields.RetiredAt),
			}})
		})
		retirementDone <- retireErr
	}()
	require.Eventually(t, func() bool {
		var ready bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM pg_stat_activity a
 JOIN pg_locks held ON held.pid=a.pid AND held.locktype='relation' AND held.granted
 JOIN pg_class c ON c.oid=held.relation
 WHERE a.application_name='r23-actual-retirement' AND a.wait_event_type='Lock'
   AND c.relname='role_space_retirement_receipts' AND held.mode='AccessShareLock')`).Scan(&ready)
		return err == nil && ready
	}, 5*time.Second, 20*time.Millisecond, "actual store retirement must hold receipt access before waiting on lifecycle")

	downPool := r22SecondRolePool(t, ctx, st.Pool, "r23-receipt-first-down")
	downDone := make(chan error, 1)
	go func() {
		_, downErr := downPool.Exec(ctx, r23RetirementMigrationSQL(t, r23RetirementDown))
		downDone <- downErr
	}()
	require.Eventually(t, func() bool {
		var waitingForReceipt bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM pg_stat_activity a
 JOIN pg_locks waiting ON waiting.pid=a.pid AND waiting.locktype='relation' AND NOT waiting.granted
 JOIN pg_class c ON c.oid=waiting.relation
 WHERE a.application_name='r23-receipt-first-down'
   AND c.relname='role_space_retirement_receipts' AND waiting.mode='AccessExclusiveLock')`).Scan(&waitingForReceipt)
		return err == nil && waitingForReceipt
	}, 5*time.Second, 20*time.Millisecond, "DOWN must wait on the receipt table before lifecycle")

	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-retirementDone)
	pgErr := r23DownError(t, <-downDone)
	require.Contains(t, pgErr.Message, "permanent Role retirement evidence")
	var fenceTable, receiptTable string
	var fenceRows, receiptRows int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT
 to_regclass('role_space_lifecycle')::text,
 to_regclass('role_space_retirement_receipts')::text,
 (SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL),
 (SELECT count(*) FROM role_space_retirement_receipts WHERE space_id=$1)`, e.spaceID).Scan(&fenceTable, &receiptTable, &fenceRows, &receiptRows))
	require.Equal(t, "role_space_lifecycle", fenceTable)
	require.Equal(t, "role_space_retirement_receipts", receiptTable)
	require.Equal(t, 1, fenceRows)
	require.Equal(t, 1, receiptRows)
}
