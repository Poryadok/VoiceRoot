package roomlifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RedisMirrorCandidate struct {
	Origin    string
	Operation LifecycleOperation
}

type RedisReconcileResult struct {
	ReceiptBytes []byte
}

type RedisMirrorBridge struct {
	store  *PostgresLifecycleStore
	ledger *RedisLedger
	clock  func() time.Time
}

func NewRedisMirrorBridge(store *PostgresLifecycleStore, ledger *RedisLedger, clock func() time.Time) *RedisMirrorBridge {
	return &RedisMirrorBridge{store: store, ledger: ledger, clock: clock}
}

func (store *PostgresLifecycleStore) LoadRedisMirrorCandidate(ctx context.Context, origin string, actorProfileID, operationID uuid.UUID, now time.Time) (RedisMirrorCandidate, error) {
	if store == nil || store.pool == nil || !validRedisMirrorOrigin(origin) || actorProfileID == uuid.Nil || operationID == uuid.Nil || now.IsZero() {
		return RedisMirrorCandidate{}, ErrUnavailable
	}
	available, err := redisDivergenceTableAvailable(ctx, store)
	if err != nil {
		return RedisMirrorCandidate{}, err
	}
	if available {
		if _, found, loadErr := store.LoadRedisDivergence(ctx, actorProfileID, operationID); loadErr != nil {
			return RedisMirrorCandidate{}, loadErr
		} else if found {
			return RedisMirrorCandidate{}, ErrRedisMirrorQuarantined
		}
	}
	operation, found, err := store.LoadOperation(ctx, actorProfileID, operationID)
	if err != nil {
		return RedisMirrorCandidate{}, err
	}
	if !found {
		return RedisMirrorCandidate{}, ErrInvariant
	}
	return RedisMirrorCandidate{Origin: origin, Operation: cloneLifecycleOperationForMirror(operation)}, nil
}

func (bridge *RedisMirrorBridge) ReconcileCompleted(ctx context.Context, candidate RedisMirrorCandidate) (RedisReconcileResult, error) {
	if bridge == nil || bridge.store == nil || bridge.store.pool == nil || bridge.ledger == nil || bridge.clock == nil ||
		!validRedisMirrorOrigin(candidate.Origin) || candidate.Operation.State != LifecycleOperationCompleted || candidate.Operation.ReplayUntil == nil {
		return RedisReconcileResult{}, ErrInvariant
	}
	available, err := redisDivergenceTableAvailable(ctx, bridge.store)
	if err != nil {
		return RedisReconcileResult{}, err
	}
	if available {
		if _, found, loadErr := bridge.store.LoadRedisDivergence(ctx, candidate.Operation.ActorProfileID, candidate.Operation.OperationID); loadErr != nil {
			return RedisReconcileResult{}, loadErr
		} else if found {
			return RedisReconcileResult{}, ErrRedisMirrorQuarantined
		}
	}
	deadline := candidate.Operation.ReplayUntil.UTC()
	if now := bridge.clock().UTC(); !now.Before(deadline) {
		return RedisReconcileResult{}, ErrRedisMirrorExpired
	}
	reservation, err := NewRedisMirrorReservation(candidate.Origin, candidate.Operation)
	if err != nil {
		return RedisReconcileResult{}, err
	}
	receipt, err := NewRedisMirrorReceipt(candidate.Operation)
	if err != nil {
		return RedisReconcileResult{}, err
	}
	if _, err = bridge.ledger.CompleteMirror(ctx, reservation, receipt); err != nil {
		return RedisReconcileResult{}, err
	}
	if now := bridge.clock().UTC(); !now.Before(deadline) {
		return RedisReconcileResult{}, ErrRedisMirrorExpired
	}
	return RedisReconcileResult{ReceiptBytes: append([]byte(nil), receipt.Bytes...)}, nil
}

func redisDivergenceTableAvailable(ctx context.Context, store *PostgresLifecycleStore) (bool, error) {
	var available bool
	if err := store.pool.QueryRow(ctx, `SELECT to_regclass('voice_lifecycle_redis_divergences') IS NOT NULL`).Scan(&available); err != nil {
		return false, mapScanError(err)
	}
	return available, nil
}

func cloneLifecycleOperationForMirror(operation LifecycleOperation) LifecycleOperation {
	operation.BindingBytes = append([]byte(nil), operation.BindingBytes...)
	operation.SourceVoiceRoomID = cloneUUIDPointer(operation.SourceVoiceRoomID)
	operation.DestinationVoiceRoomID = cloneUUIDPointer(operation.DestinationVoiceRoomID)
	operation.SourceRoomID = cloneUUIDPointer(operation.SourceRoomID)
	operation.DestinationRoomID = cloneUUIDPointer(operation.DestinationRoomID)
	operation.SourceMediaEpoch = cloneUUIDPointer(operation.SourceMediaEpoch)
	operation.DestinationMediaEpoch = cloneUUIDPointer(operation.DestinationMediaEpoch)
	operation.Authority.ActorSourceRolePolicyEpoch = cloneInt64Pointer(operation.Authority.ActorSourceRolePolicyEpoch)
	operation.Authority.ActorDestinationRolePolicyEpoch = cloneInt64Pointer(operation.Authority.ActorDestinationRolePolicyEpoch)
	operation.Authority.ActorCanMoveSource = cloneBoolPointer(operation.Authority.ActorCanMoveSource)
	operation.Authority.ActorCanMoveDestination = cloneBoolPointer(operation.Authority.ActorCanMoveDestination)
	operation.LeaseOwner = cloneUUIDPointer(operation.LeaseOwner)
	operation.LeaseUntil = cloneTimePointer(operation.LeaseUntil)
	operation.ReceiptBytes = append([]byte(nil), operation.ReceiptBytes...)
	operation.ReceiptHash = cloneDigestPointer(operation.ReceiptHash)
	operation.CompletedAt = cloneTimePointer(operation.CompletedAt)
	operation.ReplayUntil = cloneTimePointer(operation.ReplayUntil)
	operation.QuarantineClass = cloneStringPointer(operation.QuarantineClass)
	operation.QuarantineDetail = cloneStringPointer(operation.QuarantineDetail)
	operation.QuarantinedAt = cloneTimePointer(operation.QuarantinedAt)
	if operation.Receipt != nil {
		value := *operation.Receipt
		value.SourceVoiceRoomID = cloneUUIDPointer(value.SourceVoiceRoomID)
		value.DestinationVoiceRoomID = cloneUUIDPointer(value.DestinationVoiceRoomID)
		value.RoomID = cloneUUIDPointer(value.RoomID)
		value.SourceRosterVersion = cloneInt64Pointer(value.SourceRosterVersion)
		value.DestinationRosterVersion = cloneInt64Pointer(value.DestinationRosterVersion)
		value.MediaEpoch = cloneUUIDPointer(value.MediaEpoch)
		value.SpaceAccessEpoch = cloneInt64Pointer(value.SpaceAccessEpoch)
		value.RolePolicyEpoch = cloneInt64Pointer(value.RolePolicyEpoch)
		value.AuthorizationDigest = cloneDigestPointer(value.AuthorizationDigest)
		operation.Receipt = &value
	}
	return operation
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneDigestPointer(value *LifecycleDigest) *LifecycleDigest {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
