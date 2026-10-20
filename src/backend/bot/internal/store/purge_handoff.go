package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

// This is a P3-owned snapshot namespace, never a synthetic verified-owner
// global decision. Only actual eligible source rows enter it.
type BotPurgeSource struct {
	Version                 int                      `json:"version"`
	BotID                   uuid.UUID                `json:"bot_id"`
	SpaceID                 uuid.UUID                `json:"space_id"`
	ActorProfileID          uuid.UUID                `json:"actor_profile_id"`
	InstalledID             *uuid.UUID               `json:"installed_id,omitempty"`
	Intent                  *InstallationIntent      `json:"intent,omitempty"`
	NativeCancelOperationID uuid.UUID                `json:"native_cancel_operation_id"`
	NativeDrainOperationID  uuid.UUID                `json:"native_drain_operation_id"`
	NativeDrains            []BotPurgeNativeDrain    `json:"native_drains,omitempty"`
	ChatSources             []ChatScopeCleanupSource `json:"chat_sources,omitempty"`
	Retained                []BotPurgeRetainedWork   `json:"retained,omitempty"`
}

// The nested cancellation is an existing native installation assertion; its
// operation is never replaced by the separate P3 child operation. Retained
// accepted targets also enter this list after local cancellation retires the
// disposable live intent. Legacy evidence cannot manufacture a native tuple.
type BotPurgeNativeDrain struct {
	OperationID   uuid.UUID `json:"operation_id"`
	RequestBytes  []byte    `json:"request_bytes"`
	RequestSHA256 []byte    `json:"request_sha256"`
}

type BotPurgeRetainedWork struct {
	Purpose        string                   `json:"purpose"`
	OperationID    uuid.UUID                `json:"operation_id"`
	DecisionBytes  []byte                   `json:"decision_bytes"`
	DecisionSHA256 []byte                   `json:"decision_sha256"`
	Target         BotDeletionTarget        `json:"target"`
	TargetBytes    []byte                   `json:"target_bytes"`
	TargetSHA256   []byte                   `json:"target_sha256"`
	Effects        []BotPurgeRetainedEffect `json:"effects"`
}

type BotPurgeRetainedEffect struct {
	EffectID      uuid.UUID  `json:"effect_id"`
	Kind          string     `json:"kind"`
	ChatID        *uuid.UUID `json:"chat_id,omitempty"`
	State         string     `json:"state"`
	RequestBytes  []byte     `json:"request_bytes,omitempty"`
	RequestSHA256 []byte     `json:"request_sha256,omitempty"`
	ReceiptBytes  []byte     `json:"receipt_bytes,omitempty"`
	ReceiptSHA256 []byte     `json:"receipt_sha256,omitempty"`
}

