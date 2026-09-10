package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func ownershipJournalBindingFixture() OwnershipBinding {
	return OwnershipBinding{ProtocolVersion: 2,
		OperationID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SpaceID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		AccountID:         uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		ActorProfileID:    uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		NewOwnerProfileID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		SessionEpoch:      0x0102030405060708, ProofDigest: strings.Repeat("ab", 32)}
}

func TestOwnershipJournalBindingCodec_ExactVersionedTypedEncoding(t *testing.T) {
	binding := ownershipJournalBindingFixture()
	// Literal wire fixture keeps UUID order, epoch endian and decoded digest observable.
	suffix, err := hex.DecodeString("00000002" + strings.Repeat("11", 16) + strings.Repeat("22", 16) + strings.Repeat("33", 16) + strings.Repeat("44", 16) + strings.Repeat("55", 16) + "0102030405060708" + strings.Repeat("ab", 32))
	require.NoError(t, err)
	expected := append([]byte("voice.space.ownership.binding.v1\x00"), suffix...)
	got, hash, err := EncodeOwnershipBinding(binding)
	require.NoError(t, err)
	require.Equal(t, expected, got)
	require.Equal(t, sha256.Sum256(expected), hash)
	rebuilt := ownershipJournalBindingFixture()
	again, againHash, err := EncodeOwnershipBinding(rebuilt)
	require.NoError(t, err)
	require.Equal(t, got, again)
	require.Equal(t, hash, againHash)
	got[0] ^= 0xff
	final, finalHash, err := EncodeOwnershipBinding(binding)
	require.NoError(t, err)
	require.Equal(t, expected, final)
	require.Equal(t, hash, finalHash)
}

func invalidOwnershipBindings() map[string]func(*OwnershipBinding) {
	return map[string]func(*OwnershipBinding){
		"version zero":      func(b *OwnershipBinding) { b.ProtocolVersion = 0 },
		"version one":       func(b *OwnershipBinding) { b.ProtocolVersion = 1 },
		"version future":    func(b *OwnershipBinding) { b.ProtocolVersion = 3 },
		"operation missing": func(b *OwnershipBinding) { b.OperationID = uuid.Nil },
		"space missing":     func(b *OwnershipBinding) { b.SpaceID = uuid.Nil },
		"account missing":   func(b *OwnershipBinding) { b.AccountID = uuid.Nil },
		"actor missing":     func(b *OwnershipBinding) { b.ActorProfileID = uuid.Nil },
		"new owner missing": func(b *OwnershipBinding) { b.NewOwnerProfileID = uuid.Nil },
		"self transfer":     func(b *OwnershipBinding) { b.NewOwnerProfileID = b.ActorProfileID },
		"epoch zero":        func(b *OwnershipBinding) { b.SessionEpoch = 0 },
		"epoch negative":    func(b *OwnershipBinding) { b.SessionEpoch = -1 },
		"digest empty":      func(b *OwnershipBinding) { b.ProofDigest = "" },
		"digest uppercase":  func(b *OwnershipBinding) { b.ProofDigest = strings.Repeat("AB", 32) },
		"digest short":      func(b *OwnershipBinding) { b.ProofDigest = strings.Repeat("a", 63) },
		"digest long":       func(b *OwnershipBinding) { b.ProofDigest = strings.Repeat("a", 65) },
		"digest nonhex":     func(b *OwnershipBinding) { b.ProofDigest = strings.Repeat("g", 64) },
		"digest whitespace": func(b *OwnershipBinding) { b.ProofDigest = " " + strings.Repeat("a", 63) },
	}
}
func TestOwnershipJournal_InvalidTypedBindingsRejectBeforePersistence(t *testing.T) {
	for name, change := range invalidOwnershipBindings() {
		t.Run(name, func(t *testing.T) {
			binding := ownershipJournalBindingFixture()
			change(&binding)
			_, _, err := EncodeOwnershipBinding(binding)
			require.ErrorIs(t, err, ErrOwnershipBindingInvalid)
			// Nil persistence proves validation cannot create partial reservations or access DB.
			row, err := (&SpaceStore{}).ReserveOwnership(context.Background(), binding)
			require.ErrorIs(t, err, ErrOwnershipBindingInvalid)
			require.Nil(t, row)
		})
	}
}

func ownershipJournalStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL ownership journal migration")
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	migration, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000008_ownership_journal.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	return &SpaceStore{Pool: pool}
}
func seedOwnershipJournalBinding(t *testing.T, st *SpaceStore) OwnershipBinding {
	t.Helper()
	binding := ownershipJournalBindingFixture()
	binding.OperationID = uuid.New()
	binding.AccountID = uuid.New()
	binding.ActorProfileID = uuid.New()
	binding.NewOwnerProfileID = uuid.New()
	row, err := st.CreateSpace(context.Background(), binding.ActorProfileID, "Ownership reservation", "unchanged", "private")
	require.NoError(t, err)
	binding.SpaceID = row.ID
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, binding.SpaceID, binding.NewOwnerProfileID)
	require.NoError(t, err)
	return binding
}
func assertOwnershipReservation(t *testing.T, row *OwnershipJournal, binding OwnershipBinding) {
	t.Helper()
	require.NotNil(t, row)
	require.Equal(t, binding, row.Binding)
	require.Equal(t, "reserved", row.State)
	expected, hash, err := EncodeOwnershipBinding(binding)
	require.NoError(t, err)
	require.Equal(t, expected, row.BindingBytes)
	require.Equal(t, hash, row.BindingHash)
	require.NotEqual(t, uuid.Nil, row.AuditID)
	require.NotEqual(t, uuid.Nil, row.EventID)
	require.NotEqual(t, row.AuditID, row.EventID)
}
func assertReservationHasNoPublicEffects(t *testing.T, st *SpaceStore, b OwnershipBinding) {
	t.Helper()
	row, err := st.GetSpace(context.Background(), b.SpaceID)
	require.NoError(t, err)
	require.Equal(t, b.ActorProfileID, row.OwnerProfileID)
	var audits int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE space_id=$1`, b.SpaceID).Scan(&audits))
	require.Zero(t, audits)
}

func TestOwnershipJournal_ReserveReplayAndLoadSurviveNewPool(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	first, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	assertOwnershipReservation(t, first, binding)
	replay, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	// Closing an independently opened writer and replacing it models process loss:
	// no Go object/cache from Reserve can supply the authoritative Load result.
	config := st.Pool.Config()
	writerPool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	writer := &SpaceStore{Pool: writerPool}
	replay, err = writer.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	writerPool.Close()
	readerPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	defer readerPool.Close()
	reader := &SpaceStore{Pool: readerPool}
	loaded, err := reader.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, first, loaded)
	replay, err = reader.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	// Returned mutable bytes cannot mutate durable binding evidence.
	loaded.BindingBytes[0] ^= 0xff
	after, err := reader.LoadOwnership(context.Background(), binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, first, after)
	another := binding
	another.OperationID = uuid.New()
	_, err = reader.ReserveOwnership(context.Background(), another)
	require.ErrorIs(t, err, ErrOwnershipActive)
	assertReservationHasNoPublicEffects(t, reader, binding)
}

func TestOwnershipJournal_ImmutableBindingConflictsLeaveReservationUnchanged(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	first, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	otherSpace := seedOwnershipJournalBinding(t, st)
	for name, change := range map[string]func(*OwnershipBinding){
		"cross space":    func(b *OwnershipBinding) { b.SpaceID = otherSpace.SpaceID },
		"account":        func(b *OwnershipBinding) { b.AccountID = uuid.New() },
		"actor":          func(b *OwnershipBinding) { b.ActorProfileID = uuid.New() },
		"new owner":      func(b *OwnershipBinding) { b.NewOwnerProfileID = uuid.New() },
		"original epoch": func(b *OwnershipBinding) { b.SessionEpoch++ },
		"proof digest":   func(b *OwnershipBinding) { b.ProofDigest = strings.Repeat("cd", 32) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := binding
			change(&changed)
			_, err := st.ReserveOwnership(context.Background(), changed)
			require.ErrorIs(t, err, ErrOwnershipConflict)
			got, err := st.LoadOwnership(context.Background(), binding.OperationID)
			require.NoError(t, err)
			require.Equal(t, first, got)
		})
	}
	// Changing operation_id is a new operation, so it hits the active-space fence.
	another := binding
	another.OperationID = uuid.New()
	_, err = st.ReserveOwnership(context.Background(), another)
	require.ErrorIs(t, err, ErrOwnershipActive)
	// Cross-space conflict must not leave a phantom fence on the other space.
	_, err = st.ReserveOwnership(context.Background(), otherSpace)
	require.NoError(t, err)
}

func TestOwnershipJournal_ReplayPrecedesCurrentOwnerAndMemberValidation(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	first, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	// Test-only DB changes simulate replay against changed live state; ordinary
	// handler freeze belongs to a later cycle and is deliberately not exercised.
	_, err = st.Pool.Exec(context.Background(), `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, binding.SpaceID, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, binding.SpaceID, binding.NewOwnerProfileID)
	require.NoError(t, err)
	replay, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, first, replay)
}

