package store

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/spacemutationlock"
)

// SpaceFileProducerReferences snapshots after Messaging's durable FROZEN fence.
// It survives child payload deletion and returns identical tuples on restart.
func (s *MessagesStore) SpaceFileProducerReferences(ctx context.Context, space, operation uuid.UUID, generation uint64) ([]*filev1.FileReferenceKey, error) {
	if s == nil || s.Pool == nil || space == uuid.Nil || operation == uuid.Nil || generation == 0 {
		return nil, ErrSpaceManifestBinding
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(space)); err != nil {
		return nil, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT references_bytes FROM messaging_space_file_producers WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3`, space, operation, generation).Scan(&raw)
	if err == nil {
		var saved filev1.AcquireFileReferencesRequest
		if err = proto.Unmarshal(raw, &saved); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return saved.References, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var state string
	var storedGeneration uint64
	var savedOperation uuid.UUID
	err = tx.QueryRow(ctx, `SELECT state,generation,deletion_operation_id FROM messaging_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, space).Scan(&state, &storedGeneration, &savedOperation)
	if err != nil || state != "FROZEN" || storedGeneration != generation || savedOperation != operation {
		return nil, ErrSpaceLifecycleOrder
	}
	rows, err := tx.Query(ctx, `SELECT m.id,m.attachments::text FROM messages m JOIN messaging_space_chat_manifest_items i ON i.chat_id=m.chat_id WHERE i.space_id=$1 AND i.deletion_operation_id=$2 AND i.schedule_generation=$3 ORDER BY m.id`, space, operation, generation)
	if err != nil {
		return nil, err
	}
	var refs []*filev1.FileReferenceKey
	for rows.Next() {
		var id uuid.UUID
		var attachments string
		if err = rows.Scan(&id, &attachments); err != nil {
			rows.Close()
			return nil, err
		}
		message, parseErr := managedChatPurgeMessage(id, attachments)
		if parseErr != nil {
			rows.Close()
			return nil, parseErr
		}
		for _, fileID := range message.FileIDs {
			scope := space.String()
			refs = append(refs, &filev1.FileReferenceKey{FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: id.String(), ScopeSpaceId: &scope})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].FileId != refs[j].FileId {
			return refs[i].FileId < refs[j].FileId
		}
		return refs[i].OwnerId < refs[j].OwnerId
	})
	raw, err = (proto.MarshalOptions{Deterministic: true}).Marshal(&filev1.AcquireFileReferencesRequest{References: refs})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messaging_space_file_producers(space_id,deletion_operation_id,schedule_generation,references_bytes) VALUES($1,$2,$3,$4)`, space, operation, generation, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return refs, nil
}
