package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

type r20AllowRoleClient struct {
	rolev1.RoleServiceClient
	calls atomic.Int64
}

type r20MutationLockerSpy struct {
	calls atomic.Int64
}

func (l *r20MutationLockerSpy) Acquire(context.Context, uuid.UUID) (func(), error) {
	l.calls.Add(1)
	return func() {}, nil
}

func (c *r20AllowRoleClient) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	c.calls.Add(1)
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

func applyR20OwnershipFreezePrerequisites(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySpaceMigrationsThrough7(t, ctx, pool)
	for _, name := range []string{
		"000008_ownership_journal.up.sql", "000009_ownership_journal_decision.up.sql",
		"000010_ownership_journal_commit.up.sql", "000011_voice_access_epoch.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
}

type r20GRPCFreezeFixture struct {
	store                         *store.SpaceStore
	owner, member, account, space uuid.UUID
	room                          uuid.UUID
	invite                        *store.InviteRow
	binding                       store.OwnershipBinding
}

func newR20GRPCFreezeFixture(t *testing.T) r20GRPCFreezeFixture {
	t.Helper()
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applyR20OwnershipFreezePrerequisites(t, ctx, pool)
	st := &store.SpaceStore{Pool: pool}
	f := r20GRPCFreezeFixture{store: st, owner: uuid.New(), member: uuid.New(), account: uuid.New()}
	space, err := st.CreateSpace(ctx, f.owner, "r20 frozen", "unchanged", "private")
	require.NoError(t, err)
	f.space = space.ID
	_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, f.space, f.member)
	require.NoError(t, err)
	room, _, err := st.CreateVoiceRoom(ctx, f.space, "r20 room", nil)
	require.NoError(t, err)
	f.room = room.ID
	f.invite, err = st.CreateInvite(ctx, store.CreateInviteInput{SpaceID: f.space, CreatorProfileID: f.owner})
	require.NoError(t, err)
	f.binding = store.OwnershipBinding{
		ProtocolVersion: 2, OperationID: uuid.New(), SpaceID: f.space, AccountID: f.account,
		ActorProfileID: f.owner, NewOwnerProfileID: f.member, SessionEpoch: 1,
		ProofDigest: strings.Repeat("a", 64),
	}
	_, err = st.ReserveOwnership(ctx, f.binding)
	require.NoError(t, err)
	return f
}

func r20IncomingProfile(accountID, profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderUserID, accountID.String(), authctx.HeaderProfileID, profileID.String(),
	))
}

