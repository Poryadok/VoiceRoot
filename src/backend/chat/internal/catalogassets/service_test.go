package catalogassets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type fakeRepository struct {
	bindings map[uuid.UUID]Binding
	saves    int
	capture  ActorCapture
	closed   bool
}

func (s *fakeRepository) CaptureActor(_ context.Context, _ Actor, initialize bool) (ActorCapture, error) {
	if s.closed {
		return ActorCapture{}, status.Error(codes.FailedPrecondition, "actor retired")
	}
	if s.capture.AccountGeneration == uuid.Nil {
		if !initialize {
			return ActorCapture{}, status.Error(codes.NotFound, "actor unavailable")
		}
		s.capture = ActorCapture{AccountGeneration: uuid.New(), ProfileGeneration: uuid.New()}
	}
	return s.capture, nil
}
func (s *fakeRepository) SaveBinding(_ context.Context, b Binding, capture ActorCapture) error {
	if s.closed || capture != s.capture || b.ActorCapture != capture {
		return status.Error(codes.FailedPrecondition, "actor retired")
	}
	s.saves++
	s.bindings[b.FileID] = b
	return nil
}
func (s *fakeRepository) RetireBinding(_ context.Context, actor Actor, file uuid.UUID) error {
	if b, ok := s.bindings[file]; ok && b.Actor == actor {
		delete(s.bindings, file)
	}
	return nil
}
func (s *fakeRepository) FindBinding(_ context.Context, actor Actor, sel Selection, file uuid.UUID, capture ActorCapture) (Binding, bool, error) {
	if s.closed || capture != s.capture {
		return Binding{}, false, status.Error(codes.FailedPrecondition, "actor retired")
	}
	for _, b := range s.bindings {
		if b.Actor == actor && b.Selection == sel && b.FileID == file {
			return b, true, nil
		}
	}
	return Binding{}, false, nil
}

type fakeResolver struct {
	calls int
	err   error
}

func (r *fakeResolver) Resolve(_ context.Context, s Selection) (ResolvedSource, error) {
	r.calls++
	return ResolvedSource{Selection: s, URL: "https://cdn.example.test/private-signed.gif?secret=one"}, r.err
}

type fakeFiles struct {
	imports, statuses, operationStatuses int
	state                                FileState
	last                                 ImportRequest
	err                                  error
}

func (f *fakeFiles) Import(_ context.Context, r ImportRequest) (FileState, error) {
	f.imports++
	f.last = r
	f.state = FileState{Actor: r.Actor, ActorCapture: r.ActorCapture, Selection: r.Selection, SourceID: r.SourceID, FileID: uuid.New(), Status: "processing"}
	return f.state, f.err
}
func (f *fakeFiles) Status(context.Context, Binding) (FileState, error) {
	f.statuses++
	return f.state, f.err
}
func (f *fakeFiles) StatusByOperation(_ context.Context, s OperationSelector) (FileState, error) {
	f.operationStatuses++
	if f.last.OperationID != s.OperationID {
		return FileState{}, status.Error(codes.NotFound, "operation not found")
	}
	if f.last.Actor != s.Actor || f.last.Selection != s.Selection || !proto.Equal(f.last.Destination, s.Destination) {
		return FileState{}, status.Error(codes.AlreadyExists, "selection changed")
	}
	return f.state, f.err
}

type fakeAuthority struct {
	calls int
	err   error
}

func (a *fakeAuthority) ObservePlaintextMember(ctx context.Context, actor Actor, ref *chatv1.ChatRef) (chatv1.ChatType, error) {
	return chatv1.ChatType_CHAT_TYPE_DM, a.RequirePlaintextMember(ctx, actor, ref)
}

func (a *fakeAuthority) RequirePlaintextMember(context.Context, Actor, *chatv1.ChatRef) error {
	a.calls++
	return a.err
}

func serviceFixture() (*Service, *chatv1.ResolveGifAssetRequest, Actor) {
	s := &Service{Store: &fakeRepository{bindings: map[uuid.UUID]Binding{}}, Resolver: &fakeResolver{}, Files: &fakeFiles{}, Authority: &fakeAuthority{}, SessionEpoch: func(context.Context, string, int64) error { return nil }}
	req := &chatv1.ResolveGifAssetRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), Provider: "fixture", ProviderId: "one", DestinationChat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_DM.Enum()}}
	return s, req, Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
}

