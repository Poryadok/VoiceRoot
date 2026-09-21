package profileprojection

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
)

// TestApplyAndCheckpoint_CommitsProjectionAndOffsetTogether is the restart
// fence: a committed journal offset must never exist without its projection.
func TestApplyAndCheckpoint_CommitsProjectionAndOffsetTogether(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{
		"000003_space_lifecycle.up.sql",
		"000002_verification_type.up.sql",
		"000004_user_profile_projection.up.sql",
	} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}

	profileID, accountID := uuid.NewString(), uuid.NewString()
	event := &userv1.SearchProfileProjectionEvent{
		ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID, SourceRevision: 1, JournalOffset: 7,
		OccurredAt: timestamppb.Now(),
		Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{
			AccountId: accountID, Username: "atomic", Discriminator: "0001", DisplayName: "Atomic",
			UsernameSearchKey: "atomic", DisplayNameSearchKey: "atomic", NormalizationVersion: 1,
		}},
	}

	adapter := &StoreAdapter{Pool: pool}
	result, err := adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset())
	require.NoError(t, err)
	require.Equal(t, Applied, result)

	var checkpoint uint64
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_checkpoint WHERE singleton=true`).Scan(&checkpoint))
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM profile_search_documents WHERE profile_id=$1`, profileID).Scan(&revision))
	require.Equal(t, uint64(7), checkpoint)
	require.Equal(t, int64(1), revision)
}

func searchProjectionRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", ".."))
}
