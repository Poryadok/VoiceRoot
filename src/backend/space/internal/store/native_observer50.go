package store

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
)

// These complete definitions are finite serializations from the exact source
// migrations on PostgreSQL16 and18, including the original M44 body's CRLF.
// Both qualified engines emit the same texts. They are compiled installer
// inputs, never definitions learned from the target being enrolled.
//
//go:embed nativecatalog/source_after45.sql
var nativeObserver45Definition string

//go:embed nativecatalog/source_after50.sql
var nativeObserver50Definition string

const nativeObserver50BodySHA = "1fd5020206aac576dc641ebfd1d0d3b22eca3e34854f07663c5ca100897fa5d6"

func nativeObserver50Definitions(serverVersion int) (string, string, error) {
	major := serverVersion / 10000
	if major != 16 && major != 18 {
		return "", "", ErrMemberAuthorityUnavailable
	}
	oldSHA, newSHA := sha256.Sum256([]byte(nativeObserver45Definition)), sha256.Sum256([]byte(nativeObserver50Definition))
	if hex.EncodeToString(oldSHA[:]) != "c80b82f6c4bc1140a71fea01b6135bdac6fba6c2a32988e4b8173011ef75776d" ||
		hex.EncodeToString(newSHA[:]) != "4ae8979ad327f5fc9139f55b1e903fe3e8b52fb8f1dd0cdb88c3ec9aaaa79e57" {
		return "", "", ErrMemberAuthorityUnavailable
	}
	return nativeObserver45Definition, nativeObserver50Definition, nil
}

// The caller also authenticates the complete guard metadata, closure and ACLs.
// Return ONLY the fixed original definition for this exact live50 function OID;
// the catalog preimage retains that OID, owner, ACL and every other live field.
func requireNativeObserver50Definition(ctx context.Context, tx pgx.Tx) (uint32, string, error) {
	var serverVersion int
	if err := tx.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&serverVersion); err != nil {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	oldDefinition, newDefinition, err := nativeObserver50Definitions(serverVersion)
	if err != nil {
		return 0, "", err
	}
	var oid uint32
	var actual string
	if err = tx.QueryRow(ctx, `SELECT p.oid,pg_get_functiondef(p.oid)
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname='native_membership_guard' AND p.proname='source_after'
 AND p.pronargs=0 AND p.prokind='f' AND p.prorettype='trigger'::regtype`).Scan(&oid, &actual); err != nil || actual != newDefinition {
		return 0, "", ErrMemberAuthorityUnavailable
	}
	return oid, oldDefinition, nil
}
