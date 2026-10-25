package catalogassets

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type Actor struct{ AccountID, ProfileID uuid.UUID }

// ActorCapture is a local anti-resurrection generation, not a global User lease.
type ActorCapture struct{ AccountGeneration, ProfileGeneration uuid.UUID }
type Selection struct{ Provider, ProviderID string }
type ResolvedSource struct {
	Selection
	URL string
}

// Binding contains only admitted File identity. Raw URLs and full requests are
// never durable Chat data; this mapping follows its File and actor lifecycle.
type Binding struct {
	Actor
	ActorCapture
	Selection
	SourceID, FileID uuid.UUID
}
type ImportRequest struct {
	Actor
	ActorCapture
	Selection
	OperationID, SourceID uuid.UUID
	SourceURL             string
	Destination           *chatv1.ChatRef
}
type FileState struct {
	Actor
	ActorCapture
	Selection
	SourceID, FileID uuid.UUID
	Status           string
}
type OperationSelector struct {
	Actor
	Selection
	OperationID uuid.UUID
	Destination *chatv1.ChatRef
}

type Resolver interface {
	Resolve(context.Context, Selection) (ResolvedSource, error)
}
type Repository interface {
	CaptureActor(context.Context, Actor, bool) (ActorCapture, error)
	SaveBinding(context.Context, Binding, ActorCapture) error
	FindBinding(context.Context, Actor, Selection, uuid.UUID, ActorCapture) (Binding, bool, error)
	RetireBinding(context.Context, Actor, uuid.UUID) error
}
type FileClient interface {
	Import(context.Context, ImportRequest) (FileState, error)
	StatusByOperation(context.Context, OperationSelector) (FileState, error)
	Status(context.Context, Binding) (FileState, error)
}
type DestinationAuthority interface {
	RequirePlaintextMember(context.Context, Actor, *chatv1.ChatRef) error
}

type Service struct {
	chatv1.UnimplementedCatalogAssetServiceServer
	Store        Repository
	Resolver     Resolver
	Files        FileClient
	Authority    DestinationAuthority
	SessionEpoch principal.SessionEpochChecker
}

var ErrBindingConflict = errors.New("catalog binding conflict")

func (s *Service) ResolveGifAsset(ctx context.Context, req *chatv1.ResolveGifAssetRequest) (*chatv1.ResolveGifAssetResponse, error) {
	caller, err := requireCaller(ctx, req, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName)
	if err != nil {
		return nil, err
	}
	actor := Actor{AccountID: uuid.MustParse(caller.principal.AccountID), ProfileID: uuid.MustParse(caller.principal.ProfileID)}
	if req.GetProtocolVersion() != 1 || !canonicalID(req.GetOperationId()) || !validSelection(req.GetProvider(), req.GetProviderId()) || !validDestination(req.GetDestinationChat()) {
		return nil, status.Error(codes.InvalidArgument, "invalid GIF selection")
	}
	if s == nil || s.Store == nil || s.Files == nil || s.Authority == nil || s.SessionEpoch == nil {
		return nil, status.Error(codes.Unavailable, "catalog authority unavailable")
	}
	if err := s.checkSession(ctx, caller); err != nil {
		return nil, err
	}
	if err := s.Authority.RequirePlaintextMember(ctx, actor, req.DestinationChat); err != nil {
		return nil, destinationError(err)
	}
	selection := Selection{Provider: req.Provider, ProviderID: req.ProviderId}
	capture, err := s.Store.CaptureActor(ctx, actor, true)
	if err != nil {
		return nil, catalogError(err)
	}
	if !validCapture(capture) {
		return nil, status.Error(codes.Unavailable, "invalid catalog actor generation")
	}
	operation := uuid.MustParse(req.OperationId)
	// File owns the durable operation fence. Consult it before resolving a new URL
	// so a committed File survives a lost Chat response and CDN URL rotation.
	state, err := s.Files.StatusByOperation(ctx, OperationSelector{Actor: actor, Selection: selection, OperationID: operation, Destination: req.DestinationChat})
	if status.Code(err) == codes.NotFound {
		if s.Resolver == nil {
			return nil, status.Error(codes.Unavailable, "GIF source unavailable")
		}
		source, resolveErr := s.Resolver.Resolve(ctx, selection)
		if resolveErr != nil {
			return nil, catalogError(resolveErr)
		}
		parsed, parseErr := url.ParseRequestURI(source.URL)
		if source.Selection != selection || parseErr != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return nil, status.Error(codes.Unavailable, "GIF source unavailable")
		}
		sourceID := sourceGeneration(actor, capture, caller.requestHash, source.URL)
		state, err = s.Files.Import(ctx, ImportRequest{Actor: actor, ActorCapture: capture, Selection: selection, OperationID: operation, SourceID: sourceID, SourceURL: source.URL, Destination: proto.Clone(req.DestinationChat).(*chatv1.ChatRef)})
		if err == nil && state.SourceID != sourceID {
			return nil, status.Error(codes.Unavailable, "invalid catalog File binding")
		}
	}
	if err != nil {
		return nil, catalogError(err)
	}
	if !validState(state, actor, selection) {
		return nil, status.Error(codes.Unavailable, "invalid catalog File binding")
	}
	if state.ActorCapture != capture {
		return nil, status.Error(codes.FailedPrecondition, "catalog operation belongs to a retired actor generation")
	}
	if _, err := requireCaller(ctx, req, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName); err != nil {
		return nil, err
	}
	if err := s.checkSession(ctx, caller); err != nil {
		return nil, err
	}
	if err := s.Authority.RequirePlaintextMember(ctx, actor, req.DestinationChat); err != nil {
		return nil, destinationError(err)
	}
	if terminalState(state.Status) {
		err = s.Store.RetireBinding(ctx, actor, state.FileID)
	} else {
		err = s.Store.SaveBinding(ctx, Binding{Actor: actor, ActorCapture: capture, Selection: selection, SourceID: state.SourceID, FileID: state.FileID}, capture)
	}
	if err != nil {
		return nil, catalogError(err)
	}
	return &chatv1.ResolveGifAssetResponse{SourceId: state.SourceID.String(), Provider: selection.Provider, ProviderId: selection.ProviderID, FileId: state.FileID.String(), Status: state.Status}, nil
}

