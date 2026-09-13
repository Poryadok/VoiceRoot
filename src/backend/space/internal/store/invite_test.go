package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInvite_Create_List_Revoke(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Invite Space", "", "private")
	require.NoError(t, err)

	inv, err := st.CreateInvite(ctx, CreateInviteInput{
		SpaceID:          space.ID,
		CreatorProfileID: owner,
	})
	require.NoError(t, err)
	require.NotEmpty(t, inv.Code)
	require.Equal(t, int32(0), inv.UseCount)

	list, err := st.ListInvites(ctx, space.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, st.RevokeInvite(ctx, inv.ID, owner))
	revoked, err := st.GetInviteByCode(ctx, inv.Code)
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt)

	list, err = st.ListInvites(ctx, space.ID)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestInvite_Join_IncrementsUseCount(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	joiner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Join Space", "", "private")
	require.NoError(t, err)

	inv, err := st.CreateInvite(ctx, CreateInviteInput{
		SpaceID:          space.ID,
		CreatorProfileID: owner,
	})
	require.NoError(t, err)

	member, err := st.JoinByInvite(ctx, inv.Code, joiner, uuid.New())
	require.NoError(t, err)
	require.Equal(t, joiner, member.ProfileID)

	got, err := st.GetInviteByCode(ctx, inv.Code)
	require.NoError(t, err)
	require.Equal(t, int32(1), got.UseCount)

	ok, err := st.IsSpaceMember(ctx, space.ID, joiner)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestInvite_GuestJoinsAcrossStoresSerializeSpaceUpdate proves two concurrent
// guest joins with distinct invites do not deadlock while both increment the
// space member counter. Separate stores model independent service instances.
func TestInvite_GuestJoinsAcrossStoresSerializeSpaceUpdate(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	ownerStore := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := ownerStore.CreateSpace(ctx, owner, "Concurrent guest admission", "", "private")
	require.NoError(t, err)
	allow := true
	_, err = ownerStore.UpdateSpace(ctx, space.ID, UpdateSpaceInput{AllowGuests: &allow})
	require.NoError(t, err)

	firstInvite, err := ownerStore.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	secondInvite, err := ownerStore.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)

	joinCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, inv := range []*InviteRow{firstInvite, secondInvite} {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			<-start
			_, err := (&SpaceStore{Pool: pool}).JoinGuestByInvite(joinCtx, code, uuid.New(), uuid.New())
			errs <- err
		}(inv.Code)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	updated, err := ownerStore.GetSpace(ctx, space.ID)
	require.NoError(t, err)
	require.Equal(t, int32(3), updated.MemberCount)
}

func TestInvite_Join_IdempotentForExistingMember(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Idempotent", "", "private")
	require.NoError(t, err)

	inv, err := st.CreateInvite(ctx, CreateInviteInput{
		SpaceID:          space.ID,
		CreatorProfileID: owner,
	})
	require.NoError(t, err)

	ownerAcct := uuid.New()
	first, err := st.JoinByInvite(ctx, inv.Code, owner, ownerAcct)
	require.NoError(t, err)
	second, err := st.JoinByInvite(ctx, inv.Code, owner, ownerAcct)
	require.NoError(t, err)
	require.Equal(t, first.JoinedAt, second.JoinedAt)

	got, err := st.GetInviteByCode(ctx, inv.Code)
	require.NoError(t, err)
	require.Equal(t, int32(0), got.UseCount)
}

func TestInvite_ExpiredRejected(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	joiner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Expired", "", "private")
	require.NoError(t, err)

	past := time.Now().UTC().Add(-time.Hour)
	inv, err := st.CreateInvite(ctx, CreateInviteInput{
		SpaceID:          space.ID,
		CreatorProfileID: owner,
		ExpiresAt:        &past,
	})
	require.NoError(t, err)

	_, err = st.JoinByInvite(ctx, inv.Code, joiner, uuid.New())
	require.ErrorIs(t, err, ErrInviteExpired)
}

func TestInvite_MaxUsesRejected(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "MaxUses", "", "private")
	require.NoError(t, err)

	max := int32(1)
	inv, err := st.CreateInvite(ctx, CreateInviteInput{
		SpaceID:          space.ID,
		CreatorProfileID: owner,
		MaxUses:          &max,
	})
	require.NoError(t, err)

	joiner1 := uuid.New()
	_, err = st.JoinByInvite(ctx, inv.Code, joiner1, uuid.New())
	require.NoError(t, err)

	joiner2 := uuid.New()
	_, err = st.JoinByInvite(ctx, inv.Code, joiner2, uuid.New())
	require.ErrorIs(t, err, ErrInviteMaxUses)
}

func TestInvite_RevokedRejected(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	joiner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Revoked", "", "private")
	require.NoError(t, err)

	inv, err := st.CreateInvite(ctx, CreateInviteInput{
		SpaceID:          space.ID,
		CreatorProfileID: owner,
	})
	require.NoError(t, err)
	require.NoError(t, st.RevokeInvite(ctx, inv.ID, owner))

	_, err = st.JoinByInvite(ctx, inv.Code, joiner, uuid.New())
	require.ErrorIs(t, err, ErrInviteRevoked)
}
