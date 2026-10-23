package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NativeMembershipCatalog51Receipt is emitted only after the owning atomic
// migration executor confirms commit, never by an ordinary reader or RPC.
type NativeMembershipCatalog51Receipt struct {
	NativeMembershipCatalog45Receipt
	Profile string `json:"profile"`
	Engine  string `json:"migration_engine"`
}

type nativeCatalog51Phase uint8

const (
	catalog51New nativeCatalog51Phase = iota
	catalog51Locked50
	catalog51Dirty
	catalog51Body
	catalog51Qualified
	catalog51Committed
)

type nativeCatalog51State struct {
	phase             nativeCatalog51Phase
	abort             error
	frameworkUnlocked bool
}

func (s *nativeCatalog51State) fail(err error) error {
	if err == nil {
		err = ErrMemberAuthorityUnavailable
	}
	s.abort = errors.Join(s.abort, err)
	return err
}

func (s *nativeCatalog51State) allowOuterCommit(engineErr error) error {
	if engineErr != nil {
		return s.fail(engineErr)
	}
	if s.abort != nil {
		return s.abort
	}
	if s.phase != catalog51Qualified {
		return s.fail(ErrMemberAuthorityUnavailable)
	}
	return nil
}

// NativeCatalog51Migration implements only the real migrate engine's forward
// 50→51 database.Driver protocol. It owns the backend/transaction; it is not a
// general migration driver or an authority to adopt an unknown database.
type NativeCatalog51Migration struct {
	// migrate's lock timeout does not join its Driver.Lock goroutine. Ownership
	// must therefore outlive that framework call, until cancellation is joined.
	ownershipMu sync.Mutex
	cleanupMu   sync.Mutex
	lockCancel  context.CancelFunc
	lockDone    chan struct{}
	closed      bool
	store       *SpaceStore
	ctx         context.Context
	prior       NativeInstallerSourceFacts
	window      func(context.Context) error
	conn        *pgxpool.Conn
	tx          pgx.Tx
	state       nativeCatalog51State
	key         string
	sessionLock bool
	catalogOID  uint32
	receipt     *NativeMembershipCatalog51Receipt
}

var _ database.Driver = (*NativeCatalog51Migration)(nil)

// NewNativeCatalog51Migration requires actual same-invocation trusted50 facts.
// The controller must retain exclusive privileged-DDL custody independently of
// these locks. There is no caller-selected hash or runtime admission switch.
func (s *SpaceStore) NewNativeCatalog51Migration(ctx context.Context, prior *NativeInstallerSourceFacts, window func(context.Context) error) (*NativeCatalog51Migration, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || prior == nil || prior.Version != 50 || window == nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err := requireNativeCatalogRowJSONAdmission(); err != nil {
		return nil, err
	}
	// Pinned postgres.Open passes parsed URI.Path (including its leading slash)
	// as Config.DatabaseName. Match our actual canonical child URI, rather than
	// hashing a different key from the WithConnection empty-config fallback.
	key, err := database.GenerateAdvisoryLockId("/"+prior.DatabaseName, "public", "schema_migrations")
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	return &NativeCatalog51Migration{store: s, ctx: ctx, prior: *prior, window: window, key: key}, nil
}

func (d *NativeCatalog51Migration) Open(string) (database.Driver, error) {
	return nil, d.state.fail(ErrMemberAuthorityUnavailable)
}

func (d *NativeCatalog51Migration) Drop() error {
	return d.state.fail(ErrMemberAuthorityUnavailable)
}

