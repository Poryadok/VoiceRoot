package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
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
	"google.golang.org/protobuf/proto"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/evidence"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

func TestAutoModLifecycleRPC_AuthOnlyPrivateRepairAndCommittedReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000013_account_lifecycle_receiver.up.sql"))
	s := automod.NewStore(pool)
	gate := automod.NewBootGate()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	verifier := &socialprincipal.Verifier{Target: "moderation", Capability: "automod-evidence", Issuers: map[string]bool{"auth": true, "messaging": true}, Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	server := grpc.NewServer(grpc.UnaryInterceptor(socialprincipal.StrictUnaryInterceptor(verifier)))
	automodv1.RegisterAccountLifecycleReceiverServiceServer(server, &evidence.LifecycleService{Store: s, Gate: gate})
	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	connection, err := grpc.NewClient("passthrough:///lifecycle-private-test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	client := automodv1.NewAccountLifecycleReceiverServiceClient(connection)
	generation := uuid.New()
	request := &automodv1.ApplyLifecycleFactRequest{Fact: &automodv1.AccountLifecycleFact{ProtocolVersion: 1, AuthorityGeneration: generation.String(), EventId: uuid.NewString(), SubjectId: uuid.NewString(), State: automodv1.SubjectState_SUBJECT_STATE_LIVE, LifecycleRevision: 1}}
	signed := func(caller string, request proto.Message) context.Context {
		issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: caller, KeyID: "current", PrivateKey: key})
		require.NoError(t, err)
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "moderation", RPC: automodv1.AccountLifecycleReceiverService_ApplyLifecycleFact_FullMethodName, RequestID: id, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	_, err = client.ApplyLifecycleFact(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = client.ApplyLifecycleFact(signed("messaging", request), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.ApplyLifecycleFact(signed("auth", request), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	boot, bootGeneration := gate.Identity()
	for phase := automod.BootLocalIssuer; phase <= automod.BootTerminalImported; phase++ {
		require.NoError(t, gate.Advance(automod.BootReceipt{BootID: boot, Generation: bootGeneration, Phase: phase, ProofID: uuid.New()}))
	}
	check, err := gate.AcquirePrivateRepair(ctx)
	require.NoError(t, err)
	// An Auth RPC cannot bootstrap its own authority from the incoming UUID.
	_, err = client.ApplyLifecycleFact(signed("auth", request), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NoError(t, s.InstallLifecycleAuthority(ctx, generation, check))
	response, err := client.ApplyLifecycleFact(signed("auth", request), request)
	require.NoError(t, err)
	require.EqualValues(t, 1, response.GetLifecycleRevision())
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_lifecycle_current`).Scan(&count))
	require.Equal(t, 1, count)
	tampered := proto.Clone(request).(*automodv1.ApplyLifecycleFactRequest)
	tampered.Fact.SubjectId = uuid.NewString()
	_, err = client.ApplyLifecycleFact(signed("auth", request), tampered)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	gate.Close()
	_, err = client.ApplyLifecycleFact(signed("auth", tampered), tampered)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_lifecycle_current`).Scan(&count))
	require.Equal(t, 1, count)
}
