package lifecycleprincipal

import (
	"context"
	"errors"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
)

const admissionRPCPrefix = "/voice.space.v1.SpaceService/"

func admissionMethod(method string) bool {
	switch method {
	case admissionRPCPrefix + "StartJoinAttempt", admissionRPCPrefix + "GetJoinAttempt", admissionRPCPrefix + "SubmitJoinAttempt", admissionRPCPrefix + "ListJoinRequests", admissionRPCPrefix + "GetJoinRequest", admissionRPCPrefix + "ApproveJoinRequest", admissionRPCPrefix + "JoinSpace", admissionRPCPrefix + "JoinByInvite":
		return true
	}
	return false
}

type admissionCallKey struct{}
type admissionCallProof struct {
	actor        principal.Principal
	method       string
	message      protoreflect.FullName
	clock        func() time.Time
	ownerStarted time.Time
}

// VerifiedAdmissionCall proves only exact interactive request authentication.
// It cannot establish account/profile eligibility, local policy, an approval,
// Role readiness or a durable FIRST_PUBLISH continuation.
type VerifiedAdmissionCall struct{ proof admissionCallProof }

func validAdmissionMessage(message proto.Message, depth int) bool {
	if message == nil || !message.ProtoReflect().IsValid() || depth > 64 || len(message.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	valid := true
	message.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() {
			if field.MapValue().Kind() == protoreflect.MessageKind {
				value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
					valid = validAdmissionMessage(item.Message().Interface(), depth+1)
					return valid
				})
			}
		} else if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
			if field.IsList() {
				list := value.List()
				for i := 0; i < list.Len() && valid; i++ {
					valid = validAdmissionMessage(list.Get(i).Message().Interface(), depth+1)
				}
			} else {
				valid = validAdmissionMessage(value.Message().Interface(), depth+1)
			}
		}
		return valid
	})
	return valid
}

// ActorFor must be checked again after local lock waits. The monotonic budget
// starts at ingress's first required owner read and never restarts at a handler.
func (v *VerifiedAdmissionCall) ActorFor(request proto.Message) (principal.Principal, error) {
	deny := status.Error(codes.Unauthenticated, "verified admission call required")
	if v == nil || v.proof.clock == nil || !admissionMethod(v.proof.method) || !validAdmissionMessage(request, 0) || request.ProtoReflect().Descriptor().FullName() != v.proof.message {
		return principal.Principal{}, deny
	}
	p := v.proof.actor
	now := v.proof.clock()
	hash, err := principal.RequestHash(request)
	if err != nil || p.Issuer != "gateway" || p.Audience != "space" || p.RPC != v.proof.method || p.RequestHash != hash || p.Subject != p.AccountID || !canonicalID(p.AccountID) || !canonicalID(p.ProfileID) || !canonicalID(p.RequestID) || p.SessionEpoch <= 0 || (p.AccountType != "regular" && p.AccountType != "guest") || !now.Before(p.ExpiresAt) {
		return principal.Principal{}, deny
	}
	if v.proof.ownerStarted.IsZero() || now.Before(v.proof.ownerStarted) || !now.Before(v.proof.ownerStarted.Add(5*time.Second)) {
		return principal.Principal{}, status.Error(codes.Unavailable, "admission observation expired")
	}
	return p, nil
}

func requireAdmissionCall(ctx context.Context, method string, request proto.Message) (*VerifiedAdmissionCall, error) {
	if ctx == nil {
		return nil, status.Error(codes.Unauthenticated, "verified admission call required")
	}
	proof, ok := ctx.Value(admissionCallKey{}).(*admissionCallProof)
	if !ok || proof == nil || proof.method != method || ctx.Err() != nil {
		return nil, status.Error(codes.Unauthenticated, "verified admission call required")
	}
	actor, ok := principal.FromContext(ctx)
	if !ok || actor != proof.actor {
		return nil, status.Error(codes.Unauthenticated, "verified admission call required")
	}
	verified := &VerifiedAdmissionCall{proof: *proof}
	if _, err := verified.ActorFor(request); err != nil {
		return nil, err
	}
	return verified, nil
}

func RequireAdmissionStart(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"StartJoinAttempt", request)
}
func RequireAdmissionAttemptRead(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"GetJoinAttempt", request)
}
func RequireAdmissionSubmit(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"SubmitJoinAttempt", request)
}
func RequireAdmissionRequestList(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"ListJoinRequests", request)
}
func RequireAdmissionRequestRead(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"GetJoinRequest", request)
}
func RequireAdmissionApproval(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"ApproveJoinRequest", request)
}
func RequireAdmissionJoinSpace(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"JoinSpace", request)
}
func RequireAdmissionJoinInvite(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"JoinByInvite", request)
}

func (v Verifier) verifyAdmissionCall(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if v.Keys == nil || v.AdmissionReplay == nil || v.AdmissionEpoch == nil {
		return nil, status.Error(codes.PermissionDenied, "admission authentication is not configured")
	}
	deny := status.Error(codes.Unauthenticated, "invalid admission principal")
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || !canonicalID(transport.RequestID) {
		return nil, deny
	}
	message, ok := request.(proto.Message)
	if !ok || !validAdmissionMessage(message, 0) {
		return nil, status.Error(codes.InvalidArgument, "invalid admission request")
	}
	hash, err := principal.RequestHash(message)
	if err != nil {
		return nil, deny
	}
	clock := v.Clock
	if clock == nil {
		clock = time.Now
	}
	var started time.Time
	epoch := func(ownerCtx context.Context, account string, value int64) error {
		started = clock()
		ownerCtx, cancel := context.WithTimeout(ownerCtx, 2*time.Second)
		defer cancel()
		if err := v.AdmissionEpoch(ownerCtx, account, value); err != nil {
			return err
		}
		if ownerCtx.Err() != nil || clock().Before(started) || !clock().Before(started.Add(2*time.Second)) {
			return ErrDependencyUnavailable
		}
		return nil
	}
	actor, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.AdmissionReplay, SessionEpochChecker: epoch, Clock: clock})
	if errors.Is(err, ErrDependencyUnavailable) || ctx.Err() != nil {
		return nil, status.Error(codes.Unavailable, "admission authentication unavailable")
	}
	if err != nil || !canonicalID(actor.AccountID) || !canonicalID(actor.ProfileID) || (actor.AccountType != "regular" && actor.AccountType != "guest") {
		return nil, deny
	}
	proof := &admissionCallProof{actor: actor, method: info.FullMethod, message: message.ProtoReflect().Descriptor().FullName(), clock: clock, ownerStarted: started}
	ctx = context.WithValue(principal.WithVerified(ctx, actor), admissionCallKey{}, proof)
	if _, err = (&VerifiedAdmissionCall{proof: *proof}).ActorFor(message); err != nil {
		return nil, err
	}
	// Compatibility projection remains server-owned and preserves explicit type.
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderUserID, actor.AccountID, authctx.HeaderProfileID, actor.ProfileID, authctx.HeaderSessionEpoch, strconv.FormatInt(actor.SessionEpoch, 10), "x-voice-account-type", actor.AccountType))
	return handler(ctx, request)
}
