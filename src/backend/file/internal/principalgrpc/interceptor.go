package principalgrpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

type Verifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func IsProtectedMethod(method string) bool {
	switch method {
	case filev1.FileService_ValidateStoryMedia_FullMethodName,
		filev1.FileService_AcquireFileReferences_FullMethodName,
		filev1.FileService_ReleaseFileReferences_FullMethodName,
		filev1.FileService_GetFileReferenceGCStatus_FullMethodName,
		filev1.FileService_IssueFileAccessCapability_FullMethodName,
		filev1.FileService_GetSpacePurgeReceipt_FullMethodName,
		filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName,
		filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName,
		filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName,
		filev1.FileService_ApplySpaceLifecycleFence_FullMethodName,
		filev1.FileService_PurgeSpace_FullMethodName:
		return true
	default:
		return false
	}
}

func OrdinaryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if IsProtectedMethod(info.FullMethod) {
			return nil, status.Error(codes.Unavailable, "protected method unavailable on ordinary listener")
		}
		return handler(ctx, request)
	}
}

func StrictUnaryInterceptor(verifier Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		expectedIssuer := expectedIssuerForRequest(info.FullMethod, request)
		if expectedIssuer == "" {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on protected listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || verifier == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || hasUnknownFields(message) {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		p, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if p.Kind != "service" || p.Subject != "service:"+expectedIssuer || p.Issuer != expectedIssuer || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
			return nil, status.Error(codes.PermissionDenied, expectedIssuer+" service required")
		}
		if p.Audience != "file" || p.RPC != info.FullMethod || p.RequestID != transport.RequestID || p.RequestHash != hash {
			return nil, status.Error(codes.Unauthenticated, "invalid principal binding")
		}
		return handler(principal.WithVerified(ctx, p), request)
	}
}

func expectedIssuerForMethod(method string) string {
	switch method {
	case filev1.FileService_ValidateStoryMedia_FullMethodName:
		return "story"
	case filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, filev1.FileService_PurgeSpace_FullMethodName, filev1.FileService_GetSpacePurgeReceipt_FullMethodName:
		return "space"
	case filev1.FileService_AcquireFileReferences_FullMethodName,
		filev1.FileService_ReleaseFileReferences_FullMethodName,
		filev1.FileService_GetFileReferenceGCStatus_FullMethodName:
		return "messaging"
	default:
		return ""
	}
}

func expectedIssuerForRequest(method string, request any) string {
	var producer filev1.FileReferenceProducerId
	switch method {
	case filev1.FileService_IssueFileAccessCapability_FullMethodName:
		r, ok := request.(*filev1.IssueFileAccessCapabilityRequest)
		if !ok {
			return ""
		}
		switch r.GetReference().GetOwnerType() {
		case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STICKER, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_GIF_ASSET:
			return "messaging"
		case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR:
			return "user"
		case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR:
			return "chat"
		case filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR:
			return "space"
		default:
			return ""
		}
	case filev1.FileService_AcquireFileReferences_FullMethodName:
		r, ok := request.(*filev1.AcquireFileReferencesRequest)
		if !ok {
			return ""
		}
		producer = r.GetProducerId()
	case filev1.FileService_ReleaseFileReferences_FullMethodName:
		r, ok := request.(*filev1.ReleaseFileReferencesRequest)
		if !ok {
			return ""
		}
		producer = r.GetProducerId()
	case filev1.FileService_GetFileReferenceGCStatus_FullMethodName:
		r, ok := request.(*filev1.GetFileReferenceGCStatusRequest)
		if !ok {
			return ""
		}
		producer = r.GetProducerId()
	case filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName:
		r, ok := request.(*filev1.RegisterSpaceDeletionReferenceChunkRequest)
		if !ok {
			return ""
		}
		producer = r.GetProducerId()
	case filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName:
		r, ok := request.(*filev1.ReleaseSpaceDeletionProducerReferencesRequest)
		if !ok {
			return ""
		}
		producer = r.GetProducerId()
	default:
		return expectedIssuerForMethod(method)
	}
	switch producer {
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE:
		return "space"
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT:
		return "chat"
	case filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING:
		return "messaging"
	default:
		return ""
	}
}

func hasUnknownFields(message proto.Message) bool {
	reflect := message.ProtoReflect()
	if len(reflect.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	reflect.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() {
			if field.MapValue().Message() != nil {
				value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
					unknown = hasUnknownFields(item.Message().Interface())
					return !unknown
				})
			}
		} else if field.IsList() {
			if field.Message() != nil {
				list := value.List()
				for i := 0; i < list.Len() && !unknown; i++ {
					unknown = hasUnknownFields(list.Get(i).Message().Interface())
				}
			}
		} else if field.Message() != nil {
			unknown = hasUnknownFields(value.Message().Interface())
		}
		return !unknown
	})
	return unknown
}

// Unavailable preserves dependency errors for the common invalid-credential deny path.
func Unavailable(err error) error { return err }
