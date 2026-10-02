package grpcsvc

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/gameprincipal"
	"voice/backend/voice/internal/gameprovision"
	voicestore "voice/backend/voice/internal/store"
)

type conversionFenceStoreStub struct {
	reservation gameprovision.SdkConversionFenceReservation
}

func (s *conversionFenceStoreStub) ReserveSdkConversionFence(_ context.Context, _ *callsv1.FenceSdkConversionRequest, _ bool, _ string, _ time.Time) (gameprovision.SdkConversionFenceReservation, error) {
	return s.reservation, nil
}
func (s *conversionFenceStoreStub) CompleteSdkConversionFence(_ context.Context, _ uuid.UUID, at time.Time) (*callsv1.FenceSdkConversionResponse, error) {
	response := proto.Clone(s.reservation.Response).(*callsv1.FenceSdkConversionResponse)
	response.ObservedEjectionAt = timestamppb.New(at)
	response.CommittedAt = timestamppb.New(at)
	return response, nil
}

func (*conversionFenceStoreStub) CompleteSdkConversionActivation(_ context.Context, request *callsv1.CompleteSdkConversionActivationRequest, at time.Time) (*callsv1.CompleteSdkConversionActivationResponse, error) {
	return &callsv1.CompleteSdkConversionActivationResponse{OperationId: request.GetOperationId(), BindingId: request.GetBindingId(),
		FrozenAuthorityEpoch: request.GetFrozenAuthorityEpoch(), VoiceReceiptId: request.GetVoiceReceiptId(),
		ActivationReceiptId: request.GetActivationReceiptId(), ReceiptId: uuid.NewString(), State: "activated",
		CommittedAt: timestamppb.New(at)}, nil
}

type conversionFenceRoomCloserStub struct{ rooms []string }

func (s *conversionFenceRoomCloserStub) CloseRoom(_ context.Context, room string) error {
	s.rooms = append(s.rooms, room)
	return nil
}

func TestFenceSdkConversionEndsSourceAndReturnsVerifiedDurableReceipt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	operation, binding, sourceAccount, actor, sourceProfile, targetAccount, targetProfile, freezeReceipt, voiceReceipt :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	request := &callsv1.FenceSdkConversionRequest{OperationId: operation.String(), BindingId: binding.String(),
		SourceAccountId: sourceAccount.String(), SourceActorId: actor.String(), SourceProfileId: sourceProfile.String(),
		TargetAccountId: targetAccount.String(), TargetProfileId: targetProfile.String(), FrozenAuthorityEpoch: 12,
		FreezeReceiptId: freezeReceipt.String()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	response := &callsv1.FenceSdkConversionResponse{OperationId: operation.String(), BindingId: binding.String(),
		SourceAccountId: sourceAccount.String(), SourceActorId: actor.String(), SourceProfileId: sourceProfile.String(),
		TargetAccountId: targetAccount.String(), TargetProfileId: targetProfile.String(), FrozenAuthorityEpoch: 12,
		FreezeReceiptId: freezeReceipt.String(), ReceiptId: voiceReceipt.String(), RequestHash: hashBytes(t, hash),
		MediaGeneration: 3, SourceRoomId: "source-room"}
	reservation := gameprovision.SdkConversionFenceReservation{Response: response, SourceRoom: "source-room"}
	callStore := voicestore.NewMemoryCallStore()
	_, err = callStore.CreateCall(ctx, voicestore.Call{RoomID: "source-room", LivekitRoomName: "lk-source-room",
		InitiatorProfileID: sourceProfile.String(), CalleeProfileID: uuid.NewString(), Status: callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)})
	require.NoError(t, err)
	closer := &conversionFenceRoomCloserStub{}
	service := &GameSessionProvisioningGRPC{Conversion: &conversionFenceStoreStub{reservation: reservation}, Calls: callStore,
		CallCloser: closer, Now: func() time.Time { return now }}
	ctx = principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration",
		Audience: "voice", RPC: gameprincipal.FenceConversionMethod, RequestID: operation.String(), RequestHash: hash})
	got, err := service.FenceSdkConversion(ctx, request)
	require.NoError(t, err)
	require.Equal(t, voiceReceipt.String(), got.GetReceiptId())
	require.Equal(t, hash, "sha256:"+hex.EncodeToString(got.GetRequestHash()))
	require.Equal(t, []string{"lk-source-room"}, closer.rooms)
	ended, err := callStore.GetCall(ctx, "source-room")
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, ended.Status)
}

func hashBytes(t *testing.T, requestHash string) []byte {
	t.Helper()
	value, err := hex.DecodeString(strings.TrimPrefix(requestHash, "sha256:"))
	require.NoError(t, err)
	return value
}
