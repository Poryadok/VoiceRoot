package nativeapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/matchmaking/internal/mmadmission"
	w "voice/backend/pkg/mmadmissionwire"
	"voice/backend/pkg/mmnativecontract"
)

func localBinding(b w.Binding) mmadmission.Binding {
	return mmadmission.Binding{ProtocolVersion: b.ProtocolVersion, EnvironmentID: uuid.UUID(b.EnvironmentID), InstallationID: uuid.UUID(b.InstallationID), AuthorityIncarnation: uuid.UUID(b.AuthorityIncarnation), TransitionID: uuid.UUID(b.TransitionID), AdmissionGeneration: b.AdmissionGeneration, CredentialCohortID: uuid.UUID(b.CredentialCohortID), DatabaseBindingHash: [32]byte(b.DatabaseBindingHash), SchemaContractHash: [32]byte(b.SchemaContractHash), SourceFactoryManifestHash: [32]byte(b.SourceFactoryManifestHash), FreshRouteManifestHash: [32]byte(b.FreshRouteManifestHash), AdmissionReceiptHash: [32]byte(b.AdmissionReceiptHash), LaunchManifestHash: [32]byte(b.LaunchManifestHash), ExpectedLoginRoleOID: b.ExpectedLoginRoleOID, ExpectedLoginRoleName: b.ExpectedLoginRoleName}
}

func checkNativeDatabase(ctx context.Context, pool *pgxpool.Pool, descriptor w.DescriptorV1, expected w.AdmissionStampV1) error {
	if ctx == nil || pool == nil || descriptor.Validate() != nil || expected.Validate() != nil {
		return errUnavailable
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return errUnavailable
	}
	defer tx.Rollback(ctx)
	binding := localBinding(descriptor.Binding)
	if mmadmission.Acquire(ctx, tx, &binding) != nil {
		return errUnavailable
	}
	facts, err := mmnativecontract.ObserveDatabase(ctx, tx)
	if err != nil || !facts.SessionEqualsCurrent || facts.SessionRoleOID != descriptor.Binding.ExpectedLoginRoleOID || facts.SessionRoleName != descriptor.Binding.ExpectedLoginRoleName || facts.DatabaseBindingHash != [32]byte(descriptor.Binding.DatabaseBindingHash) || facts.SchemaContractHash != [32]byte(descriptor.Binding.SchemaContractHash) {
		return errUnavailable
	}
	if checkRuntimePrivileges(ctx, tx) != nil {
		return errUnavailable
	}
	history, err := mmadmission.ReadHistory(ctx, tx, descriptor.Binding)
	if err != nil || history.Stamp() != expected {
		return errUnavailable
	}
	scope, scopeID, controller := history.Authority()
	if scope != descriptor.AuthorityScope || scopeID != descriptor.ScopeID || controller != descriptor.ControllerID || expected.DatabaseLineageID != descriptor.DatabaseLineageID {
		return errUnavailable
	}
	if tx.Commit(ctx) != nil {
		return errUnavailable
	}
	return nil
}

func checkRuntimePrivileges(ctx context.Context, tx pgx.Tx) error {
	if ctx == nil || tx == nil {
		return errUnavailable
	}
	var denied bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM pg_catalog.pg_roles r
 WHERE pg_has_role(session_user,r.oid,'MEMBER') AND (
 r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls
 OR has_schema_privilege(r.oid,'public','CREATE')
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S') AND c.relowner=r.oid)
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.prosecdef AND has_function_privilege(r.oid,p.oid,'EXECUTE'))
 OR EXISTS (SELECT 1 FROM (VALUES ('mm_native_admission_head'),('mm_native_admission_receipts'),('mm_native_continuity_records')) p(name)
  WHERE has_table_privilege(r.oid,('public.'||p.name),'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
     OR has_any_column_privilege(r.oid,('public.'||p.name),'INSERT,UPDATE,REFERENCES'))
 OR has_table_privilege(r.oid,'public.mm_native_party_origins','UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
 OR has_any_column_privilege(r.oid,'public.mm_native_party_origins','UPDATE,REFERENCES')
 )) OR EXISTS(SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relrowsecurity)`).Scan(&denied)
	if err != nil || denied {
		return errUnavailable
	}
	var login bool
	if tx.QueryRow(ctx, `SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname=session_user`).Scan(&login) != nil || !login {
		return errUnavailable
	}
	return nil
}
