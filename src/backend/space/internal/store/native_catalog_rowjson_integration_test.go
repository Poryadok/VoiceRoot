//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type native51ProofCheck uint8

const (
	native51EnvDSN native51ProofCheck = iota
	native51PoolOpen
	native51OldTxBegin
	native51OldSource
	native51OldRollback
	native51CandidateBegin
	native51CandidateRollback
	native51InstallDefinition
	native51OldPreimage
	native51NewPreimage
	native51NarrowedDistinct
	native51SessionApply
	native51SessionQuery
	native51ActualDigestLength
	native51OracleDigestLength
	native51CompleteMetadata
	native51NarrowedRejected
	native51DigestEquality
	native51RawStability
	native51Nonvacuous
	native51UnknownInstall
	native51UnknownRejected
)

func native51ProofCheckName(check native51ProofCheck) string {
	names := [...]string{"ENV_DSN_PRESENT", "POOL_OPEN", "OLD50_TX_BEGIN", "OLD50_SOURCE", "OLD50_TX_ROLLBACK", "CANDIDATE_TX_BEGIN", "CANDIDATE_TX_ROLLBACK", "INSTALL_DEFINITION", "OLD_PREIMAGE_CONTRACT", "NEW_PREIMAGE_CONTRACT", "NARROWED_CONTROL_DISTINCT", "SESSION_APPLY", "SESSION_QUERY", "ACTUAL_DIGEST_LENGTH", "ORACLE_DIGEST_LENGTH", "COMPLETE_METADATA", "NARROWED_CONTROL_REJECTED", "ACTUAL_DIGEST_EQUALS_ORACLE", "RAW_STABILITY_IMPLICATION", "NONVACUOUS_STABLE_PAIRS", "UNKNOWN_INSTALL", "UNKNOWN_REJECTED"}
	if int(check) >= len(names) {
		return "INVALID_CHECK"
	}
	return names[check]
}

func native51ProofSession(session int) string {
	if session == -1 {
		return "NONE"
	}
	if session < 0 || session > 10 {
		return "INVALID_SESSION"
	}
	return "S" + strconv.Itoa(session)
}

func native51ProofErrorClass(err error) string {
	switch {
	case err == nil:
		return "NONE"
	case errors.Is(err, context.Canceled):
		return "CANCELED"
	case errors.Is(err, context.DeadlineExceeded):
		return "DEADLINE"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "55000" {
			return "PG_55000"
		}
		return "OTHER_PG"
	}
	return "OTHER"
}

// Only source-bound identities/error enums cross the proof output boundary.
// FailNow never formats errors, SQL, DSNs, catalog bytes or server diagnostics.
func native51ProofRequire(t *testing.T, check native51ProofCheck, session int, ok bool, err error) {
	t.Helper()
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	t.Logf("NATIVE51_R0_V1 check=%s session=%s result=%s error=%s", native51ProofCheckName(check), native51ProofSession(session), status, native51ProofErrorClass(err))
	if !ok {
		t.FailNow()
	}
}

