package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

// Constructor availability is tested independently from the later signed
// publication tests: a compile-only imported historical helper is not an
// executable current Store entry point. No ordinary creation fallback exists.
func TestNative45CurrentCreatorEntryPoint(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	var version int64
	var dirty bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 34, version)
	require.False(t, dirty)
	method, exists := reflect.TypeOf(&SpaceStore{Pool: db.Pool}).MethodByName("CreateSpaceWithCreator")
	require.True(t, exists, "current production Store lacks the protected native Creator entry point consumed by342; ordinary CreateSpace is not a substitute")
	require.Equal(t, 4, method.Type.NumIn())
	require.Equal(t, "*lifecycleprincipal.VerifiedCreator", method.Type.In(2).String())
	require.Equal(t, reflect.TypeOf(&spacev1.CreateSpaceRequest{}), method.Type.In(3))
	require.Equal(t, 2, method.Type.NumOut())
	require.Equal(t, reflect.TypeOf(&SpaceRow{}), method.Type.Out(0))
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.spaces`).Scan(&count))
	require.Zero(t, count, "entry-point qualification does not create fixtures or membership")
}
