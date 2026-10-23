package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBotRemovalCatalogBeforeEffects(t *testing.T) {
	ctx := context.Background()
	for _, corruption := range []string{
		`UPDATE schema_migrations SET dirty=true`,
		`ALTER TABLE role_bot_global_removals DISABLE TRIGGER role_bot_global_removals_guard`,
		`ALTER TABLE role_member_contributions DISABLE TRIGGER role_authority_revision_member_contributions`,
	} {
		t.Run(corruption, func(t *testing.T) {
			f := removalFixture(t)
			s := &RoleStore{Pool: f.Pool}
			// A clean catalog must execute the same path; unrelated dependency
			// failure cannot establish the negative control.
			_, err := s.ApplyBotInstallationRemoval(ctx, removalRequest())
			require.NoError(t, err)
			_, err = f.Pool.Exec(ctx, corruption)
			require.NoError(t, err)
			_, err = s.ApplyBotInstallationRemoval(ctx, removalRequest())
			require.ErrorIs(t, err, ErrScopeUnavailable)
			var bindings, receipts int
			require.NoError(t, f.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM role_bot_actor_bindings),(SELECT count(*) FROM role_bot_removal_operations)`).Scan(&bindings, &receipts))
			require.Equal(t, 1, bindings)
			require.Equal(t, 1, receipts)
		})
	}
}
