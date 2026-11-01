package sourcecustody

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WatchActivation monitors an already active leader from a separate process.
// It is not deployment custody or a guarantee that the monitor itself survives;
// independent supervision and admission gating remain external prerequisites.
// Once leader loss is observed, the monitor holds the leader lock through the
// complete physical fence, so another leader cannot reopen in the interval.
func WatchActivation(parent context.Context, cfg WriterConfig, ready func()) error {
	r := cfg.Recovery
	if parent == nil || parent.Err() != nil || ready == nil || !validControlURL(r.ControlURL) || !validControlURL(r.SourceAdminURL) || forbiddenPGEnvironment() || !databaseName.MatchString(cfg.WriterRole) || !sourceIDPattern.MatchString(r.Request.SourceID) || r.Request.Epoch <= 0 || !databaseName.MatchString(r.Request.Database) || !systemID.MatchString(r.Request.ControlSystemID) || !systemID.MatchString(r.Request.SourceSystemID) || r.Request.ControlSystemID == r.Request.SourceSystemID || r.Physical.SystemIdentifier != r.Request.SourceSystemID || r.Physical.Database != r.Request.Database || r.Physical.DatabaseOID == 0 {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(parent, r.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	maintenance, err := openPinnedPostgres(parent, r.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer maintenance.Close()
	owner, err := control.Acquire(parent)
	if err != nil {
		return ErrUnavailable
	}
	locked := false
	uncertain := false
	defer func() {
		if uncertain {
			destroy(owner)
			return
		}
		if !locked {
			owner.Release()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var released bool
		if owner.QueryRow(ctx, `SELECT pg_advisory_unlock($1::bigint)`, leaderKey).Scan(&released) != nil || !released {
			destroy(owner)
		} else {
			owner.Release()
		}
	}()
	probe := func() error {
		// Finish an in-flight probe even if shutdown arrives, so the monitor
		// can perform its bounded fence instead of abandoning an uncertain lock.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if owner.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked) != nil {
			uncertain = true
			return ErrUnavailable
		}
		return nil
	}
	if err = probe(); err != nil {
		return err
	}
	if !locked {
		if err = verifyWatchedActivation(parent, owner, control, maintenance, cfg); err != nil {
			return err
		}
		// Bind shutdown to the leader observed at readiness, including its
		// backend start time so a recycled PID cannot identify a new process.
		var leaderPID int32
		var leaderStarted time.Time
		if err = owner.QueryRow(parent, `SELECT a.pid,a.backend_start FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database()) AND l.classid=($1::bigint >> 32)::oid AND l.objid=($1::bigint & 4294967295)::oid AND l.objsubid=1 AND l.mode='ExclusiveLock' AND l.granted`, leaderKey).Scan(&leaderPID, &leaderStarted); err != nil {
			// The leader may have disappeared after verification. If its lock
			// is now ours, fence rather than exiting without a monitor.
			if !errors.Is(err, pgx.ErrNoRows) || probe() != nil || !locked {
				return ErrUnavailable
			}
			goto fence
		}
		ready()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for !locked {
			select {
			case <-parent.Done():
				// A normal monitor stop must not leave its observed leader
				// serving without this monitor. Do not terminate a replacement
				// leader if the old backend has already disappeared.
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				var terminated bool
				err = owner.QueryRow(ctx, `SELECT pg_terminate_backend(a.pid) FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database()) AND l.classid=($1::bigint >> 32)::oid AND l.objid=($1::bigint & 4294967295)::oid AND l.objsubid=1 AND l.mode='ExclusiveLock' AND l.granted AND a.pid=$2 AND a.backend_start=$3`, leaderKey, leaderPID, leaderStarted).Scan(&terminated)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) || err == nil && !terminated {
					return ErrUnavailable
				}
				for !locked {
					if owner.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked) != nil {
						uncertain = true
						return ErrUnavailable
					}
					if !locked {
						select {
						case <-ctx.Done():
							return ErrUnavailable
						case <-ticker.C:
						}
					}
				}
				return recoverActivationLocked(ctx, owner, control, maintenance, r.Request, r.Physical, cfg.WriterRole)
			case <-ticker.C:
				if err = probe(); err != nil {
					return err
				}
			}
		}
	}
fence:
	// Do not abandon an initiated fence merely because the caller disconnects.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return recoverActivationLocked(ctx, owner, control, maintenance, r.Request, r.Physical, cfg.WriterRole)
}

func verifyWatchedActivation(parent context.Context, owner *pgxpool.Conn, control, maintenance *pgxpool.Pool, cfg WriterConfig) error {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	r := cfg.Recovery
	var system, database string
	var oid uint32
	if owner.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &oid) != nil || system != r.Request.ControlSystemID || database != control.Config().ConnConfig.Database || oid == 0 {
		return ErrUnavailable
	}
	var valid bool
	if owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.user_custody_creation c JOIN public.user_custody_state s ON s.singleton JOIN public.user_custody_physical_source p ON p.source_id=c.source_id AND p.epoch=c.epoch WHERE c.singleton AND c.phase='ACTIVE' AND s.state='OPEN' AND c.source_id=$1 AND c.epoch=$2 AND s.source_id=c.source_id AND s.epoch=c.epoch AND c.control_system_id=$3 AND c.source_system_id=$4 AND c.database_name=$5 AND c.database_oid=$6 AND p.system_identifier=$4 AND p.database_name=$5 AND p.database_oid=$6 AND EXISTS(SELECT 1 FROM public.user_custody_open_evidence e WHERE e.source_id=c.source_id AND e.epoch=c.epoch AND e.writer_role=$7) AND NOT EXISTS(SELECT 1 FROM public.user_custody_open_evidence e WHERE e.source_id=c.source_id AND e.epoch=c.epoch AND e.writer_role<>$7))`, r.Request.SourceID, r.Request.Epoch, r.Request.ControlSystemID, r.Physical.SystemIdentifier, r.Physical.Database, r.Physical.DatabaseOID, cfg.WriterRole).Scan(&valid) != nil || !valid {
		return ErrUnavailable
	}
	var sourceSystem, current, user string
	var superuser bool
	admin, err := maintenance.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Release()
	if admin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),current_user,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&sourceSystem, &current, &user, &superuser) != nil || sourceSystem != r.Physical.SystemIdentifier || current == r.Physical.Database || user == cfg.WriterRole || !superuser || admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, r.Physical.Database).Scan(&oid) != nil || oid != r.Physical.DatabaseOID {
		return ErrUnavailable
	}
	return nil
}
