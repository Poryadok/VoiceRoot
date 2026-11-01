package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PrepareOpenSource durably records the one-use custodian statement before
// allowing maintenance connections. It never enables the writer or admission.
// Exclusive administrator, endpoint and restore custody remain attested
// external requirements, and custodianKey must be independently pinned.
func (c *Controller) PrepareOpenSource(ctx context.Context, maintenance *pgxpool.Pool, claim OpenAttestation, custodianKey ed25519.PublicKey) error {
	if c == nil || ctx == nil || ctx.Err() != nil || maintenance == nil {
		return ErrUnavailable
	}
	claim.Signature = append([]byte(nil), claim.Signature...)
	custodianKey = append(ed25519.PublicKey(nil), custodianKey...)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.leader == nil || c.parent.Err() != nil || c.activationUsed || c.verifyOpenAttestation(claim, custodianKey, time.Now()) != nil {
		return ErrUnavailable
	}
	c.activationUsed = true
	admin, err := maintenance.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Release()
	var system, database string
	var oid, writerOID uint32
	var superuser, allowed, login, publicConnect, unsafe bool
	if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&system, &database, &superuser) != nil || system != claim.Physical.SystemIdentifier || database == claim.Physical.Database || !superuser || admin.QueryRow(ctx, `SELECT oid,datallowconn,EXISTS(SELECT 1 FROM aclexplode(COALESCE(datacl,acldefault('d',datdba))) a WHERE a.grantee=0 AND a.privilege_type='CONNECT') FROM pg_database WHERE datname=$1`, claim.Physical.Database).Scan(&oid, &allowed, &publicConnect) != nil || oid != claim.Physical.DatabaseOID || allowed || publicConnect || admin.QueryRow(ctx, `SELECT oid,rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&writerOID, &login) != nil || writerOID == 0 || login {
		return ErrUnavailable
	}
	if admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolcanlogin AND NOT r.rolsuper AND r.oid<>$1 AND (has_database_privilege(r.oid,$2::oid,'CONNECT') OR pg_has_role(r.oid,$1::oid,'MEMBER')))`, writerOID, oid).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	var sessions, prepared int
	if admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1 OR usename=$3),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2 OR owner=$3)`, oid, claim.Physical.Database, claim.WriterRole).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 {
		return ErrUnavailable
	}
	if c.pool == nil || c.pool.Config().MaxConns < 2 {
		return ErrUnavailable
	}
	pinConn, err := c.pool.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	if !c.matchesControl(ctx, pinConn) {
		destroy(pinConn)
		c.poisonLocked()
		return ErrUnavailable
	}
	pin, err := pinConn.Begin(ctx)
	if err != nil {
		destroy(pinConn)
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if rollbackErr := pin.Rollback(cleanup); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			destroy(pinConn)
		} else {
			pinConn.Release()
		}
	}()
	var pinned PhysicalSource
	if pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR UPDATE`, c.binding.SourceID, c.binding.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != claim.Physical {
		return ErrUnavailable
	}
	control, err := c.leader.Begin(ctx)
	if err != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	intentMayExist := false
	preparedOpen := false
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		_ = control.Rollback(cleanup)
		stop()
		if intentMayExist && !preparedOpen {
			c.abortOpenLocked(admin, claim)
		}
	}()
	var state, sourceID, phase, controlSystem, sourceSystem, sourceDatabase string
	var epoch int64
	var history, generation uuid.UUID
	var digest []byte
	var sourceOID uint32
	if control.QueryRow(ctx, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton FOR UPDATE`).Scan(&state, &epoch, &sourceID, &history, &generation, &digest) != nil || state != "CLOSED" || !same(c.binding, epoch, sourceID, history, generation, digest) {
		return ErrUnavailable
	}
	if control.QueryRow(ctx, `SELECT phase,control_system_id,source_system_id,database_name,database_oid,history_generation,user_source_generation,keyset_digest FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2 FOR UPDATE`, c.binding.SourceID, c.binding.Epoch).Scan(&phase, &controlSystem, &sourceSystem, &sourceDatabase, &sourceOID, &history, &generation, &digest) != nil || (phase != "INITIALIZED_CLOSED" && phase != "EXISTING_INITIALIZED_CLOSED" && phase != "RECOVERED_CLOSED") || controlSystem != c.controlSystem || sourceSystem != claim.Physical.SystemIdentifier || sourceDatabase != claim.Physical.Database || sourceOID != claim.Physical.DatabaseOID || !same(c.binding, c.binding.Epoch, c.binding.SourceID, history, generation, digest) {
		return ErrUnavailable
	}
	if phase == "RECOVERED_CLOSED" {
		var count, roles int
		var writer string
		if control.QueryRow(ctx, `SELECT count(*),count(DISTINCT writer_role),COALESCE(min(writer_role),'') FROM public.user_custody_open_evidence WHERE source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch).Scan(&count, &roles, &writer) != nil || count == 0 || roles != 1 || writer != claim.WriterRole {
			return ErrUnavailable
		}
	}
	payload, err := OpenAttestationPayload(claim)
	if err != nil {
		return ErrUnavailable
	}
	keyDigest := sha256.Sum256(custodianKey)
	if _, err = control.Exec(ctx, `INSERT INTO public.user_custody_open_evidence(challenge,source_id,epoch,writer_role,payload,signature,key_digest) VALUES($1,$2,$3,$4,$5,$6,$7)`, claim.Challenge[:], c.binding.SourceID, c.binding.Epoch, claim.WriterRole, payload, claim.Signature, keyDigest[:]); err != nil {
		return ErrUnavailable
	}
	tag, err := control.Exec(ctx, `UPDATE public.user_custody_creation SET phase='PREPARING_OPEN' WHERE singleton AND phase=$1 AND source_id=$2 AND epoch=$3`, phase, c.binding.SourceID, c.binding.Epoch)
	if err != nil || tag.RowsAffected() != 1 || c.parent.Err() != nil || c.verifyOpenAttestation(claim, custodianKey, time.Now()) != nil {
		return ErrUnavailable
	}
	intentMayExist = true
	if control.Commit(ctx) != nil {
		return ErrUnavailable
	}
	if c.afterPrepareIntent != nil {
		c.afterPrepareIntent(pinConn.Conn().PgConn().PID())
	}
	if pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != claim.Physical || c.parent.Err() != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&system, &database) != nil || system != claim.Physical.SystemIdentifier || database == claim.Physical.Database || admin.QueryRow(ctx, `SELECT oid,datallowconn,(SELECT rolcanlogin FROM pg_roles WHERE oid=$2),(SELECT count(*) FROM pg_stat_activity WHERE datid=$1 OR usename=$3),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$4 OR owner=$3) FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID, writerOID, claim.WriterRole, claim.Physical.Database).Scan(&oid, &allowed, &login, &sessions, &prepared) != nil || oid != claim.Physical.DatabaseOID || allowed || login || sessions != 0 || prepared != 0 {
		return ErrUnavailable
	}
	if _, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{claim.Physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`); err != nil || admin.QueryRow(ctx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, claim.Physical.Database).Scan(&oid, &allowed) != nil || oid != claim.Physical.DatabaseOID || !allowed || c.parent.Err() != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&system, &database) != nil || system != claim.Physical.SystemIdentifier || database == claim.Physical.Database {
		return ErrUnavailable
	}
	if c.afterPrepareEnabled != nil {
		c.afterPrepareEnabled()
	}
	if pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != claim.Physical || pin.Commit(ctx) != nil {
		return ErrUnavailable
	}
	c.activationPrepared = true
	preparedOpen = true
	return nil
}
