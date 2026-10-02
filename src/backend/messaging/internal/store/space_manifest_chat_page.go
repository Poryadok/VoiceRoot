package store

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const spaceManifestChatPageSize = 1000

// SpaceManifestChatPage returns one bounded page from the sealed Chat manifest
// only when its full binding matches the purge request.
func (s *MessagesStore) SpaceManifestChatPage(ctx context.Context, spaceID, operationID uuid.UUID, scheduleGeneration uint64, manifestID string, manifestSHA256 []byte, itemCount uint64, pageIndex uint64) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil || spaceID == uuid.Nil || operationID == uuid.Nil || scheduleGeneration == 0 ||
		manifestID == "" || len(manifestSHA256) != 32 || itemCount == 0 || pageIndex >= (itemCount+spaceManifestChatPageSize-1)/spaceManifestChatPageSize {
		return nil, ErrSpaceManifestBinding
	}
	var storedGeneration, storedCount uint64
	var storedID string
	var storedSHA []byte
	var sealed bool
	err := s.Pool.QueryRow(ctx, `SELECT schedule_generation,manifest_id,manifest_sha256,item_count,sealed
FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).
		Scan(&storedGeneration, &storedID, &storedSHA, &storedCount, &sealed)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!sealed || storedGeneration != scheduleGeneration || storedID != manifestID || !bytes.Equal(storedSHA, manifestSHA256) || storedCount != itemCount) {
		return nil, ErrSpaceManifestBinding
	}
	if err != nil {
		return nil, err
	}
	var count int
	if pageIndex+1 == (itemCount+spaceManifestChatPageSize-1)/spaceManifestChatPageSize {
		count = int(itemCount - pageIndex*spaceManifestChatPageSize)
	} else {
		count = spaceManifestChatPageSize
	}
	rows, err := s.Pool.Query(ctx, `SELECT chat_id FROM messaging_space_chat_manifest_items
WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND page_index=$4
ORDER BY item_index LIMIT $5`, spaceID, operationID, storedGeneration, pageIndex, count)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0, count)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id == uuid.Nil || len(ids) > 0 && bytes.Compare(ids[len(ids)-1][:], id[:]) >= 0 {
			return nil, ErrSpaceManifestBinding
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) != count {
		return nil, ErrSpaceManifestNotSealed
	}
	return ids, nil
}
