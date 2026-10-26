package grpcsvc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

func TestSearchPublicSpacesCatalog(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)
	owner, account, ownerCtx := profileFixture(t)
	_, _, viewerCtx := profileFixture(t)
	makeSpace := func(name, visibility string, count int32, verified bool) *spacev1.Space {
		created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: name, Description: "Friday topic", Visibility: visibility})
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), `UPDATE spaces SET member_count=$2,is_verified=$3,icon_url='https://cdn.voice.gg/icon.webp',entry_questions='[{"answer":"secret"}]' WHERE id=$1`, created.Space.Id, count, verified)
		require.NoError(t, err)
		return &spacev1.Space{Id: created.Space.Id, Name: name, Description: "Friday topic", IconUrl: strPtrCatalog("https://cdn.voice.gg/icon.webp"), MemberCount: count, IsVerified: verified}
	}
	public := makeSpace("Public", "public", 100, false)
	invite := makeSpace("Invite", "invite_only", 2, true)
	makeSpace("Hidden", "private", 999, true)
	_, err := client.SearchPublicSpaces(context.Background(), &spacev1.SearchPublicSpacesRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	first, err := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	require.Len(t, first.SpaceList.Spaces, 1)
	require.True(t, proto.Equal(invite, first.SpaceList.Spaces[0]), "catalog must disclose only approved card metadata")
	require.NotEmpty(t, first.SpaceList.NextCursor)
	second, err := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{Page: &commonv1.CursorPageRequest{PageSize: 1, Cursor: first.SpaceList.NextCursor}})
	require.NoError(t, err)
	require.Len(t, second.SpaceList.Spaces, 1)
	require.True(t, proto.Equal(public, second.SpaceList.Spaces[0]))
	require.Empty(t, second.SpaceList.NextCursor)
	// Within the same verification tier member count determines rank.
	_, err = pool.Exec(context.Background(), `UPDATE spaces SET is_verified=true WHERE id=$1`, public.Id)
	require.NoError(t, err)
	ranked, err := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	require.Equal(t, public.Id, ranked.SpaceList.Spaces[0].Id)
	_, err = pool.Exec(context.Background(), `UPDATE spaces SET is_verified=false WHERE id=$1`, public.Id)
	require.NoError(t, err)
	_, err = client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{Query: "changed", Page: &commonv1.CursorPageRequest{Cursor: first.SpaceList.NextCursor}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	for _, query := range []string{"Friday", "INVITE", "%"} {
		result, err := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{Query: query})
		require.NoError(t, err)
		want := 2
		if query == "INVITE" {
			want = 1
		}
		if query == "%" {
			want = 0
		}
		require.Len(t, result.SpaceList.Spaces, want)
	}
	_, err = client.GetSpace(viewerCtx, &spacev1.GetSpaceRequest{SpaceId: public.Id})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "catalog must not change GetSpace ACL")
	mine, err := client.ListMySpaces(viewerCtx, &spacev1.ListMySpacesRequest{})
	require.NoError(t, err)
	require.Empty(t, mine.SpaceList.Spaces)
	mine, err = client.ListMySpaces(ownerCtx, &spacev1.ListMySpacesRequest{})
	require.NoError(t, err)
	require.Len(t, mine.SpaceList.Spaces, 3)
	// Hold the production mutation lease until discovery has selected candidates.
	// Switching public->private before its lock is released cannot leak the old row.
	release, err := store.NewSpaceMutationLocker(pool).Acquire(context.Background(), uuid.MustParse(public.Id))
	require.NoError(t, err)
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	type catalogResult struct {
		result *spacev1.SearchPublicSpacesResponse
		err    error
	}
	finished := make(chan catalogResult, 1)
	go func() {
		result, callErr := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{})
		finished <- catalogResult{result, callErr}
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND query LIKE '%pg_advisory_xact_lock%')`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	// Current visibility/lifecycle authority is applied before pagination, with
	// no reliance on the asynchronous Search projection.
	_, err = pool.Exec(context.Background(), `UPDATE spaces SET visibility='private' WHERE id=$1`, public.Id)
	require.NoError(t, err)
	release()
	released = true
	var settled catalogResult
	select {
	case settled = <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("catalog did not settle after visibility mutation")
	}
	visible, err := settled.result, settled.err
	require.NoError(t, err)
	require.Len(t, visible.SpaceList.Spaces, 1)
	require.Equal(t, invite.Id, visible.SpaceList.Spaces[0].Id)
	_, err = pool.Exec(context.Background(), `UPDATE spaces SET visibility='public' WHERE id=$1`, public.Id)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation)
	 VALUES($1,$2,'SCHEDULE_PENDING',1)`, public.Id, uuid.New())
	require.NoError(t, err)
	visible, err = client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	require.Len(t, visible.SpaceList.Spaces, 1)
	require.Equal(t, invite.Id, visible.SpaceList.Spaces[0].Id)
	require.Empty(t, visible.SpaceList.NextCursor)
	target := uuid.New()
	_, err = pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, invite.Id, target)
	require.NoError(t, err)
	_, err = (&store.SpaceStore{Pool: pool}).ReserveOwnership(context.Background(), store.OwnershipBinding{
		ProtocolVersion: 2, OperationID: uuid.New(), SpaceID: uuid.MustParse(invite.Id), AccountID: account,
		ActorProfileID: owner, NewOwnerProfileID: target, SessionEpoch: 1, ProofDigest: strings.Repeat("a", 64),
	})
	require.NoError(t, err)
	frozen, err := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{})
	require.Nil(t, frozen)
	require.Equal(t, codes.Unavailable, status.Code(err))
	// Schema failure in freeze authority must not return a partial catalog.
	_, err = pool.Exec(context.Background(), `ALTER TABLE ownership_journal RENAME TO catalog_unavailable_journal`)
	require.NoError(t, err)
	result, err := client.SearchPublicSpaces(viewerCtx, &spacev1.SearchPublicSpacesRequest{})
	require.Nil(t, result)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func strPtrCatalog(value string) *string { return &value }
