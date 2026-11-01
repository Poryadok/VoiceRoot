package sourcecustody

import (
	"context"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var databaseName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var systemID = regexp.MustCompile(`^[0-9]{1,20}$`)

// PhysicalSource is an independently supplied expected PostgreSQL identity.
// Its values alone do not establish source custody or currentness.
type PhysicalSource struct {
	SystemIdentifier string
	Database         string
	DatabaseOID      uint32
}

// FenceSource is a closed-source transition primitive. The operator must own
// the source and exclude concurrent administrative changes for its lifetime.
func (c *Controller) FenceSource(ctx context.Context, admin *pgxpool.Pool, source PhysicalSource) error {
	if c == nil || ctx == nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// A declared transition closes admission even when the physical source can
	// no longer be identified or reached. No source-side operation follows an
	// uncertain close acknowledgement.
	if c.CloseAdmission(ctx) != nil {
		return ErrUnavailable
	}
	if admin == nil || !databaseName.MatchString(source.Database) || !systemID.MatchString(source.SystemIdentifier) || source.DatabaseOID == 0 {
		return ErrUnavailable
	}
	conn, err := admin.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Release()
	check := func() (bool, error) {
		var actualSystem, current string
		var oid uint32
		var allowed bool
		if err := conn.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&actualSystem, &current); err != nil || actualSystem != source.SystemIdentifier || current == source.Database {
			return false, ErrUnavailable
		}
		if err := conn.QueryRow(ctx, `SELECT oid,datallowconn FROM pg_database WHERE datname=$1`, source.Database).Scan(&oid, &allowed); err != nil || oid != source.DatabaseOID {
			return false, ErrUnavailable
		}
		return allowed, nil
	}
	if _, err = check(); err != nil {
		return ErrUnavailable
	}
	// A control database on this physical cluster cannot witness its replacement.
	c.mu.Lock()
	var controlSystem string
	if c.stopped || c.leader == nil || c.leader.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&controlSystem) != nil || controlSystem == source.SystemIdentifier {
		c.mu.Unlock()
		return ErrUnavailable
	}
	c.mu.Unlock()
	if c.WaitPermits(ctx) != nil {
		return ErrUnavailable
	}
	// Serialize the physical operation with controller shutdown and checks.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.parent.Err() != nil || c.leader == nil || c.leader.Ping(ctx) != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	tx, err := c.leader.Begin(ctx)
	if err != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var state, sourceID string
	var epoch int64
	var history, generation [16]byte
	var digest []byte
	if tx.QueryRow(ctx, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton FOR SHARE`).Scan(&state, &epoch, &sourceID, &history, &generation, &digest) != nil || state != "CLOSED" || !same(c.binding, epoch, sourceID, history, generation, digest) {
		c.poisonLocked()
		return ErrUnavailable
	}
	var pinned PhysicalSource
	if tx.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR SHARE`, sourceID, epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != source {
		return ErrUnavailable
	}
	if _, err = check(); err != nil {
		return ErrUnavailable
	}
	if _, err = conn.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{source.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`); err != nil {
		return ErrUnavailable
	}
	allowed, err := check()
	if err != nil || allowed {
		return ErrUnavailable
	}
	var terminated bool
	if err = conn.QueryRow(ctx, `SELECT COALESCE(bool_and(pg_terminate_backend(pid)),true) FROM pg_stat_activity WHERE datid=$1`, source.DatabaseOID).Scan(&terminated); err != nil || !terminated {
		return ErrUnavailable
	}
	for {
		var sessions, prepared int
		if err = conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1), (SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, source.DatabaseOID, source.Database).Scan(&sessions, &prepared); err != nil || prepared != 0 {
			return ErrUnavailable
		}
		if sessions == 0 {
			allowed, err = check()
			if err != nil || allowed || c.parent.Err() != nil {
				return ErrUnavailable
			}
			if tx.QueryRow(ctx, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton`).Scan(&state, &epoch, &sourceID, &history, &generation, &digest) != nil || state != "CLOSED" || !same(c.binding, epoch, sourceID, history, generation, digest) {
				c.poisonLocked()
				return ErrUnavailable
			}
			if tx.Commit(ctx) != nil {
				c.poisonLocked()
				return ErrUnavailable
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ErrUnavailable
		case <-time.After(25 * time.Millisecond):
		}
	}
}
