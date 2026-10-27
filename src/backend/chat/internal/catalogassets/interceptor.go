package catalogassets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type Authenticator struct {
	Keys         principal.KeyResolver
	Replay       principal.ReplayGuard
	SessionEpoch principal.SessionEpochChecker
	Clock        func() time.Time
}

type callerContextKey struct{}
type verifiedCaller struct {
	principal   principal.Principal
	requestHash string
	method      string
}

func (a Authenticator) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info == nil || (info.FullMethod != chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName && info.FullMethod != chatv1.CatalogAssetService_ValidateGifAsset_FullMethodName && info.FullMethod != chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName) {
			return nil, status.Error(codes.PermissionDenied, "method unavailable on catalog listener")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil || !canonicalID(transport.RequestID) {
			return nil, status.Error(codes.Unauthenticated, "invalid catalog principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || hasUnknown(message) {
			return nil, status.Error(codes.InvalidArgument, "invalid catalog request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid catalog request")
		}
		if a.Keys == nil || a.Replay == nil {
			return nil, status.Error(codes.Unavailable, "catalog verifier unavailable")
		}
		cfg := principal.VerifyConfig{ExpectedAudience: "chat", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: a.Keys, ReplayGuard: a.Replay, Clock: a.Clock}
		var verified principal.Principal
		if info.FullMethod == chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName {
			if a.SessionEpoch == nil {
				return nil, status.Error(codes.Unavailable, "catalog session authority unavailable")
			}
			cfg.ExpectedIssuer = "gateway"
			cfg.SessionEpochChecker = a.SessionEpoch
			verified, err = principal.VerifyDelegatedUser(ctx, transport.BearerToken, cfg)
			if err == nil && (!canonicalID(verified.AccountID) || !canonicalID(verified.ProfileID)) {
				err = errors.New("invalid actor")
			}
		} else {
			cfg.ExpectedIssuer = "messaging"
			if info.FullMethod == chatv1.CatalogAssetService_AuthorizeCatalogDestination_FullMethodName {
				cfg.ExpectedIssuer = "file"
			}
			verified, err = principal.VerifyService(ctx, transport.BearerToken, cfg)
			if err == nil && (verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 || verified.AccountType != "") {
				err = errors.New("mixed actor")
			}
		}
		if err != nil {
			if status.Code(err) == codes.Unavailable {
				return nil, status.Error(codes.Unavailable, "catalog verifier unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid catalog principal")
		}
		// A private capability prevents another listener's generic verified principal
		// context from reaching the asset handlers through an accidental registration.
		ctx = context.WithValue(ctx, callerContextKey{}, verifiedCaller{principal: verified, requestHash: hash, method: info.FullMethod})
		ctx, cancel := context.WithDeadline(ctx, verified.ExpiresAt)
		defer cancel()
		return handler(ctx, request)
	}
}

func canonicalID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}

func hasUnknown(message proto.Message) bool {
	reflect := message.ProtoReflect()
	if len(reflect.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	reflect.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() && field.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				unknown = hasUnknown(item.Message().Interface())
				return !unknown
			})
		} else if field.IsList() && field.Message() != nil {
			list := value.List()
			for i := 0; i < list.Len() && !unknown; i++ {
				unknown = hasUnknown(list.Get(i).Message().Interface())
			}
		} else if field.Message() != nil {
			unknown = hasUnknown(value.Message().Interface())
		}
		return !unknown
	})
	return unknown
}