func (s *Service) ValidateGifAsset(ctx context.Context, req *chatv1.ValidateGifAssetRequest) (*chatv1.ValidateGifAssetResponse, error) {
	if _, err := requireCaller(ctx, req, chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName); err != nil {
		return nil, err
	}
	if req.GetProtocolVersion() != 1 || !canonicalID(req.GetActorAccountId()) || !canonicalID(req.GetActorProfileId()) || !canonicalID(req.GetFileId()) || !validSelection(req.GetProvider(), req.GetProviderId()) || !validDestination(req.GetDestinationChat()) {
		return nil, status.Error(codes.InvalidArgument, "invalid GIF source tuple")
	}
	if s == nil || s.Store == nil || s.Authority == nil {
		return nil, status.Error(codes.Unavailable, "catalog authority unavailable")
	}
	actor := Actor{AccountID: uuid.MustParse(req.ActorAccountId), ProfileID: uuid.MustParse(req.ActorProfileId)}
	if err := s.Authority.RequirePlaintextMember(ctx, actor, req.DestinationChat); err != nil {
		return nil, destinationError(err)
	}
	selection := Selection{Provider: req.Provider, ProviderID: req.ProviderId}
	file := uuid.MustParse(req.FileId)
	capture, err := s.Store.CaptureActor(ctx, actor, false)
	if err != nil {
		return nil, catalogError(err)
	}
	if !validCapture(capture) {
		return nil, status.Error(codes.Unavailable, "invalid catalog actor generation")
	}
	binding, found, err := s.Store.FindBinding(ctx, actor, selection, file, capture)
	if err != nil {
		return nil, catalogError(err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "catalog asset unavailable")
	}
	if binding.Actor != actor || binding.ActorCapture != capture || binding.Selection != selection || binding.FileID != file || binding.SourceID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "invalid catalog source binding")
	}
	if _, err := requireCaller(ctx, req, chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName); err != nil {
		return nil, err
	}
	if err := s.Authority.RequirePlaintextMember(ctx, actor, req.DestinationChat); err != nil {
		return nil, destinationError(err)
	}
	return &chatv1.ValidateGifAssetResponse{SourceId: binding.SourceID.String(), ActorAccountId: actor.AccountID.String(), ActorProfileId: actor.ProfileID.String(), Provider: selection.Provider, ProviderId: selection.ProviderID, FileId: file.String()}, nil
}

