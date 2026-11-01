package sourcecustody

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/pkg/runtimeconfig"
)

type RecoveryConfig struct {
	ControlURL, SourceAdminURL string
	Request                    CreationRequest
	Physical                   PhysicalSource
}

func LoadRecoveryConfig(lookup func(string) (string, bool)) (RecoveryConfig, error) {
	var empty RecoveryConfig
	if lookup == nil {
		return empty, ErrUnavailable
	}
	names := []string{"CUSTODY_CONTROL_DATABASE_URL", "CUSTODY_SOURCE_ADMIN_DATABASE_URL", "CUSTODY_CONTROL_SYSTEM_ID", "CUSTODY_SOURCE_SYSTEM_ID", "CUSTODY_SOURCE_DATABASE", "CUSTODY_SOURCE_DATABASE_OID", "CUSTODY_SOURCE_ID", "CUSTODY_EPOCH"}
	values := make([]string, len(names))
	for i, name := range names {
		value, ok := lookup(name)
		if !ok || value == "" || value != strings.TrimSpace(value) {
			return empty, ErrUnavailable
		}
		values[i] = value
	}
	oid, err := strconv.ParseUint(values[5], 10, 32)
	if err != nil || oid == 0 || strconv.FormatUint(oid, 10) != values[5] {
		return empty, ErrUnavailable
	}
	epoch, err := strconv.ParseInt(values[7], 10, 64)
	if err != nil || epoch <= 0 || strconv.FormatInt(epoch, 10) != values[7] || !validControlURL(values[0]) || !validControlURL(values[1]) || !systemID.MatchString(values[2]) || !systemID.MatchString(values[3]) || values[2] == values[3] || !databaseName.MatchString(values[4]) || !sourceIDPattern.MatchString(values[6]) {
		return empty, ErrUnavailable
	}
	return RecoveryConfig{ControlURL: values[0], SourceAdminURL: values[1], Request: CreationRequest{ControlSystemID: values[2], SourceSystemID: values[3], Database: values[4], SourceID: values[6], Epoch: epoch}, Physical: PhysicalSource{SystemIdentifier: values[3], Database: values[4], DatabaseOID: uint32(oid)}}, nil
}

// RunRecovery is a closed-only administrative operation. Its input pins and
// TLS credentials do not themselves prove production administrative custody.
func RunRecovery(ctx context.Context, cfg RecoveryConfig) error {
	if ctx == nil || ctx.Err() != nil || !validControlURL(cfg.ControlURL) || !validControlURL(cfg.SourceAdminURL) {
		return ErrUnavailable
	}
	if forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, cfg.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	admin, err := openPinnedPostgres(ctx, cfg.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	return RecoverInterruptedInitialization(ctx, control, admin, cfg.Request, cfg.Physical)
}

// RunQuarantine fences an explicitly pinned existing database; it neither
// validates its history nor makes it available to capture.
func RunQuarantine(ctx context.Context, cfg RecoveryConfig) error {
	if ctx == nil || ctx.Err() != nil || !validControlURL(cfg.ControlURL) || !validControlURL(cfg.SourceAdminURL) || forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, cfg.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	admin, err := openPinnedPostgres(ctx, cfg.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	return QuarantineExistingSource(ctx, control, admin, cfg.Request, cfg.Physical)
}

func forbiddenPGEnvironment() bool {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "PG") {
			return true
		}
	}
	return false
}

func openPinnedPostgres(ctx context.Context, raw string) (*pgxpool.Pool, error) {
	config, err := pinnedPostgresConfig(raw)
	if err != nil {
		return nil, ErrUnavailable
	}
	return pgxpool.NewWithConfig(ctx, config)
}

func pinnedPostgresConfig(raw string) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, ErrUnavailable
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, ErrUnavailable
	}
	pg := config.ConnConfig
	if pg.Host != parsed.Hostname() || len(pg.Fallbacks) != 0 || pg.TLSConfig == nil || pg.TLSConfig.InsecureSkipVerify || pg.TLSConfig.ServerName != parsed.Hostname() || pg.TLSConfig.RootCAs == nil || len(pg.RuntimeParams) != 0 {
		return nil, ErrUnavailable
	}
	return config, nil
}

// ValidateUserSourceURL refuses libpq overrides and requires one verified TLS
// endpoint. It does not establish the actual physical source identity.
func ValidateUserSourceURL(raw string) error {
	if !validControlURL(raw) || forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	return nil
}

// OpenUserSourcePool checks transport; callers must still check physical source
// identity under a held qualified custody grant on the actual transaction.
func OpenUserSourcePool(ctx context.Context, raw string) (*pgxpool.Pool, error) {
	if ctx == nil || ctx.Err() != nil || ValidateUserSourceURL(raw) != nil {
		return nil, ErrUnavailable
	}
	config, err := pinnedPostgresConfig(raw)
	if err != nil {
		return nil, ErrUnavailable
	}
	config.MaxConns = runtimeconfig.PostgresMaxConnsFromEnv()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, ErrUnavailable
	}
	if pool.Ping(ctx) != nil || ctx.Err() != nil {
		pool.Close()
		return nil, ErrUnavailable
	}
	return pool, nil
}
