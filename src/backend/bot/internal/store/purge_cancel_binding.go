package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
)

// The caller already owns Space -> Bot -> live-intent guards. This only
// selects an existing exact cancellation identity; admission/authority remains
// the caller's responsibility. Immutable divergent bindings are never aliased.
func frozenNativeCancelTx(ctx context.Context, tx pgx.Tx, i *InstallationIntent, candidate uuid.UUID) (uuid.UUID, error) {
	choose := func(id uuid.UUID) error {
		if id == uuid.Nil {
			return ErrInstallationConflict
		}
		if candidate != uuid.Nil && candidate != id {
			return ErrInstallationConflict
		}
		candidate = id
		return nil
	}
	if i.CancelOperationID != uuid.Nil {
		if err := choose(i.CancelOperationID); err != nil {
			return uuid.Nil, err
		}
	}
	if i.LegacyEvidence {
		if candidate == uuid.Nil {
			candidate = uuid.New()
		}
		return candidate, nil
	}
	for _, purpose := range []string{"global_delete", "space_uninstall"} {
		claim := &BotDeletion{Purpose: purpose}
		rows, err := tx.Query(ctx, lifecycleQuery(claim, `SELECT t.actor_profile_id,t.install_operation_id,t.bot_generation,t.cancel_operation_id,t.legacy_evidence FROM bot_deletion_installations t JOIN bot_deletion_operations o ON o.operation_id=t.operation_id WHERE t.bot_id=$1 AND t.space_id=$2 AND t.installation_id=$3 AND o.state<>'COMPLETE' ORDER BY t.target_id`), i.BotID, i.SpaceID, i.InstallationID)
		if err != nil {
			return uuid.Nil, err
		}
		for rows.Next() {
			var actor, add, cancel uuid.UUID
			var generation int64
			var legacy bool
			if err = rows.Scan(&actor, &add, &generation, &cancel, &legacy); err != nil {
				break
			}
			if legacy || actor != i.ActorProfileID || add != i.OperationID || generation != i.Generation {
				err = ErrInstallationConflict
				break
			}
			if err = choose(cancel); err != nil {
				break
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return uuid.Nil, err
		}
	}
	var raw, digest []byte
	err := tx.QueryRow(ctx, `SELECT source_bytes,source_sha256 FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, i.SpaceID, i.BotID).Scan(&raw, &digest)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if err == nil {
		var source BotPurgeSource
		sum := sha256.Sum256(raw)
		if !bytes.Equal(sum[:], digest) || json.Unmarshal(raw, &source) != nil || source.Version != 1 || source.BotID != i.BotID || source.SpaceID != i.SpaceID || source.ActorProfileID != i.ActorProfileID {
			return uuid.Nil, ErrInstallationConflict
		}
		canonical, e := json.Marshal(source)
		if e != nil || !bytes.Equal(raw, canonical) {
			return uuid.Nil, ErrInstallationConflict
		}
		for _, drain := range source.NativeDrains {
			q := new(spacev1.RemoveBotMemberRequest)
			sum := sha256.Sum256(drain.RequestBytes)
			if !bytes.Equal(sum[:], drain.RequestSHA256) || !purgeProofCanonical(drain.RequestBytes, q) {
				return uuid.Nil, ErrInstallationConflict
			}
			v := q.InstallationIntent
			if v == nil {
				return uuid.Nil, ErrInstallationConflict
			}
			if v.InstallationId != i.InstallationID.String() {
				continue
			}
			if q.SpaceId != i.SpaceID.String() || q.ProfileId != i.ActorProfileID.String() || q.InstallOperationId != i.OperationID.String() || v.BotId != i.BotID.String() || v.SpaceId != i.SpaceID.String() || v.ActorProfileId != i.ActorProfileID.String() || v.Generation != uint64(i.Generation) {
				return uuid.Nil, ErrInstallationConflict
			}
			id, e := lifecycleID(v.OperationId)
			if e != nil {
				return uuid.Nil, ErrInstallationConflict
			}
			if e = choose(id); e != nil {
				return uuid.Nil, e
			}
		}
	}
	if candidate == uuid.Nil {
		candidate = uuid.New()
	}
	return candidate, nil
}
