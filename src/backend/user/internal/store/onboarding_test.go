package store

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func userStoreRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestOnboardingStore_postgres(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	migrationPath := filepath.Join(userStoreRepoRoot(t), "src", "backend", "migrations", "user_db", "000001_init.up.sql")
	pool := integrationtest.StartPostgres(t, ctx, "userdb", migrationPath)

	accountID := uuid.New()
	profileID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'onboard', '0001', 'Onboard User', true)`,
		profileID, accountID)
	require.NoError(t, err)

	st := NewProfileStore(pool)

	t.Run("GetOrCreateOnboardingState creates empty row", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM onboarding_state WHERE profile_id = $1`, profileID)
		require.NoError(t, err)

		row, err := st.GetOrCreateOnboardingState(ctx, accountID, &profileID)
		require.NoError(t, err)
		require.Equal(t, profileID, row.ProfileID)
		require.False(t, row.Completed)
		require.Empty(t, row.CompletedSteps)
		require.Nil(t, row.CompletedAt)

		again, err := st.GetOrCreateOnboardingState(ctx, accountID, &profileID)
		require.NoError(t, err)
		require.Equal(t, row.ProfileID, again.ProfileID)
	})

	t.Run("CompleteOnboardingStep each canonical step", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM onboarding_state WHERE profile_id = $1`, profileID)
		require.NoError(t, err)

		steps := []string{
			OnboardingStepSaveAccount,
			OnboardingStepChatsNav,
			OnboardingStepSpaces,
			OnboardingStepMatchmaking,
			OnboardingStepWrapUp,
		}
		for i, step := range steps {
			row, err := st.CompleteOnboardingStep(ctx, accountID, &profileID, step)
			require.NoError(t, err)
			require.Contains(t, row.CompletedSteps, step)
			if i < len(steps)-1 {
				require.False(t, row.Completed, "step %q should not complete tutorial yet", step)
			}
		}
		final, err := st.GetOrCreateOnboardingState(ctx, accountID, &profileID)
		require.NoError(t, err)
		require.True(t, final.Completed)
		require.NotNil(t, final.CompletedAt)
	})

	t.Run("CompleteOnboardingStep dismiss sets completed", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM onboarding_state WHERE profile_id = $1`, profileID)
		require.NoError(t, err)

		row, err := st.CompleteOnboardingStep(ctx, accountID, &profileID, OnboardingStepDismiss)
		require.NoError(t, err)
		require.True(t, row.Completed)
		require.NotNil(t, row.CompletedAt)
		require.Contains(t, row.CompletedSteps, OnboardingStepDismiss)
	})

	t.Run("CompleteOnboardingStep invalid step rejected", func(t *testing.T) {
		_, err := st.CompleteOnboardingStep(ctx, accountID, &profileID, "")
		require.ErrorIs(t, err, ErrInvalidOnboardingStep)

		_, err = st.CompleteOnboardingStep(ctx, accountID, &profileID, "   ")
		require.ErrorIs(t, err, ErrInvalidOnboardingStep)
	})

	t.Run("CompleteOnboardingStep idempotent re-complete", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM onboarding_state WHERE profile_id = $1`, profileID)
		require.NoError(t, err)

		first, err := st.CompleteOnboardingStep(ctx, accountID, &profileID, OnboardingStepSaveAccount)
		require.NoError(t, err)
		second, err := st.CompleteOnboardingStep(ctx, accountID, &profileID, OnboardingStepSaveAccount)
		require.NoError(t, err)
		require.Equal(t, first.CompletedSteps, second.CompletedSteps)
		require.Len(t, second.CompletedSteps, 1)
	})
}
