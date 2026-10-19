// Package mmnativecontract observes code-owned native MM composition facts.
// Observations are data, never installation admission or resource fencing.
package mmnativecontract

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/jackc/pgx/v5"
	"strconv"
)

var ErrUnavailable = errors.New("native MM database observation unavailable")

type DatabaseFacts struct {
	DatabaseBindingHash, SchemaContractHash [32]byte
	SessionRoleOID                          uint32
	SessionRoleName                         string
	SessionEqualsCurrent                    bool
}

func ObserveDatabase(ctx context.Context, tx pgx.Tx) (DatabaseFacts, error) {
	var facts DatabaseFacts
	if ctx == nil || tx == nil || ctx.Err() != nil {
		return facts, ErrUnavailable
	}
	var systemID, name, schema string
	var databaseOID, serverVersion uint32
	err := tx.QueryRow(ctx, `SELECT control.system_identifier::text,d.oid::bigint,d.datname,
current_setting('server_version_num')::bigint,r.oid::bigint,session_user::text,session_user=current_user
FROM pg_catalog.pg_control_system() control
JOIN pg_catalog.pg_database d ON d.datname=current_database()
JOIN pg_catalog.pg_roles r ON r.rolname=session_user`).Scan(&systemID, &databaseOID, &name, &serverVersion, &facts.SessionRoleOID, &facts.SessionRoleName, &facts.SessionEqualsCurrent)
	if err != nil || databaseOID == 0 || name == "" || serverVersion == 0 || facts.SessionRoleOID == 0 {
		return DatabaseFacts{}, ErrUnavailable
	}
	n, err := strconv.ParseUint(systemID, 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != systemID {
		return DatabaseFacts{}, ErrUnavailable
	}
	if err = tx.QueryRow(ctx, schemaObservationSQL).Scan(&schema); err != nil || schema == "" {
		return DatabaseFacts{}, ErrUnavailable
	}
	db := sha256.New()
	db.Write([]byte("voice.mm.database-binding.v1\x00"))
	writeU32 := func(value uint32) { var raw [4]byte; binary.BigEndian.PutUint32(raw[:], value); db.Write(raw[:]) }
	writeU32(1)
	writeU32(uint32(len(systemID)))
	db.Write([]byte(systemID))
	writeU32(databaseOID)
	writeU32(uint32(len(name)))
	db.Write([]byte(name))
	copy(facts.DatabaseBindingHash[:], db.Sum(nil))
	ddl := sha256.New()
	ddl.Write([]byte("voice.mm.public-schema-contract.v1\x00"))
	var version [4]byte
	binary.BigEndian.PutUint32(version[:], serverVersion)
	ddl.Write(version[:])
	ddl.Write([]byte(schema))
	copy(facts.SchemaContractHash[:], ddl.Sum(nil))
	return facts, nil
}

// Runtime data, physical object OIDs, owners and ACLs are intentionally absent.
// The caller separately checks login and effective privilege closure. Catalogue
// definitions, trigger enablement, indexes and all functions are covered; a
// dropped retention guard cannot hide behind the same table/column names.
const schemaObservationSQL = `WITH relations AS (
 SELECT c.* FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S')
) SELECT jsonb_build_object(
'relations',COALESCE((SELECT jsonb_agg(jsonb_build_array(c.relname,c.relkind,c.relrowsecurity,c.relforcerowsecurity,
CASE WHEN c.relkind IN ('v','m') THEN pg_get_viewdef(c.oid,true) ELSE NULL END,
CASE WHEN c.relkind='p' THEN pg_get_partkeydef(c.oid) ELSE NULL END,pg_get_expr(c.relpartbound,c.oid)) ORDER BY c.relname) FROM relations c),'[]'::jsonb),
'columns',COALESCE((SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,a.attgenerated,pg_get_expr(d.adbin,d.adrelid)) ORDER BY c.relname,a.attnum)
FROM relations c JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum),'[]'::jsonb),
'constraints',COALESCE((SELECT jsonb_agg(jsonb_build_array(c.relname,x.conname,x.contype,x.condeferrable,x.condeferred,x.convalidated,pg_get_constraintdef(x.oid,true)) ORDER BY c.relname,x.conname)
FROM relations c JOIN pg_catalog.pg_constraint x ON x.conrelid=c.oid),'[]'::jsonb),
'indexes',COALESCE((SELECT jsonb_agg(jsonb_build_array(c.relname,i.relname,x.indisvalid,x.indisready,pg_get_indexdef(i.oid)) ORDER BY c.relname,i.relname)
FROM relations c JOIN pg_catalog.pg_index x ON x.indrelid=c.oid JOIN pg_catalog.pg_class i ON i.oid=x.indexrelid),'[]'::jsonb),
'triggers',COALESCE((SELECT jsonb_agg(jsonb_build_array(c.relname,t.tgname,t.tgenabled,pg_get_triggerdef(t.oid,true)) ORDER BY c.relname,t.tgname)
FROM relations c JOIN pg_catalog.pg_trigger t ON t.tgrelid=c.oid AND NOT t.tgisinternal),'[]'::jsonb),
'functions',COALESCE((SELECT jsonb_agg(jsonb_build_array(p.proname,pg_get_function_identity_arguments(p.oid),pg_get_functiondef(p.oid)) ORDER BY p.proname,pg_get_function_identity_arguments(p.oid))
FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.prokind IN ('f','p')),'[]'::jsonb),
'sequences',COALESCE((SELECT jsonb_agg(jsonb_build_array(c.relname,s.seqstart,s.seqincrement,s.seqmax,s.seqmin,s.seqcache,s.seqcycle) ORDER BY c.relname)
FROM relations c JOIN pg_catalog.pg_sequence s ON s.seqrelid=c.oid),'[]'::jsonb)
)::text`
