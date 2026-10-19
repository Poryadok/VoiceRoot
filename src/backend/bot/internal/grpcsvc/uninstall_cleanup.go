package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
)

// This is Bot's observation of an authenticated native Empty response, NOT a
// Space-owned receipt or evidence of a global terminal marker.
type nativeCancellationObservation struct {
	ProtocolVersion int
	Kind            string
	Method          string
	RequestSHA256   []byte
	ResponseBytes   []byte
	ObservedAt      time.Time
}

func buildNativeCancellation(op *store.BotDeletion, effect store.BotDeletionEffect) (*spacev1.RemoveBotMemberRequest, error) {
	if cleanupPurpose(op) != "space_uninstall" || effect.Kind != "SPACE_CLEANUP" || effect.ChatID != nil || validateDeletionSource(op, effect) != nil {
		return nil, errDeletionEvidence
	}
	t := effect.Target
	if t.LegacyEvidence || t.InstallationID == [16]byte{} || t.InstallOperationID == [16]byte{} || t.CancelOperationID == [16]byte{} || t.BotGeneration <= 0 {
		return nil, errDeletionEvidence
	}
	i := &store.InstallationIntent{OperationID: t.InstallOperationID, BotID: t.BotID, InstallationID: t.InstallationID, SpaceID: t.SpaceID, ActorProfileID: t.ActorProfileID, Generation: t.BotGeneration, CancelOperationID: t.CancelOperationID}
	return &spacev1.RemoveBotMemberRequest{SpaceId: t.SpaceID.String(), ProfileId: t.ActorProfileID.String(), InstallationIntent: installationIdentity(i, true), InstallOperationId: t.InstallOperationID.String()}, nil
}

func validateCompletedNativeCancellation(op *store.BotDeletion, effect store.BotDeletionEffect) error {
	req, err := buildNativeCancellation(op, effect)
	if err != nil {
		return err
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil || !bytes.Equal(body, effect.RequestBytes) {
		return errDeletionEvidence
	}
	var observed nativeCancellationObservation
	decoder := json.NewDecoder(bytes.NewReader(effect.ReceiptBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&observed) != nil {
		return errDeletionEvidence
	}
	canonical, err := json.Marshal(observed)
	if err != nil || !bytes.Equal(canonical, effect.ReceiptBytes) {
		return errDeletionEvidence
	}
	sum := sha256.Sum256(body)
	if observed.ProtocolVersion != 1 || observed.Kind != "BOT_LOCAL_NATIVE_CANCEL_ACK" || observed.Method != spacev1.SpaceService_RemoveBotMember_FullMethodName || !bytes.Equal(observed.RequestSHA256, sum[:]) || observed.ResponseBytes == nil || len(observed.ResponseBytes) != 0 || observed.ObservedAt.IsZero() {
		return errDeletionEvidence
	}
	return nil
}

func (s *BotGRPC) applyNativeCancellation(ctx context.Context, claim *store.BotDeletion, effect store.BotDeletionEffect) error {
	if s.InstallationSpace == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected native cancellation unavailable")
	}
	request, err := buildNativeCancellation(claim, effect)
	if err != nil {
		return status.Error(codes.Unavailable, "native cancellation source is not qualified")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return err
	}
	if len(effect.RequestBytes) != 0 && !bytes.Equal(body, effect.RequestBytes) {
		return store.ErrInstallationConflict
	}
	if err = s.Store.SealBotDeletionRequest(ctx, claim, effect.EffectID, body); err != nil {
		return err
	}
	callCtx, cancel, err := s.installationCallContext(ctx, request, spacev1.SpaceService_RemoveBotMember_FullMethodName, request.InstallationIntent.OperationId)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := s.InstallationSpace.RemoveBotMember(callCtx, request)
	if err != nil {
		return err
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.Unavailable, "native cancellation acknowledgement unavailable")
	}
	// Empty is retained truthfully. No invented Space receipt UUID/count/fence.
	responseBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return err
	}
	if responseBytes == nil {
		responseBytes = []byte{}
	}
	sum := sha256.Sum256(body)
	receipt, err := json.Marshal(nativeCancellationObservation{1, "BOT_LOCAL_NATIVE_CANCEL_ACK", spacev1.SpaceService_RemoveBotMember_FullMethodName, sum[:], responseBytes, time.Now().UTC()})
	if err != nil {
		return err
	}
	return s.Store.SaveUninstallNativeAcknowledgement(ctx, claim, effect.EffectID, body, receipt)
}

func (s *BotGRPC) nativeCancellationAccepted(ctx context.Context, claim *store.BotDeletion) bool {
	effects, err := s.Store.LifecycleEffects(ctx, claim)
	if err != nil {
		return false
	}
	for _, effect := range effects {
		if effect.Kind == "SPACE_CLEANUP" && effect.State == "COMPLETE" {
			return validateCompletedNativeCancellation(claim, effect) == nil
		}
	}
	return false
}
