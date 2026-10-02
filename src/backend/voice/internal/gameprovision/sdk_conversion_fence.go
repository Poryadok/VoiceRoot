package gameprovision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
)

var ErrSdkConversionFencePending = errors.New("sdk conversion fence pending")

type SdkConversionFenceReservation struct {
	Response   *callsv1.FenceSdkConversionResponse
	SourceRoom string
	Complete   bool
}

// ReserveSdkConversionFence persists the immutable request before media side effects.
// Exact retries recover the stored source room and finish the same fence operation.
func (s *PostgresStore) ReserveSdkConversionFence(ctx context.Context,
	req *callsv1.FenceSdkConversionRequest, targetConflict bool, sourceRoom string, now time.Time) (SdkConversionFenceReservation, error) {
	ids, err := conversionFenceIDs(req)
	if err != nil || s == nil || s.pool == nil {
		return SdkConversionFenceReservation{}, ErrInvalidRequest
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return SdkConversionFenceReservation{}, ErrInvalidRequest
	}
	hash := sha256.Sum256(requestBytes)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SdkConversionFenceReservation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, ids.operation); err != nil {
		return SdkConversionFenceReservation{}, err
	}
	var storedHash []byte
	var state string
	var room string
	var response callsv1.FenceSdkConversionResponse
	var observedAt, committedStampAt *time.Time
	err = tx.QueryRow(ctx, `SELECT request_hash,state,COALESCE(source_room_id,''),receipt_id::text,
		observed_ejection_at,committed_at,target_session_conflict,media_generation
		FROM voice_sdk_conversion_fence_receipts WHERE operation_id=$1 FOR UPDATE`, ids.operation).Scan(
		&storedHash, &state, &room, &response.ReceiptId, &observedAt,
		&committedStampAt, &response.TargetSessionConflict, &response.MediaGeneration)
	if err == nil {
		if observedAt != nil {
			response.ObservedEjectionAt = timestamppb.New(*observedAt)
		}
		if committedStampAt != nil {
			response.CommittedAt = timestamppb.New(*committedStampAt)
		}
		if !bytes.Equal(storedHash, hash[:]) {
			return SdkConversionFenceReservation{}, ErrConflict
		}
		if state == "fenced" || state == "conflict" {
			response = *conversionFenceResponse(req, ids, response.ReceiptId, hash[:], state == "conflict", room,
				response.MediaGeneration, response.ObservedEjectionAt, response.CommittedAt)
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return SdkConversionFenceReservation{}, commitErr
			}
			return SdkConversionFenceReservation{Response: &response, SourceRoom: room, Complete: true}, nil
		}
		if state != "pending" {
			return SdkConversionFenceReservation{}, ErrConflict
		}
		response = *conversionFenceResponse(req, ids, response.ReceiptId, hash[:], response.TargetSessionConflict, room, response.MediaGeneration, nil, nil)
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return SdkConversionFenceReservation{}, commitErr
		}
		return SdkConversionFenceReservation{Response: &response, SourceRoom: room}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SdkConversionFenceReservation{}, err
	}
	receiptID := uuid.New()
	state = "pending"
	if _, err := tx.Exec(ctx, `INSERT INTO voice_sdk_conversion_fence_receipts(
		operation_id,binding_id,source_account_id,source_actor_id,source_profile_id,target_account_id,target_profile_id,
		frozen_authority_epoch,freeze_receipt_id,request_hash,receipt_id,state,source_room_id,target_session_conflict,committed_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULL)`, ids.operation, ids.binding, ids.sourceAccount,
		ids.sourceActor, ids.sourceProfile, ids.targetAccount, ids.targetProfile, req.GetFrozenAuthorityEpoch(),
		ids.freezeReceipt, hash[:], receiptID, state, nullableRoom(sourceRoom), targetConflict); err != nil {
		return SdkConversionFenceReservation{}, err
	}
	response = *conversionFenceResponse(req, ids, receiptID.String(), hash[:], targetConflict, sourceRoom, 1, nil, nil)
	if err := tx.Commit(ctx); err != nil {
		return SdkConversionFenceReservation{}, err
	}
	return SdkConversionFenceReservation{Response: &response, SourceRoom: sourceRoom}, nil
}

