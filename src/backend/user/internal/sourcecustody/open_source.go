package sourcecustody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/user/internal/identityassociation"
)

// OpenQualified consumes an independently signed custodian statement for this
// leader and opens admission only for the checked dormant writer and source.
// The caller must independently pin custodianKey and enforce the attested
// deployment, administrator, endpoint, Auth and restore/promotion custody.
// PrepareOpenSource must first have committed the intent and enabled only
// maintenance access with the writer NOLOGIN.
func (c *Controller) OpenQualified(ctx context.Context, maintenance, source *pgxpool.Pool, keys *identityassociation.Keyset, claim OpenAttestation, custodianKey ed25519.PublicKey) error {
	if c == nil || ctx == nil || ctx.Err() != nil || maintenance == nil || source == nil || keys == nil {
		return ErrUnavailable
	}
	claim.Signature = append([]byte(nil), claim.Signature...)
	custodianKey = append(ed25519.PublicKey(nil), custodianKey...)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.leader == nil || c.parent.Err() != nil || !c.activationUsed || !c.activationPrepared || c.verifyOpenAttestation(claim, custodianKey, time.Now()) != nil {
		return ErrUnavailable
	}
	c.activationPrepared = false
	admin, err := maintenance.Acquire(ctx)
	if err != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	defer admin.Release()
	opened := false
	defer func() {
		if !opened {
			c.abortOpenLocked(admin, claim)
		}
	}()
	checkMaintenance := func(checkCtx context.Context) error {
		var system, database string
		var oid uint32
		var allowed, superuser bool
		if admin.QueryRow(checkCtx, `SELECT system_identifier::text,current_database(),(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&system, &database, &superuser) != nil || system != claim.Physical.SystemIdentifier || database == claim.Physical.Database || !superuser || admin.QueryRow(checkCtx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, claim.Physical.Database).Scan(&oid, &allowed) != nil || oid != claim.Physical.DatabaseOID || !allowed {
			return ErrUnavailable
		}
		return nil
	}
	if checkMaintenance(ctx) != nil {
		return ErrUnavailable
	}
	control, err := c.leader.Begin(ctx)
	if err != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = control.Rollback(cleanup)
	}()
	var state, sourceID, phase, controlSystem, sourceSystem, database string
	var epoch int64
	var history, generation uuid.UUID
	var digest []byte
	var oid uint32
	if control.QueryRow(ctx, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton FOR UPDATE`).Scan(&state, &epoch, &sourceID, &history, &generation, &digest) != nil || state != "CLOSED" || !same(c.binding, epoch, sourceID, history, generation, digest) {
		return ErrUnavailable
	}
	if control.QueryRow(ctx, `SELECT phase,control_system_id,source_system_id,database_name,database_oid,history_generation,user_source_generation,keyset_digest FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2 FOR UPDATE`, c.binding.SourceID, c.binding.Epoch).Scan(&phase, &controlSystem, &sourceSystem, &database, &oid, &history, &generation, &digest) != nil || phase != "PREPARING_OPEN" || controlSystem != c.controlSystem || sourceSystem != claim.Physical.SystemIdentifier || database != claim.Physical.Database || oid != claim.Physical.DatabaseOID || !same(c.binding, c.binding.Epoch, c.binding.SourceID, history, generation, digest) {
		return ErrUnavailable
	}
	var pinned PhysicalSource
	if control.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR UPDATE`, c.binding.SourceID, c.binding.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != claim.Physical {
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
	var writerLogin bool
	if tx.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&writerLogin) != nil || writerLogin {
		return ErrUnavailable
	}
	// This no-op obtains the writer's role lock before catalog validation.
	if _, err = tx.Exec(ctx, `ALTER ROLE `+pgx.Identifier{claim.WriterRole}.Sanitize()+` NOLOGIN`); err != nil || checkWriterConfinementTx(ctx, tx, claim.Physical, claim.WriterRole, false) != nil {
		return ErrUnavailable
	}
	physical := identityassociation.PhysicalSourceIdentity{SystemIdentifier: claim.Physical.SystemIdentifier, Database: claim.Physical.Database, DatabaseOID: claim.Physical.DatabaseOID}
	if store.ValidateForActivationTx(ctx, identity, c.binding.HistoryGeneration, c.binding.UserSourceGeneration, c.binding.KeysetDigest, physical) != nil {
		return ErrUnavailable
	}
	var sessions, prepared int
	if tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND (datid=$1 OR usename=$3)),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2 OR owner=$3)`, claim.Physical.DatabaseOID, claim.Physical.Database, claim.WriterRole).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 || checkMaintenance(ctx) != nil || c.parent.Err() != nil || c.verifyOpenAttestation(claim, custodianKey, time.Now()) != nil {
		return ErrUnavailable
	}
	if _, err = tx.Exec(ctx, `ALTER ROLE `+pgx.Identifier{claim.WriterRole}.Sanitize()+` LOGIN`); err != nil {
		return ErrUnavailable
	}
	payload, err := OpenAttestationPayload(claim)
	if err != nil {
		return ErrUnavailable
	}
	keyDigest := sha256.Sum256(custodianKey)
	var recordedPayload, recordedSignature, recordedKey []byte
	if control.QueryRow(ctx, `SELECT payload,signature,key_digest FROM public.user_custody_open_evidence WHERE challenge=$1 AND source_id=$2 AND epoch=$3 AND writer_role=$4 FOR SHARE`, claim.Challenge[:], c.binding.SourceID, c.binding.Epoch, claim.WriterRole).Scan(&recordedPayload, &recordedSignature, &recordedKey) != nil || !bytes.Equal(recordedPayload, payload) || !bytes.Equal(recordedSignature, claim.Signature) || !bytes.Equal(recordedKey, keyDigest[:]) {
		return ErrUnavailable
	}
	tag, err := control.Exec(ctx, `UPDATE public.user_custody_creation SET phase='ACTIVE' WHERE singleton AND phase=$1 AND source_id=$2 AND epoch=$3`, phase, c.binding.SourceID, c.binding.Epoch)
	if err != nil || tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	tag, err = control.Exec(ctx, `UPDATE public.user_custody_state SET state='OPEN' WHERE singleton AND state='CLOSED' AND source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch)
	if err != nil || tag.RowsAffected() != 1 || c.parent.Err() != nil || c.verifyOpenAttestation(claim, custodianKey, time.Now()) != nil {
		return ErrUnavailable
	}
	// Hold c.mu until both commits complete: no new permit can be issued in the
	// interval where control says OPEN but the dormant role is not yet enabled.
	if control.Commit(ctx) != nil {
		return ErrUnavailable
	}
	if c.parent.Err() != nil || ctx.Err() != nil || identity.Commit(ctx) != nil {
		return ErrUnavailable
	}
	if c.parent.Err() != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	c.authorizedOpen = true
	opened = true
	return nil
}

// abortOpenLocked is best-effort failure fencing after an uncertain commit.
// It always poisons this leader; reopening requires independent recovery.
func (c *Controller) abortOpenLocked(admin *pgxpool.Conn, claim OpenAttestation) {
	cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c.leader != nil {
		tx, err := c.leader.Begin(cleanup)
		if err == nil {
			_, e1 := tx.Exec(cleanup, `UPDATE public.user_custody_state SET state='CLOSED' WHERE singleton AND source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch)
			_, e2 := tx.Exec(cleanup, `UPDATE public.user_custody_creation SET phase='RECOVERY_REQUIRED' WHERE singleton AND source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch)
			if e1 == nil && e2 == nil {
				_ = tx.Commit(cleanup)
			} else {
				_ = tx.Rollback(cleanup)
			}
		}
	}
	if admin != nil {
		var system, database string
		var oid uint32
		if admin.QueryRow(cleanup, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&system, &database) == nil && system == claim.Physical.SystemIdentifier && database != claim.Physical.Database && admin.QueryRow(cleanup, `SELECT oid FROM pg_database WHERE datname=$1`, claim.Physical.Database).Scan(&oid) == nil && oid == claim.Physical.DatabaseOID {
			_, _ = admin.Exec(cleanup, `ALTER ROLE `+pgx.Identifier{claim.WriterRole}.Sanitize()+` NOLOGIN`)
			_, _ = admin.Exec(cleanup, `ALTER DATABASE `+pgx.Identifier{claim.Physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
			_, _ = admin.Exec(cleanup, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datid=$1 OR usename=$2`, claim.Physical.DatabaseOID, claim.WriterRole)
		}
	}
	c.poisonLocked()
}
