package grpcsvc

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/filerefmanifest"
	"voice/backend/pkg/principal"
)

type SpaceFileProducerStore interface {
	SpaceFileProducerReferences(context.Context, uuid.UUID, uuid.UUID, uint64) ([]*filev1.FileReferenceKey, error)
}
type SpaceFileProducerOwner interface {
	RegisterSpaceDeletionReferenceChunk(context.Context, *filev1.RegisterSpaceDeletionReferenceChunkRequest, ...grpc.CallOption) (*filev1.RegisterSpaceDeletionReferenceChunkResponse, error)
	ReleaseSpaceDeletionProducerReferences(context.Context, *filev1.ReleaseSpaceDeletionProducerReferencesRequest, ...grpc.CallOption) (*filev1.ReleaseSpaceDeletionProducerReferencesResponse, error)
}
type SpaceFileProducerCoordinator struct {
	Store  SpaceFileProducerStore
	Files  SpaceFileProducerOwner
	Issuer *principal.Issuer
}

func (c *SpaceFileProducerCoordinator) references(ctx context.Context, space, operation string, generation uint64) ([]*filev1.FileReferenceKey, []byte, error) {
	if c == nil || c.Store == nil || c.Files == nil || c.Issuer == nil {
		return nil, nil, status.Error(codes.Unavailable, "Messaging File producer unavailable")
	}
	spaceID, err := canonicalSpaceLifecycleUUID(space)
	if err != nil {
		return nil, nil, err
	}
	operationID, err := canonicalSpaceLifecycleUUID(operation)
	if err != nil {
		return nil, nil, err
	}
	original, err := c.Store.SpaceFileProducerReferences(ctx, spaceID, operationID, generation)
	if err != nil {
		return nil, nil, err
	}
	refs := make([]*filev1.FileReferenceKey, 0, len(original))
	for _, ref := range original {
		if ref == nil || ref.GetScopeSpaceId() != space || ref.OwnerType != filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE {
			return nil, nil, status.Error(codes.DataLoss, "invalid frozen Messaging File reference")
		}
		if _, err := canonicalSpaceLifecycleUUID(ref.FileId); err != nil {
			return nil, nil, err
		}
		if _, err := canonicalSpaceLifecycleUUID(ref.OwnerId); err != nil {
			return nil, nil, err
		}
		refs = append(refs, proto.Clone(ref).(*filev1.FileReferenceKey))
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].FileId != refs[j].FileId {
			return refs[i].FileId < refs[j].FileId
		}
		return refs[i].OwnerId < refs[j].OwnerId
	})
	wire := make([][]byte, 0, len(refs))
	for i, ref := range refs {
		if i > 0 && proto.Equal(refs[i-1], ref) {
			return nil, nil, status.Error(codes.DataLoss, "duplicate frozen File reference")
		}
		raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(ref)
		if err != nil {
			return nil, nil, err
		}
		wire = append(wire, raw)
	}
	digest, err := filerefmanifest.Hash(operationID, spaceID, generation, 3, wire)
	return refs, digest, err
}
func (c *SpaceFileProducerCoordinator) signed(ctx context.Context, method, id string, request proto.Message) (context.Context, context.CancelFunc, error) {
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, nil, err
	}
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "file", RPC: method, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, nil, err
	}
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), cancel, nil
}
func (c *SpaceFileProducerCoordinator) Seal(ctx context.Context, fence *commonv1.SpaceLifecycleFenceRequest) error {
	refs, digest, err := c.references(ctx, fence.SpaceId, fence.DeletionOperationId, fence.Generation)
	if err != nil {
		return err
	}
	chunks := (len(refs) + 999) / 1000
	if chunks == 0 {
		chunks = 1
	}
	for index := 0; index < chunks; index++ {
		end := (index + 1) * 1000
		if end > len(refs) {
			end = len(refs)
		}
		id := uuid.NewSHA1(uuid.MustParse(fence.DeletionOperationId), []byte(fmt.Sprintf("messaging.file-producer.v1\x00%d\x00%d", fence.Generation, index))).String()
		request := &filev1.RegisterSpaceDeletionReferenceChunkRequest{ProtocolVersion: 1, OperationId: id, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, DeletionOperationId: fence.DeletionOperationId, SpaceId: fence.SpaceId, ScheduleGeneration: fence.Generation, ChunkIndex: uint64(index), References: refs[index*1000 : end], ExpectedTotalCount: uint64(len(refs)), ExpectedReferencesSha256: digest, SealsProducer: index+1 == chunks}
		call, cancel, err := c.signed(ctx, filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, id, request)
		if err != nil {
			return err
		}
		response, err := c.Files.RegisterSpaceDeletionReferenceChunk(call, request)
		cancel()
		if err != nil {
			return err
		}
		receipt := response.GetReceipt()
		if receipt == nil || receipt.ProtocolVersion != 1 || receipt.ReceiptId == "" || receipt.OperationId != id || receipt.DeletionOperationId != request.DeletionOperationId || receipt.SpaceId != request.SpaceId || receipt.ScheduleGeneration != request.ScheduleGeneration || receipt.ChunkIndex != request.ChunkIndex || receipt.AcceptedCount != uint64(end-index*1000) || receipt.ProducerId != request.ProducerId || receipt.ExpectedTotalCount != request.ExpectedTotalCount || !bytes.Equal(receipt.ExpectedReferencesSha256, digest) || receipt.ProducerSealed != request.SealsProducer || !bytes.Equal(receipt.RequestSha256, lifecycleHash(request)) || receipt.CompletedAt == nil || receipt.CompletedAt.CheckValid() != nil {
			return status.Error(codes.DataLoss, "File producer seal receipt mismatch")
		}
	}
	return nil
}

