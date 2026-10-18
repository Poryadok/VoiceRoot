package lifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	realtimev1 "voice.app/voice/realtime/v1"
)

func canonicalID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, ErrConflict
	}
	return id, nil
}
func known(message proto.Message) bool {
	if message == nil || !message.ProtoReflect().IsValid() {
		return false
	}
	return knownReflect(message.ProtoReflect())
}
func knownReflect(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return false
	}
	ok := true
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() != protoreflect.MessageKind {
			return true
		}
		if field.IsList() {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if !knownReflect(list.Get(i).Message()) {
					ok = false
					return false
				}
			}
		} else if !knownReflect(value.Message()) {
			ok = false
			return false
		}
		return true
	})
	return ok
}
func deterministic(message proto.Message) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(message)
}
func digest(message proto.Message, wire []byte) []byte {
	h := sha256.New()
	h.Write([]byte(message.ProtoReflect().Descriptor().FullName()))
	h.Write([]byte{0})
	h.Write(wire)
	return h.Sum(nil)
}
func pageHash(root []byte, index uint64, ids []uuid.UUID) []byte {
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	h.Write(root)
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], index)
	h.Write(number[:])
	binary.BigEndian.PutUint64(number[:], uint64(len(ids)))
	h.Write(number[:])
	for _, id := range ids {
		h.Write(id[:])
	}
	return h.Sum(nil)
}
func rootHash(space, operation uuid.UUID, generation uint64, ids []uuid.UUID) []byte {
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	h.Write(space[:])
	h.Write(operation[:])
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], generation)
	h.Write(number[:])
	binary.BigEndian.PutUint64(number[:], uint64(len(ids)))
	h.Write(number[:])
	for _, id := range ids {
		h.Write(id[:])
	}
	return h.Sum(nil)
}

type manifestHead struct {
	operation, manifest                       uuid.UUID
	schedule, generation, count, pages, items uint64
	state                                     string
	root                                      []byte
	sealed, complete                          bool
}

