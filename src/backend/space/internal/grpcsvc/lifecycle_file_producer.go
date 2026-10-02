package grpcsvc

import (
 "bytes"
 "context"
 "fmt"

 "github.com/google/uuid"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
 "google.golang.org/protobuf/proto"
 filev1 "voice.app/voice/file/v1"
 "voice/backend/pkg/filerefmanifest"
 "voice/backend/space/internal/spacecore"
)

func lifecycleDomainDigest(request proto.Message)[]byte{
 wire,err:=(proto.MarshalOptions{Deterministic:true}).Marshal(request);if err!=nil{return nil}
 digest:=lifecycleEvidenceHash(string(request.ProtoReflect().Descriptor().FullName()),wire);return digest[:]
}

// The current Space schema stores icon_url, not File-reference keys. Its
// declared producer set is empty. File validates and owns the aggregate root.
func (o *LifecycleManifestOwners) SealSpaceFileProducer(ctx context.Context,snapshot spacecore.LifecycleSnapshot) error {
 if o==nil || o.File==nil {return status.Error(codes.Unavailable,"Space File producer unavailable")}
 operation,err:=uuid.Parse(snapshot.DeletionOperationID);if err!=nil{return err};space,err:=uuid.Parse(snapshot.SpaceID);if err!=nil{return err}
 digest,err:=filerefmanifest.Hash(operation,space,snapshot.Generation,1,nil);if err!=nil{return err}
 id:=uuid.NewSHA1(operation,[]byte(fmt.Sprintf("space.file-producer.v1\x00%d",snapshot.Generation))).String()
 request:=&filev1.RegisterSpaceDeletionReferenceChunkRequest{ProtocolVersion:1,OperationId:id,ProducerId:filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE,DeletionOperationId:snapshot.DeletionOperationID,SpaceId:snapshot.SpaceID,ScheduleGeneration:snapshot.Generation,ExpectedReferencesSha256:digest,SealsProducer:true}
 signed,err:=o.signed(ctx,"file",filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName,request,id);if err!=nil{return err}
 response,err:=o.File.RegisterSpaceDeletionReferenceChunk(signed,request);if err!=nil{return err};r:=response.GetReceipt()
 if r==nil || r.ProtocolVersion!=1 || r.ReceiptId=="" || r.OperationId!=id || r.DeletionOperationId!=request.DeletionOperationId || r.SpaceId!=request.SpaceId || r.ScheduleGeneration!=request.ScheduleGeneration || r.ChunkIndex!=0 || r.AcceptedCount!=0 || r.ProducerId!=request.ProducerId || r.ExpectedTotalCount!=0 || !bytes.Equal(r.ExpectedReferencesSha256,digest) || !r.ProducerSealed || !bytes.Equal(r.RequestSha256,lifecycleDomainDigest(request)) || r.CompletedAt==nil || r.CompletedAt.CheckValid()!=nil{return status.Error(codes.DataLoss,"File Space producer seal mismatch")}
 return nil
}

func (o *LifecycleManifestOwners) ReleaseSpaceFileProducer(ctx context.Context,snapshot spacecore.LifecycleSnapshot)error {
 if o==nil || o.File==nil || snapshot.Generation<2{return status.Error(codes.Unavailable,"Space File producer unavailable")}
 digest,err:=filerefmanifest.Hash(uuid.MustParse(snapshot.DeletionOperationID),uuid.MustParse(snapshot.SpaceID),snapshot.Generation-1,1,nil);if err!=nil{return err}
 request:=&filev1.ReleaseSpaceDeletionProducerReferencesRequest{ProtocolVersion:1,DeletionOperationId:snapshot.DeletionOperationID,SpaceId:snapshot.SpaceID,PurgeGeneration:snapshot.Generation,SourceScheduleGeneration:snapshot.Generation-1,ProducerId:filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE,ExpectedReferencesSha256:digest}
 signed,err:=o.signed(ctx,"file",filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName,request,snapshot.DeletionOperationID);if err!=nil{return err}
 response,err:=o.File.ReleaseSpaceDeletionProducerReferences(signed,request);if err!=nil{return err};r:=response.GetReceipt()
 if r==nil || r.ProtocolVersion!=1 || r.ReceiptId=="" || r.DeletionOperationId!=request.DeletionOperationId || r.SpaceId!=request.SpaceId || r.PurgeGeneration!=request.PurgeGeneration || r.SourceScheduleGeneration!=request.SourceScheduleGeneration || r.ProducerId!=request.ProducerId || !bytes.Equal(r.ExpectedReferencesSha256,digest) || !bytes.Equal(r.RequestSha256,lifecycleDomainDigest(request)) || r.CompletedAt==nil || r.CompletedAt.CheckValid()!=nil{return status.Error(codes.DataLoss,"File Space producer release mismatch")}
 return nil
}
