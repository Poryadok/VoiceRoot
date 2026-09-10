package grpcsvc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/authctx"
	"voice/backend/role/internal/roleevents"
	"voice/backend/role/internal/store"
	"voice/backend/role/permissions"
)

// Record every publisher method, including methods absent from older test fakes.
// A created event also observes the role from a separate pool connection.
type ordinaryScopeEvents struct {
	roleevents.NoopPublisher
	pool             *pgxpool.Pool
	mu               sync.Mutex
	count            int
	visibilityErrors []error
}

func (p *ordinaryScopeEvents) record() { p.mu.Lock(); defer p.mu.Unlock(); p.count++ }
func (p *ordinaryScopeEvents) snapshot() (int, []error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count, append([]error(nil), p.visibilityErrors...)
}
func (p *ordinaryScopeEvents) PublishRoleCreated(ctx context.Context, spaceID, roleID, name string) error {
	var visible bool
	err := p.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM roles WHERE id=$1 AND space_id=$2 AND name=$3)", roleID, spaceID, name).Scan(&visible)
	if err == nil && !visible {
		err = fmt.Errorf("role.created observed uncommitted or absent role %s", roleID)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.count++
	if err != nil {
		p.visibilityErrors = append(p.visibilityErrors, err)
	}
	return err
}
func (p *ordinaryScopeEvents) PublishRoleUpdated(context.Context, string, string, []string) error {
	p.record()
	return nil
}
func (p *ordinaryScopeEvents) PublishRoleDeleted(context.Context, string, string) error {
	p.record()
	return nil
}
func (p *ordinaryScopeEvents) PublishRoleAssigned(context.Context, string, string, string) error {
	p.record()
	return nil
}
func (p *ordinaryScopeEvents) PublishRoleRevoked(context.Context, string, string, string) error {
	p.record()
	return nil
}
func (p *ordinaryScopeEvents) PublishChatOverrideSet(context.Context, string, string) error {
	p.record()
	return nil
}
func (p *ordinaryScopeEvents) PublishVoiceOverrideSet(context.Context, string, string) error {
	p.record()
	return nil
}

type ordinaryHandlerFixture struct {
	svc                                              *RoleGRPC
	ctx                                              context.Context
	events                                           *ordinaryScopeEvents
	space, owner, empty, custom, member, chat, voice uuid.UUID
}

func newOrdinaryHandlerFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) ordinaryHandlerFixture {
	t.Helper()
	f := ordinaryHandlerFixture{space: uuid.New(), owner: uuid.New(), empty: uuid.New(), chat: uuid.New(), voice: uuid.New()}
	f.events = &ordinaryScopeEvents{pool: pool}
	f.svc = &RoleGRPC{Store: &store.RoleStore{Pool: pool}, Events: f.events}
	f.ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderProfileID, f.owner.String()))
	require.NoError(t, f.svc.Store.BootstrapSpaceRoles(ctx, f.space, f.owner))
	var err error
	f.member, err = f.svc.Store.RoleIDByName(ctx, f.space, permissions.RoleMember)
	require.NoError(t, err)
	role, err := f.svc.Store.CreateCustomRole(ctx, f.space, "fixture role", 1, 1, &f.owner)
	require.NoError(t, err)
	f.custom = role.ID
	require.NoError(t, f.svc.Store.AssignMemberRole(ctx, f.space, f.owner, f.custom, f.owner))
	require.NoError(t, f.svc.Store.SetChatOverride(ctx, f.chat, f.custom, 1, 2))
	require.NoError(t, f.svc.Store.SetVoiceRoomOverride(ctx, f.voice, f.custom, 1, 2))
	return f
}
func prepareOrdinaryHandlerFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, f ordinaryHandlerFixture) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO ownership_transfer_v2
 (operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash)
 VALUES($1,$2,2,$3,$4,$5,$6,'prepared',$6)`, uuid.New(), f.space, f.owner, uuid.New(), []byte("handler fixture"), make([]byte, 32))
	require.NoError(t, err)
}
func ordinaryHandlerSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var result string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_build_object(
 'roles',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM roles r),
 'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY space_id,profile_id,role_id) FROM member_roles m),
 'chat',(SELECT jsonb_agg(to_jsonb(c) ORDER BY chat_id,role_id) FROM chat_overrides c),
 'voice',(SELECT jsonb_agg(to_jsonb(v) ORDER BY voice_room_id,role_id) FROM voice_room_overrides v))::text`).Scan(&result))
	return result
}

