package authoritysource

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/principal"
)

type fixtureReader struct {
	state State
	err   error
	calls int
	delay time.Duration
}

func (reader *fixtureReader) ReadAuthoritySnapshot(_ context.Context, _ *authorityv1.SourceScope) (State, error) {
	reader.calls++
	time.Sleep(reader.delay)
	return reader.state, reader.err
}
func (reader *fixtureReader) ReadAuthorityRevision(_ context.Context, _ *authorityv1.SourceScope) (uint64, error) {
	reader.calls++
	return reader.state.Revision, reader.err
}

func fixtureScope() *authorityv1.SourceScope {
	return &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: uuid.NewString()}
}
func fixtureVerified(t *testing.T, request proto.Message, rpc string, expires time.Time) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "federation", Subject: "service:federation", Audience: "space", RPC: rpc, RequestHash: hash, RequestID: uuid.NewString(), ExpiresAt: expires})
}

func TestSourceDomainRejectsIncompleteReaderState(t *testing.T) {
	reader := &fixtureReader{state: State{Revision: 1, CanonicalState: []byte(`{"schema_version":1}`)}}
	server, err := NewServer(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, reader)
	require.NoError(t, err)
	request := &authorityv1.ReadSnapshotRequest{Scope: fixtureScope()}
	response, err := server.ReadSnapshot(fixtureVerified(t, request, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, time.Now().Add(time.Minute)), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "partial owner state must never become a complete snapshot")
	require.Nil(t, response)
}

func TestSourceDomainRechecksPrincipalAfterBlockingRead(t *testing.T) {
	reader := &fixtureReader{state: State{Complete: true, Revision: 1, CanonicalState: []byte(`{"schema_version":1}`)}, delay: 60 * time.Millisecond}
	server, err := NewServer(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, reader)
	require.NoError(t, err)
	request := &authorityv1.ReadSnapshotRequest{Scope: fixtureScope()}
	response, err := server.ReadSnapshot(fixtureVerified(t, request, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, time.Now().Add(30*time.Millisecond)), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "expired source credential must not receive completed protected data")
	require.Nil(t, response)
	require.Equal(t, 1, reader.calls)
}

func TestSourceDomainRejectsUnverifiedWrongScopeAndUnavailableReader(t *testing.T) {
	reader := &fixtureReader{state: State{Complete: true, Revision: 1, CanonicalState: []byte(`{"schema_version":1}`)}}
	server, err := NewServer(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, reader)
	require.NoError(t, err)
	request := &authorityv1.ReadSnapshotRequest{Scope: fixtureScope()}
	_, err = server.ReadSnapshot(context.Background(), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, reader.calls)
	for _, mutation := range []func(*authorityv1.SourceScope){
		func(scope *authorityv1.SourceScope) { scope.SchemaVersion = 2 },
		func(scope *authorityv1.SourceScope) { scope.ProfileIds = []string{uuid.NewString()} },
		func(scope *authorityv1.SourceScope) { scope.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01}) },
	} {
		changed := proto.Clone(request).(*authorityv1.ReadSnapshotRequest)
		mutation(changed.Scope)
		_, err = server.ReadSnapshot(fixtureVerified(t, changed, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, time.Now().Add(time.Minute)), changed)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Zero(t, reader.calls)
	}
	reader.err = errors.New("private storage diagnostic")
	_, err = server.ReadSnapshot(fixtureVerified(t, request, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName, time.Now().Add(time.Minute)), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NotContains(t, err.Error(), "private storage")
}

func TestSourceSignedRequestBindingReplayAndRawMetadataBeforeHandler(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "federation", KeyID: "source-fixture", PrivateKey: key})
	require.NoError(t, err)
	var lock sync.Mutex
	used := map[string]bool{}
	replay := func(_ context.Context, issuer, jti string, _ time.Time) error {
		lock.Lock()
		defer lock.Unlock()
		if used[issuer+jti] {
			return errors.New("replay")
		}
		used[issuer+jti] = true
		return nil
	}
	interceptor, err := UnaryInterceptor(authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, replay)
	require.NoError(t, err)
	request := &authorityv1.ReadSnapshotRequest{Scope: fixtureScope()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	info := &grpc.UnaryServerInfo{FullMethod: authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName}
	calls := 0
	handler := func(context.Context, any) (any, error) { calls++; return "verified", nil }
	for _, name := range []string{"scope tamper", "wrong audience", "wrong RPC", "duplicate authorization", "raw identity", "success replay"} {
		t.Run(name, func(t *testing.T) {
			input := principal.ServiceInput{Audience: "space", RPC: info.FullMethod, RequestID: uuid.NewString(), RequestHash: hash}
			changed := proto.Clone(request).(*authorityv1.ReadSnapshotRequest)
			if name == "wrong audience" {
				input.Audience = "role"
			}
			if name == "wrong RPC" {
				input.RPC = authorityv1.AuthoritySourceService_ReadRevision_FullMethodName
			}
			token, err := issuer.IssueService(input)
			require.NoError(t, err)
			values := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", input.RequestID)
			if name == "scope tamper" {
				changed.Scope.SpaceId = uuid.NewString()
			}
			if name == "duplicate authorization" {
				values.Append("authorization", "Bearer "+token)
			}
			if name == "raw identity" {
				values.Append("x-internal-caller", "federation")
			}
			ctx := metadata.NewIncomingContext(context.Background(), values)
			before := calls
			_, err = interceptor(ctx, changed, info, handler)
			if name == "success replay" {
				require.NoError(t, err)
				require.Equal(t, before+1, calls)
				_, err = interceptor(ctx, changed, info, handler)
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				require.Equal(t, before+1, calls)
			} else {
				require.Equal(t, codes.Unauthenticated, status.Code(err))
				require.Equal(t, before, calls)
			}
		})
	}
}

func TestSourceScopeShapeAndCanonicalSubjectSets(t *testing.T) {
	scope := fixtureScope()
	scope.ProfileIds = []string{uuid.NewString()}
	require.NoError(t, ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER, scope))
	scope.ProfileIds = append(scope.ProfileIds, scope.ProfileIds[0])
	require.Error(t, ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER, scope))
	scope = fixtureScope()
	scope.AccountIds = []string{uuid.NewString()}
	require.NoError(t, ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_AUTH, scope))
	scope.ProfileIds = []string{uuid.NewString()}
	require.Error(t, ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_AUTH, scope))
	require.Error(t, ValidateScope(authorityv1.AuthorityOwner_AUTHORITY_OWNER_UNSPECIFIED, fixtureScope()))
}
