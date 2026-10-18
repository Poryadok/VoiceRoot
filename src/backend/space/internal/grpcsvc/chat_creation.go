package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
	"voice/backend/space/internal/chatcreation"
	"voice/backend/space/internal/store"
)

// CurrentBotInstallation is supplied by the narrow authenticated Bot-owned
// decision read, never a legacy list or a cached response from Chat admission.
type CurrentBotInstallation interface {
	CheckCurrentBotInstallation(context.Context, *spacev1.SpaceChatCreationBinding) error
}

type chatCreationAdmission struct {
	space *SpaceGRPC
	bots  CurrentBotInstallation
}

func NewChatCreationServer(space *SpaceGRPC, bots CurrentBotInstallation) *chatcreation.Server {
	server := &chatcreation.Server{Admission: chatCreationAdmission{space: space, bots: bots}, UnknownError: store.ErrCompositeUnknown, MapError: mapChatCreationError}
	if space != nil {
		server.Store = space.Store
	}
	return server
}

func mapChatCreationError(err error) error {
	switch {
	case errors.Is(err, store.ErrCompositeUnknown):
		return status.Error(codes.NotFound, "composite outcome unknown")
	case errors.Is(err, store.ErrCompositeConflict):
		return status.Error(codes.AlreadyExists, "composite binding conflict")
	case errors.Is(err, store.ErrCompositeResultExpired):
		result := status.New(codes.FailedPrecondition, "COMPOSITE_RESULT_EXPIRED")
		if detailed, e := result.WithDetails(&errdetails.ErrorInfo{Reason: "COMPOSITE_RESULT_EXPIRED", Domain: "space.chat_creation"}); e == nil {
			result = detailed
		}
		return result.Err()
	case errors.Is(err, store.ErrLifecycleFrozen), errors.Is(err, store.ErrOwnershipFrozen):
		return status.Error(codes.FailedPrecondition, "Space mutation frozen")
	default:
		return status.Error(codes.Unavailable, "composite persistence unavailable")
	}
}

func (a chatCreationAdmission) CheckChatCreation(ctx context.Context, b *spacev1.SpaceChatCreationBinding) error {
	if a.space == nil || a.space.Store == nil || a.space.OwnershipRoles == nil || a.space.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "composite admission unavailable")
	}
	if b.GetBot() != nil {
		if a.bots == nil {
			return status.Error(codes.Unavailable, "Bot installation authority unavailable")
		}
		if err := a.bots.CheckCurrentBotInstallation(ctx, b); err != nil {
			return err
		}
	}
	request := &rolev1.CheckPermissionRequest{SpaceId: b.SpaceId, ProfileId: b.CreatorProfileId, PermissionName: permissions.TextChatCreateInSpace}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid composite permission request")
	}
	token, err := a.space.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: b.OperationId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Role decision credential unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", b.OperationId))
	response, err := a.space.OwnershipRoles.CheckPermission(call, request)
	if err != nil {
		// Only an authenticated allowed=false response is a domain refusal.
		// A protected listener's authentication/method denial is UNKNOWN.
		return status.Error(codes.Unavailable, "Role authority unavailable")
	}
	if response == nil {
		return status.Error(codes.Unavailable, "Role authority unavailable")
	}
	if !response.Allowed {
		return status.Error(codes.PermissionDenied, "permission denied")
	}
	return nil
}

func (a chatCreationAdmission) CheckOutcomeDisclosure(ctx context.Context, p principal.Principal, b *spacev1.SpaceChatCreationBinding) error {
	deny := status.Error(codes.NotFound, "composite outcome not found")
	if b.GetUser() == nil || p.ProfileID != b.CreatorProfileId || p.AccountID != b.GetUser().AccountId {
		return deny
	}
	return nil
}

func (a chatCreationAdmission) CheckOutcomeReadScope(ctx context.Context, p principal.Principal, spaceID uuid.UUID) error {
	deny := status.Error(codes.NotFound, "composite outcome not found")
	if !chatcreation.CanonicalID(p.ProfileID) || !chatcreation.CanonicalID(p.AccountID) || (p.AccountType != "regular" && p.AccountType != "guest") {
		return status.Error(codes.Unauthenticated, "verified explicit actor required")
	}
	if a.space == nil || a.space.Store == nil {
		return status.Error(codes.Unavailable, "composite disclosure unavailable")
	}
	profileID := uuid.MustParse(p.ProfileID)
	if err := a.space.Store.CheckOwnershipAvailable(ctx, spaceID); err != nil {
		if errors.Is(err, store.ErrLifecycleFrozen) {
			return deny
		}
		return mapSpaceStoreError(err)
	}
	member, err := a.space.Store.IsSpaceMember(ctx, spaceID, profileID)
	if err != nil {
		return status.Error(codes.Unavailable, "Space membership unavailable")
	}
	if !member {
		return deny
	}
	row, err := a.space.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return mapSpaceStoreError(err)
	}
	if row == nil || p.AccountType == "guest" && !row.AllowGuests {
		return deny
	}
	return nil
}
