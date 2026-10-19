package principalruntime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestRuntimePublicDelegatedActor(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.DelegatedUserEnabled = true
	f.config.JWKSURLs["gateway"] = f.config.JWKSURLs["space"]
	account, profile := uuid.NewString(), uuid.NewString()
	var calls atomic.Int64
	var floor atomic.Int64
	floor.Store(7)
	runtime, err := NewWithSessionEpochChecker(context.Background(), f.config, func(_ context.Context, observed string, epoch int64) error {
		calls.Add(1)
		if observed != account || epoch < floor.Load() {
			return errors.New("stale session")
		}
		return nil
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: f.key})
	require.NoError(t, err)
	method := rolev1.RoleService_ListRoles_FullMethodName
	requestID := uuid.NewString()
	token, err := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "role", RPC: method, RequestID: requestID, RequestHash: runtimeHash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	verified, err := runtime.Verify(context.Background(), token, method, requestID, runtimeHash)
	require.NoError(t, err, "protected public receiver must accept a fresh exact Gateway delegated actor")
	require.Equal(t, profile, verified.ProfileID)
	require.Equal(t, account, verified.AccountID)
	require.EqualValues(t, 1, calls.Load())
	floor.Store(8)
	newID := uuid.NewString()
	stale, err := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "role", RPC: method, RequestID: newID, RequestHash: runtimeHash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	_, err = runtime.Verify(context.Background(), stale, method, newID, runtimeHash)
	require.Error(t, err, "fresh transport retry must check the current session floor again")
	require.EqualValues(t, 2, calls.Load())
}

func TestRuntimeSpaceReadDoesNotBorrowGatewayActivation(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	runtime, err := New(context.Background(), f.config)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, runtime.Close()) })
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: f.key})
	require.NoError(t, err)
	method := rolev1.RoleService_GetMemberRoles_FullMethodName
	id := uuid.NewString()
	token, err := signer.IssueService(principal.ServiceInput{Audience: "role", RPC: method, RequestID: id, RequestHash: runtimeHash})
	require.NoError(t, err)
	p, err := runtime.Verify(context.Background(), token, method, id, runtimeHash)
	require.NoError(t, err)
	require.Equal(t, "service:space", p.Subject)
}
