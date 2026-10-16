package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type managedChatPurgeStore interface {
	StartManagedChatPurge(context.Context, uuid.UUID, uuid.UUID, time.Time, []byte) (*store.ManagedChatPurgeWork, error)
	RequireManagedChatPurgeEventsPublished(context.Context, uuid.UUID) error
	CompleteManagedChatPurge(context.Context, uuid.UUID, []byte, []byte) (*store.ManagedChatPurgeWork, error)
}

type managedChatFileOwner interface {
	ReleaseFileReferences(context.Context, *filev1.ReleaseFileReferencesRequest, ...grpc.CallOption) (*filev1.ReleaseFileReferencesResponse, error)
	GetFileReferenceGCStatus(context.Context, *filev1.GetFileReferenceGCStatusRequest, ...grpc.CallOption) (*filev1.GetFileReferenceGCStatusResponse, error)
}

type managedChatSearchOwner interface {
	PurgeManagedChatMessages(context.Context, *searchv1.PurgeManagedChatMessagesRequest, ...grpc.CallOption) (*searchv1.PurgeManagedChatMessagesResponse, error)
}

// ManagedChatPurgeCoordinator freezes the work set, obtains terminal owner
// receipts, then lets Messaging delete its payloads in one local transaction.
type ManagedChatPurgeCoordinator struct {
	Store  managedChatPurgeStore
	Files  managedChatFileOwner
	Search managedChatSearchOwner
	Issuer *principal.Issuer
}

func (c *ManagedChatPurgeCoordinator) PurgeManagedChatContent(ctx context.Context, request *messagingv1.PurgeManagedChatContentRequest) (*messagingv1.PurgeManagedChatContentResponse, error) {
	if c == nil || c.Store == nil || c.Issuer == nil || request == nil || request.GetPurgeAfter() == nil || !request.GetPurgeAfter().IsValid() {
		return nil, errors.New("managed chat purge dependencies or request are unavailable")
	}
	operationID, err := uuid.Parse(request.GetOperationId())
	if err != nil || operationID == uuid.Nil || operationID.String() != request.GetOperationId() {
		return nil, errors.New("invalid managed chat purge operation")
	}
	chatID, err := uuid.Parse(request.GetChatId())
	if err != nil || chatID == uuid.Nil || chatID.String() != request.GetChatId() {
		return nil, errors.New("invalid managed chat purge chat")
	}
	requestHash, requestHashBytes, err := hashManagedChatPurgeRequest(request)
	if err != nil {
		return nil, err
	}
	work, err := c.Store.StartManagedChatPurge(ctx, operationID, chatID, request.GetPurgeAfter().AsTime(), requestHashBytes)
	if errors.Is(err, store.ErrManagedChatPurgeNotDue) {
		return nil, status.Error(codes.FailedPrecondition, "managed chat retention cutoff has not elapsed")
	}
	if errors.Is(err, store.ErrManagedChatPurgeOperationConflict) {
		return nil, ErrManagedChatPurgeConflict
	}
	if err != nil {
		return nil, err
	}
	if work == nil || work.OperationID != operationID || work.ChatID != chatID || !work.PurgeAfter.Equal(request.GetPurgeAfter().AsTime()) || !bytes.Equal(work.RequestSHA256, requestHashBytes) {
		return nil, errors.New("messaging returned a mismatched frozen purge work set")
	}
	if work.State == "COMPLETED" {
		if len(work.FileReceiptSHA256) != sha256.Size || len(work.SearchReceiptSHA256) != sha256.Size || work.CompletedAt == nil {
			return nil, errors.New("completed Messaging purge lacks persisted owner evidence")
		}
		return managedChatPurgeResponse(work, requestHashBytes, request.GetPurgeAfter().AsTime(), 0), nil
	}
	if err := c.Store.RequireManagedChatPurgeEventsPublished(ctx, operationID); err != nil {
		return nil, status.Error(codes.Unavailable, "Messaging event outbox has not confirmed the frozen purge event set")
	}
	refs, err := managedChatFileReferences(work)
	if err != nil {
		return nil, err
	}
	if scope, ok := store.SpacePurgeScope(ctx); ok {
		for _, reference := range refs {
			value := scope.String()
			reference.ScopeSpaceId = &value
		}
	}
	fileReceiptHash, err := c.releaseAndVerifyFile(ctx, operationID, requestHash, requestHashBytes, refs)
	if err != nil {
		return nil, err
	}
	searchReceiptHash, searchCount, err := c.purgeAndVerifySearch(ctx, operationID, chatID, requestHash, work.MessageIDs)
	if err != nil {
		return nil, err
	}
	completed, err := c.Store.CompleteManagedChatPurge(ctx, operationID, fileReceiptHash, searchReceiptHash)
	if errors.Is(err, store.ErrManagedChatPurgeOperationConflict) {
		return nil, ErrManagedChatPurgeConflict
	}
	if err != nil {
		return nil, err
	}
	if completed == nil || completed.State != "COMPLETED" || completed.CompletedAt == nil || !bytes.Equal(completed.FileReceiptSHA256, fileReceiptHash) || !bytes.Equal(completed.SearchReceiptSHA256, searchReceiptHash) {
		return nil, errors.New("messaging failed to persist both owner receipts")
	}
	return managedChatPurgeResponse(completed, requestHashBytes, request.GetPurgeAfter().AsTime(), searchCount), nil
}

