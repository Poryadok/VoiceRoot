package roomlifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
)

type roomAuthorityStub struct {
	access RoomAccess
	err    error
	calls  []roomAuthorityCall
	after  func()
}

type roomAuthorityCall struct {
	roomID         uuid.UUID
	actorProfileID uuid.UUID
}

func (s *roomAuthorityStub) ResolveJoinAccess(_ context.Context, roomID, actorProfileID uuid.UUID) (RoomAccess, error) {
	s.calls = append(s.calls, roomAuthorityCall{roomID: roomID, actorProfileID: actorProfileID})
	if s.after != nil {
		s.after()
	}
	return s.access, s.err
}

type permissionAuthorityStub struct {
	allowed bool
	err     error
	calls   []JoinPermissionQuery
	after   func()
}

func (s *permissionAuthorityStub) CheckJoinPermission(_ context.Context, query JoinPermissionQuery) (bool, error) {
	s.calls = append(s.calls, query)
	if s.after != nil {
		s.after()
	}
	return s.allowed, s.err
}

func TestAuthorizeJoin_ForwardsOnlyTheCanonicalVerifiedTuple(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	accountID, profileID, spaceID, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	rooms := &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID.String(), Member: true, Active: true}}
	permissions := &permissionAuthorityStub{allowed: true}

	decision, err := NewAuthorizer(rooms, permissions, func() time.Time { return now }).AuthorizeJoin(
		verifiedContext(accountID, profileID, now.Add(time.Minute)), spaceID.String(), roomID.String(),
	)
	require.NoError(t, err)
	require.Equal(t, profileID.String(), decision.ActorProfileID())
	require.Equal(t, spaceID.String(), decision.SpaceID())
	require.Equal(t, roomID.String(), decision.RoomID())
	require.Equal(t, []roomAuthorityCall{{roomID: roomID, actorProfileID: profileID}}, rooms.calls)
	require.Equal(t, []JoinPermissionQuery{{
		SpaceID: spaceID, RoomID: roomID, ActorProfileID: profileID, Permission: VoiceJoinPermission,
	}}, permissions.calls)
}

func TestAuthorizeJoin_RejectsUnverifiedOrInvalidPrincipalBeforeAuthorityCalls(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	accountID, profileID := uuid.New(), uuid.New()
	base := validPrincipal(accountID, profileID, now.Add(time.Minute))

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{"missing principal", context.Background()},
		{"raw identity metadata", metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID.String()))},
		{"wrong kind", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.Kind = "service" }))},
		{"wrong issuer", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.Issuer = "space" }))},
		{"wrong audience", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.Audience = "role" }))},
		{"malformed account", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.AccountID = "bad" }))},
		{"empty account", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.AccountID = "" }))},
		{"nil account", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.AccountID = uuid.Nil.String() }))},
		{"malformed profile", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.ProfileID = "bad" }))},
		{"empty profile", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.ProfileID = "" }))},
		{"nil profile", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.ProfileID = uuid.Nil.String() }))},
		{"subject differs from account", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.Subject = uuid.NewString() }))},
		{"zero session epoch", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.SessionEpoch = 0 }))},
		{"zero expiry", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.ExpiresAt = time.Time{} }))},
		{"expired", contextWithPrincipal(changedPrincipal(base, func(p *principal.Principal) { p.ExpiresAt = now }))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rooms := &roomAuthorityStub{access: RoomAccess{SpaceID: uuid.NewString(), Member: true, Active: true}}
			permissions := &permissionAuthorityStub{allowed: true}
			_, err := NewAuthorizer(rooms, permissions, func() time.Time { return now }).AuthorizeJoin(tc.ctx, uuid.NewString(), uuid.NewString())
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Empty(t, rooms.calls)
			require.Empty(t, permissions.calls)
		})
	}
}

func TestAuthorizeJoin_ValidatesPathUUIDsBeforePrincipalAndDependencies(t *testing.T) {
	for _, tc := range []struct{ name, spaceID, roomID string }{
		{"bad space", "bad", uuid.NewString()},
		{"empty space", "", uuid.NewString()},
		{"nil space", uuid.Nil.String(), uuid.NewString()},
		{"bad room", uuid.NewString(), "bad"},
		{"empty room", uuid.NewString(), ""},
		{"nil room", uuid.NewString(), uuid.Nil.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthorizer(nil, nil, nil).AuthorizeJoin(context.Background(), tc.spaceID, tc.roomID)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestAuthorizeJoin_FailsClosedForMissingOrFailedAuthorities(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	ctx := verifiedContext(uuid.New(), uuid.New(), now.Add(time.Minute))
	spaceID, roomID := uuid.NewString(), uuid.NewString()

	tests := []struct {
		name         string
		rooms        RoomAuthority
		permissions  PermissionAuthority
		noPolicyCall bool
	}{
		{"missing room authority", nil, &permissionAuthorityStub{allowed: true}, true},
		{"missing permission authority", &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID, Member: true, Active: true}}, nil, true},
		{"room dependency error", &roomAuthorityStub{err: errors.New("space down")}, &permissionAuthorityStub{allowed: true}, true},
		{"room unexpected status", &roomAuthorityStub{err: status.Error(codes.PermissionDenied, "detail")}, &permissionAuthorityStub{allowed: true}, true},
		{"permission dependency error", &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID, Member: true, Active: true}}, &permissionAuthorityStub{err: errors.New("role down")}, false},
		{"permission unexpected status", &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID, Member: true, Active: true}}, &permissionAuthorityStub{err: status.Error(codes.NotFound, "detail")}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthorizer(tc.rooms, tc.permissions, func() time.Time { return now }).AuthorizeJoin(ctx, spaceID, roomID)
			require.Equal(t, codes.Unavailable, status.Code(err))
			if tc.noPolicyCall && tc.permissions != nil {
				require.Empty(t, tc.permissions.(*permissionAuthorityStub).calls)
			}
		})
	}
}