// CompleteSdkConversionFence commits the owner receipt only after the source media is fenced.
func (s *PostgresStore) CompleteSdkConversionFence(ctx context.Context, operationID uuid.UUID, observedAt time.Time) (*callsv1.FenceSdkConversionResponse, error) {
	if s == nil || s.pool == nil || operationID == uuid.Nil || observedAt.IsZero() {
		return nil, ErrInvalidRequest
	}
	var req callsv1.FenceSdkConversionRequest
	var hash []byte
	var receiptID string
	var state string
	var room string
	var generation int64
	var conflict bool
	var committed time.Time
	var observed *time.Time
	err := s.pool.QueryRow(ctx, `UPDATE voice_sdk_conversion_fence_receipts
		SET state=CASE WHEN target_session_conflict THEN 'conflict' ELSE 'fenced' END,
		observed_ejection_at=$2,committed_at=$2
		WHERE operation_id=$1 AND state='pending'
		RETURNING binding_id::text,source_account_id::text,source_actor_id::text,source_profile_id::text,
			target_account_id::text,target_profile_id::text,frozen_authority_epoch,freeze_receipt_id::text,
			request_hash,receipt_id::text,state,COALESCE(source_room_id,''),media_generation,target_session_conflict,
			observed_ejection_at,committed_at`, operationID, observedAt.UTC()).Scan(
		&req.BindingId, &req.SourceAccountId, &req.SourceActorId, &req.SourceProfileId, &req.TargetAccountId,
		&req.TargetProfileId, &req.FrozenAuthorityEpoch, &req.FreezeReceiptId, &hash, &receiptID, &state,
		&room, &generation, &conflict, &observed, &committed)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `SELECT binding_id::text,source_account_id::text,source_actor_id::text,source_profile_id::text,
			target_account_id::text,target_profile_id::text,frozen_authority_epoch,freeze_receipt_id::text,
			request_hash,receipt_id::text,state,COALESCE(source_room_id,''),media_generation,target_session_conflict,
			observed_ejection_at,committed_at FROM voice_sdk_conversion_fence_receipts WHERE operation_id=$1`, operationID).Scan(
			&req.BindingId, &req.SourceAccountId, &req.SourceActorId, &req.SourceProfileId, &req.TargetAccountId,
			&req.TargetProfileId, &req.FrozenAuthorityEpoch, &req.FreezeReceiptId, &hash, &receiptID, &state,
			&room, &generation, &conflict, &observed, &committed)
	}
	if err != nil {
		return nil, err
	}
	if state != "fenced" && state != "conflict" {
		return nil, ErrSdkConversionFencePending
	}
	req.OperationId = operationID.String()
	ids, err := conversionFenceIDs(&req)
	if err != nil {
		return nil, ErrConflict
	}
	var observedStamp, committedStamp *timestamppb.Timestamp
	if observed != nil {
		observedStamp = timestamppb.New(*observed)
	}
	if !committed.IsZero() {
		committedStamp = timestamppb.New(committed)
	}
	return conversionFenceResponse(&req, ids, receiptID, hash, conflict, room, uint64(generation), observedStamp, committedStamp), nil
}

