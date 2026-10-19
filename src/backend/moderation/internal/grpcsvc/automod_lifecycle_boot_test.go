package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/moderation/internal/automod"
)

func TestAutoModLifecycle_ClosedBootRollsBackAllPrivateMutations(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	for _, operation := range []string{"provision", "bind", "refresh", "snapshot"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			pool := automodAdmissionDatabase(t)
			integrationtestApplyAutoMod(t, ctx, pool)
			s := automod.NewStore(pool)
			binding := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
			if operation == "refresh" || operation == "snapshot" {
				require.NoError(t, s.Provision(ctx, binding))
			}
			g := repairGate(t)
			captured, err := g.AcquirePrivateRepair(ctx)
			require.NoError(t, err)
			calls := 0
			check := func(ctx context.Context) error {
				calls++
				if calls == 3 {
					g.Close()
				}
				return captured(ctx)
			}
			switch operation {
			case "provision":
				err = s.ProvisionChecked(ctx, binding, check)
			case "bind":
				binding.Epoch = uuid.Nil
				err = s.BindSubjectChecked(ctx, binding, check)
			case "refresh":
				binding.Revision = 2
				err = s.RefreshSubjectChecked(ctx, binding, check)
			case "snapshot":
				_, err = s.RestrictionSnapshotChecked(ctx, binding.AccountID, binding.ID, check)
			}
			require.ErrorIs(t, err, automod.ErrBootClosed)
			require.Equal(t, 3, calls)
			var count, revision, restriction int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),COALESCE(max(lifecycle_revision),0),COALESCE(max(restriction_revision),0) FROM automod_subjects`).Scan(&count, &revision, &restriction))
			if operation == "refresh" || operation == "snapshot" {
				require.Equal(t, 1, count)
				require.Equal(t, 1, revision)
			} else {
				require.Zero(t, count)
			}
			require.Zero(t, restriction)
			pending, err := s.PendingRestrictions(ctx)
			require.NoError(t, err)
			require.Empty(t, pending)
		})
	}
}
