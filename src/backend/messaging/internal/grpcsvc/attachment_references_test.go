package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	filev1 "voice.app/voice/file/v1"
)

type attachmentReferenceStub struct {
	filev1.FileServiceClient
	mutate func(*filev1.AcquireFileReferencesReceipt)
	t      *testing.T
}

func (s *attachmentReferenceStub) AcquireFileReferences(ctx context.Context, r *filev1.AcquireFileReferencesRequest, _ ...grpc.CallOption) (*filev1.AcquireFileReferencesResponse, error) {
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(s.t, ok)
	require.Equal(s.t, []string{r.OperationId}, md.Get("x-request-id"))
	require.Len(s.t, md.Get("authorization"), 1)
	receipt := &filev1.AcquireFileReferencesReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: r.OperationId, ProducerId: r.ProducerId, ReferenceCount: uint64(len(r.References)), RequestSha256: lifecycleHash(r), CompletedAt: timestamppb.Now()}
	if s.mutate != nil {
		s.mutate(receipt)
	}
	return &filev1.AcquireFileReferencesResponse{Receipt: receipt}, nil
}

func TestAttachmentReferenceAcquisitionRequiresExactOwnerReceipt(t *testing.T) {
	issuer := testSpaceFileProducer(t).Issuer
	request := &filev1.AcquireFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{{FileId: uuid.NewString(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString()}}}
	cases := []struct {
		name   string
		mutate func(*filev1.AcquireFileReferencesReceipt)
	}{
		{"valid", nil},
		{"operation", func(r *filev1.AcquireFileReferencesReceipt) { r.OperationId = uuid.NewString() }},
		{"producer", func(r *filev1.AcquireFileReferencesReceipt) {
			r.ProducerId = filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT
		}},
		{"count", func(r *filev1.AcquireFileReferencesReceipt) { r.ReferenceCount = 0 }},
		{"hash", func(r *filev1.AcquireFileReferencesReceipt) { r.RequestSha256 = make([]byte, 32) }},
		{"completion", func(r *filev1.AcquireFileReferencesReceipt) { r.CompletedAt = nil }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			coordinator := &AttachmentReferenceCoordinator{Issuer: issuer, Files: &attachmentReferenceStub{t: t, mutate: test.mutate}}
			err := coordinator.Acquire(context.Background(), request)
			if test.mutate == nil {
				require.NoError(t, err)
			} else {
				require.Equal(t, codes.DataLoss, status.Code(err))
			}
		})
	}
}