func (d *NativeCatalog51Migration) Lock() error {
	if d == nil {
		return ErrMemberAuthorityUnavailable
	}
	d.ownershipMu.Lock()
	if d.closed || d.lockDone != nil || d.ctx == nil {
		d.ownershipMu.Unlock()
		return ErrMemberAuthorityUnavailable
	}
	d.ctx, d.lockCancel = context.WithCancel(d.ctx)
	d.lockDone = make(chan struct{})
	done := d.lockDone
	d.ownershipMu.Unlock()
	defer close(done)
	if d.ctx == nil || d.store == nil || d.store.Pool == nil || d.window == nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if d.state.phase != catalog51New || d.state.abort != nil || d.ctx.Err() != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if err := d.window(d.ctx); err != nil {
		return d.state.fail(err)
	}
	var err error
	d.conn, err = d.store.Pool.Acquire(d.ctx)
	if err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if _, err = d.conn.Exec(d.ctx, `SELECT pg_advisory_lock($1::bigint)`, d.key); err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	d.sessionLock = true
	d.tx, err = d.conn.BeginTx(d.ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if _, err = d.tx.Exec(d.ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, d.key); err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if _, err = d.tx.Exec(d.ctx, `LOCK TABLE public.schema_migrations IN ACCESS EXCLUSIVE MODE;
LOCK TABLE native_membership_guard.epoch IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	names := []string{"native_membership_guard.spaces", "native_membership_guard.frames", "native_membership_guard.profiles", "public.space_join_operations"}
	for _, table := range append(append([]string{}, nativeMembershipGuardCoverage...), accountBanTables...) {
		names = append(names, pgx.Identifier{"public", table}.Sanitize())
	}
	if _, err = d.tx.Exec(d.ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	facts, err := nativeCatalog51SourceFacts(d.ctx, d.tx, 50)
	if err != nil || *facts != d.prior {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if d.catalogOID, _, err = requireNativeCatalog50Definition(d.ctx, d.tx); err != nil {
		return d.state.fail(err)
	}
	d.state.phase = catalog51Locked50
	return nil
}

func (d *NativeCatalog51Migration) Version() (int, bool, error) {
	if d.state.phase != catalog51Locked50 || d.state.abort != nil {
		return 0, false, d.state.fail(ErrMemberAuthorityUnavailable)
	}
	return 50, false, nil
}

func (d *NativeCatalog51Migration) SetVersion(version int, dirty bool) error {
	if version != 51 || d.tx == nil || d.state.abort != nil || d.ctx.Err() != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if dirty {
		if d.state.phase != catalog51Locked50 {
			return d.state.fail(ErrMemberAuthorityUnavailable)
		}
		tag, err := d.tx.Exec(d.ctx, `UPDATE public.schema_migrations SET version=51,dirty=true WHERE version=50 AND NOT dirty`)
		if err != nil || tag.RowsAffected() != 1 {
			return d.state.fail(ErrMemberAuthorityUnavailable)
		}
		d.state.phase = catalog51Dirty
		return nil
	}
	if d.state.phase != catalog51Body {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	tag, err := d.tx.Exec(d.ctx, `UPDATE public.schema_migrations SET dirty=false WHERE version=51 AND dirty`)
	if err != nil || tag.RowsAffected() != 1 {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if err = d.qualifyAndCAS(); err != nil {
		return d.state.fail(err)
	}
	d.state.phase = catalog51Qualified
	return nil
}

func (d *NativeCatalog51Migration) Run(stream io.Reader) error {
	if d.state.phase != catalog51Dirty || d.state.abort != nil || stream == nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	const maxMigrationBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(stream, maxMigrationBytes+1))
	if err != nil || len(raw) > maxMigrationBytes || nativeSourceBodySHA(string(raw)) != nativeCatalogRowJSONMigrationSHA256 {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	// Exact compiled file hash selects SQL without transaction-control commands.
	// The file is chosen by the actual source/file engine, not a caller callback.
	if _, err = d.tx.Exec(d.ctx, string(raw)); err != nil {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	d.state.phase = catalog51Body
	return nil
}

func (d *NativeCatalog51Migration) qualifyAndCAS() error {
	if err := checkMemberAuthorityCatalog(d.ctx, d.tx); err != nil {
		return err
	}
	oid, parentDefinition, err := requireNativeCatalogRowJSONDefinition(d.ctx, d.tx)
	if err != nil || oid != d.catalogOID {
		return ErrMemberAuthorityUnavailable
	}
	// Authenticate the complete new definition BEFORE substituting only its
	// original50 source at the sameOID. Every other live field/owner/ACL remains.
	needle := "jsonb_build_array(p.oid,pg_get_functiondef(p.oid),p.proowner,p.proacl)"
	if strings.Count(nativeCatalog51ParentExpression, needle) != 1 {
		return ErrMemberAuthorityUnavailable
	}
	projection := strings.Replace(nativeCatalog51ParentExpression, needle, "jsonb_build_array(p.oid,CASE WHEN p.oid=$1::oid THEN $2::text ELSE pg_get_functiondef(p.oid) END,p.proowner,p.proacl)", 1)
	var parent, current, retained []byte
	if err = d.tx.QueryRow(d.ctx, nativeCatalog51DigestSQL(projection), oid, parentDefinition).Scan(&parent); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var equal bool
	if err = d.tx.QueryRow(d.ctx, "SELECT native_membership_guard.catalog()=("+nativeCatalogRowJSONLiteralExpression+")").Scan(&equal); err != nil || !equal {
		return ErrMemberAuthorityUnavailable
	}
	if err = d.tx.QueryRow(d.ctx, `SELECT native_membership_guard.catalog()`).Scan(&current); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	var epoch uuid.UUID
	var single bool
	if err = d.tx.QueryRow(d.ctx, `SELECT count(*)=1 FROM native_membership_guard.epoch`).Scan(&single); err != nil || !single {
		return ErrMemberAuthorityUnavailable
	}
	if err = d.tx.QueryRow(d.ctx, `SELECT epoch,catalog_sha256 FROM native_membership_guard.epoch WHERE singleton FOR UPDATE`).Scan(&epoch, &retained); err != nil || epoch.String() != d.prior.Epoch || len(parent) != 32 || len(current) != 32 || bytes.Equal(parent, current) || !bytes.Equal(parent, retained) || hex.EncodeToString(parent) != d.prior.SealSHA256 {
		return ErrMemberAuthorityUnavailable
	}
	tag, err := d.tx.Exec(d.ctx, `UPDATE native_membership_guard.epoch SET catalog_sha256=$1 WHERE singleton AND epoch=$2 AND catalog_sha256=$3`, current, epoch, parent)
	if err != nil || tag.RowsAffected() != 1 {
		return ErrMemberAuthorityUnavailable
	}
	d.receipt = &NativeMembershipCatalog51Receipt{NativeMembershipCatalog45Receipt: NativeMembershipCatalog45Receipt{
		DatabaseName: d.prior.DatabaseName, DatabaseOID: d.prior.DatabaseOID, DatabaseSystemID: d.prior.DatabaseSystemID,
		InstallerRole: d.prior.InstallerRole, Epoch: epoch.String(), OldSealSHA256: hex.EncodeToString(parent), NewSealSHA256: hex.EncodeToString(current),
		MigrationSHA256: nativeCatalogRowJSONMigrationSHA256, Changed: true,
	}, Profile: NativeCatalogRowJSONProfile, Engine: "github.com/golang-migrate/migrate/v4@v4.18.1"}
	if err = d.tx.QueryRow(d.ctx, `SELECT pg_current_xact_id()::text`).Scan(&d.receipt.TransactionID); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	return d.finalCheck()
}

func (d *NativeCatalog51Migration) finalCheck() error {
	if d.tx == nil || d.receipt == nil || d.ctx.Err() != nil {
		return ErrMemberAuthorityUnavailable
	}
	if err := d.window(d.ctx); err != nil {
		return err
	}
	facts, err := nativeCatalog51SourceFacts(d.ctx, d.tx, 51)
	if err != nil || facts.DatabaseName != d.prior.DatabaseName || facts.DatabaseOID != d.prior.DatabaseOID || facts.DatabaseSystemID != d.prior.DatabaseSystemID || facts.InstallerRole != d.prior.InstallerRole || facts.Epoch != d.prior.Epoch || facts.SealSHA256 != d.receipt.NewSealSHA256 {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}

// Unlock is called by migrate even on source/control errors outside this
// adapter. NEVER commit here. The same-backend xact key and table locks remain.
func (d *NativeCatalog51Migration) Unlock() error {
	if d.conn == nil || !d.sessionLock || d.state.frameworkUnlocked {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var unlocked bool
	if err := d.conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1::bigint)`, d.key).Scan(&unlocked); err != nil || !unlocked {
		return d.state.fail(ErrMemberAuthorityUnavailable)
	}
	d.sessionLock = false
	d.state.frameworkUnlocked = true
	return nil
}

// CommitQualified is called only by the outer installer after the actual
// engine returns. Its error is supplied so no source/unlock error can be hidden.
func (d *NativeCatalog51Migration) CommitQualified(engineErr error) (*NativeMembershipCatalog51Receipt, error) {
	d.cleanupMu.Lock()
	defer d.cleanupMu.Unlock()
	// On timeout the engine may have returned while Lock is still acquiring.
	// Cancel and join before touching any state or backend owned by that worker.
	d.joinLock(engineErr != nil)
	if err := d.state.allowOuterCommit(engineErr); err != nil {
		return nil, err
	}
	if !d.state.frameworkUnlocked || d.tx == nil {
		return nil, d.state.fail(ErrMemberAuthorityUnavailable)
	}
	if err := d.finalCheck(); err != nil {
		return nil, d.state.fail(err)
	}
	if err := d.tx.Commit(d.ctx); err != nil {
		return nil, d.state.fail(ErrMemberAuthorityUnavailable)
	}
	d.tx = nil
	d.state.phase = catalog51Committed
	r := *d.receipt
	return &r, nil
}

func (d *NativeCatalog51Migration) Close() error {
	if d == nil {
		return nil
	}
	d.cleanupMu.Lock()
	defer d.cleanupMu.Unlock()
	d.joinLock(true)
	var result error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if d.tx != nil {
		if err := d.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			result = ErrMemberAuthorityUnavailable
		}
		d.tx = nil
	}
	if d.sessionLock {
		if err := d.Unlock(); err != nil {
			result = errors.Join(result, err)
		}
	}
	if d.conn != nil {
		// This backend is dedicated to the atomic migration window. Never put a
		// potentially ambiguous session lock back into the caller's pool.
		if err := d.conn.Conn().Close(ctx); err != nil {
			result = errors.Join(result, ErrMemberAuthorityUnavailable)
		}
		d.conn.Release()
		d.conn = nil
	}
	return result
}

