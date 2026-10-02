package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/spacemutationlock"
)

var (
	ErrSpaceManifestBinding   = errors.New("Messaging Space manifest binding mismatch")
	ErrSpaceManifestOrder     = errors.New("Messaging Space manifest pages are out of order")
	ErrSpaceManifestNotSealed = errors.New("Messaging Space manifest is not sealed")
	ErrSpaceLifecycleConflict = errors.New("Messaging Space lifecycle request conflicts with saved evidence")
	ErrSpaceLifecycleOrder    = errors.New("Messaging Space lifecycle transition is out of order")
)

type SpacePurgeManifestPageInput struct {
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	ScheduleGeneration  uint64
	Page                *chatv1.SpacePurgeManifestPage
	SealsManifest       bool
	RequestBytes        []byte
	RequestSHA256       []byte
	ReceiptBytes        []byte
}

type SpaceLifecycleFenceInput struct {
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	Generation          uint64
	State               commonv1.LifecycleFenceState
	Manifest            *commonv1.ManifestBinding
	RequestBytes        []byte
	RequestSHA256       []byte
	ReceiptBytes        []byte
}

// ApplySpaceLifecycleFence persists a monotonic Messaging participant fence
// only after the exact imported Chat manifest is complete and sealed.
func (s *MessagesStore) ApplySpaceLifecycleFence(ctx context.Context, input SpaceLifecycleFenceInput) ([]byte, error) {
	if s == nil || s.Pool == nil || input.SpaceID == uuid.Nil || input.DeletionOperationID == uuid.Nil || input.Generation == 0 ||
		input.Manifest == nil || input.Manifest.GetManifestId() == "" || len(input.Manifest.GetManifestSha256()) != sha256.Size ||
		len(input.RequestBytes) == 0 || len(input.RequestSHA256) != sha256.Size || len(input.ReceiptBytes) == 0 {
		return nil, errors.New("invalid Messaging Space lifecycle fence")
	}
	state := messagingLifecycleStateName(input.State)
	if state == "" {
		return nil, ErrSpaceLifecycleOrder
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(input.SpaceID)); err != nil {
		return nil, err
	}
	var priorRequestHash, priorRequestBytes, priorReceiptBytes []byte
	err = tx.QueryRow(ctx, `SELECT request_sha256,request_bytes,receipt_bytes
FROM messaging_space_lifecycle_operations
WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`, input.SpaceID, input.DeletionOperationID, input.Generation).
		Scan(&priorRequestHash, &priorRequestBytes, &priorReceiptBytes)
	if err == nil {
		if !bytes.Equal(priorRequestHash, input.RequestSHA256) || !bytes.Equal(priorRequestBytes, input.RequestBytes) {
			return nil, ErrSpaceLifecycleConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return priorReceiptBytes, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var currentGeneration uint64
	var currentState string
	var currentOperation uuid.UUID
	var currentManifestID string
	var currentManifestHash []byte
	var currentManifestItems uint64
	var currentReceipt []byte
	err = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id,source_manifest_id,source_manifest_sha256,source_manifest_item_count,receipt_bytes
FROM messaging_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, input.SpaceID).
		Scan(&currentGeneration, &currentState, &currentOperation, &currentManifestID, &currentManifestHash, &currentManifestItems, &currentReceipt)
	currentExists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if currentExists && input.Generation < currentGeneration {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return currentReceipt, nil
	}
	if currentExists && input.Generation == currentGeneration {
		return nil, ErrSpaceLifecycleConflict
	}
	if input.Generation > math.MaxInt64 {
		return nil, ErrSpaceLifecycleOrder
	}
	var manifestGeneration uint64
	var manifestID string
	var manifestHash []byte
	var manifestItems uint64
	var sealed bool
	err = tx.QueryRow(ctx, `SELECT schedule_generation,manifest_id,manifest_sha256,item_count,sealed
FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2 FOR SHARE`, input.SpaceID, input.DeletionOperationID).
		Scan(&manifestGeneration, &manifestID, &manifestHash, &manifestItems, &sealed)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !sealed {
		return nil, ErrSpaceManifestNotSealed
	}
	if err != nil {
		return nil, err
	}
	if manifestID != input.Manifest.GetManifestId() || !bytes.Equal(manifestHash, input.Manifest.GetManifestSha256()) || manifestItems != input.Manifest.GetItemCount() {
		return nil, ErrSpaceManifestBinding
	}
	switch input.State {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		if input.Generation != manifestGeneration || currentExists && (currentState != "LIVE" || input.Generation != currentGeneration+1) {
			return nil, ErrSpaceLifecycleOrder
		}
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE,
		commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		if !currentExists || currentState != "FROZEN" || input.Generation != currentGeneration+1 ||
			currentOperation != input.DeletionOperationID || currentManifestID != manifestID ||
			!bytes.Equal(currentManifestHash, manifestHash) || currentManifestItems != manifestItems || currentGeneration != manifestGeneration {
			return nil, ErrSpaceLifecycleOrder
		}
	default:
		return nil, ErrSpaceLifecycleOrder
	}
	var appliedAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, err
	}
	if !currentExists {
		_, err = tx.Exec(ctx, `INSERT INTO messaging_space_lifecycle_fences(space_id,deletion_operation_id,generation,state,source_schedule_generation,source_manifest_id,source_manifest_sha256,source_manifest_item_count,request_bytes,request_sha256,receipt_bytes,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, input.SpaceID, input.DeletionOperationID, input.Generation, state, manifestGeneration,
			manifestID, manifestHash, manifestItems, input.RequestBytes, input.RequestSHA256, input.ReceiptBytes, appliedAt)
	} else {
		_, err = tx.Exec(ctx, `UPDATE messaging_space_lifecycle_fences SET deletion_operation_id=$2,generation=$3,state=$4,source_schedule_generation=$5,source_manifest_id=$6,source_manifest_sha256=$7,source_manifest_item_count=$8,request_bytes=$9,request_sha256=$10,receipt_bytes=$11,updated_at=$12
		WHERE space_id=$1`, input.SpaceID, input.DeletionOperationID, input.Generation, state, manifestGeneration,
			manifestID, manifestHash, manifestItems, input.RequestBytes, input.RequestSHA256, input.ReceiptBytes, appliedAt)
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messaging_space_lifecycle_operations(space_id,deletion_operation_id,generation,request_bytes,request_sha256,receipt_bytes,applied_at,retain_until)
VALUES($1,$2,$3,$4,$5,$6,$7,$7::timestamptz+interval '30 days')`, input.SpaceID, input.DeletionOperationID, input.Generation, input.RequestBytes, input.RequestSHA256, input.ReceiptBytes, appliedAt); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return append([]byte(nil), input.ReceiptBytes...), nil
}

func messagingLifecycleStateName(state commonv1.LifecycleFenceState) string {
	switch state {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		return "FROZEN"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		return "LIVE"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		return "PURGE_DECIDED"
	default:
		return ""
	}
}

// ImportSpacePurgeManifestPage stores an exact Chat page and its immutable
// receipt. Only a complete, root-hash-verified sequence becomes sealed.
func (s *MessagesStore) ImportSpacePurgeManifestPage(ctx context.Context, input SpacePurgeManifestPageInput) ([]byte, error) {
	if s == nil || s.Pool == nil || input.SpaceID == uuid.Nil || input.DeletionOperationID == uuid.Nil ||
		input.ScheduleGeneration == 0 || input.Page == nil || len(input.RequestBytes) == 0 || len(input.RequestSHA256) != sha256.Size || len(input.ReceiptBytes) == 0 {
		return nil, errors.New("invalid Messaging Space manifest import")
	}
	page := input.Page
	if page.GetManifest() == nil || page.GetPageIndex() > uint64(^uint32(0)) || len(page.GetPageSha256()) != sha256.Size {
		return nil, ErrSpaceManifestBinding
	}
	itemCount := page.GetManifest().GetItemCount()
	pageCount := (itemCount + 999) / 1000
	if pageCount == 0 {
		pageCount = 1
	}
	if page.GetPageIndex() >= pageCount {
		return nil, ErrSpaceManifestOrder
	}
	expectedItems := 1000
	finalPage := page.GetPageIndex()+1 == pageCount
	if finalPage {
		expectedItems = int(itemCount - page.GetPageIndex()*1000)
	}
	if len(page.GetItemIds()) != expectedItems || input.SealsManifest != finalPage ||
		finalPage && page.GetNextPageToken() != "" || !finalPage && page.GetNextPageToken() == "" {
		return nil, ErrSpaceManifestBinding
	}
	pageBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(page)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spacemutationlock.Key(input.SpaceID)); err != nil {
		return nil, err
	}
	var storedPageCount uint64
	var manifestID string
	var manifestHash []byte
	var savedItemCount uint64
	var sealed bool
	err = tx.QueryRow(ctx, `SELECT page_count,manifest_id,manifest_sha256,item_count,sealed
FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2 FOR UPDATE`, input.SpaceID, input.DeletionOperationID).
		Scan(&storedPageCount, &manifestID, &manifestHash, &savedItemCount, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		if page.GetPageIndex() != 0 {
			return nil, ErrSpaceManifestOrder
		}
		manifestID = page.GetManifest().GetManifestId()
		manifestHash = append([]byte(nil), page.GetManifest().GetManifestSha256()...)
		savedItemCount = itemCount
		if _, err = tx.Exec(ctx, `INSERT INTO messaging_space_chat_manifests(space_id,deletion_operation_id,schedule_generation,manifest_id,manifest_sha256,item_count,page_count)
VALUES($1,$2,$3,$4,$5,$6,$7)`, input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration, manifestID, manifestHash, savedItemCount, pageCount); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		var storedGeneration uint64
		if err = tx.QueryRow(ctx, `SELECT schedule_generation FROM messaging_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, input.SpaceID, input.DeletionOperationID).Scan(&storedGeneration); err != nil {
			return nil, err
		}
		if storedGeneration != input.ScheduleGeneration || manifestID != page.GetManifest().GetManifestId() ||
			!bytes.Equal(manifestHash, page.GetManifest().GetManifestSha256()) || savedItemCount != itemCount || storedPageCount != pageCount {
			return nil, ErrSpaceManifestBinding
		}
	}
	var savedRequestHash, savedRequestBytes, savedReceiptBytes []byte
	err = tx.QueryRow(ctx, `SELECT request_sha256,request_bytes,receipt_bytes FROM messaging_space_chat_manifest_pages
WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND page_index=$4`,
		input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration, page.GetPageIndex()).
		Scan(&savedRequestHash, &savedRequestBytes, &savedReceiptBytes)
	if err == nil {
		if !bytes.Equal(savedRequestHash, input.RequestSHA256) || !bytes.Equal(savedRequestBytes, input.RequestBytes) {
			return nil, ErrSpaceManifestBinding
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return savedReceiptBytes, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var importedPageCount int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM messaging_space_chat_manifest_pages WHERE space_id=$1 AND deletion_operation_id=$2`, input.SpaceID, input.DeletionOperationID).Scan(&importedPageCount); err != nil {
		return nil, err
	}
	if sealed || page.GetPageIndex() != uint64(importedPageCount) {
		return nil, ErrSpaceManifestOrder
	}
	if page.GetPageIndex() > 0 {
		var previousToken string
		if err = tx.QueryRow(ctx, `SELECT next_page_token FROM messaging_space_chat_manifest_pages
WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND page_index=$4`, input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration, page.GetPageIndex()-1).Scan(&previousToken); err != nil || previousToken == "" {
			return nil, ErrSpaceManifestOrder
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO messaging_space_chat_manifest_pages(space_id,deletion_operation_id,schedule_generation,page_index,page_bytes,page_sha256,item_count,next_page_token,request_bytes,request_sha256,receipt_bytes)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration,
		page.GetPageIndex(), pageBytes, page.GetPageSha256(), len(page.GetItemIds()), page.GetNextPageToken(), input.RequestBytes, input.RequestSHA256, input.ReceiptBytes); err != nil {
		return nil, err
	}
	for index, raw := range page.GetItemIds() {
		chatID, parseErr := uuid.Parse(raw)
		if parseErr != nil || chatID == uuid.Nil || chatID.String() != raw {
			return nil, ErrSpaceManifestBinding
		}
		if _, err = tx.Exec(ctx, `INSERT INTO messaging_space_chat_manifest_items(space_id,deletion_operation_id,schedule_generation,page_index,item_index,chat_id)
VALUES($1,$2,$3,$4,$5,$6)`, input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration, page.GetPageIndex(), index, chatID); err != nil {
			return nil, err
		}
	}
	if finalPage {
		rows, queryErr := tx.Query(ctx, `SELECT chat_id FROM messaging_space_chat_manifest_items
WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 ORDER BY page_index,item_index`, input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration)
		if queryErr != nil {
			return nil, queryErr
		}
		ids := make([]uuid.UUID, 0, savedItemCount)
		for rows.Next() {
			var id uuid.UUID
			if queryErr = rows.Scan(&id); queryErr != nil {
				rows.Close()
				return nil, queryErr
			}
			if len(ids) > 0 && bytes.Compare(ids[len(ids)-1][:], id[:]) >= 0 {
				rows.Close()
				return nil, ErrSpaceManifestBinding
			}
			ids = append(ids, id)
		}
		if queryErr = rows.Err(); queryErr != nil {
			rows.Close()
			return nil, queryErr
		}
		rows.Close()
		if uint64(len(ids)) != savedItemCount || !bytes.Equal(messagingChatManifestSHA(input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration, ids), manifestHash) {
			return nil, ErrSpaceManifestBinding
		}
		if _, err = tx.Exec(ctx, `UPDATE messaging_space_chat_manifests SET sealed=TRUE,sealed_at=clock_timestamp()
WHERE space_id=$1 AND deletion_operation_id=$2 AND schedule_generation=$3 AND sealed=FALSE`, input.SpaceID, input.DeletionOperationID, input.ScheduleGeneration); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return append([]byte(nil), input.ReceiptBytes...), nil
}

func messagingChatManifestSHA(spaceID, operationID uuid.UUID, generation uint64, ids []uuid.UUID) []byte {
	payload := make([]byte, 0, len("voice.chat.v1.SpaceDeletionManifest")+1+16+16+16+16*len(ids))
	payload = append(payload, "voice.chat.v1.SpaceDeletionManifest"...)
	payload = append(payload, 0)
	payload = append(payload, spaceID[:]...)
	payload = append(payload, operationID[:]...)
	var generationBytes [8]byte
	binary.BigEndian.PutUint64(generationBytes[:], generation)
	payload = append(payload, generationBytes[:]...)
	binary.BigEndian.PutUint64(generationBytes[:], uint64(len(ids)))
	payload = append(payload, generationBytes[:]...)
	for _, id := range ids {
		payload = append(payload, id[:]...)
	}
	sum := sha256.Sum256(payload)
	return sum[:]
}
