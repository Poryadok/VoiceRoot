package grpcsvc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

// These files only configure/synchronize the isolated provider fixture. The
// production signed RPC and independently persisted receipt remain authority.
type roleNonzeroSeed struct {
	SchemaVersion      int    `json:"schema_version"`
	BotID              string `json:"bot_id"`
	ActorProfileID     string `json:"actor_profile_id"`
	SpaceID            string `json:"space_id"`
	InstallationID     string `json:"installation_id"`
	InstallOperationID string `json:"install_operation_id"`
	Generation         uint64 `json:"generation"`
}

func writeRoleFixtureJSON(t *testing.T, dir, name string, value any) {
	t.Helper()
	path := filepath.Join(dir, name)
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "fresh fixture synchronization path required")
	body, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path+".tmp", body, 0600))
	require.NoError(t, os.Rename(path+".tmp", path))
}

func readRoleFixtureJSON(t *testing.T, dir, name string, target any) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.Eventually(t, func() bool { _, err := os.Stat(path); return err == nil }, 10*time.Second, 25*time.Millisecond, "source provider must publish its own committed database postcondition")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, target))
}

func seedActualRoleCleanupTarget(t *testing.T, dir string, bot store.BotRow, intent *store.InstallationIntent) *roleNonzeroSeed {
	t.Helper()
	require.Positive(t, intent.Generation)
	seed := &roleNonzeroSeed{1, bot.ID.String(), bot.ActorProfileID.String(), intent.SpaceID.String(), intent.InstallationID.String(), intent.OperationID.String(), uint64(intent.Generation)}
	writeRoleFixtureJSON(t, dir, "seed.request.json", seed)
	var reply struct {
		SchemaVersion          int             `json:"schema_version"`
		Seed                   roleNonzeroSeed `json:"seed"`
		CreatedRoleID          string          `json:"created_role_id"`
		ControlRoleID          string          `json:"control_role_id"`
		ControlProfileID       string          `json:"control_profile_id"`
		SystemRoleID           string          `json:"system_role_id"`
		DeletedRoleCount       uint64          `json:"deleted_role_count"`
		RemovedAssignmentCount uint64          `json:"removed_assignment_count"`
	}
	readRoleFixtureJSON(t, dir, "seed.ready.json", &reply)
	require.Equal(t, 1, reply.SchemaVersion)
	require.Equal(t, *seed, reply.Seed, "seed must use this exact persisted Bot installation, not provider-generated identities")
	for _, id := range []string{reply.CreatedRoleID, reply.ControlRoleID, reply.ControlProfileID, reply.SystemRoleID} {
		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, parsed)
		require.Equal(t, parsed.String(), id)
	}
	require.NotEqual(t, reply.CreatedRoleID, reply.ControlRoleID)
	require.EqualValues(t, 1, reply.DeletedRoleCount)
	require.EqualValues(t, 2, reply.RemovedAssignmentCount)
	return seed
}

func checkActualRoleCleanupTarget(t *testing.T, dir string, seed roleNonzeroSeed, operation, receipt, decision string) {
	t.Helper()
	post := struct {
		roleNonzeroSeed
		OperationID string `json:"operation_id"`
		ReceiptID   string `json:"receipt_id"`
		DecisionID  string `json:"decision_id"`
	}{seed, operation, receipt, decision}
	writeRoleFixtureJSON(t, dir, "postcondition.request.json", post)
	var reply struct {
		SchemaVersion          int    `json:"schema_version"`
		OperationID            string `json:"operation_id"`
		ReceiptID              string `json:"receipt_id"`
		DecisionID             string `json:"decision_id"`
		DeletedRoleCount       uint64 `json:"deleted_role_count"`
		RemovedAssignmentCount uint64 `json:"removed_assignment_count"`
		DatabaseChecked        bool   `json:"database_checked"`
	}
	readRoleFixtureJSON(t, dir, "postcondition.ready.json", &reply)
	require.Equal(t, 1, reply.SchemaVersion)
	require.Equal(t, operation, reply.OperationID)
	require.Equal(t, receipt, reply.ReceiptID)
	require.Equal(t, decision, reply.DecisionID)
	require.EqualValues(t, 1, reply.DeletedRoleCount)
	require.EqualValues(t, 2, reply.RemovedAssignmentCount)
	require.True(t, reply.DatabaseChecked, "synchronization is accepted only after the source provider independently reads actual target/control rows and committed request/receipt")
}
