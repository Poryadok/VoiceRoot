package store

import (
	"context"
	_ "embed"
	"github.com/jackc/pgx/v5"
	"strings"
)

const NativeCatalogRowJSONProfile = "native_catalog_rowjson_51"
const nativeCatalogRowJSONDefinitionSHA256 = "81b5cd6de226052b5d2a476e36c8641baa8008657d7fcf8a8bb6f28ffe5f57fd"
const nativeCatalogRowJSONBodySHA256 = "251e5add35a90548dd6ec0e4760d6b6007106aa2bb72b30f9fe6d23d1e9bc1b2"

//go:embed nativecatalog/catalog-rowjson.sql
var nativeCatalogRowJSONDefinition string

//go:embed nativecatalog/catalog-rowjson-expression.sql
var nativeCatalogRowJSONLiteralExpression string

//go:embed nativecatalog/catalog50-expression.sql
var nativeCatalog51ParentExpression string

var nativeCatalogRowJSONBody = strings.Split(strings.Split(nativeCatalogRowJSONDefinition, "AS $function$")[1], "$function$")[0]

func nativeCatalogRowJSONParentDefinition(server int) (string, error) {
	_, parent, err := nativeCatalog50Definitions(server)
	return parent, err
}

func requireNativeCatalogRowJSONDefinition(ctx context.Context, tx pgx.Tx) (uint32, string, error) {
	if err := requireNativeCatalogRowJSONAdmission(); err != nil {
		return 0, "", err
	}
	if nativeSourceBodySHA(nativeCatalogRowJSONDefinition) != nativeCatalogRowJSONDefinitionSHA256 || nativeSourceBodySHA(nativeCatalogRowJSONBody) != nativeCatalogRowJSONBodySHA256 {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	var server int
	if err := tx.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&server); err != nil {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	parent, err := nativeCatalogRowJSONParentDefinition(server)
	if err != nil {
		return 0, "", err
	}
	var oid uint32
	var actual string
	if err := tx.QueryRow(ctx, `SELECT p.oid,pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='native_membership_guard' AND p.proname='catalog' AND p.pronargs=0 AND p.prokind='f' AND p.prorettype='bytea'::regtype`).Scan(&oid, &actual); err != nil || actual != nativeCatalogRowJSONDefinition {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	return oid, parent, nil
}

const nativeCatalogRowJSONMigrationSHA256 = "23b70cc0ad088e03ef928a70fce5177c4d7d21c89efa6c0368d0aea98814bbaf"

// NativeCatalog51MigrationSHA256 binds retained installer evidence to the exact
// selected migration body. It is source identity, never caller authority.
const NativeCatalog51MigrationSHA256 = nativeCatalogRowJSONMigrationSHA256
