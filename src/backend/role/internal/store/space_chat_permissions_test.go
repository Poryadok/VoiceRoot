package store

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/memberadmission"
)

func spaceChatStore(t *testing.T) (context.Context, *RoleStore) {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL source-bound grants")
	}
	ctx, st := roleSourceStore(t)
	for _, migration := range []string{"000017_admin_role_management.up.sql", "000018_space_audit_outbox.up.sql", "000019_approve_join_requests.up.sql", "000020_space_chat_permissions.up.sql"} {
		_, err := st.Pool.Exec(ctx, role14SQL(t, migration))
		require.NoError(t, err)
	}
	return ctx, st
}

func TestSpaceChatMutationCASAndHistoricalReplay(t *testing.T) {
	ctx, st := spaceChatStore(t)
	space, actor, chat, incarnation, target := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, actor))
	binding := memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 4, PublicationIdentity: uuid.NewString()}}
	snapshot, err := st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, binding)
	require.NoError(t, err)
	require.Positive(t, snapshot.Revision, "snapshot must use a persistent nonzero Role revision")
	require.True(t, snapshot.MemberAdditionsEnabled, "new scope defaults to additions enabled")
	require.False(t, snapshot.ExplicitPostGrant, "role-wide allows create no explicit grant")
	mutation := SpaceChatMutation{OperationID: uuid.New(), ActorProfileID: actor, SpaceID: space, ChatID: chat, ChatScopeIncarnation: incarnation, TargetProfileID: target, Kind: "post_as_chat", Allowed: true, RequestedBinding: &binding, ResolvedBinding: &binding}
	called := 0
	localAuthorize := func(scoped *RoleStore) error { require.NotNil(t, scoped.tx); called++; return nil }
	_, err = st.Pool.Exec(ctx, `UPDATE roles SET position=position+1 WHERE space_id=$1`, space)
	require.NoError(t, err)
	_, err = st.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, localAuthorize)
	require.ErrorIs(t, err, ErrSpaceChatAuthorityChanged)
	require.Zero(t, called, "stale CAS must not authorize or write")
	for _, table := range []string{"role_space_chat_grants", "role_space_chat_operations"} {
		require.JSONEq(t, `[]`, role14Rows(t, ctx, st.Pool, table))
	}
	snapshot, err = st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, binding)
	require.NoError(t, err)
	denied := errors.New("local manager was demoted")
	_, err = st.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, func(*RoleStore) error { return denied })
	require.ErrorIs(t, err, denied)
	require.JSONEq(t, `[]`, role14Rows(t, ctx, st.Pool, "role_space_chat_operations"))
	allow, err := st.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, localAuthorize)
	require.NoError(t, err)
	require.True(t, allow.Allowed)
	require.Greater(t, allow.Revision, snapshot.Revision)
	require.False(t, allow.Historical)
	snapshot, err = st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, binding)
	require.NoError(t, err)
	require.True(t, snapshot.ExplicitPostGrant)
	revoke := mutation
	revoke.OperationID = uuid.New()
	revoke.Allowed = false
	_, err = st.CommitSpaceChatMutation(ctx, revoke, snapshot.Revision, localAuthorize)
	require.NoError(t, err)
	before := role14Rows(t, ctx, st.Pool, "role_voice_policy_epochs")
	replay, err := st.CommitSpaceChatMutation(ctx, mutation, 1, func(*RoleStore) error { t.Fatal("historical replay must not reauthorize/reapply"); return nil })
	require.NoError(t, err)
	require.True(t, replay.Historical)
	require.True(t, replay.Allowed)
	require.Equal(t, allow.Revision, replay.Revision)
	require.Equal(t, before, role14Rows(t, ctx, st.Pool, "role_voice_policy_epochs"))
	snapshot, err = st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, binding)
	require.NoError(t, err)
	require.False(t, snapshot.ExplicitPostGrant, "old allow replay cannot undo revoke")
	changed := mutation
	changed.Allowed = false
	_, err = st.CommitSpaceChatMutation(ctx, changed, snapshot.Revision, localAuthorize)
	require.ErrorIs(t, err, ErrSpaceChatOperationConflict)
	mutation.OperationID = uuid.New()
	_, err = st.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, localAuthorize)
	require.NoError(t, err)
	newBinding := binding
	ordinary := *binding.Ordinary
	ordinary.MembershipGeneration++
	newBinding.Ordinary = &ordinary
	newSnapshot, err := st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, newBinding)
	require.NoError(t, err)
	require.False(t, newSnapshot.ExplicitPostGrant, "grant never rebinds to a later episode")
	newSnapshot, err = st.ReadSpaceChatPermissions(ctx, space, chat, uuid.New(), target, binding)
	require.NoError(t, err)
	require.False(t, newSnapshot.ExplicitPostGrant, "move-away-and-back scope cannot inherit old grant")
}

