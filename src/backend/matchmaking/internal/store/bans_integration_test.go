package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBanStore_InsertMMPeerBanPersistsBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	banner := uuid.New()
	target := uuid.New()
	bans := &BanStore{Pool: pool}

	err := bans.InsertMMPeerBan(ctx, InsertMMPeerBanParams{
		BannerProfileID: banner,
		TargetProfileID: target,
	})
	require.NoError(t, err)

	banned, err := bans.IsPeerBanned(ctx, banner, target)
	require.NoError(t, err)
	require.True(t, banned)
}

func TestBanStore_IsPeerBannedFalseWhenNoBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	bans := &BanStore{Pool: pool}
	banned, err := bans.IsPeerBanned(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	require.False(t, banned)
}

func TestBanStore_PeerBanIsDirectional(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	banner := uuid.New()
	target := uuid.New()
	bans := &BanStore{Pool: pool}
	require.NoError(t, bans.InsertMMPeerBan(ctx, InsertMMPeerBanParams{
		BannerProfileID: banner,
		TargetProfileID: target,
	}))

	forward, err := bans.IsPeerBanned(ctx, banner, target)
	require.NoError(t, err)
	require.True(t, forward)

	reverse, err := bans.IsPeerBanned(ctx, target, banner)
	require.NoError(t, err)
	require.False(t, reverse)
}

func TestBanStore_IsPairBannedEitherDirection(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	a := uuid.New()
	b := uuid.New()
	bans := &BanStore{Pool: pool}
	require.NoError(t, bans.InsertMMPeerBan(ctx, InsertMMPeerBanParams{
		BannerProfileID: a,
		TargetProfileID: b,
	}))

	pair, err := bans.IsPairBanned(ctx, a, b)
	require.NoError(t, err)
	require.True(t, pair)

	reverse, err := bans.IsPairBanned(ctx, b, a)
	require.NoError(t, err)
	require.True(t, reverse)
}

func TestBanStore_RemoveMMPeerBanClearsBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	a := uuid.New()
	b := uuid.New()
	bans := &BanStore{Pool: pool}
	require.NoError(t, bans.InsertMMPeerBan(ctx, InsertMMPeerBanParams{
		BannerProfileID: a,
		TargetProfileID: b,
	}))
	require.NoError(t, bans.RemoveMMPeerBan(ctx, a, b))

	ok, err := bans.IsPeerBanned(ctx, a, b)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestBanStore_InsertMMPeerBanRejectsSelfBan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	profile := uuid.New()
	bans := &BanStore{Pool: pool}
	err := bans.InsertMMPeerBan(ctx, InsertMMPeerBanParams{
		BannerProfileID: profile,
		TargetProfileID: profile,
	})
	require.Error(t, err)
}
