package recoveryread

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

type fakeStore struct {
	calls        int
	actor, space uuid.UUID
	result       *spacev1.Space
	err          error
}

func (f *fakeStore) GetLifecycleRecoverySpace(_ context.Context, space, actor uuid.UUID) (*spacev1.Space, error) {
	f.calls++
	f.space = space
	f.actor = actor
	return f.result, f.err
}

func fixture(t *testing.T) (Reader, *principal.Issuer, *fakeStore, principal.DelegatedUserInput, *spacev1.GetSpaceRequest) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "key", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	st := &fakeStore{}
	reader := Reader{Store: st, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }, Clock: func() time.Time { return now }}
	req := &spacev1.GetSpaceRequest{SpaceId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	input := principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_GetSpace_FullMethodName, RequestID: "recovery-read", RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)}
	return reader, issuer, st, input, req
}
func incoming(t *testing.T, issuer *principal.Issuer, input principal.DelegatedUserInput) context.Context {
	t.Helper()
	token, err := issuer.IssueDelegatedUser(input)
	require.NoError(t, err)
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "recovery-read"))
}

func TestReader_UsesAuthenticatedProfileAndMinimalProjection(t *testing.T) {
	r, issuer, st, input, req := fixture(t)
	st.result = &spacev1.Space{Id: req.SpaceId, Name: "Frozen"}
	response, err := r.GetSpace(incoming(t, issuer, input), req)
	require.NoError(t, err)
	require.Equal(t, st.result, response.GetSpace())
	require.Equal(t, uuid.MustParse(input.ProfileID), st.actor)
	require.Equal(t, uuid.MustParse(req.SpaceId), st.space)
}
func TestReader_RejectsCredentialsBeforeStore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*principal.DelegatedUserInput)
	}{
		{"audience", func(i *principal.DelegatedUserInput) { i.Audience = "role" }},
		{"rpc", func(i *principal.DelegatedUserInput) { i.RPC = spacev1.SpaceService_RestoreSpace_FullMethodName }},
		{"request id", func(i *principal.DelegatedUserInput) { i.RequestID = "other" }},
		{"request hash", func(i *principal.DelegatedUserInput) {
			i.RequestHash, _ = principal.RequestHash(&spacev1.GetSpaceRequest{SpaceId: uuid.NewString()})
		}},
		{"account", func(i *principal.DelegatedUserInput) { i.AccountID = uuid.Nil.String() }},
		{"profile", func(i *principal.DelegatedUserInput) { i.ProfileID = "bad" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, issuer, st, input, req := fixture(t)
			tc.change(&input)
			response, err := r.GetSpace(incoming(t, issuer, input), req)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.Nil(t, response)
			require.Zero(t, st.calls)
		})
	}
}
func TestReader_RawIdentityAndGenericContextCannotAuthorize(t *testing.T) {
	r, _, st, input, req := fixture(t)
	for _, ctx := range []context.Context{context.Background(), metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", input.ProfileID)), principal.WithVerified(context.Background(), principal.Principal{AccountID: input.AccountID, ProfileID: input.ProfileID, SessionEpoch: 7})} {
		response, err := r.GetSpace(ctx, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, response)
	}
	require.Zero(t, st.calls)
}
func TestReader_MissingOrUnavailableSecurityDependenciesDeny(t *testing.T) {
	for _, change := range []func(*Reader){func(r *Reader) { r.KeyResolver = nil }, func(r *Reader) { r.ReplayGuard = nil }, func(r *Reader) { r.SessionEpochChecker = nil }, func(r *Reader) {
		r.ReplayGuard = func(context.Context, string, string, time.Time) error { return errors.New("replay") }
	}, func(r *Reader) {
		r.SessionEpochChecker = func(context.Context, string, int64) error { return errors.New("revoked") }
	}} {
		r, issuer, st, input, req := fixture(t)
		change(&r)
		response, err := r.GetSpace(incoming(t, issuer, input), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, response)
		require.Zero(t, st.calls)
	}
}
func TestReader_SafeDisclosureErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{pgx.ErrNoRows, codes.NotFound}, {store.ErrLifecycleStateTransition, codes.FailedPrecondition}, {errors.New("database password or internal details"), codes.Unavailable}} {
		r, issuer, st, input, req := fixture(t)
		st.err = tc.err
		response, err := r.GetSpace(incoming(t, issuer, input), req)
		require.Equal(t, tc.code, status.Code(err))
		require.Nil(t, response)
		require.NotContains(t, err.Error(), "password")
	}
}
func TestReader_RejectsUnexpectedStoreProjection(t *testing.T) {
	for _, value := range []*spacev1.Space{nil, {Id: uuid.NewString(), Name: "foreign"}, {Name: "missing id"}, {Id: "valid", OwnerProfileId: uuid.NewString()}} {
		r, issuer, st, input, req := fixture(t)
		st.result = value
		response, err := r.GetSpace(incoming(t, issuer, input), req)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Nil(t, response)
	}
}