func TestSpaceChatPolicyAndAutomaticSelectorStayScopeBound(t *testing.T) {
	ctx, st := spaceChatStore(t)
	space, actor, chat, incarnation, target := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, actor))
	binding := memberadmission.MembershipBinding{Community: &memberadmission.CommunityMembershipBinding{OwnerGeneration: 2, MembershipGeneration: 3, MembershipEpisodeID: uuid.NewString()}}
	read := func(scope uuid.UUID) SpaceChatPermissionSnapshot {
		snapshot, err := st.ReadSpaceChatPermissions(ctx, space, chat, scope, target, binding)
		require.NoError(t, err)
		return snapshot
	}
	policy := SpaceChatMutation{OperationID: uuid.New(), ActorProfileID: actor, SpaceID: space, ChatID: chat, ChatScopeIncarnation: incarnation, Kind: "add_members", Allowed: false}
	_, err := st.CommitSpaceChatMutation(ctx, policy, read(incarnation).Revision, func(*RoleStore) error { return nil })
	require.NoError(t, err)
	require.False(t, read(incarnation).MemberAdditionsEnabled)
	require.True(t, read(uuid.New()).MemberAdditionsEnabled, "new incarnation starts with its own default")
	grant := policy
	grant.OperationID = uuid.New()
	grant.Kind = "post_as_chat"
	grant.TargetProfileID = target
	grant.ResolvedBinding = &binding
	grant.Allowed = true
	first, err := st.CommitSpaceChatMutation(ctx, grant, read(incarnation).Revision, func(*RoleStore) error { return nil })
	require.NoError(t, err)
	newBinding := binding
	community := *binding.Community
	community.MembershipGeneration++
	community.MembershipEpisodeID = uuid.NewString()
	newBinding.Community = &community
	grant.ResolvedBinding = &newBinding
	replay, err := st.CommitSpaceChatMutation(ctx, grant, 1, func(*RoleStore) error { t.Fatal("auto-selected retry must preserve first resolution"); return nil })
	require.NoError(t, err)
	require.True(t, replay.Historical)
	require.Equal(t, first.MembershipBinding, replay.MembershipBinding)
	grant.ActorProfileID = uuid.New()
	_, err = st.CommitSpaceChatMutation(ctx, grant, 1, func(*RoleStore) error { return nil })
	require.ErrorIs(t, err, ErrSpaceChatOperationConflict)
	_, err = st.Pool.Exec(ctx, `UPDATE role_space_chat_operations SET allowed=NOT allowed`)
	require.Error(t, err, "historical outcome cannot be edited")
}

func TestSpaceChatConcurrentExactOperationCommitsOnce(t *testing.T) {
	ctx, st := spaceChatStore(t)
	secondPool, err := pgxpool.NewWithConfig(ctx, st.Pool.Config())
	require.NoError(t, err)
	defer secondPool.Close()
	other := &RoleStore{Pool: secondPool}
	space, actor, chat, incarnation, target := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, actor))
	binding := memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: uuid.NewString()}}
	snapshot, err := st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, binding)
	require.NoError(t, err)
	mutation := SpaceChatMutation{OperationID: uuid.New(), ActorProfileID: actor, SpaceID: space, ChatID: chat, ChatScopeIncarnation: incarnation, TargetProfileID: target, Kind: "post_as_chat", Allowed: true, ResolvedBinding: &binding}
	type result struct {
		receipt SpaceChatMutationReceipt
		err     error
	}
	results := make(chan result, 2)
	entered := make(chan struct{})
	release := make(chan struct{})
	var authorizations atomic.Int32
	go func() {
		receipt, err := st.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, func(*RoleStore) error { authorizations.Add(1); close(entered); <-release; return nil })
		results <- result{receipt, err}
	}()
	<-entered
	started := make(chan struct{})
	go func() {
		close(started)
		receipt, err := other.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, func(*RoleStore) error { authorizations.Add(1); return nil })
		results <- result{receipt, err}
	}()
	<-started
	close(release)
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotEqual(t, first.receipt.Historical, second.receipt.Historical)
	require.Equal(t, first.receipt.Revision, second.receipt.Revision)
	require.Equal(t, int32(1), authorizations.Load())
	var operations, events int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM role_space_chat_operations),(SELECT count(*) FROM role_space_chat_outbox)`).Scan(&operations, &events))
	require.Equal(t, 1, operations)
	require.Equal(t, 1, events)
}

func TestSpaceChatOutboxFailureRollsBackGrantReceiptAndRevision(t *testing.T) {
	ctx, st := spaceChatStore(t)
	space, actor, chat, incarnation, target := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, actor))
	binding := memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: uuid.NewString()}}
	snapshot, err := st.ReadSpaceChatPermissions(ctx, space, chat, incarnation, target, binding)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION test_space_chat_outbox_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected durable outbox failure'; END $$;
CREATE TRIGGER test_space_chat_outbox_failure BEFORE INSERT ON role_space_chat_outbox FOR EACH ROW EXECUTE FUNCTION test_space_chat_outbox_failure();`)
	require.NoError(t, err)
	tables := []string{"role_space_chat_grants", "role_space_chat_operations", "role_space_chat_outbox", "role_voice_policy_epochs", "role_voice_policy_outbox"}
	before := make(map[string]string)
	for _, table := range tables {
		before[table] = role14Rows(t, ctx, st.Pool, table)
	}
	mutation := SpaceChatMutation{OperationID: uuid.New(), ActorProfileID: actor, SpaceID: space, ChatID: chat, ChatScopeIncarnation: incarnation, TargetProfileID: target, Kind: "post_as_chat", Allowed: true, ResolvedBinding: &binding}
	receipt, err := st.CommitSpaceChatMutation(ctx, mutation, snapshot.Revision, func(*RoleStore) error { return nil })
	require.Error(t, err)
	require.Zero(t, receipt)
	for _, table := range tables {
		require.Equal(t, before[table], role14Rows(t, ctx, st.Pool, table), "rollback includes %s", table)
	}
}
