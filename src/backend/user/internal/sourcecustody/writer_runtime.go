package sourcecustody

import (
	"context"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WriterConfig struct {
	Recovery   RecoveryConfig
	WriterRole string
}

func LoadWriterConfig(lookup func(string) (string, bool)) (WriterConfig, error) {
	if lookup == nil {
		return WriterConfig{}, ErrUnavailable
	}
	recovery, err := LoadRecoveryConfig(lookup)
	role, present := lookup("CUSTODY_WRITER_ROLE")
	if err != nil || !present || !databaseName.MatchString(role) {
		return WriterConfig{}, ErrUnavailable
	}
	return WriterConfig{Recovery: recovery, WriterRole: role}, nil
}

// RunWriterConfinement is a read-only operator inspection, not a reopening or
// deployment qualification. Exclusive administration and session-pinned
// transport remain external prerequisites.
func RunWriterConfinement(ctx context.Context, cfg WriterConfig) error {
	r := cfg.Recovery
	if ctx == nil || ctx.Err() != nil || !validControlURL(r.ControlURL) || !validControlURL(r.SourceAdminURL) || forbiddenPGEnvironment() || !databaseName.MatchString(cfg.WriterRole) || !databaseName.MatchString(r.Request.Database) || !sourceIDPattern.MatchString(r.Request.SourceID) || r.Request.Epoch <= 0 || !systemID.MatchString(r.Request.ControlSystemID) || !systemID.MatchString(r.Request.SourceSystemID) || r.Request.ControlSystemID == r.Request.SourceSystemID || r.Physical.SystemIdentifier != r.Request.SourceSystemID || r.Physical.Database != r.Request.Database || r.Physical.DatabaseOID == 0 {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	controlConfig, err := pgxpool.ParseConfig(r.ControlURL)
	if err != nil || controlConfig.ConnConfig.Database == "" {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, r.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	tx, err := control.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	var system, database string
	var controlOID uint32
	var pinned PhysicalSource
	if tx.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &controlOID) != nil || system != r.Request.ControlSystemID || system == r.Physical.SystemIdentifier || database != controlConfig.ConnConfig.Database || controlOID == 0 || tx.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR SHARE`, r.Request.SourceID, r.Request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != r.Physical {
		return ErrUnavailable
	}
	u, err := url.Parse(r.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	u.Path = "/" + r.Physical.Database
	u.RawPath = ""
	source, err := openPinnedPostgres(ctx, u.String())
	if err != nil {
		return ErrUnavailable
	}
	defer source.Close()
	if CheckWriterConfinement(ctx, source, r.Physical, cfg.WriterRole) != nil || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}

// RunActivationRecovery invokes the fencing recovery through independently
// verified control and maintenance connections. It never reopens admission.
func RunActivationRecovery(ctx context.Context, cfg WriterConfig) error {
	r := cfg.Recovery
	if ctx == nil || ctx.Err() != nil || !validControlURL(r.ControlURL) || !validControlURL(r.SourceAdminURL) || forbiddenPGEnvironment() || !databaseName.MatchString(cfg.WriterRole) {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, r.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	maintenance, err := openPinnedPostgres(ctx, r.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer maintenance.Close()
	return RecoverInterruptedActivation(ctx, control, maintenance, r.Request, r.Physical, cfg.WriterRole)
}
