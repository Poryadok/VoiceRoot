package catalogassets

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

type fileServerFixture struct {
	filev1.UnimplementedCatalogAssetServiceServer
	key         *rsa.PublicKey
	ids         []string
	state       FileState
	operation   string
	destination *chatv1.ChatRef
	bad         string
}

func TestCatalogAssetTerminalProofIsNarrow(t *testing.T) {
	b := Binding{Actor: Actor{AccountID: uuid.New(), ProfileID: uuid.New()}, ActorCapture: ActorCapture{AccountGeneration: uuid.New(), ProfileGeneration: uuid.New()}, Selection: Selection{Provider: "fixture", ProviderID: "one"}, SourceID: uuid.New(), FileID: uuid.New(), SelectorSlotID: uuid.New(), SelectionRevision: 1}
	base := &filev1.GetCatalogAssetStatusResponse{SourceId: b.SourceID.String(), ActorAccountId: b.AccountID.String(), ActorProfileId: b.ProfileID.String(), Provider: b.Provider, ProviderId: b.ProviderID, FileId: b.FileID.String(), Status: "deleted", TerminalFenceId: uuid.NewString()}
	got, err := parseAssetFileState(base, b)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, got.TerminalFenceID)
	require.Equal(t, ActorCapture{}, got.ActorCapture)
	require.Empty(t, base.ChatAccountGeneration, "parsing must not mutate the authenticated response")
	for _, change := range []func(*filev1.GetCatalogAssetStatusResponse){
		func(r *filev1.GetCatalogAssetStatusResponse) { r.Status = "ready" },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.Status = "processing" },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.TerminalFenceId = "" },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.TerminalFenceId = "bad" },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.ChatAccountGeneration = b.AccountGeneration.String() },
		func(r *filev1.GetCatalogAssetStatusResponse) {
			r.ChatAccountGeneration = b.AccountGeneration.String()
			r.ChatProfileGeneration = b.ProfileGeneration.String()
			r.SelectionRevision = 1
			r.CurrentSelectionRevision = 1
		},
		func(r *filev1.GetCatalogAssetStatusResponse) { r.ActorProfileId = uuid.NewString() },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.SourceId = uuid.NewString() },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.ProviderId = "other" },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.FileId = uuid.NewString() },
		func(r *filev1.GetCatalogAssetStatusResponse) { r.OperationId = uuid.NewString() },
		func(r *filev1.GetCatalogAssetStatusResponse) {
			r.DestinationChat = &chatv1.ChatRef{Id: uuid.NewString()}
		},
		func(r *filev1.GetCatalogAssetStatusResponse) { r.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01}) },
	} {
		r := proto.Clone(base).(*filev1.GetCatalogAssetStatusResponse)
		change(r)
		_, err := parseAssetFileState(r, b)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	_, err = parseFileState(base, b.Actor, b.Selection)
	require.Equal(t, codes.Unavailable, status.Code(err), "operation/import cannot accept erased generations")
}

