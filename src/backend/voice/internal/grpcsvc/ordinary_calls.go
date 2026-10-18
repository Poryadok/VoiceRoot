package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/ordinarycall"
)

func ordinaryErr(err error) error {
	if err == nil {
		return nil
	}
	if code := status.Code(err); code != codes.Unknown {
		return status.Error(code, "ordinary call request denied or unavailable")
	}
	switch {
	case errors.Is(err, ordinarycall.ErrNotMember):
		return status.Error(codes.PermissionDenied, "not an ordinary call participant")
	case errors.Is(err, ordinarycall.ErrNotFound):
		return status.Error(codes.NotFound, "ordinary call not found")
	case errors.Is(err, ordinarycall.ErrInvalidIntent):
		return status.Error(codes.InvalidArgument, "invalid ordinary call request")
	case errors.Is(err, ordinarycall.ErrConflict), errors.Is(err, ordinarycall.ErrStaleGeneration), errors.Is(err, ordinarycall.ErrAdmissionAborted), errors.Is(err, ordinarycall.ErrExpired), errors.Is(err, ordinarycall.ErrNotRinging), errors.Is(err, ordinarycall.ErrCapacity):
		return status.Error(codes.FailedPrecondition, "ordinary call admission is not available")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "ordinary call request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "ordinary call effect deadline exceeded")
	default:
		return status.Error(codes.Unavailable, "ordinary call operation is pending or unavailable")
	}
}

func (s *VoiceGRPC) ordinaryCallerProfile(ctx context.Context) (string, error) {
	if s == nil || s.OrdinaryCalls == nil {
		return callerProfile(ctx)
	}
	if verified, ok := principal.FromContext(ctx); ok && verified.Kind != "delegated_user" {
		return "", status.Error(codes.Unauthenticated, "ordinary call requires delegated user identity")
	}
	profile, _, _, err := callerVoiceUser(ctx)
	return profile, err
}

// Existing handlers run Chat/privacy/media validation before reaching here.
// This path never uses the legacy tuple reserve/commit/release API.
func (s *VoiceGRPC) startOrdinaryCall(ctx context.Context, chatID, profileID, calleeID string, media callsv1.CallMediaKind, group bool) (*callsv1.StartCallResponse, error) {
	if s.ChatMembers == nil {
		return nil, status.Error(codes.Unavailable, "Chat membership authority unavailable")
	}
	chat, err := uuid.Parse(chatID)
	if err != nil || chat == uuid.Nil || chat.String() != chatID {
		return nil, status.Error(codes.InvalidArgument, "invalid linked chat identity")
	}
	caller, err := s.ordinaryIdentity(ctx, profileID, true)
	if err != nil {
		return nil, err
	}
	if err = s.requireOrdinaryEpoch(ctx, caller); err != nil {
		return nil, err
	}
	intent := ordinarycall.CreateIntent{OperationID: uuid.New(), RoomID: uuid.New(), ChatID: chat, Group: group, Initiator: caller, MediaKind: int32(media)}
	if !group {
		callee, err := s.ordinaryIdentity(ctx, calleeID, false)
		if err != nil {
			return nil, err
		}
		intent.Callee = &callee
	}
	r, err := s.OrdinaryCalls.Start(ctx, intent, func(ctx context.Context) error {
		if err := s.ensureSdkConversionAdmission(ctx, profileID); err != nil {
			return err
		}
		if err := s.requireOrdinaryEpoch(ctx, caller); err != nil {
			return err
		}
		if err := s.ensureChatMember(ctx, chatID, profileID); err != nil {
			return err
		}
		if !group {
			if err := s.ensureSdkConversionAdmission(ctx, calleeID); err != nil {
				return err
			}
			if err := s.ensureChatMember(ctx, chatID, calleeID); err != nil {
				return err
			}
			if err := s.ensureDirectChat(ctx, chatID); err != nil {
				return err
			}
			return s.ensureCallPrivacy(ctx, profileID, calleeID)
		}
		return nil
	})
	if err != nil {
		return nil, ordinaryErr(err)
	}
	call, err := ordinaryCallProjection(r)
	if err != nil {
		return nil, ordinaryErr(err)
	}
	return &callsv1.StartCallResponse{CallSession: callToProto(call)}, nil
}