func native51Old50SourceFixture(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	native51ProofRequire(t, native51EnvDSN, -1, dsn != "", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	native51ProofRequire(t, native51PoolOpen, -1, err == nil, err)
	t.Cleanup(pool.Close)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	native51ProofRequire(t, native51OldTxBegin, -1, err == nil, err)
	_, err = nativeCatalog51SourceFacts(ctx, tx, 50)
	native51ProofRequire(t, native51OldSource, -1, err == nil, err)
	err = tx.Rollback(ctx)
	native51ProofRequire(t, native51OldRollback, -1, err == nil, err)
	return ctx, pool
}

func native51TestPreimage(t *testing.T, preimage, digest string, check native51ProofCheck) string {
	t.Helper()
	native51ProofRequire(t, check, -1, strings.HasPrefix(preimage, "SELECT ") && digest == nativeCatalog51DigestSQL(preimage), nil)
	return preimage
}

// This qualifies the ACTUAL new catalog body on one controlled rolled-back
// source cut. It does not admit51, touch epoch/history or measure Ban performance.
func TestNative51RawCompositeSessionStability(t *testing.T) {
	ctx, pool := native51Old50SourceFixture(t)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	native51ProofRequire(t, native51CandidateBegin, -1, err == nil, err)
	defer func() {
		err := tx.Rollback(context.Background())
		native51ProofRequire(t, native51CandidateRollback, -1, err == nil, err)
	}()
	_, err = tx.Exec(ctx, nativeCatalogRowJSONDefinition+";")
	native51ProofRequire(t, native51InstallDefinition, -1, err == nil, err)
	old := native51TestPreimage(t, nativeCatalog51ParentExpression, nativeCatalog51DigestSQL(nativeCatalog51ParentExpression), native51OldPreimage)
	current := native51TestPreimage(t, nativeCatalogRowJSONPreimageSQL, nativeCatalogRowJSONLiteralExpression, native51NewPreimage)
	narrowed := strings.Replace(current, "row_to_json(a)", "json_build_object('attrelid',a.attrelid)", 1)
	native51ProofRequire(t, native51NarrowedDistinct, -1, current != narrowed, nil)
	cases := []struct{ name, settings string }{
		{"baseline", "SET LOCAL DateStyle='ISO,YMD'; SET LOCAL IntervalStyle='postgres'; SET LOCAL TimeZone='UTC'; SET LOCAL extra_float_digits=1; SET LOCAL bytea_output='hex'; SET LOCAL standard_conforming_strings=on"},
		{"SQL dates", "SET LOCAL DateStyle='SQL,DMY'"},
		{"SQL intervals", "SET LOCAL IntervalStyle='sql_standard'"},
		{"ISO intervals", "SET LOCAL IntervalStyle='iso_8601'"},
		{"Auckland timezone", "SET LOCAL TimeZone='Pacific/Auckland'"},
		{"low float digits", "SET LOCAL extra_float_digits=-3"},
		{"minimum float digits", "SET LOCAL extra_float_digits=-15"},
		{"bytea escape", "SET LOCAL bytea_output='escape'"},
		{"strings off", "SET LOCAL standard_conforming_strings=off"},
		{"combined", "SET LOCAL DateStyle='SQL,DMY'; SET LOCAL IntervalStyle='sql_standard'; SET LOCAL TimeZone='Pacific/Auckland'; SET LOCAL extra_float_digits=-3; SET LOCAL bytea_output='escape'; SET LOCAL standard_conforming_strings=off"},
		{"explicit float3", "SET LOCAL DateStyle='ISO,YMD'; SET LOCAL IntervalStyle='postgres'; SET LOCAL TimeZone='UTC'; SET LOCAL extra_float_digits=3; SET LOCAL bytea_output='hex'; SET LOCAL standard_conforming_strings=on"},
	}
	var baselineOld, baselineNew []byte
	stablePairs := 0
	for i, test := range cases {
		_, err = tx.Exec(ctx, test.settings)
		native51ProofRequire(t, native51SessionApply, i, err == nil, err)
		var oldBytes, newBytes, actual, oracle []byte
		var complete, rejectsNarrowed bool
		err = tx.QueryRow(ctx, `SELECT convert_to((`+old+`),'UTF8'),convert_to((`+current+`),'UTF8'),native_membership_guard.catalog(),(`+nativeCatalogRowJSONLiteralExpression+`),((`+old+`)::jsonb=(`+current+`)::jsonb),((`+old+`)::jsonb<>(`+narrowed+`)::jsonb)`).Scan(&oldBytes, &newBytes, &actual, &oracle, &complete, &rejectsNarrowed)
		native51ProofRequire(t, native51SessionQuery, i, err == nil, err)
		native51ProofRequire(t, native51ActualDigestLength, i, len(actual) == 32, nil)
		native51ProofRequire(t, native51OracleDigestLength, i, len(oracle) == 32, nil)
		native51ProofRequire(t, native51CompleteMetadata, i, complete, nil)
		native51ProofRequire(t, native51NarrowedRejected, i, rejectsNarrowed, nil)
		native51ProofRequire(t, native51DigestEquality, i, string(oracle) == string(actual), nil)
		if i == 0 {
			baselineOld, baselineNew = oldBytes, newBytes
		}
		if string(oldBytes) == string(baselineOld) {
			stablePairs++
			native51ProofRequire(t, native51RawStability, i, string(baselineNew) == string(newBytes), nil)
		}
		t.Logf("NATIVE51_R0_V1 session=%s oldSHA=%s rawNSHA=%s sameCut=true complete=true", native51ProofSession(i), nativeSourceBodySHA(string(oldBytes)), nativeSourceBodySHA(string(newBytes)))
	}
	native51ProofRequire(t, native51Nonvacuous, -1, stablePairs >= 3, nil)
}

func TestNative51ClassifierRejectsUnclassifiedMissingDatum(t *testing.T) {
	ctx, pool := native51Old50SourceFixture(t)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	native51ProofRequire(t, native51CandidateBegin, -1, err == nil, err)
	defer func() {
		err := tx.Rollback(context.Background())
		native51ProofRequire(t, native51CandidateRollback, -1, err == nil, err)
	}()
	_, err = tx.Exec(ctx, nativeCatalogRowJSONDefinition+`; ALTER TABLE public.spaces ADD COLUMN native51_unknown_missing integer DEFAULT 7`)
	native51ProofRequire(t, native51UnknownInstall, -1, err == nil, err)
	_, err = tx.Exec(ctx, `SELECT native_membership_guard.catalog()`)
	native51ProofRequire(t, native51UnknownRejected, -1, err != nil && strings.Contains(err.Error(), "source type qualification failed"), err)
}

func TestNative51ProofDiagnosticEnums(t *testing.T) {
	for _, check := range []native51ProofCheck{native51EnvDSN, native51UnknownRejected} {
		require.NotEqual(t, "INVALID_CHECK", native51ProofCheckName(check))
	}
	require.Equal(t, "INVALID_CHECK", native51ProofCheckName(255))
	require.Equal(t, "INVALID_SESSION", native51ProofSession(11))
	require.Equal(t, "NONE", native51ProofSession(-1))
	for _, item := range []struct {
		err  error
		want string
	}{
		{nil, "NONE"}, {context.Canceled, "CANCELED"}, {context.DeadlineExceeded, "DEADLINE"},
		{&pgconn.PgError{Code: "55000", Message: "private DSN/SQL must not escape"}, "PG_55000"},
		{&pgconn.PgError{Code: "42883", Detail: "private catalog rows"}, "OTHER_PG"},
		{errors.New("private DSN/SQL must not escape"), "OTHER"},
	} {
		require.Equal(t, item.want, native51ProofErrorClass(item.err))
	}
}