func (c *ManagedChatPurgeCoordinator) releaseAndVerifyFile(ctx context.Context, operationID uuid.UUID, parentRequestHash string, parentRequestHashBytes []byte, refs []*filev1.FileReferenceKey) ([]byte, error) {
	if scope, ok := store.SpacePurgeScope(ctx); ok {
		evidence, ok := ctx.Value(spaceFileReleaseEvidenceKey{}).(spaceFileReleaseEvidence)
		if !ok || evidence.space != scope.String() || len(evidence.hash) != sha256.Size {
			return nil, errors.New("verified Space producer release evidence required")
		}
		for _, ref := range refs {
			raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(ref)
			if err != nil || !evidence.references[string(raw)] {
				return nil, errors.New("child File reference absent from released Space producer manifest")
			}
		}
		return append([]byte(nil), evidence.hash...), nil
	}
	if len(refs) == 0 {
		return receiptHash("voice.messaging.managed_chat_purge.no_file_refs.v1", parentRequestHashBytes), nil
	}
	if c.Files == nil {
		return nil, errors.New("file purge owner is unavailable")
	}
	release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: operationID.String(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: refs}
	releaseHash := lifecycleHash(release)
	if releaseHash == nil {
		return nil, errors.New("could not hash File release request")
	}
	releaseRPC := filev1.FileService_ReleaseFileReferences_FullMethodName
	releaseCtx, err := c.outgoingServiceContext(ctx, releaseRPC, operationID.String(), release)
	if err != nil {
		return nil, err
	}
	released, err := c.Files.ReleaseFileReferences(releaseCtx, release)
	if err != nil {
		return nil, err
	}
	if released == nil || released.GetReceipt() == nil || released.GetReceipt().GetProtocolVersion() != 1 || released.GetReceipt().GetOperationId() != operationID.String() || released.GetReceipt().GetProducerId() != release.GetProducerId() || released.GetReceipt().GetReleasedCount() != uint64(len(refs)) || !bytes.Equal(released.GetReceipt().GetRequestSha256(), releaseHash) {
		return nil, errors.New("file returned an invalid reference release receipt")
	}
	statusRequest := &filev1.GetFileReferenceGCStatusRequest{ProtocolVersion: 1, OperationId: operationID.String(), ProducerId: release.GetProducerId(), References: refs, ReleaseRequestSha256: releaseHash}
	statusRPC := filev1.FileService_GetFileReferenceGCStatus_FullMethodName
	statusCtx, err := c.outgoingServiceContext(ctx, statusRPC, operationID.String(), statusRequest)
	if err != nil {
		return nil, err
	}
	statusResponse, err := c.Files.GetFileReferenceGCStatus(statusCtx, statusRequest)
	if err != nil {
		return nil, err
	}
	expectedStatusHash := lifecycleHash(statusRequest)
	if len(expectedStatusHash) != sha256.Size {
		return nil, errors.New("could not hash File GC status request")
	}
	if statusResponse == nil || statusResponse.GetOperationId() != operationID.String() || statusResponse.GetProducerId() != release.GetProducerId() || !bytes.Equal(statusResponse.GetReleaseRequestSha256(), releaseHash) || !bytes.Equal(statusResponse.GetRequestSha256(), expectedStatusHash) || len(statusResponse.GetReferences()) != len(refs) {
		return nil, errors.New("file returned mismatched GC status evidence")
	}
	for index, item := range statusResponse.GetReferences() {
		if item == nil || item.GetReference() == nil || !proto.Equal(item.GetReference(), refs[index]) {
			return nil, errors.New("file returned a different frozen reference set")
		}
		if item.GetState() == filev1.FileReferenceGCState_FILE_REFERENCE_GC_STATE_PENDING {
			return nil, status.Error(codes.Unavailable, "File garbage collection is pending")
		}
		if item.GetState() != filev1.FileReferenceGCState_FILE_REFERENCE_GC_STATE_GC_COMPLETE && item.GetState() != filev1.FileReferenceGCState_FILE_REFERENCE_GC_STATE_RETAINED_SHARED {
			return nil, errors.New("file returned an unknown GC state")
		}
	}
	if statusResponse.GetReceiptId() == "" || statusResponse.GetCompletedAt() == nil || !statusResponse.GetCompletedAt().IsValid() || len(statusResponse.GetRequestSha256()) != sha256.Size {
		return nil, errors.New("file terminal GC receipt is incomplete")
	}
	return responseHash(statusResponse)
}

