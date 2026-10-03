package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/filerefmanifest"
	"voice/backend/pkg/principal"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type SpacePurgeMessagingClient interface {
	GetSpacePurgeReceipt(context.Context, *messagingv1.GetSpacePurgeReceiptRequest, ...grpc.CallOption) (*messagingv1.GetSpacePurgeReceiptResponse, error)
}

type SpacePurgeFileClient interface {
	ReleaseSpaceDeletionProducerReferences(context.Context, *filev1.ReleaseSpaceDeletionProducerReferencesRequest, ...grpc.CallOption) (*filev1.ReleaseSpaceDeletionProducerReferencesResponse, error)
}

type SpaceLifecycleStoreAPI interface {
	PrepareSpaceDeletionManifest(context.Context, *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error)
	ApplySpaceLifecycleFence(context.Context, *chatv1.ApplySpaceLifecycleFenceRequest) (*chatv1.ApplySpaceLifecycleFenceResponse, error)
	GetSpacePurgeManifestPage(context.Context, *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error)
	ValidateSpacePurgeRequest(context.Context, *chatv1.PurgeSpaceRequest) error
	PurgeSpace(context.Context, *chatv1.PurgeSpaceRequest, store.SpacePurgeOwnerEvidence) (*chatv1.PurgeSpaceResponse, error)
}

// SpaceLifecycleGRPC is registered only on Chat's mTLS Space-principal
// listener. The listener verifies the exact RPC, request hash, operation ID,
// Space signing key and replay ID before these methods are reached.
type SpaceLifecycleGRPC struct {
	chatv1.UnimplementedChatServiceServer
	Store     SpaceLifecycleStoreAPI
	Issuer    *principal.Issuer
	Messaging SpacePurgeMessagingClient
	File      SpacePurgeFileClient
}

func (s *SpaceLifecycleGRPC) PrepareSpaceDeletionManifest(ctx context.Context, req *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, chatv1.ChatService_PrepareSpaceDeletionManifest_FullMethodName); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle store unavailable")
	}
	response, err := s.Store.PrepareSpaceDeletionManifest(ctx, req)
	if err != nil {
		return nil, mapSpaceLifecycleError(err)
	}
	return response, nil
}

func (s *SpaceLifecycleGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *chatv1.ApplySpaceLifecycleFenceRequest) (*chatv1.ApplySpaceLifecycleFenceResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, chatv1.ChatService_ApplySpaceLifecycleFence_FullMethodName); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle store unavailable")
	}
	response, err := s.Store.ApplySpaceLifecycleFence(ctx, req)
	if err != nil {
		return nil, mapSpaceLifecycleError(err)
	}
	if req.GetFence().GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
		if err := s.sealEmptyFileProducer(ctx, req.GetFence()); err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (s *SpaceLifecycleGRPC) sealEmptyFileProducer(ctx context.Context, fence *commonv1.SpaceLifecycleFenceRequest) error {
	client, ok := s.File.(interface {
		RegisterSpaceDeletionReferenceChunk(context.Context, *filev1.RegisterSpaceDeletionReferenceChunkRequest, ...grpc.CallOption) (*filev1.RegisterSpaceDeletionReferenceChunkResponse, error)
	})
	if !ok || s.Issuer == nil {
		return status.Error(codes.Unavailable, "Chat File producer transport unavailable")
	}
	operation, err := uuid.Parse(fence.GetDeletionOperationId())
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid deletion operation")
	}
	space, err := uuid.Parse(fence.GetSpaceId())
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid Space")
	}
	digest, err := filerefmanifest.Hash(operation, space, fence.Generation, 2, nil)
	if err != nil {
		return err
	}
	request := &filev1.RegisterSpaceDeletionReferenceChunkRequest{ProtocolVersion: 1, OperationId: uuid.NewSHA1(operation, []byte("chat.file-producer.v1\x00"+space.String())).String(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, DeletionOperationId: operation.String(), SpaceId: space.String(), ScheduleGeneration: fence.Generation, ExpectedReferencesSha256: digest, SealsProducer: true}
	call, err := s.ownerCallContext(ctx, "file", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, request.OperationId, request)
	if err != nil {
		return err
	}
	defer call.cancel()
	response, err := client.RegisterSpaceDeletionReferenceChunk(call.ctx, request)
	if err != nil {
		return err
	}
	receipt := response.GetReceipt()
	if receipt == nil || receipt.ProtocolVersion != 1 || receipt.ReceiptId == "" || receipt.OperationId != request.OperationId || receipt.DeletionOperationId != request.DeletionOperationId || receipt.SpaceId != request.SpaceId || receipt.ScheduleGeneration != request.ScheduleGeneration || receipt.ChunkIndex != 0 || receipt.AcceptedCount != 0 || receipt.ProducerId != request.ProducerId || receipt.ExpectedTotalCount != 0 || string(receipt.ExpectedReferencesSha256) != string(digest) || !receipt.ProducerSealed || string(receipt.RequestSha256) != string(lifecycleDomainDigest(string(request.ProtoReflect().Descriptor().FullName()), request)) || receipt.CompletedAt == nil || receipt.CompletedAt.CheckValid() != nil {
		return status.Error(codes.DataLoss, "File returned invalid Chat producer seal receipt")
	}
	return nil
}