func TestReader_InvalidRequestAndServiceCredentialDoNotRead(t *testing.T) {
	for _, req := range []*spacev1.GetSpaceRequest{nil, {}, {SpaceId: "invalid"}, {SpaceId: uuid.Nil.String()}, {SpaceId: "{11111111-1111-1111-1111-111111111111}"}} {
		r, issuer, st, input, _ := fixture(t)
		response, err := r.GetSpace(incoming(t, issuer, input), req)
		require.Error(t, err)
		require.Nil(t, response)
		require.Zero(t, st.calls)
	}
	r, issuer, st, input, req := fixture(t)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: input.Audience, RPC: input.RPC, RequestID: input.RequestID, RequestHash: input.RequestHash})
	require.NoError(t, err)
	response, err := r.GetSpace(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Nil(t, response)
	require.Zero(t, st.calls)
}

func TestReader_RejectsDuplicateAndMixedRawMetadata(t *testing.T) {
	r, issuer, st, input, req := fixture(t)
	token, err := issuer.IssueDelegatedUser(input)
	require.NoError(t, err)
	for _, pairs := range [][]string{
		{"authorization", "Bearer " + token, "authorization", "Bearer " + token, "x-request-id", input.RequestID},
		{"authorization", "Bearer " + token, "x-request-id", input.RequestID, "x-request-id", input.RequestID},
		{"authorization", "Bearer " + token, "x-request-id", input.RequestID, "x-voice-profile-id", input.ProfileID},
	} {
		response, err := r.GetSpace(metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...)), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, response)
		require.Zero(t, st.calls)
	}
}

func TestReader_ExpiredUnknownKeyAndEpochOutageDeny(t *testing.T) {
	for _, change := range []func(*Reader){
		func(r *Reader) { now := r.Clock(); r.Clock = func() time.Time { return now.Add(time.Hour) } },
		func(r *Reader) {
			r.KeyResolver = func(context.Context, string, string) (*rsa.PublicKey, error) { return nil, errors.New("unknown kid") }
		},
		func(r *Reader) {
			r.SessionEpochChecker = func(context.Context, string, int64) error { return context.DeadlineExceeded }
		},
	} {
		r, issuer, st, input, req := fixture(t)
		change(&r)
		response, err := r.GetSpace(incoming(t, issuer, input), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, response)
		require.Zero(t, st.calls)
	}
}

func TestReader_ProjectionAllowlistAndTimestampPair(t *testing.T) {
	for _, mutate := range []func(*spacev1.Space){
		func(s *spacev1.Space) { s.Description = "private description" },
		func(s *spacev1.Space) { s.OwnerProfileId = uuid.NewString() },
		func(s *spacev1.Space) { s.ProtoReflect().SetUnknown([]byte{0xF8, 0x07, 0x01}) },
		func(s *spacev1.Space) { s.DeletionScheduledAt = timestamppb.Now() },
		func(s *spacev1.Space) { s.DeletionScheduledAt = timestamppb.Now(); s.PurgeAfter = timestamppb.Now() },
	} {
		r, issuer, st, input, req := fixture(t)
		st.result = &spacev1.Space{Id: req.SpaceId, Name: "private"}
		mutate(st.result)
		response, err := r.GetSpace(incoming(t, issuer, input), req)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Nil(t, response)
	}
}