func (c *ManagedChatPurgeCoordinator) purgeAndVerifySearch(ctx context.Context, operationID, chatID uuid.UUID, requestHash string, ids []uuid.UUID) ([]byte, uint64, error) {
	if len(ids) == 0 {
		return receiptHash("voice.messaging.managed_chat_purge.no_search_docs.v1", []byte(requestHash)), 0, nil
	}
	if c.Search == nil {
		return nil, 0, errors.New("search purge owner is unavailable")
	}
	ordered := append([]uuid.UUID(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	request := &searchv1.PurgeManagedChatMessagesRequest{OperationId: operationID.String(), ChatId: chatID.String()}
	messageIDHash := sha256.New()
	_, _ = messageIDHash.Write([]byte("voice.search.managed_chat_message_ids.v1\x00"))
	for _, id := range ordered {
		if id == uuid.Nil {
			return nil, 0, errors.New("frozen Search message ID is invalid")
		}
		request.MessageIds = append(request.MessageIds, id.String())
		_, _ = messageIDHash.Write(id[:])
	}
	requestHashString, requestHashBytes, err := hashManagedChatPurgeRequest(request)
	if err != nil {
		return nil, 0, err
	}
	rpc := searchv1.SearchService_PurgeManagedChatMessages_FullMethodName
	callCtx, err := c.outgoingServiceContext(ctx, rpc, operationID.String(), request)
	if err != nil {
		return nil, 0, err
	}
	response, err := c.Search.PurgeManagedChatMessages(callCtx, request)
	if err != nil {
		return nil, 0, err
	}
	if response == nil || response.GetReceiptId() == "" || response.GetOperationId() != operationID.String() || response.GetChatId() != chatID.String() || response.GetDeletedCount() != uint64(len(ordered)) || !bytes.Equal(response.GetMessageIdsSha256(), messageIDHash.Sum(nil)) || !bytes.Equal(response.GetRequestSha256(), requestHashBytes) || response.GetCompletedAt() == nil || !response.GetCompletedAt().IsValid() {
		return nil, 0, errors.New("search returned an incomplete or mismatched purge receipt")
	}
	_ = requestHashString
	responseDigest, err := responseHash(response)
	return responseDigest, response.GetDeletedCount(), err
}

func (c *ManagedChatPurgeCoordinator) outgoingServiceContext(ctx context.Context, rpc, requestID string, request proto.Message) (context.Context, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "file", RPC: rpc, RequestID: requestID, RequestHash: hash})
	if rpc == searchv1.SearchService_PurgeManagedChatMessages_FullMethodName {
		token, err = c.Issuer.IssueService(principal.ServiceInput{Audience: "search", RPC: rpc, RequestID: requestID, RequestHash: hash})
	}
	if err != nil {
		return nil, err
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

func managedChatFileReferences(work *store.ManagedChatPurgeWork) ([]*filev1.FileReferenceKey, error) {
	refs := make([]*filev1.FileReferenceKey, 0)
	for _, message := range work.MessageAttachments {
		if message.ID == uuid.Nil {
			return nil, errors.New("frozen File owner message ID is invalid")
		}
		for _, fileID := range message.FileIDs {
			if fileID == uuid.Nil {
				return nil, errors.New("frozen File ID is invalid")
			}
			refs = append(refs, &filev1.FileReferenceKey{FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: message.ID.String()})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].GetFileId() != refs[j].GetFileId() {
			return refs[i].GetFileId() < refs[j].GetFileId()
		}
		return refs[i].GetOwnerId() < refs[j].GetOwnerId()
	})
	for i := 1; i < len(refs); i++ {
		if proto.Equal(refs[i-1], refs[i]) {
			return nil, errors.New("frozen File references contain a duplicate")
		}
	}
	return refs, nil
}

func hashManagedChatPurgeRequest(message proto.Message) (string, []byte, error) {
	hash, err := principal.RequestHash(message)
	if err != nil || len(hash) != 71 || hash[:7] != "sha256:" {
		return "", nil, errors.New("invalid request hash")
	}
	decoded, err := hex.DecodeString(hash[7:])
	if err != nil || len(decoded) != sha256.Size {
		return "", nil, errors.New("invalid request hash")
	}
	return hash, decoded, nil
}

func lifecycleHash(message proto.Message) []byte {
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil
	}
	domain := append([]byte(message.ProtoReflect().Descriptor().FullName()), 0)
	sum := sha256.Sum256(append(domain, wire...))
	return sum[:]
}

