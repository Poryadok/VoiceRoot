package sourcecustody

import (
	"context"
	"strconv"
	"strings"

	"voice/backend/user/internal/identityassociation"
)

type CreationConfig struct {
	ControlURL, SourceAdminURL          string
	Request                             CreationRequest
	MigrationsDir, KeyManifest, KeysDir string
}

func LoadCreationConfig(lookup func(string) (string, bool)) (CreationConfig, error) {
	var empty CreationConfig
	if lookup == nil {
		return empty, ErrUnavailable
	}
	names := []string{"CUSTODY_CONTROL_DATABASE_URL", "CUSTODY_SOURCE_ADMIN_DATABASE_URL", "CUSTODY_CONTROL_SYSTEM_ID", "CUSTODY_SOURCE_SYSTEM_ID", "CUSTODY_SOURCE_DATABASE", "CUSTODY_SOURCE_ID", "CUSTODY_EPOCH", "CUSTODY_USER_MIGRATIONS_DIR", "CUSTODY_USER_KEY_MANIFEST", "CUSTODY_USER_KEYS_DIR"}
	values := make([]string, len(names))
	for i, name := range names {
		value, ok := lookup(name)
		if !ok || value == "" || value != strings.TrimSpace(value) {
			return empty, ErrUnavailable
		}
		values[i] = value
	}
	epoch, err := strconv.ParseInt(values[6], 10, 64)
	if err != nil || epoch <= 0 || strconv.FormatInt(epoch, 10) != values[6] || !validControlURL(values[0]) || !validControlURL(values[1]) || !systemID.MatchString(values[2]) || !systemID.MatchString(values[3]) || values[2] == values[3] || !databaseName.MatchString(values[4]) || !sourceIDPattern.MatchString(values[5]) {
		return empty, ErrUnavailable
	}
	return CreationConfig{ControlURL: values[0], SourceAdminURL: values[1], Request: CreationRequest{ControlSystemID: values[2], SourceSystemID: values[3], Database: values[4], SourceID: values[5], Epoch: epoch}, MigrationsDir: values[7], KeyManifest: values[8], KeysDir: values[9]}, nil
}

// RunCreation creates only a new closed source. It is not a deployment custody
// proof, does not adopt an existing database and cannot open capture.
func RunCreation(ctx context.Context, cfg CreationConfig) error {
	if ctx == nil || ctx.Err() != nil || !validControlURL(cfg.ControlURL) || !validControlURL(cfg.SourceAdminURL) || forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	if _, err := migrationArchive(cfg.MigrationsDir); err != nil {
		return ErrUnavailable
	}
	keys, err := identityassociation.LoadProductionKeyset(cfg.KeyManifest, cfg.KeysDir)
	if err != nil || keys.Snapshot().Revision != 1 {
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
	physical, err := CreateClosedSource(ctx, control, admin, cfg.Request)
	if err != nil {
		return ErrUnavailable
	}
	return InitializeClosedSource(ctx, control, admin, cfg.Request, physical, cfg.MigrationsDir, keys)
}

// RunExistingMigration operates only after the independently recorded
// quarantine. The inputs are not proof of administrative custody or continuity.
func RunExistingMigration(ctx context.Context, creation CreationConfig, recovery RecoveryConfig) error {
	if ctx == nil || ctx.Err() != nil || creation.Request != recovery.Request || creation.ControlURL != recovery.ControlURL || creation.SourceAdminURL != recovery.SourceAdminURL || !validControlURL(creation.ControlURL) || !validControlURL(creation.SourceAdminURL) || forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	if _, err := migrationArchive(creation.MigrationsDir); err != nil {
		return ErrUnavailable
	}
	keys, err := identityassociation.LoadProductionKeyset(creation.KeyManifest, creation.KeysDir)
	if err != nil || keys.Snapshot().Revision != 1 {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, creation.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	admin, err := openPinnedPostgres(ctx, creation.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	return MigrateQuarantinedSource(ctx, control, admin, creation.Request, recovery.Physical, creation.MigrationsDir, keys)
}
