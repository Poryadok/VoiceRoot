package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"
)

func TestExactPresenceViewerProfile_FailsClosedForAmbiguousIdentity(t *testing.T) {
	target := uuid.New()
	other := uuid.New()
	cases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{name: "single valid identity", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, target.String())), want: true},
		{name: "duplicate includes matching first identity", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, target.String(), authctx.HeaderProfileID, other.String()))},
		{name: "malformed identity", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, "not-a-uuid"))},
		{name: "empty identity", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, ""))},
		{name: "viewerless", ctx: context.Background()},
		{name: "s2s without viewer identity", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderInternalCaller, "social"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := exactPresenceViewerProfile(tc.ctx)
			require.Equal(t, tc.want, ok)
			if ok {
				require.Equal(t, target, got)
			}
		})
	}
}

func TestPresenceRead_DuplicateMatchingIdentityIsViewerless(t *testing.T) {
	target := uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, target.String(),
		authctx.HeaderProfileID, uuid.New().String(),
	))
	// This would have been treated as self by authctx.ProfileID via vals[0].
	// No PrivacyStore is available, so a non-self viewer must fail closed.
	require.False(t, isSelfViewer(ctx, target))
}

func TestPresenceForViewer_AmbiguousProfileIdentityDoesNotBecomeSelf(t *testing.T) {
	target := uuid.New()
	other := uuid.New()
	s := &UserGRPC{}
	snap := &store.PresenceSnapshot{
		Live:         true,
		Status:       "online",
		GameTitle:    "Dota",
		LastSeenUnix: 99,
	}
	for name, ctx := range map[string]context.Context{
		"duplicate matching first": metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, target.String(), authctx.HeaderProfileID, other.String())),
		"malformed":                metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, "not-a-uuid")),
	} {
		t.Run(name, func(t *testing.T) {
			out := s.presenceForViewer(ctx, target, snap)
			require.Empty(t, out.GetStatus())
			require.Empty(t, out.GetGameTitle())
			require.Nil(t, out.GetLastSeen())
		})
	}

	selfCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, target.String()))
	self := s.presenceForViewer(selfCtx, target, snap)
	require.Equal(t, "online", self.GetStatus())
	require.Equal(t, "Dota", self.GetGameTitle())
	require.NotNil(t, self.GetLastSeen())
}