func responseHash(message proto.Message) ([]byte, error) {
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(wire)
	return sum[:], nil
}

func receiptHash(domain string, value []byte) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(value)
	return h.Sum(nil)
}

func managedChatPurgeResponse(work *store.ManagedChatPurgeWork, requestHashBytes []byte, purgeAfter time.Time, searchCount uint64) *messagingv1.PurgeManagedChatContentResponse {
	var completed *timestamppb.Timestamp
	if work.CompletedAt != nil {
		completed = timestamppb.New(*work.CompletedAt)
	}
	messageCount := uint64(len(work.MessageIDs))
	if searchCount == 0 && messageCount > 0 {
		searchCount = messageCount
	}
	return &messagingv1.PurgeManagedChatContentResponse{ReceiptId: work.OperationID.String(), OperationId: work.OperationID.String(), ChatId: work.ChatID.String(), PurgeAfter: timestamppb.New(purgeAfter), MessageCount: messageCount, FileReferenceCount: uint64(lenFileIDs(work.MessageAttachments)), SearchDocumentCount: searchCount, FileReceiptSha256: append([]byte(nil), work.FileReceiptSHA256...), SearchReceiptSha256: append([]byte(nil), work.SearchReceiptSHA256...), RequestSha256: append([]byte(nil), requestHashBytes...), CompletedAt: completed, Replayed: work.State == "COMPLETED"}
}

func lenFileIDs(messages []store.ManagedChatPurgeMessage) int {
	count := 0
	for _, message := range messages {
		count += len(message.FileIDs)
	}
	return count
}