type spaceFileReleaseEvidenceKey struct{}
type spaceFileReleaseEvidence struct {
	space      string
	hash       []byte
	references map[string]bool
}

func (c *SpaceFileProducerCoordinator) Release(ctx context.Context, purge *commonv1.SpacePurgeRequest) error {
	_, err := c.ReleaseContext(ctx, purge)
	return err
}

// ReleaseContext returns private evidence only after File accepts the complete
// immutable producer set under its matching PURGE_DECIDED fence. Physical GC
// belongs to File; a Space child must not call the ordinary LIVE-only release.
func (c *SpaceFileProducerCoordinator) ReleaseContext(ctx context.Context, purge *commonv1.SpacePurgeRequest) (context.Context, error) {
	refs, digest, err := c.references(ctx, purge.SpaceId, purge.DeletionOperationId, purge.Generation-1)
	if err != nil {
		return nil, err
	}
	request := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{ProtocolVersion: 1, SpaceId: purge.SpaceId, DeletionOperationId: purge.DeletionOperationId, PurgeGeneration: purge.Generation, SourceScheduleGeneration: purge.Generation - 1, ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, ExpectedReferencesSha256: digest}
	call, cancel, err := c.signed(ctx, filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, purge.DeletionOperationId, request)
	if err != nil {
		return nil, err
	}
	defer cancel()
	response, err := c.Files.ReleaseSpaceDeletionProducerReferences(call, request)
	if err != nil {
		return nil, err
	}
	r := response.GetReceipt()
	if r == nil || r.ProtocolVersion != 1 || r.ReceiptId == "" || r.DeletionOperationId != purge.DeletionOperationId || r.SpaceId != purge.SpaceId || r.PurgeGeneration != purge.Generation || r.SourceScheduleGeneration != purge.Generation-1 || r.ProducerId != request.ProducerId || r.ReleasedCount > uint64(len(refs)) || !bytes.Equal(r.ExpectedReferencesSha256, digest) || !bytes.Equal(r.RequestSha256, lifecycleHash(request)) || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil {
		return nil, status.Error(codes.DataLoss, "File producer release receipt mismatch")
	}
	receiptHash, err := responseHash(response)
	if err != nil {
		return nil, err
	}
	evidence := spaceFileReleaseEvidence{space: purge.SpaceId, hash: receiptHash, references: map[string]bool{}}
	for _, ref := range refs {
		raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(ref)
		if err != nil {
			return nil, err
		}
		evidence.references[string(raw)] = true
	}
	return context.WithValue(ctx, spaceFileReleaseEvidenceKey{}, evidence), nil
}