func requireCaller(ctx context.Context, message proto.Message, method string) (verifiedCaller, error) {
	fail := func() (verifiedCaller, error) {
		return verifiedCaller{}, status.Error(codes.Unauthenticated, "catalog principal required")
	}
	caller, ok := ctx.Value(callerContextKey{}).(verifiedCaller)
	if !ok || caller.method != method || caller.principal.RPC != method || caller.principal.Audience != "chat" || !caller.principal.ExpiresAt.After(time.Now()) {
		return fail()
	}
	if method == chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName {
		if caller.principal.Kind != "delegated_user" || caller.principal.Issuer != "gateway" || caller.principal.Subject != caller.principal.AccountID || caller.principal.SessionEpoch <= 0 || !canonicalID(caller.principal.AccountID) || !canonicalID(caller.principal.ProfileID) {
			return fail()
		}
	} else if caller.principal.Kind != "service" || caller.principal.Issuer != "messaging" || caller.principal.Subject != "service:messaging" || caller.principal.AccountID != "" || caller.principal.ProfileID != "" || caller.principal.SessionEpoch != 0 {
		return fail()
	}
	if message == nil || !message.ProtoReflect().IsValid() || hasUnknown(message) {
		return verifiedCaller{}, status.Error(codes.InvalidArgument, "invalid catalog request")
	}
	hash, err := principal.RequestHash(message)
	if err != nil || hash != caller.requestHash || hash != caller.principal.RequestHash {
		return fail()
	}
	if err := ctx.Err(); err != nil {
		return verifiedCaller{}, status.FromContextError(err).Err()
	}
	return caller, nil
}

func validSelection(provider, id string) bool {
	return provider != "" && id != "" && provider == strings.TrimSpace(provider) && id == strings.TrimSpace(id)
}
func validCapture(capture ActorCapture) bool {
	return capture.AccountGeneration != uuid.Nil && capture.ProfileGeneration != uuid.Nil
}
func (s *Service) checkSession(ctx context.Context, caller verifiedCaller) error {
	err := s.SessionEpoch(ctx, caller.principal.AccountID, caller.principal.SessionEpoch)
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.Unavailable {
		return status.Error(codes.Unavailable, "catalog session authority unavailable")
	}
	return status.Error(codes.Unauthenticated, "catalog actor session revoked")
}
func validDestination(ref *chatv1.ChatRef) bool {
	if ref == nil || !canonicalID(ref.GetId()) {
		return false
	}
	switch ref.GetType() {
	case chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, chatv1.ChatType_CHAT_TYPE_DM, chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL:
		return true
	default:
		return false
	}
}
func validState(state FileState, actor Actor, selection Selection) bool {
	if state.Actor != actor || state.Selection != selection || state.SourceID == uuid.Nil || state.FileID == uuid.Nil {
		return false
	}
	switch state.Status {
	case "pending_upload", "processing", "ready", "failed", "deleted", "expired":
		return true
	default:
		return false
	}
}
func terminalState(state string) bool {
	return state == "failed" || state == "deleted" || state == "expired"
}

func sourceGeneration(actor Actor, capture ActorCapture, requestHash, sourceURL string) uuid.UUID {
	// The UUID is an identity, never a capability. Full File operation fences still
	// prevent replay from recreating a purged/missing old import.
	digest := sha256.Sum256([]byte("voice.chat.gif-source.v1\x00" + actor.AccountID.String() + "\x00" + actor.ProfileID.String() + "\x00" + capture.AccountGeneration.String() + "\x00" + capture.ProfileGeneration.String() + "\x00" + requestHash + "\x00" + sourceURL))
	var id uuid.UUID
	copy(id[:], digest[:16])
	id[6] = (id[6] & 0x0f) | 0x80
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}
func destinationError(err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return status.Error(codes.InvalidArgument, "invalid catalog destination")
	case codes.PermissionDenied:
		return status.Error(codes.PermissionDenied, "catalog destination denied")
	case codes.FailedPrecondition:
		return status.Error(codes.FailedPrecondition, "catalog requires a plaintext destination")
	case codes.NotFound:
		return status.Error(codes.NotFound, "catalog destination unavailable")
	default:
		return status.Error(codes.Unavailable, "catalog destination authority unavailable")
	}
}
func catalogError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "catalog request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "catalog request deadline exceeded")
	case errors.Is(err, ErrBindingConflict) || status.Code(err) == codes.AlreadyExists:
		return status.Error(codes.AlreadyExists, "catalog operation conflicts with its original selection")
	case status.Code(err) == codes.FailedPrecondition:
		return status.Error(codes.FailedPrecondition, "catalog operation is terminal or unavailable")
	case status.Code(err) == codes.PermissionDenied:
		return status.Error(codes.PermissionDenied, "catalog asset denied")
	case status.Code(err) == codes.ResourceExhausted:
		return status.Error(codes.ResourceExhausted, "catalog rate limit exceeded")
	default:
		return status.Error(codes.Unavailable, "catalog dependency unavailable")
	}
}
