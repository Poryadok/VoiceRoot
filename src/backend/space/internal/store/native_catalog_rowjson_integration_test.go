//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func native51Old50SourceFixture(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "requires an explicitly owned genuine trusted old50 fixture; no skip or fallback")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	_, err = nativeCatalog51SourceFacts(ctx, tx, 50)
	require.NoError(t, err, "genuine old50 is a prerequisite, never an endpoint reseal")
	require.NoError(t, tx.Rollback(ctx))
	return ctx, pool
}

func native51TestPreimage(t *testing.T, expression string) string {
	t.Helper()
	const prefix = "SELECT sha256(convert_to("
	const suffix = ",'UTF8'))"
	s := strings.TrimSpace(expression)
	require.True(t, strings.HasPrefix(s, prefix))
	require.True(t, strings.HasSuffix(s, suffix))
	return strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
}

// This qualifies the ACTUAL new catalog body on one controlled rolled-back
// source cut. It does not admit51, touch epoch/history or measure Ban performance.
func TestNative51RawCompositeSessionStability(t *testing.T) {
	ctx, pool := native51Old50SourceFixture(t)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback(context.Background())) }()
	_, err = tx.Exec(ctx, nativeCatalogRowJSONDefinition+";")
	require.NoError(t, err)
	old := native51TestPreimage(t, nativeCatalog51ParentExpression)
	current := native51TestPreimage(t, nativeCatalogRowJSONLiteralExpression)
	narrowed := strings.Replace(current, "row_to_json(a)", "json_build_object('attrelid',a.attrelid)", 1)
	require.NotEqual(t, current, narrowed)
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
		require.NoError(t, err)
		var oldBytes, newBytes, actual, oracle []byte
		var complete, rejectsNarrowed bool
		require.NoError(t, tx.QueryRow(ctx, `SELECT convert_to((`+old+`),'UTF8'),convert_to((`+current+`),'UTF8'),native_membership_guard.catalog(),(`+nativeCatalogRowJSONLiteralExpression+`),((`+old+`)::jsonb=(`+current+`)::jsonb),((`+old+`)::jsonb<>(`+narrowed+`)::jsonb)`).Scan(&oldBytes, &newBytes, &actual, &oracle, &complete, &rejectsNarrowed))
		require.True(t, complete, "full parsed metadata, not an omitted-field projection")
		require.True(t, rejectsNarrowed, "the control must actually detect the narrowed encoder")
		require.Equal(t, oracle, actual, "actual installed candidate body must equal its complete source expression")
		if i == 0 {
			baselineOld, baselineNew = oldBytes, newBytes
		}
		if string(oldBytes) == string(baselineOld) {
			stablePairs++
			require.Equal(t, baselineNew, newBytes, "rawN must remain stable whenever old canonical preimage is stable: "+test.name)
		}
		t.Logf("session=%s oldSHA=%s rawNSHA=%s sameCut=true complete=true", test.name, nativeSourceBodySHA(string(oldBytes)), nativeSourceBodySHA(string(newBytes)))
	}
	require.GreaterOrEqual(t, stablePairs, 3, "the stability implication must be nonvacuous")
}

func TestNative51ClassifierRejectsUnclassifiedMissingDatum(t *testing.T) {
	ctx, pool := native51Old50SourceFixture(t)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback(context.Background())) }()
	_, err = tx.Exec(ctx, nativeCatalogRowJSONDefinition+`; ALTER TABLE public.spaces ADD COLUMN native51_unknown_missing integer DEFAULT 7`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT native_membership_guard.catalog()`)
	require.ErrorContains(t, err, "source type qualification failed", "unknown nonNULL datum must never reach a claimed sender/encoder")
}