type ordinaryHandlerCase struct {
	name string
	call func(ordinaryHandlerFixture) (any, error)
}

func ordinaryHandlerCases() []ordinaryHandlerCase {
	return []ordinaryHandlerCase{
		{"CreateRole", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.CreateRole(f.ctx, &rolev1.CreateRoleRequest{SpaceId: f.space.String(), Name: "requested role", Position: 1, PermissionsMask: 1})
		}},
		{"UpdateRole", func(f ordinaryHandlerFixture) (any, error) {
			name := "renamed"
			return f.svc.UpdateRole(f.ctx, &rolev1.UpdateRoleRequest{RoleId: f.custom.String(), Name: &name})
		}},
		{"DeleteRole", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.DeleteRole(f.ctx, &rolev1.DeleteRoleRequest{RoleId: f.custom.String()})
		}},
		{"ListRoles", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.ListRoles(f.ctx, &rolev1.ListRolesRequest{SpaceId: f.space.String()})
		}},
		{"ReorderRoles", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.ReorderRoles(f.ctx, &rolev1.ReorderRolesRequest{SpaceId: f.space.String(), OrderedRoleIds: []string{f.member.String(), f.custom.String()}})
		}},
		{"AssignRole", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.AssignRole(f.ctx, &rolev1.AssignRoleRequest{SpaceId: f.space.String(), ProfileId: f.empty.String(), RoleId: f.custom.String()})
		}},
		{"RevokeRole", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.RevokeRole(f.ctx, &rolev1.RevokeRoleRequest{SpaceId: f.space.String(), ProfileId: f.owner.String(), RoleId: f.custom.String()})
		}},
		{"GetMemberRoles", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.GetMemberRoles(f.ctx, &rolev1.GetMemberRolesRequest{SpaceId: f.space.String(), ProfileId: f.owner.String()})
		}},
		{"SetChatOverride", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.SetChatOverride(f.ctx, &rolev1.SetChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String(), AllowMask: 8})
		}},
		{"RemoveChatOverride", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.RemoveChatOverride(f.ctx, &rolev1.RemoveChatOverrideRequest{SpaceId: f.space.String(), Chat: &chatv1.ChatRef{Id: f.chat.String()}, RoleId: f.custom.String()})
		}},
		{"GetChatOverrides", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.GetChatOverrides(f.ctx, &rolev1.GetChatOverridesRequest{SpaceId: f.space.String()})
		}},
		{"SetVoiceRoomOverride", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.SetVoiceRoomOverride(f.ctx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String(), AllowMask: 8})
		}},
		{"RemoveVoiceRoomOverride", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.RemoveVoiceRoomOverride(f.ctx, &rolev1.RemoveVoiceRoomOverrideRequest{SpaceId: f.space.String(), VoiceRoomId: f.voice.String(), RoleId: f.custom.String()})
		}},
		{"GetVoiceRoomOverrides", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.GetVoiceRoomOverrides(f.ctx, &rolev1.GetVoiceRoomOverridesRequest{SpaceId: f.space.String()})
		}},
		{"CheckPermission", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.CheckPermission(f.ctx, &rolev1.CheckPermissionRequest{SpaceId: f.space.String(), ProfileId: f.owner.String(), PermissionName: permissions.SpaceManageRoles})
		}},
		{"GetEffectivePermissions", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.GetEffectivePermissions(f.ctx, &rolev1.GetEffectivePermissionsRequest{SpaceId: f.space.String(), ProfileId: f.owner.String()})
		}},
		{"SetDefaultJoinRole", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.SetDefaultJoinRole(f.ctx, &rolev1.SetDefaultJoinRoleRequest{SpaceId: f.space.String(), RoleId: f.custom.String()})
		}},
		{"GetDefaultJoinRole", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.GetDefaultJoinRole(f.ctx, &rolev1.GetDefaultJoinRoleRequest{SpaceId: f.space.String()})
		}},
		{"BootstrapSpaceRoles", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.BootstrapSpaceRoles(f.ctx, &rolev1.BootstrapSpaceRolesRequest{SpaceId: f.space.String(), OwnerProfileId: f.owner.String()})
		}},
		{"DeleteRolesCreatedByProfile", func(f ordinaryHandlerFixture) (any, error) {
			return f.svc.DeleteRolesCreatedByProfile(f.ctx, &rolev1.DeleteRolesCreatedByProfileRequest{SpaceId: f.space.String(), CreatedByProfileId: f.owner.String()})
		}},
	}
}

