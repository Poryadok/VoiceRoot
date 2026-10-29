package nativeinstaller

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InspectLegacy24 qualifies only the current read-only source snapshot. The
// controller retains actual image/operation custody and repeats admission
// under the writer cut; this observation is not future migration authority.
func InspectLegacy24(ctx context.Context, c Config) (err error) {
	if ctx == nil || ctx.Err() != nil || c.Output == nil || c.Mode != "" || c.Prior != nil {
		return errors.New("explicit legacy24 inspection configuration required")
	}
	if !legacy24Hex(SourceRevision, 20) || !legacy24Hex(BuildSourceSHA256, 32) || !legacy24Hex(MigrateSHA256, 32) ||
		(c.RequiredBuildSHA256 != "" && c.RequiredBuildSHA256 != BuildSourceSHA256) {
		return errors.New("trusted legacy24 inspection build identities required")
	}
	inventory, err := ValidateSources(c.MigrationsDir)
	if err != nil {
		return err
	}
	migrator, err := fileSHA(c.MigratePath)
	if err != nil || migrator != MigrateSHA256 {
		return errors.New("pinned migrate executable mismatch")
	}
	executable, err := os.Executable()
	if err != nil {
		return errors.New("installer executable unavailable")
	}
	executableSHA, err := fileSHA(executable)
	if err != nil {
		return errors.New("installer identity unavailable")
	}
	cfg, _, err := installerConnections(c.AdminURL)
	if err != nil {
		return err
	}
	cfg.Database = "space_db"
	cfg.RuntimeParams["search_path"] = "pg_catalog"
	cfg.RuntimeParams["application_name"] = "native-legacy24-inspect-" + uuid.NewString()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("HOLD: existing target unavailable")
	}
	defer func() {
		if closeErr := conn.Close(context.Background()); closeErr != nil {
			err = errors.Join(err, errors.New("legacy24 inspection close failed"))
		}
	}()
	origin, err := admitLegacy24Source(ctx, conn)
	if err != nil {
		return errors.New("HOLD: current legacy24 source is not admitted")
	}
	evidence := struct {
		Event            string `json:"event"`
		Schema           string `json:"schema"`
		SourceRevision   string `json:"source_revision"`
		BuildSHA256      string `json:"build_sha256"`
		ExecutableSHA256 string `json:"executable_sha256"`
		MigratorSHA256   string `json:"migrator_sha256"`
		InventorySHA256  string `json:"inventory_sha256"`
		Target           struct {
			DatabaseName     string `json:"database_name"`
			DatabaseOID      uint32 `json:"database_oid"`
			DatabaseSystemID string `json:"database_system_id"`
			DatabaseOwner    string `json:"database_owner"`
			InstallerRole    string `json:"installer_role"`
			Version          int64  `json:"version"`
		} `json:"target"`
	}{Event: "inspected-legacy24", Schema: "voice.space.native-legacy24-inspection.v1", SourceRevision: SourceRevision, BuildSHA256: BuildSourceSHA256, ExecutableSHA256: executableSHA, MigratorSHA256: migrator, InventorySHA256: inventory}
	evidence.Target.DatabaseName = origin.Target.DatabaseName
	evidence.Target.DatabaseOID = origin.Target.DatabaseOID
	evidence.Target.DatabaseSystemID = origin.Target.DatabaseSystemID
	evidence.Target.DatabaseOwner = origin.DatabaseOwner
	evidence.Target.InstallerRole = origin.Target.InstallerRole
	evidence.Target.Version = origin.Target.Version
	raw, err := json.Marshal(evidence)
	if err != nil {
		return errors.New("legacy24 inspection evidence unavailable")
	}
	raw = append(raw, '\n')
	n, err := c.Output.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}
