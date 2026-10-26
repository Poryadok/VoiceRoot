package catalogassets

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

type GRPCFileClient struct {
	Client filev1.CatalogAssetServiceClient
	Issuer *principal.Issuer
}

func (c *GRPCFileClient) Import(ctx context.Context, input ImportRequest) (FileState, error) {
	if input.AccountID == uuid.Nil || input.ProfileID == uuid.Nil || input.OperationID == uuid.Nil || input.SourceID == uuid.Nil || input.AccountGeneration == uuid.Nil || input.ProfileGeneration == uuid.Nil || !validSelection(input.Provider, input.ProviderID) || !validDestination(input.Destination) || input.SourceURL == "" {
		return FileState{}, status.Error(codes.InvalidArgument, "invalid catalog import")
	}
	req := &filev1.ImportCatalogAssetRequest{ProtocolVersion: 2, OperationId: input.OperationID.String(), SourceId: input.SourceID.String(), Provider: input.Provider, ProviderId: input.ProviderID, SourceUrl: input.SourceURL, ActorAccountId: input.AccountID.String(), ActorProfileId: input.ProfileID.String(), DestinationChat: proto.Clone(input.Destination).(*chatv1.ChatRef), ChatAccountGeneration: input.AccountGeneration.String(), ChatProfileGeneration: input.ProfileGeneration.String()}
	call, cancel, err := c.callContext(ctx, filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName, req, 25*time.Second)
	if err != nil {
		return FileState{}, err
	}
	defer cancel()
	response, err := c.Client.ImportCatalogAsset(call, req)
	if err != nil {
		return FileState{}, fileCallError(err)
	}
	state, err := parseFileState(response, input.Actor, input.Selection)
	if err != nil || state.SourceID != input.SourceID || state.ActorCapture != input.ActorCapture || response.GetOperationId() != input.OperationID.String() || !proto.Equal(response.GetDestinationChat(), input.Destination) {
		return FileState{}, invalidFileBinding()
	}
	return state, nil
}
func (c *GRPCFileClient) StatusByOperation(ctx context.Context, input OperationSelector) (FileState, error) {
	if input.AccountID == uuid.Nil || input.ProfileID == uuid.Nil || input.OperationID == uuid.Nil || !validSelection(input.Provider, input.ProviderID) || !validDestination(input.Destination) {
		return FileState{}, status.Error(codes.InvalidArgument, "invalid catalog operation selector")
	}
	req := &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 2, ActorAccountId: input.AccountID.String(), ActorProfileId: input.ProfileID.String(), Selector: &filev1.GetCatalogAssetStatusRequest_Operation{Operation: &filev1.CatalogImportOperationSelector{OperationId: input.OperationID.String(), Provider: input.Provider, ProviderId: input.ProviderID, DestinationChat: proto.Clone(input.Destination).(*chatv1.ChatRef)}}}
	call, cancel, err := c.callContext(ctx, filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName, req, 3*time.Second)
	if err != nil {
		return FileState{}, err
	}
	defer cancel()
	response, err := c.Client.GetCatalogAssetStatus(call, req)
	if err != nil {
		return FileState{}, fileCallError(err)
	}
	state, err := parseFileState(response, input.Actor, input.Selection)
	if err != nil || response.GetTerminalFenceId() != "" || response.GetOperationId() != input.OperationID.String() || !proto.Equal(response.GetDestinationChat(), input.Destination) {
		return FileState{}, invalidFileBinding()
	}
	return state, nil
}
func (c *GRPCFileClient) Status(ctx context.Context, input Binding) (FileState, error) {
	if input.AccountID == uuid.Nil || input.ProfileID == uuid.Nil || input.SourceID == uuid.Nil || input.FileID == uuid.Nil || input.AccountGeneration == uuid.Nil || input.ProfileGeneration == uuid.Nil || !validSelection(input.Provider, input.ProviderID) {
		return FileState{}, status.Error(codes.InvalidArgument, "invalid catalog asset selector")
	}
	if input.Legacy {
		if input.SelectorSlotID != uuid.Nil || input.SelectionRevision != 0 {
			return FileState{}, status.Error(codes.InvalidArgument, "invalid legacy cleanup selector")
		}
	} else if input.SelectorSlotID == uuid.Nil || input.SelectionRevision == 0 {
		return FileState{}, status.Error(codes.InvalidArgument, "ordered selector capture required")
	}
	req := &filev1.GetCatalogAssetStatusRequest{ProtocolVersion: 2, ActorAccountId: input.AccountID.String(), ActorProfileId: input.ProfileID.String(), Selector: &filev1.GetCatalogAssetStatusRequest_Asset{Asset: &filev1.CatalogAssetSelector{SourceId: input.SourceID.String(), Provider: input.Provider, ProviderId: input.ProviderID, FileId: input.FileID.String()}}}
	call, cancel, err := c.callContext(ctx, filev1.CatalogAssetService_GetCatalogAssetStatus_FullMethodName, req, 3*time.Second)
	if err != nil {
		return FileState{}, err
	}
	defer cancel()
	response, err := c.Client.GetCatalogAssetStatus(call, req)
	if err != nil {
		return FileState{}, fileCallError(err)
	}
	return parseAssetFileState(response, input)
}

