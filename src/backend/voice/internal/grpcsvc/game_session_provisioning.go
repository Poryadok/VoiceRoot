package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/gameprincipal"
	"voice/backend/voice/internal/gameprovision"
	voicestore "voice/backend/voice/internal/store"
)

type ManagedGameSessionRoomLookup interface {
	GetRoom(context.Context, string) (gameprovision.Room, error)
}

type GameSessionProvisioner interface {
	Provision(context.Context, *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error)
}

type GameSessionRosterApplier interface {
	ApplyGameSessionRoster(context.Context, *callsv1.ApplyGameSessionRosterRequest, gameprovision.ManagedGameSessionRosterMediaFencer) (*callsv1.ApplyGameSessionRosterResponse, error)
}

type GameSessionCloser interface {
	CloseGameSessionRoom(context.Context, *callsv1.CloseGameSessionRoomRequest, gameprovision.ManagedGameSessionMediaFencer) (*callsv1.CloseGameSessionRoomResponse, error)
}

type SdkConversionFenceStore interface {
	ReserveSdkConversionFence(context.Context, *callsv1.FenceSdkConversionRequest, bool, string, time.Time) (gameprovision.SdkConversionFenceReservation, error)
	CompleteSdkConversionFence(context.Context, uuid.UUID, time.Time) (*callsv1.FenceSdkConversionResponse, error)
	CompleteSdkConversionActivation(context.Context, *callsv1.CompleteSdkConversionActivationRequest, time.Time) (*callsv1.CompleteSdkConversionActivationResponse, error)
}

type SdkConversionCallStore interface {
	GetActiveCall(context.Context, string) (voicestore.Call, error)
	GetCall(context.Context, string) (voicestore.Call, error)
	SetStatus(context.Context, string, callsv1.CallStatus, time.Time) (voicestore.Call, error)
}

type SdkConversionRoomCloser interface {
	CloseRoom(context.Context, string) error
}

type GameSessionProvisioningGRPC struct {
	callsv1.UnimplementedGameSessionProvisioningServiceServer
	Store      GameSessionProvisioner
	Roster     GameSessionRosterApplier
	Closer     GameSessionCloser
	Fencer     gameprovision.ManagedGameSessionMediaFencer
	Conversion SdkConversionFenceStore
	Calls      SdkConversionCallStore
	CallCloser SdkConversionRoomCloser
	Now        func() time.Time
}

func (s *GameSessionProvisioningGRPC) FenceSdkConversion(ctx context.Context, request *callsv1.FenceSdkConversionRequest) (*callsv1.FenceSdkConversionResponse, error) {
	if err := gameprincipal.RequireFenceSdkConversion(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Conversion == nil || s.Calls == nil || s.CallCloser == nil {
		return nil, status.Error(codes.Unavailable, "SDK conversion voice authority unavailable")
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	target, targetErr := s.Calls.GetActiveCall(ctx, request.GetTargetProfileId())
	if targetErr != nil && !errors.Is(targetErr, voicestore.ErrNotFound) {
		return nil, status.Error(codes.Unavailable, "target voice session lookup failed")
	}
	targetConflict := targetErr == nil && target.IsActiveForProfile(request.GetTargetProfileId())
	source, sourceErr := s.Calls.GetActiveCall(ctx, request.GetSourceProfileId())
	if sourceErr != nil && !errors.Is(sourceErr, voicestore.ErrNotFound) {
		return nil, status.Error(codes.Unavailable, "source voice session lookup failed")
	}
	sourceRoom := ""
	if sourceErr == nil {
		sourceRoom = source.RoomID
	}
	reservation, err := s.Conversion.ReserveSdkConversionFence(ctx, request, targetConflict, sourceRoom, now)
	if errors.Is(err, gameprovision.ErrConflict) {
		return nil, status.Error(codes.AlreadyExists, "SDK conversion operation conflicts")
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid SDK conversion request")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "SDK conversion receipt persistence failed")
	}
	if reservation.Complete {
		return reservation.Response, nil
	}
	if reservation.SourceRoom != "" {
		call, lookupErr := s.Calls.GetCall(ctx, reservation.SourceRoom)
		if lookupErr != nil {
			return nil, status.Error(codes.Unavailable, "source media fence recovery unavailable")
		}
		if call.Status != callsv1.CallStatus_CALL_STATUS_ENDED {
			if _, err := s.Calls.SetStatus(ctx, call.RoomID, callsv1.CallStatus_CALL_STATUS_ENDED, now); err != nil {
				return nil, status.Error(codes.Unavailable, "source voice session fence failed")
			}
		}
		if err := s.CallCloser.CloseRoom(ctx, call.LivekitRoomName); err != nil {
			return nil, status.Error(codes.Unavailable, "source media ejection failed")
		}
	}
	operationID, parseErr := uuid.Parse(reservation.Response.GetOperationId())
	if parseErr != nil {
		return nil, status.Error(codes.Internal, "invalid stored conversion operation")
	}
	observedAt := time.Now().UTC()
	if s.Now != nil {
		observedAt = s.Now().UTC()
	}
	response, err := s.Conversion.CompleteSdkConversionFence(ctx, operationID, observedAt)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "SDK conversion fence receipt pending")
	}
	return response, nil
}

