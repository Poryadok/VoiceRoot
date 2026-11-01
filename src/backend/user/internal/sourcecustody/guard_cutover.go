package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/user/internal/identityassociation"
)

// ActivateQualifiedGuards performs an explicitly signed guard cutover while
// the writer is dormant and admission remains closed. The signer must have
// independently established complete guarded-writer deployment and custody.
// Failure after preparation fences the source and requires recovery.
func (c *Controller) ActivateQualifiedGuards(ctx context.Context, maintenance, source *pgxpool.Pool, keys *identityassociation.Keyset, claim OpenAttestation, signer ed25519.PublicKey) error {
	if c == nil || ctx == nil || ctx.Err() != nil || maintenance == nil || source == nil || keys == nil || !claim.GuardCutover {
		return ErrUnavailable
	}
	claim.Signature = append([]byte(nil), claim.Signature...)
	signer = append(ed25519.PublicKey(nil), signer...)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.leader == nil || c.parent.Err() != nil || !c.activationUsed || !c.activationPrepared || c.verifyOpenAttestation(claim, signer, time.Now()) != nil {
		return ErrUnavailable
	}
	admin, err := maintenance.Acquire(ctx)
	if err != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	defer admin.Release()
	completed := false
	defer func() {
		if !completed {
			c.activationPrepared = false
			c.abortOpenLocked(admin, claim)
		}
	}()
	var system, database string
	var oid uint32
	var superuser, allowed bool
	if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&system, &database, &superuser) != nil || system != claim.Physical.SystemIdentifier || database == claim.Physical.Database || !superuser || admin.QueryRow(ctx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, claim.Physical.Database).Scan(&oid, &allowed) != nil || oid != claim.Physical.DatabaseOID || !allowed {
		return ErrUnavailable
	}
	store, err := identityassociation.NewStore(source, keys, c.binding.HistoryGeneration)
	if err != nil {
		return ErrUnavailable
	}
	identity, err := store.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = identity.Rollback(cleanup)
	}()
	tx := identity.SQL()
	var login bool
	if tx.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login) != nil || login {
		return ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `ALTER ROLE `+pgx.Identifier{claim.WriterRole}.Sanitize()+` NOLOGIN`); err != nil || checkWriterConfinementTx(ctx, tx, claim.Physical, claim.WriterRole, false) != nil {
		return ErrUnavailable
	}
	var sessions, prepared int
	if tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND (datid=$1 OR usename=$3)),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2 OR owner=$3)`, claim.Physical.DatabaseOID, claim.Physical.Database, claim.WriterRole).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 || c.parent.Err() != nil || c.verifyOpenAttestation(claim, signer, time.Now()) != nil {
		return ErrUnavailable
	}
	physical := identityassociation.PhysicalSourceIdentity{SystemIdentifier: claim.Physical.SystemIdentifier, Database: claim.Physical.Database, DatabaseOID: claim.Physical.DatabaseOID}
	if store.ActivateQualifiedGuardsTx(ctx, identity, c.binding.HistoryGeneration, c.binding.UserSourceGeneration, c.binding.KeysetDigest, physical) != nil || c.parent.Err() != nil || c.verifyOpenAttestation(claim, signer, time.Now()) != nil || identity.Commit(ctx) != nil || ctx.Err() != nil || c.parent.Err() != nil {
		return ErrUnavailable
	}
	completed = true
	return nil
}
