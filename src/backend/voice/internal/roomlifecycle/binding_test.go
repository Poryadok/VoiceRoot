package roomlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
)

func TestBindJoin_UsesActorOnlyFromSuccessfulAccessDecision(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	accountID, profileID, spaceID, roomID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	decision := authorizeBindingDecision(t, validPrincipal(accountID, profileID, now.Add(time.Minute)), spaceID, roomID, now)

	binding, err := BindJoin(decision, operationID.String())
	require.NoError(t, err)
	require.Equal(t, profileID.String(), binding.ActorProfileID())
	require.Equal(t, operationID.String(), binding.OperationID())
	require.Equal(t, JoinMethod, binding.Method())
	require.Equal(t, expectedJoinFingerprint(spaceID, roomID), binding.Fingerprint())

	_, err = BindJoin(JoinDecision{}, uuid.NewString())
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestBindJoin_NormalizesUUIDSpelling(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	accountID, profileID, spaceID, roomID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	decision := authorizeBindingDecision(t, validPrincipal(accountID, profileID, now.Add(time.Minute)), spaceID, roomID, now)

	canonical, err := BindJoin(decision, operationID.String())
	require.NoError(t, err)
	upper, err := BindJoin(decision, bytesToUpperUUID(operationID.String()))
	require.NoError(t, err)
	require.Equal(t, canonical.OperationID(), upper.OperationID())
	require.Equal(t, canonical.Fingerprint(), upper.Fingerprint())
}

func TestBindJoin_RejectsInvalidOperationUUID(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	spaceID, roomID := uuid.New(), uuid.New()
	decision := authorizeBindingDecision(t, validPrincipal(uuid.New(), uuid.New(), now.Add(time.Minute)), spaceID, roomID, now)

	for _, operationID := range []string{"", "not-a-uuid", uuid.Nil.String()} {
		_, err := BindJoin(decision, operationID)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
}

func TestBindJoin_BindsMethodAndCanonicalJoinPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	actor := validPrincipal(uuid.New(), uuid.New(), now.Add(time.Minute))
	spaceA, spaceB, roomA, roomB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	operationID := uuid.NewString()

	base, err := BindJoin(authorizeBindingDecision(t, actor, spaceA, roomA, now), operationID)
	require.NoError(t, err)
	changedSpace, err := BindJoin(authorizeBindingDecision(t, actor, spaceB, roomA, now), operationID)
	require.NoError(t, err)
	changedRoom, err := BindJoin(authorizeBindingDecision(t, actor, spaceA, roomB, now), operationID)
	require.NoError(t, err)

	require.NotEqual(t, base.Fingerprint(), changedSpace.Fingerprint())
	require.NotEqual(t, base.Fingerprint(), changedRoom.Fingerprint())
	require.Equal(t, JoinMethod, base.Method())
	require.NotEmpty(t, base.Fingerprint())
}

func TestBindJoin_IgnoresTransportOnlyPrincipalFields(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	accountID, profileID, spaceID, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	firstPrincipal := validPrincipal(accountID, profileID, now.Add(time.Minute))
	firstPrincipal.RPC, firstPrincipal.RequestID, firstPrincipal.RequestHash, firstPrincipal.JWTID = "/rpc/one", "request-1", "sha256:one", "jwt-1"
	firstPrincipal.IssuedAt = now.Add(-time.Second)
	secondPrincipal := firstPrincipal
	secondPrincipal.RPC, secondPrincipal.RequestID, secondPrincipal.RequestHash, secondPrincipal.JWTID = "/rpc/two", "request-2", "sha256:two", "jwt-2"
	secondPrincipal.IssuedAt = now.Add(-time.Minute)
	secondPrincipal.ExpiresAt = now.Add(2 * time.Minute)
	secondPrincipal.SessionEpoch = 99
	operationID := uuid.NewString()

	first, err := BindJoin(authorizeBindingDecision(t, firstPrincipal, spaceID, roomID, now), operationID)
	require.NoError(t, err)
	second, err := BindJoin(authorizeBindingDecision(t, secondPrincipal, spaceID, roomID, now), operationID)
	require.NoError(t, err)
	require.Equal(t, first.Fingerprint(), second.Fingerprint())
	require.Equal(t, first.ActorProfileID(), second.ActorProfileID())
	require.Equal(t, first.OperationID(), second.OperationID())
	require.Equal(t, first.Method(), second.Method())
}

func authorizeBindingDecision(t *testing.T, actor principal.Principal, spaceID, roomID uuid.UUID, now time.Time) JoinDecision {
	t.Helper()
	rooms := &roomAuthorityStub{access: RoomAccess{SpaceID: spaceID.String(), Member: true, Active: true}}
	permissions := &permissionAuthorityStub{allowed: true}
	decision, err := NewAuthorizer(rooms, permissions, func() time.Time { return now }).AuthorizeJoin(
		principal.WithVerified(context.Background(), actor), spaceID.String(), roomID.String(),
	)
	require.NoError(t, err)
	return decision
}

func expectedJoinFingerprint(spaceID, roomID uuid.UUID) string {
	var canonical bytes.Buffer
	for _, field := range []string{"roomlifecycle/v1", "JOIN", spaceID.String(), roomID.String()} {
		requireUint32 := uint32(len(field))
		_ = binary.Write(&canonical, binary.BigEndian, requireUint32)
		_, _ = canonical.WriteString(field)
	}
	digest := sha256.Sum256(canonical.Bytes())
	return "sha256:" + hex.EncodeToString(digest[:])
}

func bytesToUpperUUID(value string) string {
	result := []byte(value)
	for i, b := range result {
		if b >= 'a' && b <= 'f' {
			result[i] = b - ('a' - 'A')
		}
	}
	return string(result)
}
