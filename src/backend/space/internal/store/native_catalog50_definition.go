package store

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
)

// Finite complete source-derived PostgreSQL16/18 serializations, including
// immutable original CRLF. No definition is learned from the target installer.
//
//go:embed nativecatalog/catalog45.sql
var nativeCatalog45Definition string

//go:embed nativecatalog/catalog50.sql
var nativeCatalog50Definition string

//go:embed nativecatalog/catalog44-expression.sql
var nativeCatalog44LiteralExpression string

const nativeCatalog50BodySHA = "ce9d3ab4356845d502e90e07548b2e081d8d142b4b019d57378654645733ca40"

func nativeCatalog50Definitions(serverVersion int) (string, string, error) {
	major := serverVersion / 10000
	if major != 16 && major != 18 {
		return "", "", ErrMemberAuthorityUnavailable
	}
	oldSHA, newSHA := sha256.Sum256([]byte(nativeCatalog45Definition)), sha256.Sum256([]byte(nativeCatalog50Definition))
	if hex.EncodeToString(oldSHA[:]) != "fbcc6e77757fffde5ef564595f124494028af09d6c6d72e160d38cd61993e80d" || hex.EncodeToString(newSHA[:]) != "29dd05a9e9314d33d8ebe8ca1cdd06f59cd36ab013354482d86c490c063e887e" {
		return "", "", ErrMemberAuthorityUnavailable
	}
	return nativeCatalog45Definition, nativeCatalog50Definition, nil
}

// Authenticate the complete new definition before permitting substitution of
// exactly this function's original definition. OID/owner/ACL remain live fields.
func requireNativeCatalog50Definition(ctx context.Context, tx pgx.Tx) (uint32, string, error) {
	var version int
	if err := tx.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&version); err != nil {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	oldDefinition, newDefinition, err := nativeCatalog50Definitions(version)
	if err != nil {
		return 0, "", err
	}
	var oid uint32
	var actual string
	if err = tx.QueryRow(ctx, `SELECT p.oid,pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname='native_membership_guard' AND p.proname='catalog' AND p.pronargs=0 AND p.prokind='f' AND p.prorettype='bytea'::regtype`).Scan(&oid, &actual); err != nil || actual != newDefinition {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	return oid, oldDefinition, nil
}
