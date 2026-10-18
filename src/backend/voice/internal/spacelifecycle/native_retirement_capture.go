package spacelifecycle

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacebarrier"
)

const ownerTargetEntryDomain = "voice.native-space.retirement.target-entry.v1"
const ownerUnknownIssuanceDomain = "voice.native-space.retirement.unknown-entry.v1"

type nativeOwnerTarget struct {
	ID              string
	Kind            string
	Data            json.RawMessage
	UnknownIssuance bool
}

type ownerLifecycleHead struct {
	Generation                                                      uint64
	State, DeletionOperationID, ManifestID, ManifestHash, ItemCount string
}

func readOwnerHead(ctx context.Context, tx pgx.Tx, space uuid.UUID) (retirementproof.OwnerHeadBinding, ownerLifecycleHead, error) {
	var head ownerLifecycleHead
	err := tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id::text,manifest_id::text,encode(manifest_sha256,'hex'),manifest_item_count::text FROM voice_space_lifecycle_fence_heads WHERE space_id=$1 FOR UPDATE`, space).Scan(&head.Generation, &head.State, &head.DeletionOperationID, &head.ManifestID, &head.ManifestHash, &head.ItemCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return retirementproof.OwnerHeadBinding{State: retirementproof.HeadAbsent, Hash: retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent))}, head, nil
	}
	if err != nil {
		return retirementproof.OwnerHeadBinding{}, head, err
	}
	encoded, err := json.Marshal(head)
	return retirementproof.OwnerHeadBinding{State: retirementproof.HeadPresent, CanonicalBytes: encoded, Hash: retirementproof.Hash(retirementproof.HeadDomain, encoded)}, head, err
}

// ObserveCutoverHead returns an explicit absence/presence expectation. Prepare
// rechecks it after taking the same Space barrier; this read authorizes no write.
func (o *NativeRetirementOwner) ObserveCutoverHead(ctx context.Context, space uuid.UUID) (retirementproof.OwnerHeadBinding, error) {
	if space == uuid.Nil {
		return retirementproof.OwnerHeadBinding{}, ErrInvalidRequest
	}
	if err := o.CheckSchema(ctx); err != nil {
		return retirementproof.OwnerHeadBinding{}, err
	}
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return retirementproof.OwnerHeadBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := spacebarrier.Lock(ctx, tx, space.String()); err != nil {
		return retirementproof.OwnerHeadBinding{}, err
	}
	head, _, err := readOwnerHead(ctx, tx, space)
	if err != nil {
		return retirementproof.OwnerHeadBinding{}, err
	}
	return head, tx.Commit(ctx)
}

func readPendingOwnerP3(ctx context.Context, tx pgx.Tx, space uuid.UUID, head ownerLifecycleHead) (*retirementproof.P3Binding, error) {
	if head.State != "FROZEN" || head.Generation == 0 {
		return nil, ErrConflict
	}
	var raw, digest, receipt []byte
	var sealed bool
	if err := tx.QueryRow(ctx, `SELECT request_bytes,request_sha256,receipt_bytes,coverage_sealed FROM voice_space_lifecycle_fence_receipts WHERE space_id=$1 AND generation=$2`, space, int64(head.Generation)).Scan(&raw, &digest, &receipt, &sealed); err != nil {
		return nil, err
	}
	if len(receipt) != 0 || !sealed {
		return nil, ErrUnavailable
	}
	var wrapper callsv1.ApplySpaceLifecycleFenceRequest
	if proto.Unmarshal(raw, &wrapper) != nil || validateFenceRequest(wrapper.GetFence()) != nil {
		return nil, ErrConflict
	}
	fence := wrapper.GetFence()
	encoded, hash, err := canonicalProto(fence)
	if err != nil || !bytes.Equal(encoded, raw) || !bytes.Equal(hash, digest) || fence.SpaceId != space.String() || fence.Generation != head.Generation || fence.DeletionOperationId != head.DeletionOperationID || fence.DesiredState != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN || fence.Manifest.ManifestId != head.ManifestID || hex.EncodeToString(fence.Manifest.ManifestSha256) != head.ManifestHash {
		return nil, ErrConflict
	}
	var requestHash, manifestHash retirementproof.Digest
	copy(requestHash[:], digest)
	copy(manifestHash[:], fence.Manifest.ManifestSha256)
	if manifestHash == (retirementproof.Digest{}) {
		return nil, ErrUnavailable
	}
	return &retirementproof.P3Binding{DeletionOperationID: uuid.MustParse(fence.DeletionOperationId), Generation: fence.Generation, RequestDomain: retirementproof.P3FenceRequestDomain, CanonicalRequestBytes: raw, RequestHash: requestHash, ManifestID: uuid.MustParse(fence.Manifest.ManifestId), ManifestHash: manifestHash, ItemCount: fence.Manifest.ItemCount}, nil
}

func captureOwnerTargets(ctx context.Context, tx pgx.Tx, work retirementproof.OwnerWork) ([]nativeOwnerTarget, error) {
	var targets []nativeOwnerTarget
	appendRows := func(query string, args ...any) error {
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var target nativeOwnerTarget
			if err := rows.Scan(&target.ID, &target.Kind, &target.Data); err != nil {
				return err
			}
			// Until protected historical manifests prove confinement, migrated and
			// pre-ledger records remain explicit possibly-issued credential work.
			target.UnknownIssuance = target.Kind != "event"
			targets = append(targets, target)
		}
		return rows.Err()
	}
	if work.Purpose == retirementproof.P3FrozenCompletion {
		p := work.ExistingP3
		if err := appendRows(`SELECT target_id,source_kind,target FROM voice_space_lifecycle_targets WHERE space_id=$1 AND generation=$2 AND deletion_operation_id=$3 AND request_sha256=$4 ORDER BY target_id`, work.SpaceID, int64(p.Generation), p.DeletionOperationID, p.RequestHash[:]); err != nil {
			return nil, err
		}
	} else {
		queries := []string{
			`SELECT 'native:'||a.operation_id,'native',to_jsonb(a)||jsonb_build_object('identity',a.livekit_identity,'sfu_room','voice-room-'||a.voice_room_id) FROM voice_space_media_admissions a WHERE a.space_id=$1 ORDER BY a.operation_id`,
			`SELECT 'head:'||h.voice_room_id||':'||h.room_generation,'head',to_jsonb(h) FROM voice_space_media_room_heads h WHERE h.space_id=$1 ORDER BY h.voice_room_id,h.room_generation`,
			`SELECT 'legacy:'||r.room_id||':'||m.profile_id||':'||m.media_epoch,'legacy',to_jsonb(m)||jsonb_build_object('space_id',r.space_id,'voice_room_id',r.voice_room_id,'sfu_room',r.livekit_room_name,'identity','profile:'||m.profile_id||':media:'||m.media_epoch) FROM voice_room_instances r JOIN voice_room_memberships m USING(room_id) WHERE r.space_id=$1 ORDER BY r.room_id,m.profile_id`,
			`SELECT 'legacy_head:'||r.room_id,'legacy_head',to_jsonb(r) FROM voice_room_instances r WHERE r.space_id=$1 ORDER BY r.room_id`,
		}
		for _, query := range queries {
			if err := appendRows(query, work.SpaceID); err != nil {
				return nil, err
			}
		}
	}
	// Retain the complete claims and pending event bytes even for terminal
	// admissions. Original expiry never establishes provider retirement.
	for _, query := range []string{
		`SELECT 'grant:'||g.grant_id,'grant',to_jsonb(g)||jsonb_build_object('space_id',a.space_id,'account_id',a.account_id,'profile_id',a.profile_id,'room_id',a.room_id,'voice_room_id',a.voice_room_id,'room_generation',a.room_generation,'identity',a.livekit_identity) FROM voice_space_media_grants g JOIN voice_space_media_admissions a USING(operation_id) WHERE a.space_id=$1 ORDER BY g.grant_id`,
		`SELECT 'event:'||o.event_id,'event',to_jsonb(o) FROM voice_space_media_admission_outbox o JOIN voice_space_media_admissions a USING(operation_id) WHERE a.space_id=$1 AND o.delivered_at IS NULL ORDER BY o.event_id`,
	} {
		if err := appendRows(query, work.SpaceID); err != nil {
			return nil, err
		}
	}
	history, err := json.Marshal(struct {
		SpaceID uuid.UUID
		Reason  string
	}{work.SpaceID, "historical native Space issuance before complete ledger is unqualified"})
	if err != nil {
		return nil, err
	}
	targets = append(targets, nativeOwnerTarget{ID: "history:space:" + work.SpaceID.String(), Kind: "history", Data: history, UnknownIssuance: true})
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	for i := 1; i < len(targets); i++ {
		if targets[i-1].ID == targets[i].ID {
			return nil, ErrConflict
		}
	}
	return targets, nil
}

func ownerIssuanceBindings(targets []nativeOwnerTarget) ([]retirementproof.Digest, []retirementproof.Digest) {
	bindings, markers := map[retirementproof.Digest]bool{}, map[retirementproof.Digest]bool{}
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, entry := range value {
				if key == "ProviderBinding" || key == "provider_binding" || key == "provider_bindings" {
					entries := []any{entry}
					if list, ok := entry.([]any); ok {
						entries = list
					}
					for _, entry := range entries {
						if text, ok := entry.(string); ok {
							if raw, err := hex.DecodeString(text); err == nil && len(raw) == 32 {
								var digest retirementproof.Digest
								copy(digest[:], raw)
								if digest != (retirementproof.Digest{}) {
									bindings[digest] = true
								}
							}
						}
					}
				}
				visit(entry)
			}
		case []any:
			for _, entry := range value {
				visit(entry)
			}
		}
	}
	for _, target := range targets {
		var value any
		if json.Unmarshal(target.Data, &value) == nil {
			visit(value)
		}
		if target.UnknownIssuance {
			encoded, _ := json.Marshal(target)
			markers[retirementproof.Hash(ownerUnknownIssuanceDomain, encoded)] = true
		}
	}
	ordered := func(set map[retirementproof.Digest]bool) []retirementproof.Digest {
		result := make([]retirementproof.Digest, 0, len(set))
		for digest := range set {
			result = append(result, digest)
		}
		sort.Slice(result, func(i, j int) bool { return bytes.Compare(result[i][:], result[j][:]) < 0 })
		return result
	}
	return ordered(bindings), ordered(markers)
}

func validateNativeOwnerManifest(work retirementproof.OwnerWork) ([]nativeOwnerTarget, error) {
	if !validOwnerCut(work.Cut) || work.SpaceID == uuid.Nil || !validOwnerHead(work.CapturedHead) || retirementproof.Hash(retirementproof.TargetDomain, work.SealedTargetManifestBytes) != work.SealedTargetManifestHash {
		return nil, ErrConflict
	}
	var targets []nativeOwnerTarget
	if json.Unmarshal(work.SealedTargetManifestBytes, &targets) != nil || uint64(len(targets)) != work.SealedTargetCount {
		return nil, ErrConflict
	}
	encoded, err := json.Marshal(targets)
	if err != nil || !bytes.Equal(encoded, work.SealedTargetManifestBytes) {
		return nil, ErrConflict
	}
	history := false
	for i, target := range targets {
		if target.ID == "" || !strings.HasPrefix(target.ID, target.Kind+":") || !json.Valid(target.Data) || (i > 0 && targets[i-1].ID >= target.ID) || target.UnknownIssuance != (target.Kind != "event") {
			return nil, ErrConflict
		}
		switch target.Kind {
		case "native", "head", "legacy", "legacy_head", "grant", "event":
		case "history":
			if target.ID != "history:space:"+work.SpaceID.String() {
				return nil, ErrConflict
			}
			history = true
		default:
			return nil, ErrConflict
		}
	}
	if !history {
		return nil, ErrConflict
	}
	bindings, markers := ownerIssuanceBindings(targets)
	if !sameJSON(bindings, work.CapturedProviderBindings) || !sameJSON(markers, work.UnknownIssuanceMarkers) {
		return nil, ErrConflict
	}
	if work.Purpose == retirementproof.ColdCutDrain {
		var request struct {
			Protocol string
			Cut      retirementproof.CutBinding
			Expected ExpectedOwnerBinding
		}
		if work.ExistingP3 != nil || work.OwnerRequestDomain != retirementproof.OwnerRequestDomain || json.Unmarshal(work.OwnerRequestBytes, &request) != nil || request.Protocol != NativeOwnerProtocol || request.Cut != work.Cut || request.Expected.WorkID != work.Ref.WorkID || request.Expected.SpaceID != work.SpaceID || request.Expected.Purpose != work.Purpose || request.Expected.ExistingP3 != nil || !sameOwnerHead(request.Expected.Head, work.CapturedHead) {
			return nil, ErrConflict
		}
		canonical, err := json.Marshal(request)
		if err != nil || !bytes.Equal(canonical, work.OwnerRequestBytes) {
			return nil, ErrConflict
		}
	} else if work.Purpose == retirementproof.P3FrozenCompletion {
		p := work.ExistingP3
		if p == nil || work.OwnerRequestDomain != retirementproof.P3FenceRequestDomain || p.RequestDomain != work.OwnerRequestDomain || p.RequestHash != work.Ref.RequestHash || !bytes.Equal(work.OwnerRequestBytes, p.CanonicalRequestBytes) || work.CapturedHead.State != retirementproof.HeadPresent {
			return nil, ErrConflict
		}
		var wrapper callsv1.ApplySpaceLifecycleFenceRequest
		if proto.Unmarshal(work.OwnerRequestBytes, &wrapper) != nil || validateFenceRequest(wrapper.GetFence()) != nil {
			return nil, ErrConflict
		}
		f := wrapper.GetFence()
		encoded, hash, err := canonicalProto(f)
		if err != nil || !bytes.Equal(encoded, work.OwnerRequestBytes) || !bytes.Equal(hash, p.RequestHash[:]) || f.SpaceId != work.SpaceID.String() || f.DeletionOperationId != p.DeletionOperationID.String() || f.Generation != p.Generation || f.DesiredState != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN || f.Manifest.ManifestId != p.ManifestID.String() || !bytes.Equal(f.Manifest.ManifestSha256, p.ManifestHash[:]) || f.Manifest.ItemCount != p.ItemCount {
			return nil, ErrConflict
		}
		var head ownerLifecycleHead
		if json.Unmarshal(work.CapturedHead.CanonicalBytes, &head) != nil || head.State != "FROZEN" || head.Generation != p.Generation || head.DeletionOperationID != p.DeletionOperationID.String() || head.ManifestID != p.ManifestID.String() || head.ManifestHash != hex.EncodeToString(p.ManifestHash[:]) || head.ItemCount != fmt.Sprint(p.ItemCount) {
			return nil, ErrConflict
		}
	} else {
		return nil, ErrConflict
	}
	return targets, nil
}

func compareOwnerTargetRows(ctx context.Context, q ownerQuery, id uuid.UUID, targets []nativeOwnerTarget) error {
	expected := make(map[string]nativeOwnerTarget, len(targets))
	for _, target := range targets {
		expected[target.ID] = target
	}
	rows, err := q.Query(ctx, `SELECT target_id,source_kind,target_bytes,target_sha256 FROM voice_native_owner_targets WHERE work_id=$1 ORDER BY target_id`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, kind string
		var raw, hash []byte
		if err := rows.Scan(&key, &kind, &raw, &hash); err != nil {
			return err
		}
		target, ok := expected[key]
		encoded, err := json.Marshal(target)
		digest := retirementproof.Hash(ownerTargetEntryDomain, raw)
		if !ok || err != nil || target.Kind != kind || !bytes.Equal(raw, encoded) || !bytes.Equal(hash, digest[:]) {
			return ErrConflict
		}
		delete(expected, key)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(expected) != 0 {
		return ErrConflict
	}
	return nil
}

func (o *NativeRetirementOwner) closeCapturedOwnerRooms(ctx context.Context, tx pgx.Tx, space uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT voice_room_id,room_generation FROM voice_space_media_room_heads WHERE space_id=$1 AND state<>'ENDED' ORDER BY voice_room_id,room_generation`, space)
	if err != nil {
		return err
	}
	type room struct {
		id         string
		generation uint64
	}
	var rooms []room
	for rows.Next() {
		var r room
		if err := rows.Scan(&r.id, &r.generation); err != nil {
			rows.Close()
			return err
		}
		rooms = append(rooms, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range rooms {
		if o.Store.CloseNativeRoom == nil {
			return ErrUnavailable
		}
		if err := o.Store.CloseNativeRoom(ctx, tx, r.id, r.generation); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE voice_space_media_admissions SET state='ABORTING',updated_at=clock_timestamp() WHERE space_id=$1 AND (state IN ('PREPARED','FENCED') OR (state='COMMITTED' AND projection_applied_at IS NULL))`, space)
	return err
}