func (s *GameSessionProvisioningGRPC) CompleteSdkConversionActivation(ctx context.Context, request *callsv1.CompleteSdkConversionActivationRequest) (*callsv1.CompleteSdkConversionActivationResponse, error) {
	if err := gameprincipal.RequireCompleteSdkConversionActivation(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Conversion == nil {
		return nil, status.Error(codes.Unavailable, "SDK conversion activation authority unavailable")
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	response, err := s.Conversion.CompleteSdkConversionActivation(ctx, request, now)
	if errors.Is(err, gameprovision.ErrConflict) {
		return nil, status.Error(codes.AlreadyExists, "SDK conversion activation conflicts")
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid SDK conversion activation request")
	}
	if errors.Is(err, gameprovision.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "SDK conversion fence not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "SDK conversion activation receipt unavailable")
	}
	return response, nil
}

func (s *GameSessionProvisioningGRPC) ApplyGameSessionRoster(ctx context.Context, request *callsv1.ApplyGameSessionRosterRequest) (*callsv1.ApplyGameSessionRosterResponse, error) {
	if err := gameprincipal.RequireApplyRoster(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Roster == nil {
		return nil, status.Error(codes.Unavailable, "game session roster unavailable")
	}
	var rosterFencer gameprovision.ManagedGameSessionRosterMediaFencer
	if candidate, ok := s.Fencer.(gameprovision.ManagedGameSessionRosterMediaFencer); ok {
		rosterFencer = candidate
	}
	response, err := s.Roster.ApplyGameSessionRoster(ctx, request, rosterFencer)
	if errors.Is(err, gameprovision.ErrConflict) {
		ownerErr := status.Error(codes.AlreadyExists, "roster operation or revision conflicts")
		return nil, annotateVoiceOwnerConflict(ownerErr, request, gisowner.VoiceRosterRPC)
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid game session roster request")
	}
	if errors.Is(err, gameprovision.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "managed game session room not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "game session roster apply failed")
	}
	return response, nil
}

func (s *GameSessionProvisioningGRPC) CloseGameSessionRoom(ctx context.Context, request *callsv1.CloseGameSessionRoomRequest) (*callsv1.CloseGameSessionRoomResponse, error) {
	if err := gameprincipal.RequireClose(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Closer == nil || s.Fencer == nil {
		return nil, status.Error(codes.Unavailable, "game session close unavailable")
	}
	response, err := s.Closer.CloseGameSessionRoom(ctx, request, s.Fencer)
	if errors.Is(err, gameprovision.ErrConflict) {
		return nil, status.Error(codes.AlreadyExists, "close operation or room conflicts")
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid game session close request")
	}
	if errors.Is(err, gameprovision.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "managed game session room not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "game session close failed")
	}
	return response, nil
}

func (s *GameSessionProvisioningGRPC) ProvisionGameSessionRoom(ctx context.Context, request *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error) {
	if err := gameprincipal.RequireProvisioning(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "game session provisioning unavailable")
	}
	response, err := s.Store.Provision(ctx, request)
	if errors.Is(err, gameprovision.ErrConflict) {
		ownerErr := status.Error(codes.AlreadyExists, "operation or resource binding conflicts")
		return nil, annotateVoiceOwnerConflict(ownerErr, request, gisowner.VoiceProvisionRPC)
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid game session provisioning request")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "game session provisioning failed")
	}
	return response, nil
}

func annotateVoiceOwnerConflict(err error, request proto.Message, rpc string) error {
	hash, hashErr := principal.RequestHash(request)
	if hashErr != nil {
		return err
	}
	var operationID string
	switch typed := request.(type) {
	case *callsv1.ProvisionGameSessionRoomRequest:
		operationID = typed.GetOperationId()
	case *callsv1.ApplyGameSessionRosterRequest:
		operationID = typed.GetOperationId()
	default:
		return err
	}
	return gisowner.Annotate(err, gisowner.VoiceDomain, rpc, gisowner.CategoryOperationConflict, operationID, hash)
}