func TestOwnershipJournal_FreshReservationChecksOwnerAndMembership(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	for _, name := range []string{"wrong owner", "missing member", "missing space"} {
		t.Run(name, func(t *testing.T) {
			binding := seedOwnershipJournalBinding(t, st)
			want := ErrNotSpaceOwner
			switch name {
			case "wrong owner":
				binding.ActorProfileID = uuid.New()
			case "missing member":
				binding.NewOwnerProfileID = uuid.New()
				want = ErrMemberNotFound
			case "missing space":
				binding.SpaceID = uuid.New()
				want = pgx.ErrNoRows
			}
			row, err := st.ReserveOwnership(context.Background(), binding)
			require.ErrorIs(t, err, want)
			require.Nil(t, row)
			row, err = st.LoadOwnership(context.Background(), binding.OperationID)
			require.ErrorIs(t, err, ErrOwnershipMissing)
			require.Nil(t, row)
		})
	}
}

func TestOwnershipJournal_LoadMissingAndDatabaseFailureAreDistinct(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	row, err := st.LoadOwnership(context.Background(), uuid.New())
	require.ErrorIs(t, err, ErrOwnershipMissing)
	require.Nil(t, row)
	closedPool, err := pgxpool.NewWithConfig(context.Background(), st.Pool.Config())
	require.NoError(t, err)
	closedPool.Close()
	row, err = (&SpaceStore{Pool: closedPool}).LoadOwnership(context.Background(), uuid.New())
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrOwnershipMissing))
	require.Nil(t, row)
}

type ownershipReservationResult struct {
	row *OwnershipJournal
	err error
}

func waitOwnershipReservation(t *testing.T, result <-chan ownershipReservationResult) ownershipReservationResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("ownership reservation did not finish")
		return ownershipReservationResult{}
	}
}
func TestOwnershipJournal_ConcurrentTwoPoolsHaveOneActiveWinner(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	a := seedOwnershipJournalBinding(t, st)
	b := a
	b.OperationID = uuid.New()
	poolA := independentMutationLockPool(t, st.Pool, 2)
	poolB := independentMutationLockPool(t, st.Pool, 2)
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan ownershipReservationResult, 2)
	workerCtx, cancelWorkers := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelWorkers()
	for _, worker := range []struct {
		pool    *pgxpool.Pool
		binding OwnershipBinding
	}{{poolA, a}, {poolB, b}} {
		go func(pool *pgxpool.Pool, binding OwnershipBinding) {
			ready <- struct{}{}
			<-start
			row, err := (&SpaceStore{Pool: pool}).ReserveOwnership(workerCtx, binding)
			results <- ownershipReservationResult{row, err}
		}(worker.pool, worker.binding)
	}
	<-ready
	<-ready
	close(start)
	first, second := waitOwnershipReservation(t, results), waitOwnershipReservation(t, results)
	if first.err != nil {
		first, second = second, first
	}
	require.NoError(t, first.err)
	require.ErrorIs(t, second.err, ErrOwnershipActive)
	assertOwnershipReservation(t, first.row, first.row.Binding)
	winner, err := st.LoadOwnership(context.Background(), first.row.Binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, first.row, winner)
	loser := a.OperationID
	if loser == winner.Binding.OperationID {
		loser = b.OperationID
	}
	absent, err := st.LoadOwnership(context.Background(), loser)
	require.ErrorIs(t, err, ErrOwnershipMissing)
	require.Nil(t, absent)
	assertReservationHasNoPublicEffects(t, st, a)
}

