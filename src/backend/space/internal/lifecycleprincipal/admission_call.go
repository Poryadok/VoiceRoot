package lifecycleprincipal

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"voice/backend/pkg/principal"
)

const admissionRPCPrefix = "/voice.space.v1.SpaceService/"

func admissionMethod(method string) bool {
	if method == admissionRPCPrefix+"CreateVoiceRoom" || method == admissionRPCPrefix+"CreateInvite" || method == admissionRPCPrefix+"UpdateSpace" || method == admissionRPCPrefix+"UpdateSpaceMmConfig" {
		return true
	}
	switch method {
	case admissionRPCPrefix + "StartJoinAttempt", admissionRPCPrefix + "GetJoinAttempt", admissionRPCPrefix + "SubmitJoinAttempt", admissionRPCPrefix + "ListJoinRequests", admissionRPCPrefix + "GetJoinRequest", admissionRPCPrefix + "ApproveJoinRequest", admissionRPCPrefix + "JoinSpace", admissionRPCPrefix + "JoinByInvite", admissionRPCPrefix + "SetEntryPolicy", admissionRPCPrefix + "BanMember", admissionRPCPrefix + "UnbanMember":
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

// Remaining preserves ingress's original monotonic observation phase. It never
// grants a fresh budget when a handler replans or acquires a database lock.
func (v *VerifiedAdmissionCall) Remaining(request proto.Message) (time.Duration, error) {
	p, err := v.ActorFor(request)
	if err != nil {
		return 0, err
	}
	now := v.proof.clock()
	remaining := v.proof.ownerStarted.Add(5 * time.Second).Sub(now)
	if credential := p.ExpiresAt.Sub(now); credential < remaining {
		remaining = credential
	}
	if remaining <= 0 {
		return 0, status.Error(codes.Unavailable, "admission observation expired")
	}
	return remaining, nil
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

func RequireVoiceRoomCreate(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"CreateVoiceRoom", request)
}

func RequireInviteCreate(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"CreateInvite", request)
}

func RequireSettingsUpdate(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"UpdateSpace", request)
}

func RequireMatchmakingSettingsUpdate(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"UpdateSpaceMmConfig", request)
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

// RequireEntryPolicySet authenticates the exact settings request. The separate
// settings permission and current policy CAS remain the handler's responsibility.
func RequireEntryPolicySet(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"SetEntryPolicy", request)
}

// RequireModerationBan authenticates only its exact RPC and request body;
// MemberBan permission and the separate protected User capture remain required.
func RequireModerationBan(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"BanMember", request)
}
func RequireModerationUnban(ctx context.Context, request proto.Message) (*VerifiedAdmissionCall, error) {
	return requireAdmissionCall(ctx, admissionRPCPrefix+"UnbanMember", request)
}