// joinLock closes admission to late starts before cancellation. A framework
// timeout can happen even before its Lock goroutine has been scheduled. If it
// did start, all backend/state writes finish before cleanup or error handling.
func (d *NativeCatalog51Migration) joinLock(stop bool) {
	d.ownershipMu.Lock()
	if stop {
		d.closed = true
		if d.lockCancel != nil {
			d.lockCancel()
		}
	}
	done := d.lockDone
	d.ownershipMu.Unlock()
	if done != nil {
		<-done
	}
}

// The transaction owner uses this full verifier without opening another pool
// transaction (which would deadlock against its own migration marker lock).
func nativeCatalog51SourceFacts(ctx context.Context, tx pgx.Tx, version int64) (*NativeInstallerSourceFacts, error) {
	if err := checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var valid bool
	if err := tx.QueryRow(ctx, `SELECT count(*)=1 AND bool_and(version=$1 AND NOT dirty) FROM public.schema_migrations`, version).Scan(&valid); err != nil || !valid {
		return nil, ErrMemberAuthorityUnavailable
	}
	f := &NativeInstallerSourceFacts{Version: version}
	if err := tx.QueryRow(ctx, `SELECT current_user=session_user AND session_user=pg_get_userbyid(c.relowner),session_user,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()),CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE') THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='native_membership_guard' AND c.relname='epoch'`).Scan(&valid, &f.InstallerRole, &f.DatabaseName, &f.DatabaseOID, &f.DatabaseSystemID); err != nil || !valid {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*)=1 FROM native_membership_guard.epoch) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`).Scan(&valid); err != nil || !valid {
		return nil, ErrMemberAuthorityUnavailable
	}
	var retained, current []byte
	if err := tx.QueryRow(ctx, `SELECT epoch::text,catalog_sha256,native_membership_guard.catalog() FROM native_membership_guard.epoch WHERE singleton`).Scan(&f.Epoch, &retained, &current); err != nil || f.Epoch != "94db7e08-e958-44fa-94b2-988410ab0044" || len(retained) != 32 || !bytes.Equal(retained, current) {
		return nil, ErrMemberAuthorityUnavailable
	}
	f.SealSHA256 = hex.EncodeToString(retained)
	return f, nil
}