func TestOrdinaryHandlerFences(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := store.StartRoleDBForStoreTest(t, ctx)
	defer pool.Close()
	store.ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	cases := ordinaryHandlerCases()
	// Enumerate the actual descriptor instead of silently losing coverage when an
	// ordinary RPC is added. The six ownership RPCs have separate lifecycle tests.
	covered := make(map[string]bool)
	for _, tc := range cases {
		require.False(t, covered[tc.name])
		covered[tc.name] = true
	}
	excluded := map[string]bool{"ApplyOwnershipTransfer": true, "CompensateOwnershipTransfer": true, "GetOwnershipTransferCapabilities": true, "PrepareOwnershipTransfer": true, "FinalizeOwnershipTransfer": true, "AbortOwnershipTransfer": true}
	for _, method := range rolev1.RoleService_ServiceDesc.Methods {
		require.True(t, covered[method.MethodName] || excluded[method.MethodName], method.MethodName)
	}
	f := newOrdinaryHandlerFixture(t, ctx, pool)
	prepareOrdinaryHandlerFixture(t, ctx, pool, f)
	for _, tc := range cases {
		t.Run("prepared/"+tc.name, func(t *testing.T) {
			before := ordinaryHandlerSnapshot(t, ctx, pool)
			response, err := tc.call(f)
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Nil(t, response)
			count, visibilityErrors := f.events.snapshot()
			require.Zero(t, count)
			require.Empty(t, visibilityErrors)
			require.Equal(t, before, ordinaryHandlerSnapshot(t, ctx, pool))
		})
	}
	t.Run("empty profile does not return empty authority", func(t *testing.T) {
		for _, call := range []func() (any, error){
			func() (any, error) {
				return f.svc.GetMemberRoles(f.ctx, &rolev1.GetMemberRolesRequest{SpaceId: f.space.String(), ProfileId: f.empty.String()})
			},
			func() (any, error) {
				return f.svc.CheckPermission(f.ctx, &rolev1.CheckPermissionRequest{SpaceId: f.space.String(), ProfileId: f.empty.String(), PermissionName: permissions.SpaceManageRoles})
			},
			func() (any, error) {
				return f.svc.GetEffectivePermissions(f.ctx, &rolev1.GetEffectivePermissionsRequest{SpaceId: f.space.String(), ProfileId: f.empty.String()})
			},
		} {
			response, err := call()
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Nil(t, response)
		}
	})
	t.Run("unseeded bootstrap and absent default stay frozen", func(t *testing.T) {
		fresh := f
		fresh.space = uuid.New()
		prepareOrdinaryHandlerFixture(t, ctx, pool, fresh)
		before := ordinaryHandlerSnapshot(t, ctx, pool)
		response, err := fresh.svc.BootstrapSpaceRoles(fresh.ctx, &rolev1.BootstrapSpaceRolesRequest{SpaceId: fresh.space.String(), OwnerProfileId: fresh.owner.String()})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Nil(t, response)
		defaultResponse, err := fresh.svc.GetDefaultJoinRole(fresh.ctx, &rolev1.GetDefaultJoinRoleRequest{SpaceId: fresh.space.String()})
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Nil(t, defaultResponse)
		require.Equal(t, before, ordinaryHandlerSnapshot(t, ctx, pool))
		count, _ := f.events.snapshot()
		require.Zero(t, count)
	})
	t.Run("foreign roles are not found in the request space", func(t *testing.T) {
		local := newOrdinaryHandlerFixture(t, ctx, pool)
		foreign := newOrdinaryHandlerFixture(t, ctx, pool)
		prepareOrdinaryHandlerFixture(t, ctx, pool, foreign)
		for _, tc := range []struct {
			name string
			call func() (any, error)
		}{
			{"AssignRole", func() (any, error) {
				return local.svc.AssignRole(local.ctx, &rolev1.AssignRoleRequest{SpaceId: local.space.String(), ProfileId: local.empty.String(), RoleId: foreign.custom.String()})
			}},
			{"RevokeRole", func() (any, error) {
				return local.svc.RevokeRole(local.ctx, &rolev1.RevokeRoleRequest{SpaceId: local.space.String(), ProfileId: local.owner.String(), RoleId: foreign.custom.String()})
			}},
			{"SetChatOverride", func() (any, error) {
				return local.svc.SetChatOverride(local.ctx, &rolev1.SetChatOverrideRequest{SpaceId: local.space.String(), Chat: &chatv1.ChatRef{Id: local.chat.String()}, RoleId: foreign.custom.String(), AllowMask: 8})
			}},
			{"RemoveChatOverride", func() (any, error) {
				return local.svc.RemoveChatOverride(local.ctx, &rolev1.RemoveChatOverrideRequest{SpaceId: local.space.String(), Chat: &chatv1.ChatRef{Id: local.chat.String()}, RoleId: foreign.custom.String()})
			}},
			{"SetVoiceRoomOverride", func() (any, error) {
				return local.svc.SetVoiceRoomOverride(local.ctx, &rolev1.SetVoiceRoomOverrideRequest{SpaceId: local.space.String(), VoiceRoomId: local.voice.String(), RoleId: foreign.custom.String(), AllowMask: 8})
			}},
			{"RemoveVoiceRoomOverride", func() (any, error) {
				return local.svc.RemoveVoiceRoomOverride(local.ctx, &rolev1.RemoveVoiceRoomOverrideRequest{SpaceId: local.space.String(), VoiceRoomId: local.voice.String(), RoleId: foreign.custom.String()})
			}},
			{"SetDefaultJoinRole", func() (any, error) {
				return local.svc.SetDefaultJoinRole(local.ctx, &rolev1.SetDefaultJoinRoleRequest{SpaceId: local.space.String(), RoleId: foreign.custom.String()})
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				before := ordinaryHandlerSnapshot(t, ctx, pool)
				response, err := tc.call()
				require.Equal(t, codes.NotFound, status.Code(err))
				require.Nil(t, response)
				require.Equal(t, before, ordinaryHandlerSnapshot(t, ctx, pool))
				count, visibilityErrors := local.events.snapshot()
				require.Zero(t, count)
				require.Empty(t, visibilityErrors)
			})
		}
	})
	t.Run("missing fence table remains unavailable for every ordinary RPC", func(t *testing.T) {
		live := newOrdinaryHandlerFixture(t, ctx, pool)
		_, err := pool.Exec(ctx, "ALTER TABLE ownership_transfer_v2 RENAME TO missing_handler_fence")
		require.NoError(t, err)
		defer func() {
			_, e := pool.Exec(ctx, "ALTER TABLE missing_handler_fence RENAME TO ownership_transfer_v2")
			require.NoError(t, e)
		}()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				before := ordinaryHandlerSnapshot(t, ctx, pool)
				response, err := tc.call(live)
				require.Equal(t, codes.Unavailable, status.Code(err))
				require.Nil(t, response)
				require.Equal(t, before, ordinaryHandlerSnapshot(t, ctx, pool))
				count, _ := live.events.snapshot()
				require.Zero(t, count)
			})
		}
	})
}

func TestOrdinaryHandlerTransactionAndEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := store.StartRoleDBForStoreTest(t, ctx)
	defer pool.Close()
	store.ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	f := newOrdinaryHandlerFixture(t, ctx, pool)
	// View and triggers observe PostgreSQL transaction IDs, without production
	// hooks or a mocked store. The volatile predicate cannot be optimized away.
	_, err := pool.Exec(ctx, `CREATE TABLE ordinary_handler_trace(kind TEXT NOT NULL,xid BIGINT NOT NULL);
 ALTER TABLE member_roles RENAME TO ordinary_handler_member_roles;
 CREATE FUNCTION ordinary_handler_auth_trace(UUID) RETURNS BOOLEAN LANGUAGE plpgsql VOLATILE AS $$
 BEGIN INSERT INTO ordinary_handler_trace VALUES ('auth',txid_current()); RETURN true; END $$;
 CREATE VIEW member_roles AS SELECT * FROM ordinary_handler_member_roles WHERE ordinary_handler_auth_trace(profile_id);
 CREATE FUNCTION ordinary_handler_write_trace() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN INSERT INTO ordinary_handler_trace VALUES ('write',txid_current()); RETURN NEW; END $$;
 CREATE TRIGGER ordinary_handler_write_trace AFTER INSERT ON roles FOR EACH ROW EXECUTE FUNCTION ordinary_handler_write_trace();`)
	require.NoError(t, err)
	response, err := f.svc.CreateRole(f.ctx, &rolev1.CreateRoleRequest{SpaceId: f.space.String(), Name: "atomic create", Position: 1, PermissionsMask: 1})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.NotNil(t, response.Role)
	var authCount, writeCount, distinctXIDs int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE kind='auth'),count(*) FILTER(WHERE kind='write'),count(DISTINCT xid) FROM ordinary_handler_trace`).Scan(&authCount, &writeCount, &distinctXIDs))
	require.Positive(t, authCount, "real authorization reads must be observed")
	require.Equal(t, 1, writeCount)
	require.Equal(t, 1, distinctXIDs, "authorization and mutation must share one transaction")
	count, visibilityErrors := f.events.snapshot()
	require.Equal(t, 1, count)
	require.Empty(t, visibilityErrors, "publish only after commit")

	// A deferred constraint fails specifically at outer commit. A publisher placed
	// inside that transaction would leak an event even though no role survives.
	_, err = pool.Exec(ctx, `CREATE FUNCTION ordinary_handler_reject_commit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.name='reject at commit' THEN RAISE EXCEPTION 'ordinary handler commit rejected' USING ERRCODE='23514'; END IF; RETURN NEW; END $$;
 CREATE CONSTRAINT TRIGGER ordinary_handler_reject_commit AFTER INSERT ON roles DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ordinary_handler_reject_commit();`)
	require.NoError(t, err)
	response, err = f.svc.CreateRole(f.ctx, &rolev1.CreateRoleRequest{SpaceId: f.space.String(), Name: "reject at commit", Position: 1, PermissionsMask: 1})
	require.Error(t, err)
	require.Nil(t, response)
	var surviving int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE space_id=$1 AND name='reject at commit'", f.space).Scan(&surviving))
	require.Zero(t, surviving)
	afterCount, afterVisibilityErrors := f.events.snapshot()
	require.Equal(t, count, afterCount, "failed commit must publish no event")
	require.Equal(t, visibilityErrors, afterVisibilityErrors)
}
