package store

import (
	"context"
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

	require.NoError(t, st.RevokeInvite(ctx, inv.ID))
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

	member, err := st.JoinByInvite(ctx, inv.Code, joiner)
	require.NoError(t, err)
	require.Equal(t, joiner, member.ProfileID)

	got, err := st.GetInviteByCode(ctx, inv.Code)
	require.NoError(t, err)
	require.Equal(t, int32(1), got.UseCount)

	ok, err := st.IsSpaceMember(ctx, space.ID, joiner)
	require.NoError(t, err)
	require.True(t, ok)
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

	first, err := st.JoinByInvite(ctx, inv.Code, owner)
	require.NoError(t, err)
	second, err := st.JoinByInvite(ctx, inv.Code, owner)
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

	_, err = st.JoinByInvite(ctx, inv.Code, joiner)
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
	_, err = st.JoinByInvite(ctx, inv.Code, joiner1)
	require.NoError(t, err)

	joiner2 := uuid.New()
	_, err = st.JoinByInvite(ctx, inv.Code, joiner2)
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
	require.NoError(t, st.RevokeInvite(ctx, inv.ID))

	_, err = st.JoinByInvite(ctx, inv.Code, joiner)
	require.ErrorIs(t, err, ErrInviteRevoked)
}
