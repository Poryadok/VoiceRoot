package lifecyclecoord

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// LifecycleRPCInvoker is implemented by grpc.ClientConn and keeps the
// coordinator independent of each service's generated wrapper types.
type LifecycleRPCInvoker interface {
	Invoke(context.Context, string, any, any, ...grpc.CallOption) error
}

type RoleRetirementClient interface {
	RetireSpace(context.Context, *rolev1.RetireSpaceRequest, ...grpc.CallOption) (*rolev1.RetireSpaceResponse, error)
}

// GRPCRoleRetirementParticipant signs the exact typed request and calls Role's
// protected retirement RPC over the caller-provided authenticated connection.
type GRPCRoleRetirementParticipant struct {
	Issuer *principal.Issuer
	Client RoleRetirementClient
}

func (p *GRPCRoleRetirementParticipant) RetireSpace(ctx context.Context, request *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceReceipt, error) {
	if p == nil || p.Issuer == nil || p.Client == nil || request == nil {
		return nil, status.Error(codes.Unavailable, "Role retirement transport unavailable")
	}
	operationID, err := uuid.Parse(request.GetDeletionOperationId())
	if request.GetProtocolVersion() != 1 || err != nil || operationID == uuid.Nil || operationID.String() != request.GetDeletionOperationId() ||
		request.GetGeneration() == 0 || request.GetPurgeDecidedAt() == nil || request.GetManifest() == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Role retirement request binding")
	}
	requestID, err := participantLifecycleRequestID("role", request.GetDeletionOperationId(), request.GetGeneration(), "retire")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Role retirement request binding")
	}
	ctx, err = signedLifecycleContextForAudience(ctx, p.Issuer, "role", rolev1.RoleService_RetireSpace_FullMethodName, requestID, request)
	if err != nil {
		return nil, err
	}
	response, err := p.Client.RetireSpace(ctx, request)
	if err != nil {
		return nil, err
	}
	if response == nil || response.GetReceipt() == nil {
		return nil, status.Error(codes.DataLoss, "Role omitted its retirement receipt")
	}
	return response.GetReceipt(), nil
}

// GRPCParticipant adapts a service's protected lifecycle RPCs. Service
// descriptors are registered by the generated proto packages imported by the
// Space runtime; requests and receipts remain wire-compatible protobufs.
type GRPCParticipant struct {
	Issuer        *principal.Issuer
	Client        LifecycleRPCInvoker
	ParticipantID commonv1.ParticipantId
	Audience      string
	FenceMethod   string
	PurgeMethod   string
}

var lifecycleParticipantServices = map[commonv1.ParticipantId]struct {
	audience    string
	serviceName string
}{
	commonv1.ParticipantId_PARTICIPANT_ID_ROLE:         {audience: "role", serviceName: "voice.role.v1.RoleService"},
	commonv1.ParticipantId_PARTICIPANT_ID_CHAT:         {audience: "chat", serviceName: "voice.chat.v1.ChatService"},
	commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING:    {audience: "messaging", serviceName: "voice.messaging.v1.MessagingService"},
	commonv1.ParticipantId_PARTICIPANT_ID_FILE:         {audience: "file", serviceName: "voice.file.v1.FileService"},
	commonv1.ParticipantId_PARTICIPANT_ID_VOICE:        {audience: "voice", serviceName: "voice.calls.v1.VoiceService"},
	commonv1.ParticipantId_PARTICIPANT_ID_MATCHMAKING:  {audience: "matchmaking", serviceName: "voice.matchmaking.v1.MatchmakingService"},
	commonv1.ParticipantId_PARTICIPANT_ID_SEARCH:       {audience: "search", serviceName: "voice.search.v1.SearchService"},
	commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION: {audience: "subscription", serviceName: "voice.subscription.v1.SubscriptionService"},
	commonv1.ParticipantId_PARTICIPANT_ID_BOT:          {audience: "bot", serviceName: "voice.bot.v1.BotService"},
	commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION: {audience: "notification", serviceName: "voice.notification.v1.NotificationService"},
}

