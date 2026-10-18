package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func nicknameStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	st := auditLedgerStoreFixture(t)
	for _, name := range []string{"000025_tree_audit_actions.up.sql", "000026_member_nickname_audit.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(context.Background(), string(raw))
		require.NoError(t, err)
	}
	return st
}

func TestMemberNickname_PersistenceNoopClearAndAudit(t *testing.T) {
	st := nicknameStoreFixture(t)
	ctx := context.Background()
	owner, target := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Nickname", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, target)
	require.NoError(t, err)
	before, err := scanMembershipRow(st.Pool.QueryRow(ctx, `SELECT space_id,profile_id,joined_at,nickname FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, target))
	require.NoError(t, err)
	got, err := st.UpdateMemberNickname(ctx, space.ID, target, owner, "  Игрок 😀 ")
	require.NoError(t, err)
	require.Equal(t, "Игрок 😀", *got.Nickname)
	require.Equal(t, before.JoinedAt, got.JoinedAt)
	// A fresh store instance observes the same durable row.
	reopened := &SpaceStore{Pool: st.Pool}
	rows, _, err := reopened.ListSpaceMembersPage(ctx, space.ID, 100, "")
	require.NoError(t, err)
	found := false
	for _, row := range rows {
		if row.ProfileID == target {
			found = true
			require.Equal(t, "Игрок 😀", *row.Nickname)
		}
	}
	require.True(t, found)
	_, err = st.UpdateMemberNickname(ctx, space.ID, target, owner, "Игрок 😀")
	require.NoError(t, err)
	var audits, outboxes int
	var details string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_nickname_updated'`, space.ID).Scan(&audits))
	require.Equal(t, 1, audits)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT details::text FROM audit_log WHERE space_id=$1 AND action='member_nickname_updated'`, space.ID).Scan(&details))
	require.JSONEq(t, `{"changed_fields":["nickname"]}`, details)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox o JOIN audit_log a ON a.id=o.audit_event_id WHERE a.space_id=$1 AND a.action='member_nickname_updated'`, space.ID).Scan(&outboxes))
	require.Equal(t, 1, outboxes)
	got, err = st.UpdateMemberNickname(ctx, space.ID, target, owner, "\u2003 ")
	require.NoError(t, err)
	require.Nil(t, got.Nickname)
	_, err = st.UpdateMemberNickname(ctx, space.ID, target, owner, "")
	require.NoError(t, err)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_nickname_updated'`, space.ID).Scan(&audits))
	require.Equal(t, 2, audits)
	_, err = st.UpdateMemberNickname(ctx, space.ID, uuid.New(), owner, "Ghost")
	require.ErrorIs(t, err, pgx.ErrNoRows)
	_, err = st.UpdateMemberNickname(ctx, space.ID, owner, target, "Owner edit")
	require.ErrorIs(t, err, ErrNicknameOwnerProtected)
	_, err = st.UpdateMemberNickname(ctx, space.ID, target, uuid.New(), "Outsider")
	require.ErrorIs(t, err, ErrNicknameActorNotMember)
	require.NoError(t, st.RemoveMember(ctx, space.ID, target))
	_, err = st.UpdateMemberNickname(ctx, space.ID, target, owner, "Revive")
	require.ErrorIs(t, err, pgx.ErrNoRows)
	var exists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, target).Scan(&exists))
	require.False(t, exists)
}

func TestMemberNickname_AuditFailureRollsBack(t *testing.T) {
	st := nicknameStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Rollback nickname", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION reject_nickname_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='member_nickname_updated' THEN RAISE EXCEPTION 'injected audit failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_nickname_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_nickname_audit()`)
	require.NoError(t, err)
	_, err = st.UpdateMemberNickname(ctx, space.ID, owner, owner, "Must roll back")
	require.Error(t, err)
	var nickname *string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT nickname FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, owner).Scan(&nickname))
	require.Nil(t, nickname)
}

