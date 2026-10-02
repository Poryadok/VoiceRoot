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
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
)

func (s *SettingsStore) ImportSpacePurgeManifestPage(ctx context.Context, request *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageReceipt, error) {
	if s == nil || s.Pool == nil || request == nil || request.GetPage() == nil || request.GetPage().GetManifest() == nil {
		return nil, ErrNotImplemented
	}
	spaceID, err := notificationLifecycleUUID(request.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operationID, err := notificationLifecycleUUID(request.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	page, manifest := request.GetPage(), request.GetPage().GetManifest()
	generation, count := request.GetScheduleGeneration(), manifest.GetItemCount()
	if request.GetProtocolVersion() != 1 || page.GetProtocolVersion() != 1 || generation == 0 || generation >= math.MaxInt64 || count > math.MaxInt64 || len(manifest.GetManifestSha256()) != 32 {
		return nil, errors.New("invalid Notification manifest binding")
	}
	if _, err = notificationLifecycleUUID(manifest.GetManifestId()); err != nil {
		return nil, err
	}
	pageCount := (count + 999) / 1000
	if pageCount == 0 {
		pageCount = 1
	}
	index := page.GetPageIndex()
	if index >= pageCount {
		return nil, errors.New("manifest page out of range")
	}
	final := index+1 == pageCount
	expected := uint64(1000)
	if final {
		expected = count - index*1000
	}
	if uint64(len(page.GetItemIds())) != expected || request.GetSealsManifest() != final || (page.GetNextPageToken() == "") != final {
		return nil, errors.New("invalid manifest seal position")
	}
	ids := make([]uuid.UUID, len(page.GetItemIds()))
	for i, raw := range page.GetItemIds() {
		ids[i], err = notificationLifecycleUUID(raw)
		if err != nil {
			return nil, err
		}
		if i > 0 && bytes.Compare(ids[i-1][:], ids[i][:]) >= 0 {
			return nil, errors.New("manifest IDs are not sorted and unique")
		}
	}
	if !bytes.Equal(notificationChatPageHash(manifest.GetManifestSha256(), index, ids), page.GetPageSha256()) {
		return nil, errors.New("manifest page hash mismatch")
	}
	requestBytes, requestHash, err := notificationLifecycleWrapperBytes(request)
	if err != nil {
		return nil, err
	}
	manifestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(manifest)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Every delivery/settings access locks its Chat before its Space. Import
	// drains pre-existing delivery before introducing a new immutable scope key.
	for _, id := range ids {
		if err = notificationLifecycleLock(ctx, tx, id); err != nil {
			return nil, err
		}
	}
	if err = notificationLifecycleLock(ctx, tx, spaceID); err != nil {
		return nil, err
	}
	var savedRequest, savedReceipt []byte
	err = tx.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM notification_space_chat_manifest_pages WHERE space_id=$1 AND deletion_operation_id=$2 AND page_index=$3`, spaceID, operationID, index).Scan(&savedRequest, &savedReceipt)
	if err == nil {
		if !bytes.Equal(requestBytes, savedRequest) {
			return nil, errors.New("manifest replay conflicts")
		}
		out := &notificationv1.ImportSpacePurgeManifestPageReceipt{}
		if err = proto.Unmarshal(savedReceipt, out); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var state string
	var currentGeneration uint64
	err = tx.QueryRow(ctx, `SELECT state,generation FROM notification_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&state, &currentGeneration)
	if err == nil && (state != "LIVE" || generation <= currentGeneration) {
		return nil, ErrSpaceLifecycleNotLive
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var savedManifest []byte
	var savedGeneration uint64
	var sealed bool
	err = tx.QueryRow(ctx, `SELECT manifest_bytes,schedule_generation,sealed FROM notification_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2 FOR UPDATE`, spaceID, operationID).Scan(&savedManifest, &savedGeneration, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		if index != 0 {
			return nil, errors.New("manifest page gap")
		}
		_, err = tx.Exec(ctx, `INSERT INTO notification_space_chat_manifests(space_id,deletion_operation_id,schedule_generation,manifest_bytes) VALUES($1,$2,$3,$4)`, spaceID, operationID, generation, manifestBytes)
	} else if err == nil && (sealed || savedGeneration != generation || !bytes.Equal(savedManifest, manifestBytes)) {
		return nil, errors.New("manifest root changed or already sealed")
	}
	if err != nil {
		return nil, err
	}
	var priorCount uint64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM notification_space_chat_manifest_pages WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&priorCount); err != nil {
		return nil, err
	}
	if priorCount != index {
		return nil, errors.New("manifest page gap")
	}
	for _, id := range ids {
		tag, writeErr := tx.Exec(ctx, `INSERT INTO notification_space_chat_fences(chat_id,space_id,blocked) VALUES($1,$2,TRUE) ON CONFLICT(chat_id) DO UPDATE SET blocked=TRUE WHERE notification_space_chat_fences.space_id=EXCLUDED.space_id`, id, spaceID)
		if writeErr != nil {
			return nil, writeErr
		}
		if tag.RowsAffected() != 1 {
			return nil, errors.New("Chat manifest scope changed")
		}
	}
	var completed time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completed); err != nil {
		return nil, err
	}
	receipt := &notificationv1.ImportSpacePurgeManifestPageReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: generation, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding), PageIndex: index, AcceptedCount: uint64(len(ids)), PageSha256: append([]byte(nil), page.GetPageSha256()...), ManifestSealed: final, RequestSha256: requestHash, CompletedAt: timestamppb.New(completed)}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notification_space_chat_manifest_pages(space_id,deletion_operation_id,page_index,request_bytes,receipt_bytes,chat_ids) VALUES($1,$2,$3,$4,$5,$6)`, spaceID, operationID, index, requestBytes, receiptBytes, ids); err != nil {
		return nil, err
	}
	if final {
		rows, queryErr := tx.Query(ctx, `SELECT unnest(chat_ids) FROM notification_space_chat_manifest_pages WHERE space_id=$1 AND deletion_operation_id=$2 ORDER BY page_index`, spaceID, operationID)
		if queryErr != nil {
			return nil, queryErr
		}
		all := make([]uuid.UUID, 0)
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if len(all) > 0 && bytes.Compare(all[len(all)-1][:], id[:]) >= 0 {
				rows.Close()
				return nil, errors.New("manifest global order mismatch")
			}
			all = append(all, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if uint64(len(all)) != count || !bytes.Equal(notificationChatRootHash(spaceID, operationID, generation, all), manifest.GetManifestSha256()) {
			return nil, errors.New("manifest root hash mismatch")
		}
		if _, err = tx.Exec(ctx, `UPDATE notification_space_chat_manifests SET sealed=TRUE WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return receipt, nil
}

func notificationChatRootHash(spaceID, operationID uuid.UUID, generation uint64, ids []uuid.UUID) []byte {
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	h.Write(spaceID[:])
	h.Write(operationID[:])
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], generation)
	h.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(ids)))
	h.Write(integer[:])
	for _, id := range ids {
		h.Write(id[:])
	}
	return h.Sum(nil)
}

func notificationChatPageHash(root []byte, index uint64, ids []uuid.UUID) []byte {
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	h.Write(root)
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], index)
	h.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(ids)))
	h.Write(integer[:])
	for _, id := range ids {
		h.Write(id[:])
	}
	return h.Sum(nil)
}