func TestOwnershipJournal_DifferentSpaceReservesWhileAnotherSpaceLockIsHeld(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	a := seedOwnershipJournalBinding(t, st)
	b := seedOwnershipJournalBinding(t, st)
	// Hold the canonical BIGINT space lock on a separate connection. Reservation
	// for another space must complete before release; no timing-based sleep oracle.
	release, err := NewSpaceMutationLocker(independentMutationLockPool(t, st.Pool, 1)).Acquire(context.Background(), a.SpaceID)
	require.NoError(t, err)
	release = cleanupMutationLock(t, release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row, err := (&SpaceStore{Pool: independentMutationLockPool(t, st.Pool, 2)}).ReserveOwnership(ctx, b)
	require.NoError(t, err)
	assertOwnershipReservation(t, row, b)
	release()
	row, err = st.ReserveOwnership(ctx, a)
	require.NoError(t, err)
	assertOwnershipReservation(t, row, a)
}

func TestOwnershipJournal_ConcurrentCrossSpaceOperationReuseHasOneBinding(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	a := seedOwnershipJournalBinding(t, st)
	b := seedOwnershipJournalBinding(t, st)
	b.OperationID = a.OperationID
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan ownershipReservationResult, 2)
	workerCtx, cancelWorkers := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelWorkers()
	for _, binding := range []OwnershipBinding{a, b} {
		pool := independentMutationLockPool(t, st.Pool, 2)
		go func(pool *pgxpool.Pool, binding OwnershipBinding) {
			ready <- struct{}{}
			<-start
			row, err := (&SpaceStore{Pool: pool}).ReserveOwnership(workerCtx, binding)
			results <- ownershipReservationResult{row, err}
		}(pool, binding)
	}
	<-ready
	<-ready
	close(start)
	first, second := waitOwnershipReservation(t, results), waitOwnershipReservation(t, results)
	if first.err != nil {
		first, second = second, first
	}
	require.NoError(t, first.err)
	require.ErrorIs(t, second.err, ErrOwnershipConflict)
	loaded, err := st.LoadOwnership(context.Background(), a.OperationID)
	require.NoError(t, err)
	require.Equal(t, first.row, loaded)
	loser := a
	if first.row.Binding.SpaceID == a.SpaceID {
		loser = b
	}
	loser.OperationID = uuid.New()
	_, err = st.ReserveOwnership(context.Background(), loser)
	require.NoError(t, err, "conflicting operation reuse must not occupy the losing space")
}