func loadHead(ctx context.Context, tx pgx.Tx, space uuid.UUID) (manifestHead, error) {
	var h manifestHead
	err := tx.QueryRow(ctx, `SELECT operation_id,schedule_generation,generation,state,manifest_id,manifest_sha256,item_count,received_pages,received_items,sealed,freeze_complete FROM realtime_manifest_heads WHERE space_id=$1 FOR UPDATE`, space).Scan(&h.operation, &h.schedule, &h.generation, &h.state, &h.manifest, &h.root, &h.count, &h.pages, &h.items, &h.sealed, &h.complete)
	return h, err
}
func sameManifest(h manifestHead, manifest *commonv1.ManifestBinding) bool {
	return manifest != nil && manifest.GetManifestId() == h.manifest.String() && manifest.GetItemCount() == h.count && bytes.Equal(manifest.GetManifestSha256(), h.root)
}
func manifestIDs(ctx context.Context, tx pgx.Tx, space, operation uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `SELECT chat_id FROM realtime_manifest_items WHERE space_id=$1 AND operation_id=$2 ORDER BY uuid_send(chat_id)`, space, operation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// replay reads exact first-accepted bytes. Expired full receipts never become
// a new successful operation; compact binding evidence is retained indefinitely.
func replay(ctx context.Context, tx pgx.Tx, space, operation uuid.UUID, generation uint64, kind string, request []byte, response proto.Message) (bool, error) {
	var saved, hash, wire []byte
	var completed *time.Time
	var expired bool
	err := tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,response_bytes,completed_at,completed_at < clock_timestamp()-interval '30 days' FROM realtime_lifecycle_operations WHERE space_id=$1 AND operation_id=$2 AND generation=$3 AND kind=$4`, space, operation, generation, kind).Scan(&saved, &hash, &wire, &completed, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	requestedHash := sha256.Sum256(request)
	if !bytes.Equal(hash, requestedHash[:]) || saved != nil && !bytes.Equal(saved, request) {
		return false, ErrConflict
	}
	if completed == nil {
		return false, ErrPending
	}
	if expired || len(wire) == 0 {
		return false, ErrConflict
	}
	if err = proto.Unmarshal(wire, response); err != nil {
		return false, err
	}
	return true, nil
}
func saveResponse(ctx context.Context, tx pgx.Tx, space, operation uuid.UUID, generation uint64, kind string, request []byte, response proto.Message) error {
	// Public evidence and its replay horizon share one fresh database instant.
	// Application clock skew cannot move either timestamp independently.
	var completed time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completed); err != nil {
		return err
	}
	switch r := response.(type) {
	case *realtimev1.ImportSpacePurgeManifestPageResponse:
		r.Receipt.CompletedAt = timestamppb.New(completed)
	case *realtimev1.ApplySpaceLifecycleFenceResponse:
		r.Receipt.AppliedAt = timestamppb.New(completed)
	case *realtimev1.PurgeSpaceResponse:
		r.Receipt.CompletedAt = timestamppb.New(completed)
	default:
		return ErrConflict
	}
	wire, err := deterministic(response)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(request)
	_, err = tx.Exec(ctx, `INSERT INTO realtime_lifecycle_operations(space_id,operation_id,generation,kind,request_bytes,request_sha256,response_bytes,completed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, space, operation, generation, kind, request, hash[:], wire, completed)
	return err
}

// Import closes the exact page's gates in the same transaction as its immutable
// import receipt. Import receipt is deliberately NOT a completed freeze receipt.
func (s *Store) Import(ctx context.Context, req *realtimev1.ImportSpacePurgeManifestPageRequest) (*realtimev1.ImportSpacePurgeManifestPageResponse, error) {
	if !known(req) || req.GetProtocolVersion() != 1 || req.GetScheduleGeneration() == 0 || req.GetScheduleGeneration() > math.MaxInt64 {
		return nil, ErrConflict
	}
	space, err := canonicalID(req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	operation, err := canonicalID(req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	page := req.GetPage()
	if page == nil || page.GetProtocolVersion() != 1 || page.GetManifest() == nil || len(page.GetManifest().GetManifestSha256()) != 32 || page.GetManifest().GetItemCount() > math.MaxInt64 || page.GetPageIndex() > math.MaxInt64 {
		return nil, ErrConflict
	}
	manifest, err := canonicalID(page.GetManifest().GetManifestId())
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(page.GetItemIds()))
	for i, raw := range page.GetItemIds() {
		ids[i], err = canonicalID(raw)
		if err != nil || i > 0 && bytes.Compare(ids[i-1][:], ids[i][:]) >= 0 {
			return nil, ErrConflict
		}
	}
	count, index := page.GetManifest().GetItemCount(), page.GetPageIndex()
	pageCount := uint64(1)
	if count > 0 {
		pageCount = (count + 999) / 1000
	}
	if index >= pageCount || req.GetSealsManifest() != (index+1 == pageCount) || len(ids) > 1000 {
		return nil, ErrConflict
	}
	expected := uint64(1000)
	if index+1 == pageCount {
		expected = count - index*1000
	}
	if uint64(len(ids)) != expected {
		return nil, ErrConflict
	}
	if !bytes.Equal(pageHash(page.GetManifest().GetManifestSha256(), index, ids), page.GetPageSha256()) {
		return nil, ErrConflict
	}
	if req.GetSealsManifest() {
		if page.GetNextPageToken() != "" {
			return nil, ErrConflict
		}
	} else {
		token := make([]byte, 40)
		binary.BigEndian.PutUint64(token[:8], index+1)
		copy(token[8:], page.GetManifest().GetManifestSha256())
		if page.GetNextPageToken() != base64.RawURLEncoding.EncodeToString(token) {
			return nil, ErrConflict
		}
	}
	wire, err := deterministic(req)
	if err != nil {
		return nil, err
	}
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	// Stable head row serializes operation/import/replay before any chat locks.
	_, err = tx.Exec(ctx, `INSERT INTO realtime_manifest_heads(space_id,operation_id,schedule_generation,generation,state,manifest_id,manifest_sha256,item_count) VALUES($1,$2,$3,$3,'FROZEN',$4,$5,$6) ON CONFLICT DO NOTHING`, space, operation, req.GetScheduleGeneration(), manifest, page.GetManifest().GetManifestSha256(), count)
	if err != nil {
		return nil, err
	}
	h, err := loadHead(ctx, tx, space)
	if err != nil {
		return nil, err
	}
	response := new(realtimev1.ImportSpacePurgeManifestPageResponse)
	kind := fmt.Sprintf("IMPORT-%d", index)
	if found, err := replay(ctx, tx, space, operation, req.GetScheduleGeneration(), kind, wire, response); err != nil {
		return nil, err
	} else if found {
		return response, tx.Commit(ctx)
	}
	if h.operation != operation {
		if h.state != "LIVE" || req.GetScheduleGeneration() != h.generation+1 || index != 0 {
			return nil, ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE realtime_manifest_heads SET operation_id=$2,schedule_generation=$3,generation=$3,state='FROZEN',manifest_id=$4,manifest_sha256=$5,item_count=$6,received_pages=0,received_items=0,sealed=false,freeze_complete=false WHERE space_id=$1`, space, operation, req.GetScheduleGeneration(), manifest, page.GetManifest().GetManifestSha256(), count)
		if err != nil {
			return nil, err
		}
		h, err = loadHead(ctx, tx, space)
		if err != nil {
			return nil, err
		}
	}
	if h.schedule != req.GetScheduleGeneration() || h.state != "FROZEN" || h.sealed || h.pages != index || !sameManifest(h, page.GetManifest()) {
		return nil, ErrConflict
	}
	existing, err := manifestIDs(ctx, tx, space, operation)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 && len(ids) > 0 && bytes.Compare(existing[len(existing)-1][:], ids[0][:]) >= 0 {
		return nil, ErrConflict
	}
	for _, id := range ids {
		g, err := lockGate(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if g.known && g.space != space || g.generation > h.generation || g.state == "PURGED" || g.generation == h.generation && g.operation != uuid.Nil && g.operation != operation {
			return nil, ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE realtime_chat_gates SET space_id=$2,scope_known=true,operation_id=$3,generation=$4,state='FROZEN',updated_at=clock_timestamp() WHERE chat_id=$1`, id, space, operation, h.generation); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO realtime_manifest_items(space_id,operation_id,chat_id) VALUES($1,$2,$3)`, space, operation, id); err != nil {
			return nil, err
		}
	}
	if req.GetSealsManifest() {
		all := append(existing, ids...)
		if uint64(len(all)) != count || !bytes.Equal(rootHash(space, operation, h.schedule, all), h.root) {
			return nil, ErrConflict
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE realtime_manifest_heads SET received_pages=received_pages+1,received_items=received_items+$2,sealed=$3 WHERE space_id=$1`, space, len(ids), req.GetSealsManifest()); err != nil {
		return nil, err
	}
	response.Receipt = &realtimev1.ImportSpacePurgeManifestPageReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: h.schedule, Manifest: proto.Clone(page.GetManifest()).(*commonv1.ManifestBinding), PageIndex: index, AcceptedCount: uint64(len(ids)), PageSha256: bytes.Clone(page.GetPageSha256()), ManifestSealed: req.GetSealsManifest(), RequestSha256: digest(req, wire), CompletedAt: timestamppb.Now()}
	if err = saveResponse(ctx, tx, space, operation, h.schedule, kind, wire, response); err != nil {
		return nil, err
	}
	return response, tx.Commit(ctx)
}

func validBinding(spaceRaw, operationRaw string, generation uint64, manifest *commonv1.ManifestBinding) (uuid.UUID, uuid.UUID, error) {
	if generation == 0 || generation > math.MaxInt64 || manifest == nil || len(manifest.GetManifestSha256()) != 32 || manifest.GetItemCount() > math.MaxInt64 {
		return uuid.Nil, uuid.Nil, ErrConflict
	}
	if _, err := canonicalID(manifest.GetManifestId()); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	space, err := canonicalID(spaceRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	operation, err := canonicalID(operationRaw)
	return space, operation, err
}
func exactGates(ctx context.Context, tx pgx.Tx, space uuid.UUID, h manifestHead) ([]uuid.UUID, error) {
	if !h.sealed || h.items != h.count {
		return nil, ErrConflict
	}
	ids, err := manifestIDs(ctx, tx, space, h.operation)
	if err != nil {
		return nil, err
	}
	if uint64(len(ids)) != h.count || !bytes.Equal(rootHash(space, h.operation, h.schedule, ids), h.root) {
		return nil, ErrConflict
	}
	for _, id := range ids {
		g, err := lockGate(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if !g.known || g.space != space || g.operation != h.operation || g.generation != h.generation || g.state != h.state {
			return nil, ErrConflict
		}
	}
	if err = drained(ctx, tx, ids); err != nil {
		return nil, err
	}
	return ids, nil
}
func updateGates(ctx context.Context, tx pgx.Tx, space uuid.UUID, h manifestHead, ids []uuid.UUID, generation uint64, state string) error {
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE realtime_chat_gates SET generation=$2,state=$3,updated_at=clock_timestamp() WHERE chat_id=$1`, id, generation, state); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE realtime_manifest_heads SET generation=$2,state=$3,freeze_complete=true WHERE space_id=$1`, space, generation, state)
	return err
}

func (s *Store) Apply(ctx context.Context, req *realtimev1.ApplySpaceLifecycleFenceRequest) (*realtimev1.ApplySpaceLifecycleFenceResponse, error) {
	if !known(req) || req.GetFence() == nil || req.GetFence().GetProtocolVersion() != 1 {
		return nil, ErrConflict
	}
	f := req.GetFence()
	space, operation, err := validBinding(f.GetSpaceId(), f.GetDeletionOperationId(), f.GetGeneration(), f.GetManifest())
	if err != nil {
		return nil, err
	}
	wire, err := deterministic(req)
	if err != nil {
		return nil, err
	}
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	h, err := loadHead(ctx, tx, space)
	if err != nil {
		return nil, err
	}
	response := new(realtimev1.ApplySpaceLifecycleFenceResponse)
	if found, err := replay(ctx, tx, space, operation, f.GetGeneration(), "FENCE", wire, response); err != nil {
		return nil, err
	} else if found {
		return response, tx.Commit(ctx)
	}
	if h.operation != operation || !sameManifest(h, f.GetManifest()) {
		return nil, ErrConflict
	}
	target := ""
	switch f.GetDesiredState() {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		target = "FROZEN"
		if h.state != "FROZEN" || f.GetGeneration() != h.schedule || h.generation != h.schedule {
			return nil, ErrConflict
		}
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		target = "LIVE"
		if h.state != "FROZEN" || !h.complete || f.GetGeneration() != h.generation+1 {
			return nil, ErrConflict
		}
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		target = "PURGE_DECIDED"
		if h.state != "FROZEN" || !h.complete || f.GetGeneration() != h.generation+1 {
			return nil, ErrConflict
		}
	default:
		return nil, ErrConflict
	}
	ids, err := exactGates(ctx, tx, space, h)
	if err != nil {
		return nil, err
	}
	if err = updateGates(ctx, tx, space, h, ids, f.GetGeneration(), target); err != nil {
		return nil, err
	}
	response.Receipt = &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: f.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_REALTIME, AppliedState: f.GetDesiredState(), ManifestSha256: bytes.Clone(h.root), RequestSha256: digest(req, wire), AppliedAt: timestamppb.Now()}
	if err = saveResponse(ctx, tx, space, operation, f.GetGeneration(), "FENCE", wire, response); err != nil {
		return nil, err
	}
	return response, tx.Commit(ctx)
}

func (s *Store) Purge(ctx context.Context, req *realtimev1.PurgeSpaceRequest) (*realtimev1.PurgeSpaceResponse, error) {
	if !known(req) || req.GetPurge() == nil || req.GetPurge().GetProtocolVersion() != 1 {
		return nil, ErrConflict
	}
	p := req.GetPurge()
	if p.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_REALTIME || p.GetPurgeDecidedAt() == nil || p.GetPurgeDecidedAt().CheckValid() != nil {
		return nil, ErrConflict
	}
	space, operation, err := validBinding(p.GetSpaceId(), p.GetDeletionOperationId(), p.GetGeneration(), p.GetManifest())
	if err != nil {
		return nil, err
	}
	wire, err := deterministic(req)
	if err != nil {
		return nil, err
	}
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	h, err := loadHead(ctx, tx, space)
	if err != nil {
		return nil, err
	}
	response := new(realtimev1.PurgeSpaceResponse)
	if found, err := replay(ctx, tx, space, operation, p.GetGeneration(), "PURGE", wire, response); err != nil {
		return nil, err
	} else if found {
		return response, tx.Commit(ctx)
	}
	if h.operation != operation || h.state != "PURGE_DECIDED" || h.generation != p.GetGeneration() || !h.complete || !sameManifest(h, p.GetManifest()) {
		return nil, ErrConflict
	}
	ids, err := exactGates(ctx, tx, space, h)
	if err != nil {
		return nil, err
	}
	if err = updateGates(ctx, tx, space, h, ids, h.generation, "PURGED"); err != nil {
		return nil, err
	}
	response.Receipt = &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: h.generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_REALTIME, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: digest(req, wire), CompletedAt: timestamppb.Now()}
	if err = saveResponse(ctx, tx, space, operation, h.generation, "PURGE", wire, response); err != nil {
		return nil, err
	}
	return response, tx.Commit(ctx)
}
