package grpcsvc

import (
 "context"
 "testing"
 "github.com/google/uuid"
 "github.com/stretchr/testify/require"
 "google.golang.org/grpc"
 "google.golang.org/protobuf/types/known/timestamppb"
 chatv1 "voice.app/voice/chat/v1"
 commonv1 "voice.app/voice/common/v1"
 filev1 "voice.app/voice/file/v1"
 "voice/backend/pkg/principal"
)
type producerFenceStore struct{spacePurgeStoreFake; applied int}
func(s *producerFenceStore)ApplySpaceLifecycleFence(context.Context,*chatv1.ApplySpaceLifecycleFenceRequest)(*chatv1.ApplySpaceLifecycleFenceResponse,error){s.applied++;return &chatv1.ApplySpaceLifecycleFenceResponse{Receipt:&commonv1.SpaceLifecycleFenceReceipt{}},nil}
type chatProducerFile struct{spacePurgeFileFake; requests []*filev1.RegisterSpaceDeletionReferenceChunkRequest; corrupt bool}
func(f *chatProducerFile)RegisterSpaceDeletionReferenceChunk(_ context.Context,r *filev1.RegisterSpaceDeletionReferenceChunkRequest,_ ...grpc.CallOption)(*filev1.RegisterSpaceDeletionReferenceChunkResponse,error){
 f.requests=append(f.requests,r)
 receipt:=&filev1.RegisterSpaceDeletionReferenceChunkReceipt{ProtocolVersion:1,ReceiptId:uuid.NewString(),OperationId:r.OperationId,DeletionOperationId:r.DeletionOperationId,SpaceId:r.SpaceId,ScheduleGeneration:r.ScheduleGeneration,ProducerId:r.ProducerId,ExpectedReferencesSha256:r.ExpectedReferencesSha256,ProducerSealed:true,RequestSha256:lifecycleDomainDigest(string(r.ProtoReflect().Descriptor().FullName()),r),CompletedAt:timestamppb.Now()}
 if f.corrupt {receipt.AcceptedCount=1}
 return &filev1.RegisterSpaceDeletionReferenceChunkResponse{Receipt:receipt},nil
}
func TestSpaceFreezeRequiresExactChatProducerSealBeforeAcknowledgement(t *testing.T){
 store:=&producerFenceStore{};files:=&chatProducerFile{}
 service:=&SpaceLifecycleGRPC{Store:store,Issuer:mustTestSpacePurgeIssuer(t),File:files}
 request:=&chatv1.ApplySpaceLifecycleFenceRequest{Fence:&commonv1.SpaceLifecycleFenceRequest{SpaceId:uuid.NewString(),DeletionOperationId:uuid.NewString(),Generation:7,DesiredState:commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN}}
 ctx:=principal.WithVerified(context.Background(),principal.Principal{Kind:"service",Issuer:"space",Subject:"service:space",Audience:"chat",RPC:chatv1.ChatService_ApplySpaceLifecycleFence_FullMethodName})
 files.corrupt=true;_,err:=service.ApplySpaceLifecycleFence(ctx,request);require.Error(t,err)
 files.corrupt=false;_,err=service.ApplySpaceLifecycleFence(ctx,request);require.NoError(t,err)
 require.Equal(t,files.requests[0],files.requests[1],"ambiguous seal retries identical producer bytes")
 require.Empty(t,files.requests[1].References);require.True(t,files.requests[1].SealsProducer)
}
