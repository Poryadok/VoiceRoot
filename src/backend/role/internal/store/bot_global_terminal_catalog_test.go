package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestRole26SourceMigration(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t,ctx,filepath.Join(repoRoot(t),"src/backend/migrations/role_db"))
	f.Run(true,"goto","25")
	f.Run(true,"goto","26")
	var relation string
	require.NoError(t,f.Pool.QueryRow(ctx,"SELECT 'public.role_bot_global_terminals'::regclass::text").Scan(&relation))
	require.Equal(t,"role_bot_global_terminals",relation)
	f.Run(true,"down","1")
	f.Run(true,"up","1")
}