func TestAuthorizeJoin_MapsCanonicalAuthorityStatesWithoutLeakingSpace(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	ctx := verifiedContext(uuid.New(), uuid.New(), now.Add(time.Minute))
	pathSpace, canonicalSpace, roomID := uuid.NewString(), uuid.NewString(), uuid.NewString()

	tests := []struct {
		name        string
		access      RoomAccess
		err         error
		want        codes.Code
		wantMessage string
	}{
		{"room absent", RoomAccess{}, status.Error(codes.NotFound, "sensitive canonical detail"), codes.NotFound, "voice room not found"},
		{"nonmember", RoomAccess{SpaceID: pathSpace, Member: false, Active: true}, nil, codes.NotFound, "voice room not found"},
		{"space mismatch", RoomAccess{SpaceID: canonicalSpace, Member: true, Active: true}, nil, codes.NotFound, "voice room not found"},
		{"inactive known room", RoomAccess{SpaceID: pathSpace, Member: true, Active: false}, nil, codes.FailedPrecondition, ""},
		{"empty canonical space", RoomAccess{Member: true, Active: true}, nil, codes.Unavailable, ""},
		{"malformed canonical space", RoomAccess{SpaceID: "bad", Member: true, Active: true}, nil, codes.Unavailable, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			permissions := &permissionAuthorityStub{allowed: true}
			_, err := NewAuthorizer(&roomAuthorityStub{access: tc.access, err: tc.err}, permissions, func() time.Time { return now }).AuthorizeJoin(ctx, pathSpace, roomID)
			require.Equal(t, tc.want, status.Code(err))
			if tc.wantMessage != "" {
				require.Equal(t, tc.wantMessage, status.Convert(err).Message())
			}
			require.NotContains(t, status.Convert(err).Message(), "sensitive canonical detail")
			if tc.access.SpaceID != "" {
				require.NotContains(t, status.Convert(err).Message(), tc.access.SpaceID)
			}
			require.Empty(t, permissions.calls)
		})
	}
}

func TestAuthorizeJoin_DistinguishesPolicyDenialFromPolicyFailure(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	spaceID, roomID := uuid.NewString(), uuid.NewString()
	ctx := verifiedContext(uuid.New(), uuid.New(), now.Add(time.Minute))
	rooms := &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID, Member: true, Active: true}}

	_, err := NewAuthorizer(rooms, &permissionAuthorityStub{allowed: false}, func() time.Time { return now }).AuthorizeJoin(ctx, spaceID, roomID)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = NewAuthorizer(rooms, &permissionAuthorityStub{err: errors.New("role down")}, func() time.Time { return now }).AuthorizeJoin(ctx, spaceID, roomID)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestAuthorizeJoin_RechecksExpiryAfterEveryAuthorityCall(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	clockNow := now
	spaceID, roomID := uuid.NewString(), uuid.NewString()
	ctx := verifiedContext(uuid.New(), uuid.New(), now.Add(time.Second))

	t.Run("after room resolution and before permission forwarding", func(t *testing.T) {
		clockNow = now
		rooms := &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID, Member: true, Active: true}, after: func() { clockNow = now.Add(time.Second) }}
		permissions := &permissionAuthorityStub{allowed: true}
		_, err := NewAuthorizer(rooms, permissions, func() time.Time { return clockNow }).AuthorizeJoin(ctx, spaceID, roomID)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Empty(t, permissions.calls)
	})

	t.Run("after permission decision and before success", func(t *testing.T) {
		clockNow = now
		rooms := &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID, Member: true, Active: true}}
		permissions := &permissionAuthorityStub{allowed: true, after: func() { clockNow = now.Add(time.Second) }}
		_, err := NewAuthorizer(rooms, permissions, func() time.Time { return clockNow }).AuthorizeJoin(ctx, spaceID, roomID)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Len(t, permissions.calls, 1)
	})
}

func verifiedContext(accountID, profileID uuid.UUID, expiresAt time.Time) context.Context {
	return contextWithPrincipal(validPrincipal(accountID, profileID, expiresAt))
}

func contextWithPrincipal(p principal.Principal) context.Context {
	return principal.WithVerified(context.Background(), p)
}

func validPrincipal(accountID, profileID uuid.UUID, expiresAt time.Time) principal.Principal {
	return principal.Principal{
		Kind: "delegated_user", Issuer: "gateway", Subject: accountID.String(), Audience: "voice",
		AccountID: accountID.String(), ProfileID: profileID.String(), SessionEpoch: 7, ExpiresAt: expiresAt,
	}
}

func changedPrincipal(p principal.Principal, change func(*principal.Principal)) principal.Principal {
	change(&p)
	return p
}
