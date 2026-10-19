package store

import (
	"bytes"
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
)

type BotPurgeChatCoverageInput struct {
	SpaceID, DeletionOperationID uuid.UUID
	PurgeGeneration              uint64
	Manifest                     *commonv1.ManifestBinding
	Chats                        []uuid.UUID
}

// BotPurgeChatCoverage is a finite membership observation, never a purge receipt.
type BotPurgeChatCoverage struct {
	SourceScheduleGeneration uint64
	Covered                  []bool
}

func (s *MessagesStore) ObserveBotPurgeChatCoverage(ctx context.Context, input BotPurgeChatCoverageInput) (*BotPurgeChatCoverage, error) {
	if s == nil || s.Pool == nil || input.SpaceID == uuid.Nil || input.DeletionOperationID == uuid.Nil || input.PurgeGeneration == 0 || input.PurgeGeneration > math.MaxInt64 || input.Manifest == nil || len(input.Manifest.ProtoReflect().GetUnknown()) != 0 || len(input.Manifest.ManifestSha256) != 32 || input.Manifest.ItemCount > math.MaxInt64 || len(input.Chats) > 1000 {
		return nil, ErrSpaceManifestBinding
	}
	manifestID, err := uuid.Parse(input.Manifest.ManifestId)
	if err != nil || manifestID == uuid.Nil || manifestID.String() != input.Manifest.ManifestId {
		return nil, ErrSpaceManifestBinding
	}
	// Own every source field before database waits; callers cannot change which
	// sealed source or chat set this observation describes while it is blocked.
	input.Manifest = proto.Clone(input.Manifest).(*commonv1.ManifestBinding)
	input.Chats = append([]uuid.UUID(nil), input.Chats...)
	for i, id := range input.Chats {
		if id == uuid.Nil || i > 0 && bytes.Compare(input.Chats[i-1][:], id[:]) >= 0 {
			return nil, ErrSpaceManifestBinding
		}
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var generation, schedule, count uint64
	var operation uuid.UUID
	var state, storedID string
	var storedSHA []byte
	err = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,source_schedule_generation,source_manifest_id::text,source_manifest_sha256,source_manifest_item_count FROM messaging_space_lifecycle_fences WHERE space_id=$1`, input.SpaceID).Scan(&generation, &state, &operation, &schedule, &storedID, &storedSHA, &count)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSpaceManifestBinding
	}
	if err != nil {
		return nil, err
	}
	if generation != input.PurgeGeneration || (state != "PURGE_DECIDED" && state != "PURGED") || operation != input.DeletionOperationID || schedule == 0 || schedule > math.MaxInt64 || storedID != input.Manifest.ManifestId || !bytes.Equal(storedSHA, input.Manifest.ManifestSha256) || count != input.Manifest.ItemCount {
		return nil, ErrSpaceManifestBinding
	}
	var sealed bool
	var sourceSchedule, sourceCount uint64
	var sourceID string
	var sourceSHA []byte
	err = tx.QueryRow(ctx, `SELECT schedule_generation,manifest_id::text,manifest_sha256,item_count,sealed FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, input.SpaceID, input.DeletionOperationID).Scan(&sourceSchedule, &sourceID, &sourceSHA, &sourceCount, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSpaceManifestBinding
	}
	if err != nil {
		return nil, err
	}
	if !sealed || sourceSchedule != schedule || sourceID != storedID || !bytes.Equal(sourceSHA, storedSHA) || sourceCount != count {
		return nil, ErrSpaceManifestBinding
	}
	observed := &BotPurgeChatCoverage{SourceScheduleGeneration: schedule, Covered: make([]bool, len(input.Chats))}
	if len(input.Chats) > 0 {
		rows, queryErr := tx.Query(ctx, `SELECT chat_id FROM messaging_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND chat_id=ANY($4::uuid[]) ORDER BY chat_id`, input.SpaceID, input.DeletionOperationID, schedule, input.Chats)
		if queryErr != nil {
			return nil, queryErr
		}
		found := make(map[uuid.UUID]bool, len(input.Chats))
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if found[id] {
				rows.Close()
				return nil, ErrSpaceManifestBinding
			}
			found[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		for i, id := range input.Chats {
			observed.Covered[i] = found[id]
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return observed, nil
}