func (s *SpaceLifecycleGRPC) GetSpacePurgeManifestPage(ctx context.Context, req *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	if err := requireManifestPagePrincipal(ctx); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle store unavailable")
	}
	response, err := s.Store.GetSpacePurgeManifestPage(ctx, req)
	if err != nil {
		return nil, mapSpaceLifecycleError(err)
	}
	return response, nil
}

func (s *SpaceLifecycleGRPC) PurgeSpace(ctx context.Context, req *chatv1.PurgeSpaceRequest) (*chatv1.PurgeSpaceResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, chatv1.ChatService_PurgeSpace_FullMethodName); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil || s.Issuer == nil || s.Messaging == nil || s.File == nil {
		return nil, status.Error(codes.Unavailable, "Chat purge proof clients are not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "valid Chat purge request required")
	}
	purge := req.GetPurge()
	if purge == nil || purge.GetProtocolVersion() != 1 || purge.GetGeneration() < 2 || purge.GetManifest() == nil || purge.GetPurgeDecidedAt() == nil || purge.GetPurgeDecidedAt().CheckValid() != nil || purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_CHAT {
		return nil, status.Error(codes.InvalidArgument, "valid Chat purge request required")
	}
	if err := s.Store.ValidateSpacePurgeRequest(ctx, req); err != nil {
		return nil, mapSpaceLifecycleError(err)
	}
	messagingPurge := proto.Clone(purge).(*commonv1.SpacePurgeRequest)
	messagingPurge.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
	messagingRequest := &messagingv1.PurgeSpaceRequest{Purge: messagingPurge}
	messagingCall, err := s.ownerCallContext(ctx, "messaging", messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName, purge.GetDeletionOperationId(), &messagingv1.GetSpacePurgeReceiptRequest{
		SpaceId: purge.GetSpaceId(), DeletionOperationId: purge.GetDeletionOperationId(), PurgeGeneration: purge.GetGeneration(),
		SourceScheduleGeneration: purge.GetGeneration() - 1, MessagingRequestSha256: lifecycleDomainDigest("voice.messaging.v1.PurgeSpaceRequest", messagingRequest),
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "cannot authorize Messaging purge receipt lookup")
	}
	defer messagingCall.cancel()
	messagingResponse, err := s.Messaging.GetSpacePurgeReceipt(messagingCall.ctx, messagingCall.request.(*messagingv1.GetSpacePurgeReceiptRequest))
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Messaging purge receipt lookup failed")
	}
	if messagingResponse == nil {
		return nil, status.Error(codes.FailedPrecondition, "Messaging purge receipt lookup returned no receipt")
	}
	messagingReceipt := messagingResponse.GetReceipt()
	if !validPurgeReceipt(messagingReceipt, purge, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, lifecycleDomainDigest("voice.messaging.v1.PurgeSpaceRequest", messagingRequest)) {
		return nil, status.Error(codes.FailedPrecondition, "Messaging purge receipt does not match the request")
	}

	fileHash, err := chatProducerEmptyReferenceHash(purge.GetDeletionOperationId(), purge.GetSpaceId(), purge.GetGeneration()-1)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat purge identifiers")
	}
	fileRequest := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{
		ProtocolVersion: 1, DeletionOperationId: purge.GetDeletionOperationId(), SpaceId: purge.GetSpaceId(),
		PurgeGeneration: purge.GetGeneration(), SourceScheduleGeneration: purge.GetGeneration() - 1,
		ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, ExpectedReferencesSha256: fileHash[:],
	}
	fileCall, err := s.ownerCallContext(ctx, "file", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, purge.GetDeletionOperationId(), fileRequest)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "cannot authorize File reference release")
	}
	defer fileCall.cancel()
	fileResponse, err := s.File.ReleaseSpaceDeletionProducerReferences(fileCall.ctx, fileRequest)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "File reference release failed")
	}
	if fileResponse.GetReceipt() == nil {
		return nil, status.Error(codes.FailedPrecondition, "File reference release returned no receipt")
	}
	response, err := s.Store.PurgeSpace(ctx, req, store.SpacePurgeOwnerEvidence{MessagingReceipt: messagingReceipt, FileReleaseReceipt: fileResponse.GetReceipt()})
	if err != nil {
		return nil, mapSpaceLifecycleError(err)
	}
	return response, nil
}

