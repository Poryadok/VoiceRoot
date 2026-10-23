package store

import (
	"context"
	_ "embed"
	"strings"

	"github.com/jackc/pgx/v5"
)

const NativeCatalogRowJSONProfile = "native_catalog_rowjson_51"
const nativeCatalogRowJSONDefinitionSHA256 = "acaa61a52c60931e4faba5edd2d6227cdd90a52bed3a586131694e9294d28b95"
const nativeCatalogRowJSONBodySHA256 = "a79f86b8d9f0692cc25f2e896890106161dfeca185ee17e9793c92fbfc29cc02"

//go:embed nativecatalog/catalog-rowjson.sql
var nativeCatalogRowJSONDefinition string

//go:embed nativecatalog/catalog-rowjson-expression.sql
var nativeCatalogRowJSONPreimageSQL string

// The original compiled asset returns raw TEXT, not a bytea digest.
//
//go:embed nativecatalog/catalog50-expression.sql
var nativeCatalog51ParentExpression string

var nativeCatalogRowJSONLiteralExpression = nativeCatalog51DigestSQL(nativeCatalogRowJSONPreimageSQL)

// nativeCatalog51DigestSQL wraps only compiled source preimages and the exact
// source-bound parent-definition substitution. It accepts no runtime SQL input.
func nativeCatalog51DigestSQL(preimageSQL string) string {
	return "SELECT sha256(convert_to((" + preimageSQL + "),'UTF8'))"
}

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

const nativeCatalogRowJSONMigrationSHA256 = "9271fe4135719a204a57e6b0c868f81038d4f320b03ada228f4c5b9fca40da47"

// NativeCatalog51MigrationSHA256 binds retained installer evidence to the exact
// selected migration body. It is source identity, never caller authority.
const NativeCatalog51MigrationSHA256 = nativeCatalogRowJSONMigrationSHA256