// Unit service tests create the private capability explicitly. Actual JWT
// verification/replay/header rejection is exercised by interceptor_test.go.
func serviceContext(t *testing.T, req proto.Message, actor Actor, method string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	p := principal.Principal{Audience: "chat", RPC: method, RequestHash: hash, RequestID: uuid.NewString(), ExpiresAt: time.Now().Add(time.Minute)}
	if method == chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName {
		p.Kind = "delegated_user"
		p.Issuer = "gateway"
		p.AccountID = actor.AccountID.String()
		p.ProfileID = actor.ProfileID.String()
		p.Subject = p.AccountID
		p.SessionEpoch = 1
	} else {
		p.Kind = "service"
		p.Issuer = "messaging"
		p.Subject = "service:messaging"
		if method == chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName {
			p.Issuer = "file"
			p.Subject = "service:file"
		}
	}
	return context.WithValue(context.Background(), callerContextKey{}, verifiedCaller{principal: p, requestHash: hash, method: method})
}

func TestResolveGIFAssetBindsActualFileAndReauthorizesReplay(t *testing.T) {
	s, req, actor := serviceFixture()
	ctx := serviceContext(t, req, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName)
	response, err := s.ResolveGifAsset(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	files := s.Files.(*fakeFiles)
	resolver := s.Resolver.(*fakeResolver)
	store := s.Store.(*fakeRepository)
	if response.FileId != files.state.FileID.String() || response.SourceId != files.state.SourceID.String() || response.Status != "processing" || files.last.Actor != actor || files.last.SourceURL == "" || store.saves != 1 {
		t.Fatalf("response=%v import=%+v saves=%d", response, files.last, store.saves)
	}
	files.state.Status = "ready"
	again, err := s.ResolveGifAsset(serviceContext(t, req, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), req)
	if err != nil || again.GetStatus() != "ready" || files.imports != 1 || files.operationStatuses != 2 || resolver.calls != 1 {
		t.Fatalf("again=%v err=%v imports=%d statuses=%d resolves=%d", again, err, files.imports, files.operationStatuses, resolver.calls)
	}
	changed := proto.Clone(req).(*chatv1.ResolveGifAssetRequest)
	changed.ProviderId = "other"
	if _, err := s.ResolveGifAsset(serviceContext(t, changed, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), changed); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("changed retry=%v", err)
	}
	s.Authority.(*fakeAuthority).err = status.Error(codes.PermissionDenied, "left chat")
	statusesBefore := files.operationStatuses
	if _, err := s.ResolveGifAsset(serviceContext(t, req, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), req); status.Code(err) != codes.PermissionDenied || files.operationStatuses != statusesBefore {
		t.Fatalf("unauthorized replay=%v statuses=%d", err, files.operationStatuses)
	}
}

func TestResolveGIFAssetRejectsUntrustedAndE2EBeforeResolution(t *testing.T) {
	s, req, actor := serviceFixture()
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user", Issuer: "gateway", AccountID: actor.AccountID.String(), ProfileID: actor.ProfileID.String()})
	if _, err := s.ResolveGifAsset(ctx, req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("generic context=%v", err)
	}
	s.Authority.(*fakeAuthority).err = status.Error(codes.FailedPrecondition, "encrypted destination")
	ctx = serviceContext(t, req, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName)
	if _, err := s.ResolveGifAsset(ctx, req); status.Code(err) != codes.FailedPrecondition || s.Resolver.(*fakeResolver).calls != 0 || s.Files.(*fakeFiles).imports != 0 {
		t.Fatalf("e2e=%v", err)
	}
	s.Authority.(*fakeAuthority).err = nil
	s.Resolver.(*fakeResolver).err = errors.New("signed-url-secret")
	if _, err := s.ResolveGifAsset(ctx, req); status.Code(err) != codes.Unavailable || status.Convert(err).Message() == "signed-url-secret" {
		t.Fatalf("provider error=%v", err)
	}
}

func TestValidateGIFAssetIsExactReadOnlySourceAuthority(t *testing.T) {
	s, resolve, actor := serviceFixture()
	admitted, err := s.ResolveGifAsset(serviceContext(t, resolve, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), resolve)
	if err != nil {
		t.Fatal(err)
	}
	req := &chatv1.ValidateGifAssetRequest{ProtocolVersion: 1, ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), Provider: resolve.Provider, ProviderId: resolve.ProviderId, FileId: admitted.FileId, DestinationChat: resolve.DestinationChat}
	got, err := s.ValidateGifAsset(serviceContext(t, req, Actor{}, chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName), req)
	if err != nil || got.GetSourceId() != admitted.SourceId || got.GetActorProfileId() != actor.ProfileID.String() {
		t.Fatalf("validate=%v err=%v", got, err)
	}
	req.ProviderId = "forged"
	if _, err := s.ValidateGifAsset(serviceContext(t, req, Actor{}, chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName), req); status.Code(err) != codes.NotFound {
		t.Fatalf("forged tuple=%v", err)
	}
	if s.Resolver.(*fakeResolver).calls != 1 || s.Files.(*fakeFiles).imports != 1 || s.Files.(*fakeFiles).statuses != 0 {
		t.Fatal("validation resolved/imported/read File")
	}
}