type ownerCall struct {
	ctx     context.Context
	cancel  context.CancelFunc
	request proto.Message
}

func (s *SpaceLifecycleGRPC) ownerCallContext(ctx context.Context, audience, method, requestID string, request proto.Message) (ownerCall, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return ownerCall{}, err
	}
	token, err := s.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return ownerCall{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	return ownerCall{ctx: metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), cancel: cancel, request: request}, nil
}

func validPurgeReceipt(receipt *commonv1.SpacePurgeReceipt, purge *commonv1.SpacePurgeRequest, participant commonv1.ParticipantId, expectedHash []byte) bool {
	return receipt != nil && receipt.GetProtocolVersion() == 1 && receipt.GetReceiptId() != "" && receipt.GetSpaceId() == purge.GetSpaceId() &&
		receipt.GetDeletionOperationId() == purge.GetDeletionOperationId() && receipt.GetGeneration() == purge.GetGeneration() &&
		receipt.GetParticipantId() == participant && receipt.GetState() == commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED &&
		len(receipt.GetRequestSha256()) == sha256.Size && string(receipt.GetRequestSha256()) == string(expectedHash) &&
		receipt.GetCompletedAt() != nil && receipt.GetCompletedAt().CheckValid() == nil
}

func lifecycleDomainDigest(fqn string, request proto.Message) []byte {
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil
	}
	h := sha256.New()
	_, _ = h.Write([]byte(fqn))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(wire)
	return h.Sum(nil)
}

func chatProducerEmptyReferenceHash(deletionID, spaceID string, generation uint64) ([]byte, error) {
	deletion, err := uuid.Parse(deletionID)
	if err != nil || deletion == uuid.Nil || deletion.String() != deletionID {
		return nil, errors.New("invalid deletion operation ID")
	}
	space, err := uuid.Parse(spaceID)
	if err != nil || space == uuid.Nil || space.String() != spaceID {
		return nil, errors.New("invalid Space ID")
	}
	h := sha256.New()
	_, _ = h.Write([]byte("voice.file.v1.SpaceDeletionReferenceProducer\x00"))
	_, _ = h.Write(deletion[:])
	_, _ = h.Write(space[:])
	var generationBytes [8]byte
	binary.BigEndian.PutUint64(generationBytes[:], generation)
	_, _ = h.Write(generationBytes[:])
	var producerBytes [4]byte
	binary.BigEndian.PutUint32(producerBytes[:], uint32(filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT))
	_, _ = h.Write(producerBytes[:])
	return h.Sum(nil), nil
}

func requireManifestPagePrincipal(ctx context.Context) error {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || (verified.Issuer != "space" && verified.Issuer != "search") || verified.Subject != "service:"+verified.Issuer || verified.Audience != "chat" || verified.RPC != chatv1.ChatService_GetSpacePurgeManifestPage_FullMethodName || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "request-bound manifest reader principal required")
	}
	return nil
}

func requireSpaceLifecyclePrincipal(ctx context.Context, method string) error {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" || verified.Audience != "chat" || verified.RPC != method || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "request-bound Space principal required")
	}
	return nil
}

func mapSpaceLifecycleError(err error) error {
	switch {
	case errors.Is(err, store.ErrSpaceLifecycleRequest):
		return status.Error(codes.InvalidArgument, "invalid Chat lifecycle request")
	case errors.Is(err, store.ErrSpaceLifecycleConflict):
		return status.Error(codes.Aborted, "Chat lifecycle request conflicts with saved evidence")
	case errors.Is(err, store.ErrSpaceLifecycleState):
		return status.Error(codes.FailedPrecondition, "Chat lifecycle transition is not allowed")
	case errors.Is(err, store.ErrSpaceLifecycleEvidence):
		return status.Error(codes.FailedPrecondition, "Chat purge owner evidence is invalid")
	default:
		return status.Error(codes.Unavailable, "Chat lifecycle store unavailable")
	}
}