func r20GRPCFreezeSnapshot(t *testing.T, f r20GRPCFreezeFixture) string {
	t.Helper()
	var snapshot string
	require.NoError(t, f.store.Pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
		'space',(SELECT to_jsonb(s) FROM spaces s WHERE id=$1),
		'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM space_members m WHERE space_id=$1),
		'audit',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM audit_log a WHERE space_id=$1),
		'epoch',(SELECT to_jsonb(e) FROM space_voice_access_epochs e WHERE space_id=$1),
		'epoch_outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY access_epoch) FROM space_voice_access_outbox o WHERE space_id=$1),
		'journal',(SELECT jsonb_agg(to_jsonb(j) ORDER BY operation_id) FROM ownership_journal j WHERE space_id=$1)
	)::text`, f.space).Scan(&snapshot))
	return snapshot
}

func newR20MutationLocker(t *testing.T, source *pgxpool.Pool) SpaceMutationLocker {
	t.Helper()
	config := source.Config().Copy()
	config.MaxConns = 2
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return store.NewSpaceMutationLocker(pool)
}

func requireR20UnavailableWithoutDetail(t *testing.T, response any, err error) {
	t.Helper()
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
	message := status.Convert(err).Message()
	require.NotEmpty(t, message)
	require.NotContains(t, message, "ownership_journal")
	require.NotContains(t, message, "SQLSTATE")
}

func TestOwnershipFreeze_GRPCOwnerAdminBotInviteVoiceAndBatchFailClosedWithoutEvents(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *SpaceGRPC, r20GRPCFreezeFixture) (any, error)
	}{
		{"get space read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: f.space.String()})
		}},
		{"list my spaces read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListMySpaces(ctx, &spacev1.ListMySpacesRequest{})
		}},
		{"list members read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListMembers(ctx, &spacev1.ListMembersRequest{SpaceId: f.space.String()})
		}},
		{"list invites read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListInvites(ctx, &spacev1.ListInvitesRequest{SpaceId: f.space.String()})
		}},
		{"list bans read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListBans(ctx, &spacev1.ListBansRequest{SpaceId: f.space.String()})
		}},
		{"audit read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.GetAuditLog(ctx, &spacev1.GetAuditLogRequest{SpaceId: f.space.String()})
		}},
		{"tree read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: f.space.String()})
		}},
		{"owner shortcut", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			name := "forbidden"
			return svc.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{SpaceId: f.space.String(), Name: &name})
		}},
		{"role admin allow", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			name := "forbidden"
			return svc.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{SpaceId: f.space.String(), Name: &name})
		}},
		{"bot lifecycle", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.AddBotMember(ctx, &spacev1.AddBotMemberRequest{SpaceId: f.space.String(), ProfileId: uuid.NewString()})
		}},
		{"invite indirect read", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.GetInvite(ctx, &spacev1.GetInviteRequest{Code: f.invite.Code})
		}},
		{"voice authority", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			ctx = authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityVoice)
			return svc.ResolveVoiceRoomAccess(ctx, &spacev1.ResolveVoiceRoomAccessRequest{Space: &spacev1.SpaceRef{Id: f.space.String()}, VoiceRoomId: f.room.String(), ProfileId: f.member.String()})
		}},
		{"multi-space batch", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.AreCoMembers(ctx, &spacev1.AreCoMembersRequest{ProfileIdA: f.owner.String(), ProfileIdB: f.member.String(), SpaceIds: []string{f.space.String()}})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newR20GRPCFreezeFixture(t)
			events := &spySpaceEvents{}
			roles := &r20AllowRoleClient{}
			svc := &SpaceGRPC{Store: f.store, Roles: roles, SpaceEvents: events, MutationLocker: newR20MutationLocker(t, f.store.Pool)}
			if tc.name == "owner shortcut" {
				svc.Roles = nil
			}
			before := r20GRPCFreezeSnapshot(t, f)
			caller := f.owner
			if tc.name == "role admin allow" {
				caller = f.member
			}
			ctx, cancel := context.WithTimeout(r20IncomingProfile(f.account, caller), 2*time.Second)
			defer cancel()
			response, err := tc.call(ctx, svc, f)
			requireR20UnavailableWithoutDetail(t, response, err)
			require.Equal(t, before, r20GRPCFreezeSnapshot(t, f))
			require.Empty(t, events.snapshot())
			require.Empty(t, events.snapshotUpdated())
			require.Empty(t, events.snapshotDeleted())
			require.Zero(t, roles.calls.Load(), "pending ownership must be checked before every Role fallback")
		})
	}
}

func TestOwnershipFreeze_GRPCMutationDoesNotSelfDeadlockUnderLegacySessionLease(t *testing.T) {
	f := newR20GRPCFreezeFixture(t)
	events := &spySpaceEvents{}
	legacyLocker := &r20MutationLockerSpy{}
	svc := &SpaceGRPC{Store: f.store, SpaceEvents: events, MutationLocker: legacyLocker}
	name := "forbidden"
	ctx, cancel := context.WithTimeout(r20IncomingProfile(f.account, f.owner), 2*time.Second)
	defer cancel()
	response, err := svc.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{SpaceId: f.space.String(), Name: &name})
	require.Zero(t, legacyLocker.calls.Load(), "R20 ordinary transaction scope must replace the legacy session advisory lease")
	requireR20UnavailableWithoutDetail(t, response, err)
	require.NotEqual(t, codes.DeadlineExceeded, status.Code(err))
	require.NoError(t, ctx.Err(), "the ownership scope must not reacquire its own legacy session lock until deadline")
	require.Empty(t, events.snapshotUpdated())
}

func TestOwnershipFreeze_GRPCJournalOutageMapsBroadReadListAndIndirectSurfacesToUnavailable(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *SpaceGRPC, r20GRPCFreezeFixture) (any, error)
	}{
		{"get space", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: f.space.String()})
		}},
		{"list my spaces", func(ctx context.Context, svc *SpaceGRPC, _ r20GRPCFreezeFixture) (any, error) {
			return svc.ListMySpaces(ctx, &spacev1.ListMySpacesRequest{})
		}},
		{"list members", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListMembers(ctx, &spacev1.ListMembersRequest{SpaceId: f.space.String()})
		}},
		{"invite indirect", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.GetInvite(ctx, &spacev1.GetInviteRequest{Code: f.invite.Code})
		}},
		{"tree list", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: f.space.String()})
		}},
		{"voice authority", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			ctx = authctx.WithVerifiedServiceIdentity(ctx, authctx.ServiceIdentityVoice)
			return svc.ResolveVoiceRoomAccess(ctx, &spacev1.ResolveVoiceRoomAccessRequest{Space: &spacev1.SpaceRef{Id: f.space.String()}, VoiceRoomId: f.room.String(), ProfileId: f.member.String()})
		}},
		{"batch", func(ctx context.Context, svc *SpaceGRPC, f r20GRPCFreezeFixture) (any, error) {
			return svc.AreCoMembers(ctx, &spacev1.AreCoMembersRequest{ProfileIdA: f.owner.String(), ProfileIdB: f.member.String(), SpaceIds: []string{f.space.String()}})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newR20GRPCFreezeFixture(t)
			_, err := f.store.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal RENAME TO ownership_journal_unavailable`)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, restoreErr := f.store.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal_unavailable RENAME TO ownership_journal`)
				require.NoError(t, restoreErr)
			})
			roles := &r20AllowRoleClient{}
			svc := &SpaceGRPC{Store: f.store, Roles: roles, MutationLocker: newR20MutationLocker(t, f.store.Pool)}
			ctx, cancel := context.WithTimeout(r20IncomingProfile(f.account, f.owner), 2*time.Second)
			defer cancel()
			response, callErr := tc.call(ctx, svc, f)
			requireR20UnavailableWithoutDetail(t, response, callErr)
			require.NoError(t, ctx.Err())
			require.Zero(t, roles.calls.Load(), "journal outage must fail closed before Role fallback")
		})
	}
}

func TestOwnershipFreeze_GRPCResolverFailureUsesGenericUnavailableDetail(t *testing.T) {
	f := newR20GRPCFreezeFixture(t)
	_, err := f.store.Pool.Exec(context.Background(), `ALTER TABLE invites RENAME TO invites_r20_unavailable`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, restoreErr := f.store.Pool.Exec(context.Background(), `ALTER TABLE invites_r20_unavailable RENAME TO invites`)
		require.NoError(t, restoreErr)
	})
	svc := &SpaceGRPC{Store: f.store}
	ctx, cancel := context.WithTimeout(r20IncomingProfile(f.account, f.owner), 2*time.Second)
	defer cancel()

	response, callErr := svc.GetInvite(ctx, &spacev1.GetInviteRequest{Code: f.invite.Code})
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(callErr))
	require.Equal(t, "space temporarily unavailable", status.Convert(callErr).Message())
	require.NotContains(t, status.Convert(callErr).Message(), "invites")
	require.NotContains(t, status.Convert(callErr).Message(), "relation")
	require.NotContains(t, status.Convert(callErr).Message(), "SQLSTATE")
	require.NoError(t, ctx.Err())
}
