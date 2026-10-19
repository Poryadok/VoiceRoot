package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

// This exercises the actual public receiver, RS256 verifier and durable replay
// store. bufconn is transport-only: mTLS/JWKS and the Java Auth implementation
// are independently tested boundaries, not an assembled-runtime claim here.
func TestNativeCreatorPublicReceiverPublishesOnceAndRejectsRawMetadata(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	for _, name := range []string{"000015_audit_ledger.up.sql", "000018_ownership_consume_started.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql", "000024_authority_revision.up.sql", "000028_chat_creation_outcomes.up.sql", "000030_entry_policy.up.sql", "000033_membership_authority.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err, name)
	}
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(33,false)`)
	require.NoError(t, err)
	st := &store.SpaceStore{Pool: pool}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	floorCalls := 0
	v := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, CreatorEpoch: func(context.Context, string, int64) error { floorCalls++; return nil }, CreatorReplay: func(ctx context.Context, issuer, jti string, expiry time.Time) error {
		err := st.ClaimCreatorPrincipalReplay(ctx, issuer, jti, expiry)
		if errors.Is(err, store.ErrMemberAuthorityUnavailable) {
			return lifecycleprincipal.ErrDependencyUnavailable
		}
		return err
	}}
	events := &spySpaceEvents{}
	svc := &SpaceGRPC{Store: st, SpaceEvents: events}
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(v.Unary()))
	spacev1.RegisterSpaceServiceServer(server, &PublicLifecycleSpace{SpaceGRPC: svc})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///native-creator", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(); _ = lis.Close() })
	client := spacev1.NewSpaceServiceClient(conn)
	request := &spacev1.CreateSpaceRequest{Name: "Native receiver"}
	account, profile := uuid.New(), uuid.New()
	_, err = client.CreateSpace(withAccountProfileCtx(ctx, account, profile), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	signed := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	created, err := client.CreateSpace(signed, request)
	require.NoError(t, err)
	require.Equal(t, profile.String(), created.GetSpace().GetOwnerProfileId())
	require.EqualValues(t, 1, created.GetSpace().GetMemberCount())
	require.Equal(t, "private", created.GetSpace().GetVisibility())
	require.Len(t, events.snapshot(), 1)
	require.Equal(t, 2, floorCalls, "ingress and final pre-write observations")
	_, err = client.CreateSpace(signed, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Len(t, events.snapshot(), 1, "credential replay never emits a second event")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM spaces`).Scan(&count))
	require.Equal(t, 1, count)
	observation, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: created.GetSpace().GetId(), ProfileId: profile.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, observation.GetOrdinary().GetPublicationKind())
	require.Empty(t, observation.GetOrdinary().GetRoleReceiptSha256())
}
