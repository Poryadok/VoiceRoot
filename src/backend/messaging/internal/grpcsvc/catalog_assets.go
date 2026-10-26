package grpcsvc

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/authctx"
	"voice/backend/pkg/principal"
)

type CatalogFileAuthority interface {
	LookupCatalogAsset(context.Context, *filev1.LookupCatalogAssetRequest, ...grpc.CallOption) (*filev1.LookupCatalogAssetResponse, error)
	ValidateCatalogAsset(context.Context, *filev1.ValidateCatalogAssetRequest, ...grpc.CallOption) (*filev1.ValidateCatalogAssetResponse, error)
}
type CatalogChatAuthority interface {
	ValidateGifAsset(context.Context, *chatv1.ValidateGifAssetRequest, ...grpc.CallOption) (*chatv1.ValidateGifAssetResponse, error)
}
type CatalogAssetCoordinator struct {
	Files  CatalogFileAuthority
	Chat   CatalogChatAuthority
	Issuer *principal.Issuer
}

type catalogDestinationKey struct{}
type catalogDestination struct {
	chat      uuid.UUID
	kind      chatv1.ChatType
	encrypted bool
}

// Only the send pipeline sets this after current membership/type/E2E admission.
func withCatalogDestination(ctx context.Context, chat uuid.UUID, kind chatv1.ChatType, encrypted bool) context.Context {
	return context.WithValue(ctx, catalogDestinationKey{}, catalogDestination{chat, kind, encrypted})
}

func (c *CatalogAssetCoordinator) signed(ctx context.Context, audience, method string, request proto.Message) (context.Context, context.CancelFunc, error) {
	if c == nil || c.Issuer == nil {
		return nil, nil, status.Error(codes.Unavailable, "catalog source authority unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, nil, status.Error(codes.InvalidArgument, "invalid catalog request")
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, nil, status.Error(codes.Unavailable, "catalog source authority unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), cancel, nil
}

func catalogUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func validCatalogSnapshot(r *filev1.ValidateCatalogAssetResponse, account, profile uuid.UUID, fileID string) bool {
	return r != nil && catalogUUID(r.SourceId) && r.ActorAccountId == account.String() && r.ActorProfileId == profile.String() && r.FileId == fileID && r.Provider != "" && r.ProviderId != "" && r.ObjectRevision > 0 && r.Intent == "gif" && r.FileType == "video" && (r.ScanResult == "clean" || r.ScanResult == "skipped") && len(r.SourceSha256) == 32 && len(r.DeliveredSha256) == 32 && r.DeliveredSizeBytes > 0 && r.DeliveredSizeBytes <= 5<<20 && (r.DeliveredMimeType == "video/mp4" || r.DeliveredMimeType == "video/webm")
}

// validateCatalogAttachments returns only attachments still requiring ordinary
// metadata/origin-chat validation. Errors never select the ordinary fallback.
func (s *MessagingGRPC) validateCatalogAttachments(ctx context.Context, chatID *uuid.UUID, attachments []messageAttachment, contentType string) ([]messageAttachment, error) {
	if s.CatalogAssets == nil {
		return attachments, nil
	}
	scope, _ := ctx.Value(catalogDestinationKey{}).(catalogDestination)
	// Preserve the opaque E2E path without querying any catalog/source identity.
	if scope.encrypted {
		return attachments, nil
	}
	account, accountOK := authctx.AccountID(ctx)
	profile, profileOK := authctx.ProfileID(ctx)
	if !accountOK || !profileOK || account == uuid.Nil || profile == uuid.Nil {
		return nil, status.Error(codes.Unauthenticated, "catalog actor unavailable")
	}
	c := s.CatalogAssets
	if isNilDependency(c.Files) || isNilDependency(c.Chat) || c.Issuer == nil {
		return nil, status.Error(codes.Unavailable, "catalog source authority unavailable")
	}
	ordinary := make([]messageAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		fileID := strings.TrimSpace(attachment.FileID)
		if !catalogUUID(fileID) {
			return nil, status.Error(codes.InvalidArgument, "invalid attachment file")
		}
		lookup := &filev1.LookupCatalogAssetRequest{ProtocolVersion: 1, ActorAccountId: account.String(), ActorProfileId: profile.String(), FileId: fileID}
		call, cancel, err := c.signed(ctx, "file", filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, lookup)
		if err != nil {
			return nil, err
		}
		classification, err := c.Files.LookupCatalogAsset(call, lookup)
		cancel()
		if err != nil {
			return nil, err
		}
		if classification.GetOrdinaryFile() {
			if attachment.Provider != "" || attachment.ProviderID != "" {
				return nil, status.Error(codes.InvalidArgument, "file has no catalog attribution")
			}
			ordinary = append(ordinary, attachment)
			continue
		}
		snapshot := classification.GetCatalog()
		if !validCatalogSnapshot(snapshot, account, profile, fileID) {
			return nil, status.Error(codes.FailedPrecondition, "catalog source is not ready")
		}
		if strings.TrimSpace(contentType) != "gif" || strings.TrimSpace(attachment.Type) != "gif" {
			return nil, status.Error(codes.InvalidArgument, "catalog asset requires GIF content")
		}
		if attachment.Provider != "" && attachment.Provider != snapshot.Provider || attachment.ProviderID != "" && attachment.ProviderID != snapshot.ProviderId {
			return nil, status.Error(codes.InvalidArgument, "catalog attribution mismatch")
		}
		if chatID == nil {
			return nil, status.Error(codes.FailedPrecondition, "catalog forwarding is unavailable")
		}
		if scope.chat != *chatID || scope.kind == chatv1.ChatType_CHAT_TYPE_UNSPECIFIED {
			return nil, status.Error(codes.Unavailable, "catalog destination admission unavailable")
		}
		chatRequest := &chatv1.ValidateGifAssetRequest{ProtocolVersion: 1, ActorAccountId: account.String(), ActorProfileId: profile.String(), Provider: snapshot.Provider, ProviderId: snapshot.ProviderId, FileId: fileID, DestinationChat: &chatv1.ChatRef{Id: chatID.String(), Type: &scope.kind}}
		call, cancel, err = c.signed(ctx, "chat", chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName, chatRequest)
		if err != nil {
			return nil, err
		}
		binding, err := c.Chat.ValidateGifAsset(call, chatRequest)
		cancel()
		if err != nil {
			return nil, err
		}
		if binding == nil || binding.SourceId != snapshot.SourceId || binding.ActorAccountId != snapshot.ActorAccountId || binding.ActorProfileId != snapshot.ActorProfileId || binding.Provider != snapshot.Provider || binding.ProviderId != snapshot.ProviderId || binding.FileId != snapshot.FileId {
			return nil, status.Error(codes.FailedPrecondition, "catalog source binding mismatch")
		}
		validation := &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 1, SourceId: snapshot.SourceId, Provider: snapshot.Provider, ProviderId: snapshot.ProviderId, FileId: fileID, ActorAccountId: account.String(), ActorProfileId: profile.String()}
		call, cancel, err = c.signed(ctx, "file", filev1.CatalogAssetService_ValidateCatalogAsset_FullMethodName, validation)
		if err != nil {
			return nil, err
		}
		final, err := c.Files.ValidateCatalogAsset(call, validation)
		cancel()
		if err != nil {
			return nil, err
		}
		if !validCatalogSnapshot(final, account, profile, fileID) || !proto.Equal(final, snapshot) {
			return nil, status.Error(codes.FailedPrecondition, "catalog source changed")
		}
	}
	return ordinary, nil
}
