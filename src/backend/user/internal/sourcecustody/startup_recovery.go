package sourcecustody

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// recoverBeforeStart fences a previous incarnation before a new controller
// can take leadership. It requires the independent administrator/restore
// custody named in the operator decision; identifiers alone do not prove it.
func recoverBeforeStart(ctx context.Context, control *pgxpool.Pool, cfg *ActivationOperatorConfig, binding Binding) error {
	if ctx == nil || ctx.Err() != nil || control == nil || cfg == nil || !binding.valid() || !validControlURL(cfg.AdminURL) {
		return ErrUnavailable
	}
	var phase, controlSystem, sourceSystem, database string
	var sourceOID uint32
	var history, generation uuid.UUID
	var digest []byte
	if control.QueryRow(ctx, `SELECT phase,control_system_id,source_system_id,database_name,database_oid,history_generation,user_source_generation,keyset_digest FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch).Scan(&phase, &controlSystem, &sourceSystem, &database, &sourceOID, &history, &generation, &digest) != nil || !same(binding, binding.Epoch, binding.SourceID, history, generation, digest) || !systemID.MatchString(controlSystem) || !systemID.MatchString(sourceSystem) || controlSystem == sourceSystem || !databaseName.MatchString(database) || sourceOID == 0 {
		return ErrUnavailable
	}
	switch phase {
	case "INITIALIZED_CLOSED", "EXISTING_INITIALIZED_CLOSED", "RECOVERED_CLOSED":
		return nil
	case "PREPARING_OPEN", "ACTIVE", "RECOVERY_REQUIRED":
	default:
		return ErrUnavailable
	}
	var physical PhysicalSource
	if control.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID) != nil || physical.SystemIdentifier != sourceSystem || physical.Database != database || physical.DatabaseOID != sourceOID {
		return ErrUnavailable
	}
	var count, roles int
	var writer string
	if control.QueryRow(ctx, `SELECT count(*),count(DISTINCT writer_role),COALESCE(min(writer_role),'') FROM public.user_custody_open_evidence WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch).Scan(&count, &roles, &writer) != nil || count == 0 || roles != 1 || !databaseName.MatchString(writer) {
		return ErrUnavailable
	}
	admin, err := openPinnedPostgres(ctx, cfg.AdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	request := CreationRequest{SourceID: binding.SourceID, Epoch: binding.Epoch, ControlSystemID: controlSystem, SourceSystemID: sourceSystem, Database: database}
	return RecoverInterruptedActivation(ctx, control, admin, request, physical, writer)
}