func TestResolveGIFAssetRecoversFileReceiptWithoutNewURLResolution(t *testing.T) {
	s, req, actor := serviceFixture()
	method := chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName
	original, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
	if err != nil {
		t.Fatal(err)
	}
	// Model File commit followed by loss of Chat's active projection. The owner
	// receipt, not a new CDN URL, determines the original source and File identity.
	store := s.Store.(*fakeRepository)
	store.bindings = map[uuid.UUID]Binding{}
	s.Resolver.(*fakeResolver).err = errors.New("CDN URL rotated; must not resolve")
	again, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
	if err != nil || again.GetFileId() != original.FileId || s.Resolver.(*fakeResolver).calls != 1 || len(store.bindings) != 1 {
		t.Fatalf("recovered=%v err=%v", again, err)
	}
}

func TestResolveGIFAssetOldOperationCannotRebindRestoredActor(t *testing.T) {
	s, req, actor := serviceFixture()
	method := chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName
	if _, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req); err != nil {
		t.Fatal(err)
	}
	store := s.Store.(*fakeRepository)
	store.bindings = map[uuid.UUID]Binding{}
	store.capture = ActorCapture{AccountGeneration: uuid.New(), ProfileGeneration: uuid.New()}
	if _, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req); status.Code(err) != codes.FailedPrecondition || len(store.bindings) != 0 || store.saves != 1 {
		t.Fatalf("old operation err=%v saves=%d", err, store.saves)
	}
}

func TestResolveGIFAssetCompactedOperationNeverImportsAgain(t *testing.T) {
	s, req, actor := serviceFixture()
	files := s.Files.(*fakeFiles)
	files.last = ImportRequest{Actor: actor, Selection: Selection{Provider: req.Provider, ProviderID: req.ProviderId}, OperationID: uuid.MustParse(req.OperationId), Destination: req.DestinationChat}
	files.err = status.Error(codes.FailedPrecondition, "operation compacted")
	if _, err := s.ResolveGifAsset(serviceContext(t, req, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), req); status.Code(err) != codes.FailedPrecondition || files.imports != 0 || s.Resolver.(*fakeResolver).calls != 0 {
		t.Fatalf("compacted retry=%v", err)
	}
}

func TestResolveGIFAssetRechecksSessionBeforeProjection(t *testing.T) {
	s, req, actor := serviceFixture()
	checks := 0
	s.SessionEpoch = func(context.Context, string, int64) error {
		checks++
		if checks > 1 {
			return status.Error(codes.Unauthenticated, "session revoked")
		}
		return nil
	}
	_, err := s.ResolveGifAsset(serviceContext(t, req, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), req)
	if status.Code(err) != codes.Unauthenticated || s.Store.(*fakeRepository).saves != 0 {
		t.Fatalf("revoked session=%v saves=%d checks=%d", err, s.Store.(*fakeRepository).saves, checks)
	}
}

func TestAuthorizeCatalogDestinationIsFileOnlyCurrentRead(t *testing.T) {
	s, selection, actor := serviceFixture()
	if _, err := s.ResolveGifAsset(serviceContext(t, selection, actor, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName), selection); err != nil {
		t.Fatal(err)
	}
	req := &chatv1.AuthorizeCatalogDestinationRequest{ProtocolVersion: 1, ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), DestinationChat: selection.DestinationChat}
	method := chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName
	response, err := s.AuthorizeCatalogDestination(serviceContext(t, req, Actor{}, method), req)
	capture := s.Store.(*fakeRepository).capture
	if err != nil || response.GetObservedChatType() != chatv1.ChatType_CHAT_TYPE_DM || response.GetChatAccountGeneration() != capture.AccountGeneration.String() || response.GetChatProfileGeneration() != capture.ProfileGeneration.String() || !proto.Equal(response.GetDestinationChat(), req.DestinationChat) {
		t.Fatalf("destination=%v err=%v", response, err)
	}
	if s.Resolver.(*fakeResolver).calls != 1 || s.Files.(*fakeFiles).imports != 1 {
		t.Fatal("authorization resolved or imported an asset")
	}
	s.Authority.(*fakeAuthority).err = status.Error(codes.FailedPrecondition, "encrypted")
	if _, err := s.AuthorizeCatalogDestination(serviceContext(t, req, Actor{}, method), req); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("encrypted=%v", err)
	}
}