func (s *fileServerFixture) check(ctx context.Context, req proto.Message, method string) error {
	versioned, ok := req.(interface{ GetProtocolVersion() uint32 })
	if !ok || versioned.GetProtocolVersion() != 2 {
		return status.Error(codes.InvalidArgument, "ordered protocol2 required")
	}
	md, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return err
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return err
	}
	p, err := principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "file", ExpectedRPC: method, ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return s.key, nil }})
	if err != nil {
		return err
	}
	if p.AccountID != "" || p.ProfileID != "" {
		return status.Error(codes.PermissionDenied, "mixed actor")
	}
	s.ids = append(s.ids, md.RequestID)
	return nil
}
func (s *fileServerFixture) ImportCatalogAsset(ctx context.Context, r *filev1.ImportCatalogAssetRequest) (*filev1.ImportCatalogAssetResponse, error) {
	if err := s.check(ctx, r, filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName); err != nil {
		return nil, err
	}
	s.operation = r.OperationId
	s.destination = r.DestinationChat
	s.state = FileState{Actor: Actor{AccountID: uuid.MustParse(r.ActorAccountId), ProfileID: uuid.MustParse(r.ActorProfileId)}, ActorCapture: ActorCapture{AccountGeneration: uuid.MustParse(r.ChatAccountGeneration), ProfileGeneration: uuid.MustParse(r.ChatProfileGeneration)}, Selection: Selection{Provider: r.Provider, ProviderID: r.ProviderId}, SourceID: uuid.MustParse(r.SourceId), FileID: uuid.New(), Status: "processing", SelectionRevision: 1, CurrentSelectionRevision: 1}
	response := &filev1.ImportCatalogAssetResponse{SourceId: r.SourceId, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, Provider: r.Provider, ProviderId: r.ProviderId, FileId: s.state.FileID.String(), Status: s.state.Status, OperationId: r.OperationId, DestinationChat: r.DestinationChat, ChatAccountGeneration: r.ChatAccountGeneration, ChatProfileGeneration: r.ChatProfileGeneration}
	if s.bad == "actor" {
		response.ActorProfileId = uuid.NewString()
	}
	response.SelectionRevision = s.state.SelectionRevision
	response.CurrentSelectionRevision = s.state.CurrentSelectionRevision
	if s.bad == "zero revision" {
		response.SelectionRevision = 0
	}
	if s.bad == "zero head" {
		response.CurrentSelectionRevision = 0
	}
	if s.bad == "future revision" {
		response.SelectionRevision = 2
		response.CurrentSelectionRevision = 1
	}
	if s.bad == "operation" {
		response.OperationId = uuid.NewString()
	}
	if s.bad == "status" {
		response.Status = "approved"
	}
	if s.bad == "generation" {
		response.ChatProfileGeneration = uuid.NewString()
	}
	if s.bad == "destination" {
		response.DestinationChat = &chatv1.ChatRef{Id: uuid.NewString()}
	}
	if s.bad == "error" {
		return nil, status.Error(codes.ResourceExhausted, "signed-source-url-secret")
	}
	return response, nil
}
func (s *fileServerFixture) GetCatalogAssetStatus(ctx context.Context, r *filev1.GetCatalogAssetStatusRequest) (*filev1.GetCatalogAssetStatusResponse, error) {
	if err := s.check(ctx, r, filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName); err != nil {
		return nil, err
	}
	response := &filev1.GetCatalogAssetStatusResponse{SourceId: s.state.SourceID.String(), ActorAccountId: s.state.AccountID.String(), ActorProfileId: s.state.ProfileID.String(), Provider: s.state.Provider, ProviderId: s.state.ProviderID, FileId: s.state.FileID.String(), Status: "ready", ChatAccountGeneration: s.state.AccountGeneration.String(), ChatProfileGeneration: s.state.ProfileGeneration.String()}
	response.SelectionRevision = s.state.SelectionRevision
	response.CurrentSelectionRevision = s.state.CurrentSelectionRevision
	if operation := r.GetOperation(); operation != nil {
		response.OperationId = operation.OperationId
		response.DestinationChat = operation.DestinationChat
	}
	if s.bad == "source" {
		response.SourceId = uuid.NewString()
	}
	return response, nil
}

func TestCatalogFileClientUsesFreshPureBoundCredentialAndChecksEcho(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, _ := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "one", PrivateKey: key})
	fixture := &fileServerFixture{key: &key.PublicKey}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	filev1.RegisterCatalogAssetServiceServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///catalog-file", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := &GRPCFileClient{Client: filev1.NewCatalogAssetServiceClient(conn), Issuer: issuer}
	req := ImportRequest{Actor: Actor{AccountID: uuid.New(), ProfileID: uuid.New()}, ActorCapture: ActorCapture{AccountGeneration: uuid.New(), ProfileGeneration: uuid.New()}, Selection: Selection{Provider: "fixture", ProviderID: "one"}, OperationID: uuid.New(), SourceID: uuid.New(), SourceURL: "https://cdn.example.test/signed.gif?secret=value", Destination: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_DM.Enum()}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-voice-profile-id", "forged", "authorization", "Bearer wrong"))
	state, err := client.Import(ctx, req)
	if err != nil || state.Status != "processing" || state.Actor != req.Actor || state.SourceID != req.SourceID {
		t.Fatalf("import=%+v err=%v", state, err)
	}
	recovered, err := client.StatusByOperation(ctx, OperationSelector{Actor: req.Actor, Selection: req.Selection, OperationID: req.OperationID, Destination: req.Destination})
	if err != nil || recovered.FileID != state.FileID || recovered.Status != "ready" {
		t.Fatalf("status=%+v err=%v", recovered, err)
	}
	bound := Binding{Actor: req.Actor, ActorCapture: req.ActorCapture, Selection: req.Selection, SourceID: req.SourceID, FileID: state.FileID, SelectorSlotID: uuid.New(), SelectionRevision: state.SelectionRevision}
	if _, err := client.Status(ctx, bound); err != nil {
		t.Fatal(err)
	}
	if len(fixture.ids) != 3 || fixture.ids[0] == fixture.ids[1] || fixture.ids[1] == fixture.ids[2] || fixture.ids[0] == req.OperationID.String() {
		t.Fatalf("transport IDs=%v", fixture.ids)
	}
	for _, bad := range []string{"actor", "operation", "status", "destination", "generation", "zero revision", "zero head", "future revision", "error"} {
		fixture.bad = bad
		_, err := client.Import(ctx, req)
		want := codes.Unavailable
		if bad == "error" {
			want = codes.ResourceExhausted
		}
		if status.Code(err) != want || strings.Contains(status.Convert(err).Message(), "secret") {
			t.Fatalf("%s err=%v", bad, err)
		}
	}
	fixture.bad = "source"
	fixture.state = state
	if _, err := client.Status(ctx, bound); status.Code(err) != codes.Unavailable {
		t.Fatalf("wrong source=%v", err)
	}
}