func parseAssetFileState(response *filev1.GetCatalogAssetStatusResponse, input Binding) (FileState, error) {
	if response == nil || hasUnknown(response) || response.GetOperationId() != "" || response.GetDestinationChat() != nil {
		return FileState{}, invalidFileBinding()
	}
	identity, err := parseFileIdentity(response, input.Actor, input.Selection)
	if err != nil || identity.SourceID != input.SourceID || identity.FileID != input.FileID {
		return FileState{}, invalidFileBinding()
	}
	var fence uuid.UUID
	if response.TerminalFenceId != "" {
		if !canonicalID(response.TerminalFenceId) || !terminalState(response.Status) {
			return FileState{}, invalidFileBinding()
		}
		fence = uuid.MustParse(response.TerminalFenceId)
		if response.ChatAccountGeneration == "" && response.ChatProfileGeneration == "" && response.SelectionRevision == 0 && response.CurrentSelectionRevision == 0 {
			// Exact negative cleanup evidence, never synthesized actor/order proof.
			identity.TerminalFenceID = fence
			return identity, nil
		}
		// A fence is a minimized terminal discriminator, never mixed with
		// retained full-detail generations or ordering proof.
		return FileState{}, invalidFileBinding()
	}
	state, err := parseFileState(response, input.Actor, input.Selection)
	if input.Legacy || err != nil || state.ActorCapture != input.ActorCapture || state.SelectionRevision != input.SelectionRevision {
		return FileState{}, invalidFileBinding()
	}
	state.TerminalFenceID = fence
	return state, nil
}

func (c *GRPCFileClient) callContext(ctx context.Context, method string, req proto.Message, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if c == nil || c.Client == nil || c.Issuer == nil {
		return nil, nil, status.Error(codes.Unavailable, "catalog File unavailable")
	}
	return bindServiceCall(ctx, c.Issuer, "file", method, req, timeout)
}

type fileStateIdentity interface {
	GetSourceId() string
	GetActorAccountId() string
	GetActorProfileId() string
	GetProvider() string
	GetProviderId() string
	GetFileId() string
	GetStatus() string
	GetChatAccountGeneration() string
	GetChatProfileGeneration() string
	GetSelectionRevision() uint64
	GetCurrentSelectionRevision() uint64
}

func parseFileIdentity(response fileStateIdentity, actor Actor, selection Selection) (FileState, error) {
	if message, ok := response.(proto.Message); ok && (message == nil || !message.ProtoReflect().IsValid() || hasUnknown(message)) {
		return FileState{}, invalidFileBinding()
	}
	if response == nil || !canonicalID(response.GetSourceId()) || !canonicalID(response.GetFileId()) || response.GetActorAccountId() != actor.AccountID.String() || response.GetActorProfileId() != actor.ProfileID.String() || response.GetProvider() != selection.Provider || response.GetProviderId() != selection.ProviderID {
		return FileState{}, invalidFileBinding()
	}
	state := FileState{Actor: actor, Selection: selection, SourceID: uuid.MustParse(response.GetSourceId()), FileID: uuid.MustParse(response.GetFileId()), Status: response.GetStatus(), SelectionRevision: response.GetSelectionRevision(), CurrentSelectionRevision: response.GetCurrentSelectionRevision()}
	if !validState(state, actor, selection) {
		return FileState{}, invalidFileBinding()
	}
	return state, nil
}
func parseFileState(response fileStateIdentity, actor Actor, selection Selection) (FileState, error) {
	state, err := parseFileIdentity(response, actor, selection)
	if err != nil || !canonicalID(response.GetChatAccountGeneration()) || !canonicalID(response.GetChatProfileGeneration()) || !validOrderedState(state) {
		return FileState{}, invalidFileBinding()
	}
	state.ActorCapture = ActorCapture{AccountGeneration: uuid.MustParse(response.GetChatAccountGeneration()), ProfileGeneration: uuid.MustParse(response.GetChatProfileGeneration())}
	return state, nil
}
func invalidFileBinding() error {
	return status.Error(codes.Unavailable, "invalid catalog File binding")
}
func fileCallError(err error) error {
	code := status.Code(err)
	if code == codes.Unknown {
		code = codes.Unavailable
	}
	return status.Error(code, "catalog File call failed")
}
