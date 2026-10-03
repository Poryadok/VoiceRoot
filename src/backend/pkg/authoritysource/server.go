// Package authoritysource serves protected complete reads from owning services.
package authoritysource

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/pkg/principal"
)

const SchemaVersion = 1
const MaxSubjects = 10000
const MaxStateBytes = 4 << 20

type State struct {
	Complete             bool
	Revision             uint64
	CanonicalState       []byte
	ValidUntilUnixMillis int64
}

type Reader interface {
	ReadAuthoritySnapshot(context.Context, *authorityv1.SourceScope) (State, error)
	ReadAuthorityRevision(context.Context, *authorityv1.SourceScope) (uint64, error)
}

func Audience(owner authorityv1.AuthorityOwner) string {
	switch owner {
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE:
		return "space"
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE:
		return "role"
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_AUTH:
		return "auth"
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER:
		return "user"
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION:
		return "gameintegration"
	default:
		return ""
	}
}

func canonicalID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func validIDs(ids []string) bool {
	if len(ids) > MaxSubjects || !slices.IsSorted(ids) {
		return false
	}
	for index, id := range ids {
		if !canonicalID(id) || (index > 0 && ids[index-1] == id) {
			return false
		}
	}
	return true
}

func ValidateScope(owner authorityv1.AuthorityOwner, scope *authorityv1.SourceScope) error {
	if scope == nil || len(scope.ProtoReflect().GetUnknown()) != 0 || scope.SchemaVersion != SchemaVersion || !canonicalID(scope.SpaceId) || !validIDs(scope.ProfileIds) || !validIDs(scope.AccountIds) || !validIDs(scope.VoiceRoomIds) {
		return status.Error(codes.InvalidArgument, "invalid authority source scope")
	}
	switch owner {
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE, authorityv1.AuthorityOwner_AUTHORITY_OWNER_ROLE:
		if scope.EnvironmentId != "" || len(scope.ProfileIds) != 0 || len(scope.AccountIds) != 0 {
			return status.Error(codes.InvalidArgument, "invalid owner scope shape")
		}
		if owner == authorityv1.AuthorityOwner_AUTHORITY_OWNER_SPACE && len(scope.VoiceRoomIds) != 0 {
			return status.Error(codes.InvalidArgument, "invalid owner scope shape")
		}
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_AUTH:
		if scope.EnvironmentId != "" || len(scope.ProfileIds) != 0 || len(scope.VoiceRoomIds) != 0 {
			return status.Error(codes.InvalidArgument, "invalid owner scope shape")
		}
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_USER:
		if scope.EnvironmentId != "" || len(scope.AccountIds) != 0 || len(scope.VoiceRoomIds) != 0 {
			return status.Error(codes.InvalidArgument, "invalid owner scope shape")
		}
	case authorityv1.AuthorityOwner_AUTHORITY_OWNER_GAME_INTEGRATION:
		if !canonicalID(scope.EnvironmentId) || len(scope.AccountIds) != 0 || len(scope.VoiceRoomIds) != 0 {
			return status.Error(codes.InvalidArgument, "invalid owner scope shape")
		}
	default:
		return status.Error(codes.Unavailable, "authority source owner unavailable")
	}
	return nil
}

func UnaryInterceptor(owner authorityv1.AuthorityOwner, keys principal.KeyResolver, replay principal.ReplayGuard) (grpc.UnaryServerInterceptor, error) {
	if Audience(owner) == "" || keys == nil || replay == nil {
		return nil, errors.New("authority source verifier dependencies required")
	}
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName && info.FullMethod != authorityv1.AuthoritySourceService_ReadRevision_FullMethodName {
			return nil, status.Error(codes.PermissionDenied, "authority source method forbidden")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid source principal")
		}
		message, ok := request.(proto.Message)
		if !ok || message == nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid source request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid source request")
		}
		verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "federation", ExpectedAudience: Audience(owner), ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: keys, ReplayGuard: replay})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid source principal")
		}
		if verified.Subject != "service:federation" {
			return nil, status.Error(codes.PermissionDenied, "source caller forbidden")
		}
		return handler(principal.WithVerified(ctx, verified), request)
	}, nil
}

type Server struct {
	authorityv1.UnimplementedAuthoritySourceServiceServer
	owner  authorityv1.AuthorityOwner
	reader Reader
}

func NewServer(owner authorityv1.AuthorityOwner, reader Reader) (*Server, error) {
	if Audience(owner) == "" || reader == nil {
		return nil, errors.New("authority source owner and reader required")
	}
	return &Server{owner: owner, reader: reader}, nil
}

func (server *Server) require(ctx context.Context, message proto.Message, scope *authorityv1.SourceScope, rpc string) error {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "federation" || verified.Subject != "service:federation" || verified.Audience != Audience(server.owner) || verified.RPC != rpc || !time.Now().Before(verified.ExpiresAt) {
		return status.Error(codes.PermissionDenied, "verified Federation source principal required")
	}
	hash, err := principal.RequestHash(message)
	if err != nil || hash != verified.RequestHash || len(message.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.PermissionDenied, "source request binding mismatch")
	}
	return ValidateScope(server.owner, scope)
}

func (server *Server) ReadSnapshot(ctx context.Context, request *authorityv1.ReadSnapshotRequest) (*authorityv1.ReadSnapshotResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "source request required")
	}
	if err := server.require(ctx, request, request.Scope, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName); err != nil {
		return nil, err
	}
	state, err := server.reader.ReadAuthoritySnapshot(ctx, proto.Clone(request.Scope).(*authorityv1.SourceScope))
	if err != nil {
		return nil, status.Error(codes.Unavailable, "complete owner state unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if err := server.require(ctx, request, request.Scope, authorityv1.AuthoritySourceService_ReadSnapshot_FullMethodName); err != nil {
		return nil, err
	}
	if !state.Complete || state.Revision == 0 || len(state.CanonicalState) == 0 || !utf8.Valid(state.CanonicalState) || !json.Valid(state.CanonicalState) || state.ValidUntilUnixMillis < 0 || (state.ValidUntilUnixMillis != 0 && state.ValidUntilUnixMillis <= time.Now().UnixMilli()) {
		return nil, status.Error(codes.Unavailable, "complete owner state invalid")
	}
	if len(state.CanonicalState) > MaxStateBytes {
		return nil, status.Error(codes.ResourceExhausted, "complete owner state exceeds bound")
	}
	return &authorityv1.ReadSnapshotResponse{Scope: proto.Clone(request.Scope).(*authorityv1.SourceScope), Owner: server.owner, Revision: state.Revision, Complete: true, CanonicalState: slices.Clone(state.CanonicalState), ValidUntilUnixMillis: state.ValidUntilUnixMillis}, nil
}

func (server *Server) ReadRevision(ctx context.Context, request *authorityv1.ReadRevisionRequest) (*authorityv1.ReadRevisionResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "source request required")
	}
	if err := server.require(ctx, request, request.Scope, authorityv1.AuthoritySourceService_ReadRevision_FullMethodName); err != nil {
		return nil, err
	}
	revision, err := server.reader.ReadAuthorityRevision(ctx, proto.Clone(request.Scope).(*authorityv1.SourceScope))
	if err != nil || revision == 0 {
		return nil, status.Error(codes.Unavailable, "owner revision unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if err := server.require(ctx, request, request.Scope, authorityv1.AuthoritySourceService_ReadRevision_FullMethodName); err != nil {
		return nil, err
	}
	return &authorityv1.ReadRevisionResponse{Scope: proto.Clone(request.Scope).(*authorityv1.SourceScope), Owner: server.owner, Revision: revision}, nil
}