// Caller already owns the exact irreversible Space head/guard. Sources are
// captured under sorted Bot locks and committed before a recoverable pending
// return. Network reads/drain never occur inside this transaction.
func (s *BotStore) preparePurgeHandoffTx(ctx context.Context, tx pgx.Tx, space, operation uuid.UUID, generation int64, request, requestSHA []byte) (bool, error) {
	var savedRequest, savedSHA []byte
	var savedOp uuid.UUID
	var savedGen, count int64
	var state string
	err := tx.QueryRow(ctx, `SELECT deletion_operation_id,generation,request_bytes,request_sha256,state,source_count FROM bot_space_purge_handoffs WHERE space_id=$1 FOR UPDATE`, space).Scan(&savedOp, &savedGen, &savedRequest, &savedSHA, &state, &count)
	if err == nil {
		if savedOp != operation || savedGen != generation || !bytes.Equal(savedRequest, request) || !bytes.Equal(savedSHA, requestSHA) {
			return false, ErrSpaceLifecycleConflict
		}
		rows, e := tx.Query(ctx, `SELECT bot_id,source_bytes,source_sha256 FROM bot_space_purge_sources WHERE space_id=$1 ORDER BY bot_id FOR UPDATE`, space)
		if e != nil {
			return false, e
		}
		var actual int64
		for rows.Next() {
			var bot uuid.UUID
			var raw, digest []byte
			if e = rows.Scan(&bot, &raw, &digest); e != nil {
				break
			}
			var source BotPurgeSource
			sum := sha256.Sum256(raw)
			if !bytes.Equal(sum[:], digest) || json.Unmarshal(raw, &source) != nil || source.Version != 1 || source.BotID != bot || source.SpaceID != space {
				e = ErrSpaceLifecycleConflict
				break
			}
			canonical, x := json.Marshal(source)
			if x != nil || !bytes.Equal(canonical, raw) {
				e = ErrSpaceLifecycleConflict
				break
			}
			actual++
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return false, e
		}
		if actual != count {
			return false, ErrSpaceLifecycleConflict
		}
		// Positive proof adoption is the next explicit stage. Merely changing a SQL
		// state or losing a projection never authorizes a nonempty snapshot to pass.
		if count > 0 {
			return true, nil
		}
		if state != "COVERED" {
			return false, ErrSpaceLifecycleConflict
		}
		return false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	rows, err := tx.Query(ctx, `SELECT bot_id FROM bot_space_installations WHERE space_id=$1
UNION SELECT bot_id FROM bot_chat_whitelist WHERE space_id=$1
UNION SELECT bot_id FROM bot_installation_intents WHERE space_id=$1 AND state<>'CANCELED'
UNION SELECT t.bot_id FROM bot_uninstall_installations t JOIN bot_uninstall_operations o ON o.operation_id=t.operation_id WHERE t.space_id=$1 AND o.state<>'COMPLETE'
UNION SELECT t.bot_id FROM bot_deletion_installations t JOIN bot_deletion_operations o ON o.operation_id=t.operation_id WHERE t.space_id=$1 AND o.state<>'COMPLETE'
ORDER BY bot_id`, space)
	if err != nil {
		return false, err
	}
	var bots []uuid.UUID
	for rows.Next() {
		var bot uuid.UUID
		if err = rows.Scan(&bot); err != nil {
			break
		}
		bots = append(bots, bot)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return false, err
	}
	// Take the whole sorted Bot lock set before source-row locks.
	actors := make(map[uuid.UUID]uuid.UUID, len(bots))
	for _, id := range bots {
		bot, e := s.GetBotByIDForUpdate(ctx, tx, id)
		if e != nil {
			return false, e
		}
		actors[id] = bot.ActorProfileID
	}
	state = "COVERED"
	if len(bots) > 0 {
		state = "PENDING"
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_purge_handoffs(space_id,deletion_operation_id,generation,request_bytes,request_sha256,state,source_count) VALUES($1,$2,$3,$4,$5,$6,$7)`, space, operation, generation, request, requestSHA, state, len(bots))
	if err != nil {
		return false, err
	}
	for _, bot := range bots {
		source := BotPurgeSource{Version: 1, BotID: bot, SpaceID: space, ActorProfileID: actors[bot]}
		var installed uuid.UUID
		e := tx.QueryRow(ctx, `SELECT id FROM bot_space_installations WHERE bot_id=$1 AND space_id=$2 FOR UPDATE`, bot, space).Scan(&installed)
		if e == nil {
			source.InstalledID = &installed
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return false, e
		}
		source.Intent, e = scanInstallationIntent(tx.QueryRow(ctx, `SELECT `+intentColumns+` FROM bot_installation_intents WHERE bot_id=$1 AND space_id=$2 AND state<>'CANCELED' FOR UPDATE`, bot, space))
		if errors.Is(e, pgx.ErrNoRows) {
			source.Intent = nil
		} else if e != nil {
			return false, e
		}
		if source.Intent != nil && source.Intent.ActorProfileID != source.ActorProfileID {
			return false, ErrSpaceLifecycleConflict
		}
		if source.Intent != nil && !source.Intent.LegacyEvidence {
			source.NativeDrainOperationID = uuid.New()
			source.NativeCancelOperationID = source.Intent.CancelOperationID
			if source.NativeCancelOperationID == uuid.Nil {
				source.NativeCancelOperationID = uuid.New()
			}
		}
		source.ChatSources, e = snapshotChatScopeSources(ctx, tx, bot, space)
		if e != nil {
			return false, e
		}
		for _, purpose := range []string{"global_delete", "space_uninstall"} {
			retained, e := capturePurgeRetained(ctx, tx, bot, space, purpose)
			if e != nil {
				return false, e
			}
			source.Retained = append(source.Retained, retained...)
		}
		source.NativeDrains, e = capturePurgeNativeDrains(source)
		if e != nil {
			return false, e
		}
		raw, e := json.Marshal(source)
		if e != nil {
			return false, e
		}
		digest := sha256.Sum256(raw)
		if _, e = tx.Exec(ctx, `INSERT INTO bot_space_purge_sources(space_id,bot_id,source_bytes,source_sha256) VALUES($1,$2,$3,$4)`, space, bot, raw, digest[:]); e != nil {
			return false, e
		}
	}
	return len(bots) > 0, nil
}

func capturePurgeNativeDrains(source BotPurgeSource) ([]BotPurgeNativeDrain, error) {
	byInstallation := make(map[uuid.UUID]BotPurgeNativeDrain)
	add := func(installation, install, cancel, outer uuid.UUID, generation int64) error {
		if installation == uuid.Nil || install == uuid.Nil || cancel == uuid.Nil || generation <= 0 || source.BotID == uuid.Nil || source.SpaceID == uuid.Nil || source.ActorProfileID == uuid.Nil {
			return ErrSpaceLifecycleConflict
		}
		q := &spacev1.RemoveBotMemberRequest{SpaceId: source.SpaceID.String(), ProfileId: source.ActorProfileID.String(), InstallOperationId: install.String(), InstallationIntent: &spacev1.BotInstallationIntent{OperationId: cancel.String(), BotId: source.BotID.String(), InstallationId: installation.String(), SpaceId: source.SpaceID.String(), ActorProfileId: source.ActorProfileID.String(), Generation: uint64(generation)}}
		raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
		if err != nil {
			return err
		}
		if prior, exists := byInstallation[installation]; exists {
			if !bytes.Equal(prior.RequestBytes, raw) {
				return ErrSpaceLifecycleConflict
			}
			return nil
		}
		if outer == uuid.Nil {
			outer = uuid.New()
		}
		if outer == cancel || outer == install {
			return ErrSpaceLifecycleConflict
		}
		digest := sha256.Sum256(raw)
		byInstallation[installation] = BotPurgeNativeDrain{OperationID: outer, RequestBytes: raw, RequestSHA256: append([]byte(nil), digest[:]...)}
		return nil
	}
	if i := source.Intent; i != nil && !i.LegacyEvidence {
		if i.BotID != source.BotID || i.SpaceID != source.SpaceID || i.ActorProfileID != source.ActorProfileID {
			return nil, ErrSpaceLifecycleConflict
		}
		if err := add(i.InstallationID, i.OperationID, source.NativeCancelOperationID, source.NativeDrainOperationID, i.Generation); err != nil {
			return nil, err
		}
	}
	for _, retained := range source.Retained {
		target := retained.Target
		if target.LegacyEvidence {
			continue
		}
		if target.BotID != source.BotID || target.SpaceID != source.SpaceID || target.ActorProfileID != source.ActorProfileID {
			return nil, ErrSpaceLifecycleConflict
		}
		if err := add(target.InstallationID, target.InstallOperationID, target.CancelOperationID, uuid.Nil, target.BotGeneration); err != nil {
			return nil, err
		}
	}
	keys := make([]uuid.UUID, 0, len(byInstallation))
	for installation := range byInstallation {
		keys = append(keys, installation)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	result := make([]BotPurgeNativeDrain, 0, len(keys))
	for _, installation := range keys {
		result = append(result, byInstallation[installation])
	}
	return result, nil
}

func capturePurgeRetained(ctx context.Context, tx pgx.Tx, bot, space uuid.UUID, purpose string, include ...uuid.UUID) ([]BotPurgeRetainedWork, error) {
	// Purpose selects two closed repository-owned table families, never caller SQL.
	claim := &BotDeletion{Purpose: purpose}
	rows, err := tx.Query(ctx, lifecycleQuery(claim, `SELECT o.operation_id,o.decision_bytes,o.decision_sha256,t.scope_bytes,t.scope_sha256 FROM bot_deletion_operations o JOIN bot_deletion_installations t ON t.operation_id=o.operation_id WHERE t.bot_id=$1 AND t.space_id=$2 AND (o.state<>'COMPLETE' OR o.operation_id=ANY($3::uuid[])) ORDER BY o.operation_id FOR UPDATE OF o,t`), bot, space, include)
	if err != nil {
		return nil, err
	}
	var result []BotPurgeRetainedWork
	for rows.Next() {
		w := BotPurgeRetainedWork{Purpose: purpose}
		if err = rows.Scan(&w.OperationID, &w.DecisionBytes, &w.DecisionSHA256, &w.TargetBytes, &w.TargetSHA256); err != nil {
			break
		}
		sum := sha256.Sum256(w.DecisionBytes)
		targetSHA := sha256.Sum256(w.TargetBytes)
		if !bytes.Equal(sum[:], w.DecisionSHA256) || !bytes.Equal(targetSHA[:], w.TargetSHA256) || json.Unmarshal(w.TargetBytes, &w.Target) != nil || w.Target.OperationID != w.OperationID || w.Target.BotID != bot || w.Target.SpaceID != space {
			err = ErrSpaceLifecycleConflict
			break
		}
		canonical, e := json.Marshal(w.Target)
		if e != nil || !bytes.Equal(canonical, w.TargetBytes) {
			err = ErrSpaceLifecycleConflict
			break
		}
		result = append(result, w)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range result {
		w := &result[i]
		effects, e := tx.Query(ctx, lifecycleQuery(claim, `SELECT effect_id,kind,chat_id,state,request_bytes,request_sha256,receipt_bytes,receipt_sha256 FROM bot_deletion_effects WHERE target_id=$1 ORDER BY effect_id FOR UPDATE`), w.Target.TargetID)
		if e != nil {
			return nil, e
		}
		for effects.Next() {
			var effect BotPurgeRetainedEffect
			if e = effects.Scan(&effect.EffectID, &effect.Kind, &effect.ChatID, &effect.State, &effect.RequestBytes, &effect.RequestSHA256, &effect.ReceiptBytes, &effect.ReceiptSHA256); e != nil {
				break
			}
			for _, pair := range [][2][]byte{{effect.RequestBytes, effect.RequestSHA256}, {effect.ReceiptBytes, effect.ReceiptSHA256}} {
				if pair[0] != nil {
					sum := sha256.Sum256(pair[0])
					if !bytes.Equal(sum[:], pair[1]) {
						e = ErrSpaceLifecycleConflict
						break
					}
				} else if pair[1] != nil {
					e = ErrSpaceLifecycleConflict
					break
				}
			}
			if e != nil {
				break
			}
			w.Effects = append(w.Effects, effect)
		}
		if e == nil {
			e = effects.Err()
		}
		effects.Close()
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}
