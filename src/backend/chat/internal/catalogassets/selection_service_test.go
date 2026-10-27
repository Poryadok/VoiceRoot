package catalogassets

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
)

// Hooks control remote response delivery, not File's admission implementation.
// Actual projection ordering/incarnation checks also run against PostgreSQL.
type selectionFiles struct {
	FileClient
	importResponse    func(ImportRequest) (FileState, error)
	statusResponse    func(Binding) (FileState, error)
	operationResponse func(OperationSelector) (FileState, error)
}

func (f selectionFiles) Import(_ context.Context, req ImportRequest) (FileState, error) {
	return f.importResponse(req)
}
func (f selectionFiles) Status(ctx context.Context, b Binding) (FileState, error) {
	if f.statusResponse != nil {
		return f.statusResponse(b)
	}
	return f.FileClient.Status(ctx, b)
}
func (f selectionFiles) StatusByOperation(ctx context.Context, op OperationSelector) (FileState, error) {
	if f.operationResponse != nil {
		return f.operationResponse(op)
	}
	return f.FileClient.StatusByOperation(ctx, op)
}

func TestValidateRejectsNewerFileHeadBeforeChatProjection(t *testing.T) {
	for _, mutation := range []string{"newer head", "zero proof", "pending", "terminal"} {
		t.Run(mutation, func(t *testing.T) {
			s, req, actor := serviceFixture()
			method := chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName
			got, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
			require.NoError(t, err)
			files := s.Files.(*fakeFiles)
			files.state.Status = "ready"
			switch mutation {
			case "newer head":
				files.state.CurrentSelectionRevision = 2
			case "zero proof":
				files.state.SelectionRevision = 0
			case "pending":
				files.state.Status = "processing"
			case "terminal":
				files.state.Status = "failed"
			}
			validate := &chatv1.ValidateGifAssetRequest{ProtocolVersion: 2, ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), Provider: req.Provider, ProviderId: req.ProviderId, FileId: got.FileId, DestinationChat: req.DestinationChat}
			method = chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName
			_, err = s.ValidateGifAsset(serviceContext(t, validate, Actor{}, method), validate)
			require.Equal(t, codes.FailedPrecondition, status.Code(err))
			validate.ProtocolVersion = 1
			_, err = s.ValidateGifAsset(serviceContext(t, validate, Actor{}, method), validate)
			require.Equal(t, codes.InvalidArgument, status.Code(err), "legacy private protocol cannot admit")
		})
	}
}

func TestResolveDelayedResponseCannotReplaceNewerProjection(t *testing.T) {
	s, req, actor := serviceFixture()
	base := s.Files
	store := s.Store.(*fakeRepository)
	var b Binding
	s.Files = selectionFiles{FileClient: base, importResponse: func(r ImportRequest) (FileState, error) {
		// A has already been admitted at revision1. Before its response arrives,
		// B is admitted2 and projected into the same captured cache incarnation.
		b = Binding{Actor: actor, ActorCapture: r.ActorCapture, Selection: r.Selection, SourceID: uuid.New(), FileID: uuid.New(), SelectionRevision: 2}
		b.SelectorSlotID = store.slots[fakeSelectorKey{actor, r.ActorCapture, r.Selection}]
		require.NoError(t, store.SaveBinding(context.Background(), b, r.ActorCapture))
		return FileState{Actor: actor, ActorCapture: r.ActorCapture, Selection: r.Selection, SourceID: r.SourceID, FileID: uuid.New(), Status: "ready", SelectionRevision: 1, CurrentSelectionRevision: 1}, nil
	}}
	method := chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName
	_, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, map[uuid.UUID]Binding{b.FileID: b}, store.bindings)
}

func TestResolveEvictedResponseNeedsFreshCurrentReceipt(t *testing.T) {
	s, req, actor := serviceFixture()
	store := s.Store.(*fakeRepository)
	var receipt FileState
	s.Files = selectionFiles{FileClient: s.Files, importResponse: func(r ImportRequest) (FileState, error) {
		receipt = FileState{Actor: actor, ActorCapture: r.ActorCapture, Selection: r.Selection, SourceID: r.SourceID, FileID: uuid.New(), Status: "ready", SelectionRevision: 1, CurrentSelectionRevision: 1}
		old := store.slots[fakeSelectorKey{actor, r.ActorCapture, r.Selection}]
		require.NoError(t, store.DropEmptySelection(context.Background(), actor, r.ActorCapture, old))
		fresh, err := store.CaptureSelection(context.Background(), actor, r.Selection, r.ActorCapture)
		require.NoError(t, err)
		require.NotEqual(t, old, fresh)
		return receipt, nil
	}}
	method := chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName
	_, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Empty(t, store.bindings)
	s.Files = selectionFiles{FileClient: s.Files, operationResponse: func(OperationSelector) (FileState, error) { return receipt, nil }}
	got, err := s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
	require.NoError(t, err)
	require.Equal(t, receipt.FileID.String(), got.FileId)
	// B has since superseded A then become terminal. Even after cache removal,
	// an immutable old operation response must not bootstrap A as current.
	store.bindings = map[uuid.UUID]Binding{}
	store.slots = map[fakeSelectorKey]uuid.UUID{}
	receipt.CurrentSelectionRevision = 2
	_, err = s.ResolveGifAsset(serviceContext(t, req, actor, method), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Empty(t, store.bindings)
	require.Empty(t, store.slots, "failed request drops only its still-empty incarnation")
}