func NewGRPCParticipant(id commonv1.ParticipantId, issuer *principal.Issuer, client LifecycleRPCInvoker) (*GRPCParticipant, error) {
	binding, ok := lifecycleParticipantServices[id]
	if !ok || issuer == nil || client == nil {
		return nil, ErrCoordinatorNotConfigured
	}
	p := &GRPCParticipant{Issuer: issuer, Client: client, ParticipantID: id, Audience: binding.audience, FenceMethod: "/" + binding.serviceName + "/ApplySpaceLifecycleFence", PurgeMethod: "/" + binding.serviceName + "/PurgeSpace"}
	if _, _, err := lifecycleMessages(p.FenceMethod, "fence", &commonv1.SpaceLifecycleFenceRequest{}, "receipt", &commonv1.SpaceLifecycleFenceReceipt{}); err != nil {
		return nil, err
	}
	if id != commonv1.ParticipantId_PARTICIPANT_ID_ROLE {
		if _, _, err := lifecycleMessages(p.PurgeMethod, "purge", &commonv1.SpacePurgeRequest{}, "receipt", &commonv1.SpacePurgeReceipt{}); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *GRPCParticipant) ApplySpaceLifecycleFence(ctx context.Context, fence *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	if p == nil || fence == nil {
		return nil, status.Error(codes.InvalidArgument, "lifecycle fence unavailable")
	}
	requestID, err := participantLifecycleRequestID(p.Audience, fence.GetDeletionOperationId(), fence.GetGeneration(), fmt.Sprintf("fence-%d", fence.GetDesiredState()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle request binding")
	}
	response, err := p.invoke(ctx, p.FenceMethod, "fence", requestID, fence, "receipt", &commonv1.SpaceLifecycleFenceReceipt{})
	if err != nil {
		return nil, err
	}
	return response.(*commonv1.SpaceLifecycleFenceReceipt), nil
}

func (p *GRPCParticipant) PurgeSpace(ctx context.Context, purge *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	if p == nil || purge == nil {
		return nil, status.Error(codes.InvalidArgument, "lifecycle purge unavailable")
	}
	if purge.GetParticipantId() != p.ParticipantID {
		return nil, status.Error(codes.InvalidArgument, "lifecycle purge participant mismatch")
	}
	requestID, err := participantLifecycleRequestID(p.Audience, purge.GetDeletionOperationId(), purge.GetGeneration(), "purge")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid purge request binding")
	}
	response, err := p.invoke(ctx, p.PurgeMethod, "purge", requestID, purge, "receipt", &commonv1.SpacePurgeReceipt{})
	if err != nil {
		return nil, err
	}
	return response.(*commonv1.SpacePurgeReceipt), nil
}

func (p *GRPCParticipant) invoke(ctx context.Context, method, inputField, requestID string, payload proto.Message, outputField string, output proto.Message) (proto.Message, error) {
	if p == nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle participant transport unavailable")
	}
	binding, known := lifecycleParticipantServices[p.ParticipantID]
	serviceName := ""
	fenceMethod, purgeMethod := "", ""
	if known {
		serviceName = binding.serviceName
		fenceMethod = "/" + serviceName + "/ApplySpaceLifecycleFence"
		purgeMethod = "/" + serviceName + "/PurgeSpace"
	}
	if p.Issuer == nil || p.Client == nil || !known || p.Audience != binding.audience || payload == nil || output == nil ||
		(method != fenceMethod && method != purgeMethod) || p.FenceMethod != fenceMethod || p.PurgeMethod != purgeMethod {
		return nil, status.Error(codes.Unavailable, "Space lifecycle participant transport unavailable")
	}
	request, response, err := lifecycleMessages(method, inputField, payload, outputField, output)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Space lifecycle participant contract unavailable")
	}
	signed, err := signedLifecycleContextForAudience(ctx, p.Issuer, p.Audience, method, requestID, request)
	if err != nil {
		return nil, err
	}
	if err := p.Client.Invoke(signed, method, request, response); err != nil {
		return nil, err
	}
	return lifecycleResponse(response, outputField, output)
}

func participantLifecycleRequestID(audience, operationID string, generation uint64, action string) (string, error) {
	operation, err := uuid.Parse(operationID)
	if err != nil || operation == uuid.Nil || operation.String() != operationID || generation == 0 || audience == "" || action == "" {
		return "", fmt.Errorf("invalid lifecycle operation identity")
	}
	// The protected participant listener binds request_id to the business
	// deletion operation. RPC and deterministic body hashes already bind the
	// audience, action and generation; each retry gets a fresh signed token/JTI.
	return operation.String(), nil
}

func lifecycleMessages(method, inputField string, payload proto.Message, outputField string, output proto.Message) (proto.Message, proto.Message, error) {
	if !strings.HasPrefix(method, "/") {
		return nil, nil, fmt.Errorf("invalid lifecycle RPC method")
	}
	serviceName, methodName, ok := strings.Cut(strings.TrimPrefix(method, "/"), "/")
	if !ok || serviceName == "" || methodName == "" {
		return nil, nil, fmt.Errorf("invalid lifecycle RPC method")
	}
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, nil, err
	}
	service, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, nil, fmt.Errorf("lifecycle RPC service descriptor unavailable")
	}
	rpc := service.Methods().ByName(protoreflect.Name(methodName))
	if rpc == nil {
		return nil, nil, fmt.Errorf("lifecycle RPC method descriptor unavailable")
	}
	input := dynamicpb.NewMessage(rpc.Input())
	field := input.Descriptor().Fields().ByName(protoreflect.Name(inputField))
	if field == nil || field.Kind() != protoreflect.MessageKind || field.Message().FullName() != payload.ProtoReflect().Descriptor().FullName() {
		return nil, nil, fmt.Errorf("lifecycle RPC input binding mismatch")
	}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	inner := dynamicpb.NewMessage(field.Message())
	if err := proto.Unmarshal(encoded, inner); err != nil {
		return nil, nil, err
	}
	input.Set(field, protoreflect.ValueOfMessage(inner))
	response := dynamicpb.NewMessage(rpc.Output())
	responseField := response.Descriptor().Fields().ByName(protoreflect.Name(outputField))
	if responseField == nil || responseField.Kind() != protoreflect.MessageKind || responseField.Message().FullName() != output.ProtoReflect().Descriptor().FullName() {
		return nil, nil, fmt.Errorf("lifecycle RPC output binding mismatch")
	}
	return input, response, nil
}

func lifecycleResponse(response proto.Message, fieldName string, output proto.Message) (proto.Message, error) {
	field := response.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field == nil || !response.ProtoReflect().Has(field) {
		return nil, status.Error(codes.DataLoss, "lifecycle participant omitted its receipt")
	}
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response.ProtoReflect().Get(field).Message().Interface())
	if err != nil {
		return nil, status.Error(codes.DataLoss, "lifecycle participant returned an invalid receipt")
	}
	if err := proto.Unmarshal(payload, output); err != nil {
		return nil, status.Error(codes.DataLoss, "lifecycle participant returned an invalid receipt")
	}
	return output, nil
}