// CompleteSdkConversionActivation releases the conversion admission fence only
// after GIS has committed the matching activation receipt.
func (s *PostgresStore) CompleteSdkConversionActivation(ctx context.Context, req *callsv1.CompleteSdkConversionActivationRequest, now time.Time) (*callsv1.CompleteSdkConversionActivationResponse, error) {
	if s == nil || s.pool == nil || req == nil || now.IsZero() {
		return nil, ErrInvalidRequest
	}
	ids, err := conversionActivationIDs(req)
	if err != nil {
		return nil, err
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	hash := sha256.Sum256(encoded)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var bindingID, freezeReceiptID, receiptID, activationID, responseReceiptID uuid.UUID
	var fenceHash []byte
	var state string
	var conflict bool
	var activationHash []byte
	var activatedAt *time.Time
	var epoch int64
	err = tx.QueryRow(ctx, `SELECT binding_id,frozen_authority_epoch,freeze_receipt_id,request_hash,receipt_id,state,
		target_session_conflict,activation_request_hash,activation_receipt_id,activation_response_receipt_id,activated_at
		FROM voice_sdk_conversion_fence_receipts WHERE operation_id=$1 FOR UPDATE`, ids.operation).
		Scan(&bindingID, &epoch, &freezeReceiptID, &fenceHash, &receiptID, &state, &conflict, &activationHash, &activationID, &responseReceiptID, &activatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if bindingID != ids.binding || uint64(epoch) != req.GetFrozenAuthorityEpoch() || freezeReceiptID != ids.freezeReceipt ||
		receiptID != ids.voiceReceipt {
		return nil, ErrConflict
	}
	if state == "activated" {
		if activationID != ids.activationReceipt || !bytes.Equal(activationHash, hash[:]) || activatedAt == nil || responseReceiptID == uuid.Nil {
			return nil, ErrConflict
		}
		response := conversionActivationResponse(req, ids, responseReceiptID, hash[:], *activatedAt)
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response, nil
	}
	if state != "fenced" || conflict || receiptID != ids.voiceReceipt {
		return nil, ErrConflict
	}
	activationID = ids.activationReceipt
	responseReceiptID = uuid.New()
	var committedAt time.Time
	if err := tx.QueryRow(ctx, `UPDATE voice_sdk_conversion_fence_receipts SET state='activated',activation_receipt_id=$2,
		activation_response_receipt_id=$3,activation_request_hash=$4,activated_at=$5 WHERE operation_id=$1 RETURNING activated_at`,
		ids.operation, activationID, responseReceiptID, hash[:], now.UTC()).Scan(&committedAt); err != nil {
		return nil, err
	}
	response := conversionActivationResponse(req, ids, responseReceiptID, hash[:], committedAt)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return response, nil
}

// IsSdkConversionProfileFenced is checked before call creation, joining and
// token issuance so a pre-activation Voice session cannot race conversion.
func (s *PostgresStore) IsSdkConversionProfileFenced(ctx context.Context, profileID string) (bool, error) {
	profile, err := uuid.Parse(profileID)
	if err != nil || profile == uuid.Nil || profile.String() != profileID || s == nil || s.pool == nil {
		return false, ErrInvalidRequest
	}
	var fenced bool
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_sdk_conversion_fence_receipts
		WHERE (state IN ('pending','fenced') AND (source_profile_id=$1 OR target_profile_id=$1))
		OR (state='activated' AND source_profile_id=$1))`, profile).Scan(&fenced)
	return fenced, err
}

type conversionActivationIDSet struct{ operation, binding, freezeReceipt, voiceReceipt, activationReceipt uuid.UUID }

func conversionActivationIDs(req *callsv1.CompleteSdkConversionActivationRequest) (conversionActivationIDSet, error) {
	values := []string{req.GetOperationId(), req.GetBindingId(), req.GetFreezeReceiptId(), req.GetVoiceReceiptId(), req.GetActivationReceiptId()}
	ids := make([]uuid.UUID, len(values))
	for i, value := range values {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil || parsed.String() != value {
			return conversionActivationIDSet{}, ErrInvalidRequest
		}
		ids[i] = parsed
	}
	if req.GetFrozenAuthorityEpoch() == 0 {
		return conversionActivationIDSet{}, ErrInvalidRequest
	}
	return conversionActivationIDSet{ids[0], ids[1], ids[2], ids[3], ids[4]}, nil
}
func conversionActivationResponse(req *callsv1.CompleteSdkConversionActivationRequest, ids conversionActivationIDSet,
	receipt uuid.UUID, hash []byte, committed time.Time) *callsv1.CompleteSdkConversionActivationResponse {
	return &callsv1.CompleteSdkConversionActivationResponse{OperationId: ids.operation.String(), BindingId: ids.binding.String(),
		FrozenAuthorityEpoch: req.GetFrozenAuthorityEpoch(), VoiceReceiptId: ids.voiceReceipt.String(), ActivationReceiptId: ids.activationReceipt.String(),
		ReceiptId: receipt.String(), RequestHash: append([]byte(nil), hash...), State: "activated", CommittedAt: timestamppb.New(committed)}
}

type conversionFenceIDSet struct {
	operation, binding, sourceAccount, sourceActor, sourceProfile, targetAccount, targetProfile, freezeReceipt uuid.UUID
}

func conversionFenceIDs(req *callsv1.FenceSdkConversionRequest) (conversionFenceIDSet, error) {
	if req == nil || req.GetFrozenAuthorityEpoch() == 0 {
		return conversionFenceIDSet{}, ErrInvalidRequest
	}
	values := []string{req.GetOperationId(), req.GetBindingId(), req.GetSourceAccountId(), req.GetSourceActorId(),
		req.GetSourceProfileId(), req.GetTargetAccountId(), req.GetTargetProfileId(), req.GetFreezeReceiptId()}
	parsed := make([]uuid.UUID, len(values))
	for i, raw := range values {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return conversionFenceIDSet{}, ErrInvalidRequest
		}
		parsed[i] = id
	}
	if parsed[2] == parsed[5] || parsed[4] == parsed[6] {
		return conversionFenceIDSet{}, ErrInvalidRequest
	}
	return conversionFenceIDSet{parsed[0], parsed[1], parsed[2], parsed[3], parsed[4], parsed[5], parsed[6], parsed[7]}, nil
}

func conversionFenceResponse(req *callsv1.FenceSdkConversionRequest, ids conversionFenceIDSet, receipt string,
	hash []byte, conflict bool, room string, generation uint64, observed, committed *timestamppb.Timestamp) *callsv1.FenceSdkConversionResponse {
	return &callsv1.FenceSdkConversionResponse{OperationId: ids.operation.String(), BindingId: ids.binding.String(),
		SourceAccountId: ids.sourceAccount.String(), SourceActorId: ids.sourceActor.String(), SourceProfileId: ids.sourceProfile.String(),
		TargetAccountId: ids.targetAccount.String(), TargetProfileId: ids.targetProfile.String(),
		FrozenAuthorityEpoch: req.GetFrozenAuthorityEpoch(), FreezeReceiptId: ids.freezeReceipt.String(), ReceiptId: receipt,
		RequestHash: append([]byte(nil), hash...), TargetSessionConflict: conflict, SourceRoomId: room,
		MediaGeneration: generation, ObservedEjectionAt: observed, CommittedAt: committed}
}

func nullableRoom(room string) any {
	if room == "" {
		return nil
	}
	return room
}