func TestOwnershipJournal_DatabaseEnforcesGlobalOperationAndActiveSpace(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	_, err := st.ReserveOwnership(context.Background(), binding)
	require.NoError(t, err)
	var storedBytes, storedHash []byte
	var storedVersion uint32
	var storedSpace, storedAccount, storedActor, storedNewOwner uuid.UUID
	var storedEpoch int64
	var storedDigest string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT protocol_version,space_id,account_id,actor_profile_id,new_owner_profile_id,session_epoch,proof_digest,binding_bytes,binding_hash FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&storedVersion, &storedSpace, &storedAccount, &storedActor, &storedNewOwner, &storedEpoch, &storedDigest, &storedBytes, &storedHash))
	canonical, hash, err := EncodeOwnershipBinding(binding)
	require.NoError(t, err)
	require.Equal(t, canonical, storedBytes)
	require.Equal(t, hash[:], storedHash)
	require.Equal(t, binding.ProtocolVersion, storedVersion)
	require.Equal(t, binding.SpaceID, storedSpace)
	require.Equal(t, binding.AccountID, storedAccount)
	require.Equal(t, binding.ActorProfileID, storedActor)
	require.Equal(t, binding.NewOwnerProfileID, storedNewOwner)
	require.Equal(t, binding.SessionEpoch, storedEpoch)
	require.Equal(t, binding.ProofDigest, storedDigest)
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO ownership_journal SELECT * FROM ownership_journal WHERE operation_id=$1`, binding.OperationID)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code, "operation uniqueness must exist in PostgreSQL")
	require.Equal(t, "ownership_journal_pkey", pgErr.ConstraintName)
	other := binding
	other.OperationID = uuid.New()
	otherBytes, otherHash, err := EncodeOwnershipBinding(other)
	require.NoError(t, err)
	// Bypass Reserve while retaining a self-consistent typed/canonical binding and
	// independent artifact IDs. Only the database's active-space fence may reject.
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO ownership_journal SELECT (jsonb_populate_record(NULL::ownership_journal,to_jsonb(j)||jsonb_build_object('operation_id',$2::text,'binding_bytes',$3::text,'binding_hash',$4::text,'audit_id',$5::text,'event_id',$6::text))).* FROM ownership_journal j WHERE operation_id=$1`, binding.OperationID, other.OperationID.String(), "\\x"+hex.EncodeToString(otherBytes), "\\x"+hex.EncodeToString(otherHash[:]), uuid.NewString(), uuid.NewString())
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)
	require.Equal(t, "ownership_journal_one_active_space", pgErr.ConstraintName)
	var count int
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_journal WHERE space_id=$1`, binding.SpaceID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestOwnershipJournal_ReservationHonorsExternalTransactionLocks(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	for _, namespace := range []string{"canonical space bigint", "canonical operation two integers"} {
		t.Run(namespace, func(t *testing.T) {
			binding := seedOwnershipJournalBinding(t, st)
			lockPool := independentMutationLockPool(t, st.Pool, 1)
			lockerCtx, cancelLocker := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelLocker()
			tx, err := lockPool.Begin(lockerCtx)
			require.NoError(t, err)
			defer func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = tx.Rollback(cleanupCtx)
			}()
			if namespace == "canonical space bigint" {
				_, err = tx.Exec(lockerCtx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(binding.SpaceID))
			} else {
				// Derive independently from the frozen domain and raw UUID, without a
				// production helper. PostgreSQL two-int keys differ from bigint keys.
				material := append([]byte("voice.space.ownership.operation.v1\x00"), binding.OperationID[:]...)
				digest := sha256.Sum256(material)
				first := int32(binary.BigEndian.Uint32(digest[:4]))
				second := int32(binary.BigEndian.Uint32(digest[4:8]))
				_, err = tx.Exec(lockerCtx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, first, second)
			}
			require.NoError(t, err, "external transaction owns the required namespace before Reserve starts")
			worker := &SpaceStore{Pool: independentMutationLockPool(t, st.Pool, 1)}
			waitCtx, cancelWait := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancelWait()
			row, err := worker.ReserveOwnership(waitCtx, binding)
			require.ErrorIs(t, err, context.DeadlineExceeded, "Reserve must wait for the held canonical lock until its context expires")
			require.Nil(t, row)
			absent, loadErr := st.LoadOwnership(context.Background(), binding.OperationID)
			require.ErrorIs(t, loadErr, ErrOwnershipMissing)
			require.Nil(t, absent, "canceled lock waiter cannot persist a reservation")
			require.NoError(t, tx.Commit(lockerCtx))
			retryCtx, cancelRetry := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelRetry()
			row, err = worker.ReserveOwnership(retryCtx, binding)
			require.NoError(t, err, "release must unblock a fresh reservation without leaking the canceled transaction")
			assertOwnershipReservation(t, row, binding)
		})
	}
}