func TestMemberNickname_FreezeBlocksSetClearAndNoop(t *testing.T) {
	st := nicknameStoreFixture(t)
	ctx := context.Background()
	for _, kind := range []string{"ownership", "SCHEDULE_PENDING", "SCHEDULED", "RESTORE_DECIDED", "PURGED"} {
		t.Run(kind, func(t *testing.T) {
			b := seedOwnershipJournalBinding(t, st)
			if kind == "ownership" {
				_, err := st.ReserveOwnership(ctx, b)
				require.NoError(t, err)
			} else {
				seedLifecycleScopePhase(t, st, b.SpaceID, kind)
			}
			for _, value := range []string{"Changed", "", "\u2003"} {
				_, err := st.UpdateMemberNickname(ctx, b.SpaceID, b.ActorProfileID, b.ActorProfileID, value)
				if kind == "ownership" {
					require.ErrorIs(t, err, ErrOwnershipFrozen)
				} else {
					require.ErrorIs(t, err, ErrLifecycleFrozen)
				}
			}
			var nickname *string
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT nickname FROM space_members WHERE space_id=$1 AND profile_id=$2`, b.SpaceID, b.ActorProfileID).Scan(&nickname))
			require.Nil(t, nickname)
		})
	}
	live, err := st.CreateSpace(ctx, uuid.New(), "Still live", "", "private")
	require.NoError(t, err)
	_, err = st.UpdateMemberNickname(ctx, live.ID, live.OwnerProfileID, live.OwnerProfileID, "Allowed")
	require.NoError(t, err)
}

func TestMemberNickname_RegistryAndDowngradePreserveEvidence(t *testing.T) {
	st := nicknameStoreFixture(t)
	ctx := context.Background()
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Registry nickname", "", "private")
	require.NoError(t, err)
	for _, details := range []string{`{}`, `{"changed_fields":[]}`, `{"changed_fields":["nickname"],"new":"private"}`, `{"changed_fields":["name"]}`} {
		_, err = st.Pool.Exec(ctx, `INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,'member_nickname_updated','profile',$2,$3::jsonb)`, space.ID, owner, details)
		require.Error(t, err)
	}
	_, err = st.UpdateMemberNickname(ctx, space.ID, owner, owner, "Evidence")
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000026_member_nickname_audit.down.sql"))
	require.NoError(t, err)
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, string(raw))
	require.ErrorContains(t, err, "while audit evidence exists")
	require.NoError(t, tx.Rollback(ctx))
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_outbox o JOIN audit_log a ON a.id=o.audit_event_id WHERE a.space_id=$1 AND a.action='member_nickname_updated'`, space.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestMemberNickname_ConcurrentLeaveNeverRecreatesMembership(t *testing.T) {
	st := nicknameStoreFixture(t)
	ctx := context.Background()
	owner, target := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Concurrent nickname", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, target)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 5)
	var wg sync.WaitGroup
	for _, value := range []string{"First", "Second", "Third", "Fourth"} {
		wg.Add(1)
		go func(value string) {
			defer wg.Done()
			<-start
			_, err := st.UpdateMemberNickname(ctx, space.ID, target, owner, value)
			results <- err
		}(value)
	}
	wg.Add(1)
	go func() { defer wg.Done(); <-start; results <- st.RemoveMember(ctx, space.ID, target) }()
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		require.True(t, err == nil || errors.Is(err, pgx.ErrNoRows), "unexpected serialized operation failure: %v", err)
	}
	var exists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, target).Scan(&exists))
	require.False(t, exists)
}

func TestMemberNickname_ManagedRosterDoesNotCreateLocalOverride(t *testing.T) {
	st := nicknameStoreFixture(t)
	ctx := context.Background()
	app, env, owner, account, target := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env, OwnerAccountID: account, OwnerProfileID: owner, CorporationKey: "nickname-corp", TemplateID: "guild-default"})
	require.NoError(t, err)
	_, err = st.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env, CorporationKey: "nickname-corp", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1, SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{target}})
	require.NoError(t, err)
	_, err = st.UpdateMemberNickname(ctx, space.ID, target, owner, "Must not override")
	require.ErrorIs(t, err, ErrNicknameExternallyManaged)
	var exists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, target).Scan(&exists))
	require.False(t, exists)
}
