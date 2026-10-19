package ordinarycall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/retirementproof"
)

const OrdinaryCutoverOwnerProtocol = "voice.native-ordinary.retirement.v1"
const cutoverEntryDomain = "voice.native-ordinary.retirement.entry.v1"

type cutoverRequest struct {
	Protocol string
	Cut      retirementproof.CutBinding
	Expected CutoverExpectedRoom
}
type cutoverTarget struct {
	Kind, Key string
	Bytes     json.RawMessage
}
type cutoverQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func OrdinaryCutoverContractHash() retirementproof.Digest {
	return retirementproof.Hash("voice.native-ordinary.owner-contract.v1", []byte("voice025;per-room-full-retained-history;atomic-native-close;room-closure-membership-fence-outbox;unknown-source-confinement;protected-partition-steps;exact-native-terminal-receipt"))
}
func (o *CutoverDrainOwner) checkSchema(ctx context.Context) error {
	if o == nil || o.Store == nil || o.Store.pool == nil || o.Store.encode == nil {
		return ErrUnavailable
	}
	if err := o.Store.CheckSchema(ctx); err != nil {
		return err
	}
	var ready bool
	if err := o.Store.pool.QueryRow(ctx, `SELECT to_regclass('voice_ordinary_cutover_works') IS NOT NULL AND to_regclass('voice_ordinary_cutover_targets') IS NOT NULL AND to_regclass('voice_ordinary_cutover_acceptances') IS NOT NULL`).Scan(&ready); err != nil {
		return err
	}
	if !ready {
		return ErrUnavailable
	}
	return nil
}
func validOrdinaryCut(c retirementproof.CutBinding) bool {
	z := retirementproof.Digest{}
	return c.EnvironmentID != uuid.Nil && c.AuthorityIncarnation != uuid.Nil && c.CutoverOperationID != uuid.Nil && c.CoverageEpoch > 0 && c.CoverageEpoch <= math.MaxInt64 && c.PlanHash != z && c.DatabaseIdentity != z && c.SourceCatalogHash != z && c.SchemaContractHash != z && c.WriterManifestHash != z && c.ProviderManifestHash != z && c.CredentialManifestHash != z && c.ClosedAdmissionBarrierID != uuid.Nil && c.ClosedAdmissionBarrierHash != z && c.IssuerImplementationManifestHash != z
}

// PrepareCutoverDrain captures all retained native history and elects native
// closure atomically. It performs no Redis mutation before capture/partition seals.
func (o *CutoverDrainOwner) PrepareCutoverDrain(ctx context.Context, cut retirementproof.CutBinding, expected CutoverExpectedRoom) (retirementproof.OwnerWorkRef, error) {
	var zero retirementproof.OwnerWorkRef
	if o == nil || o.Authority == nil {
		return zero, ErrUnavailable
	}
	if expected.WorkID == uuid.Nil || expected.RoomID == uuid.Nil || expected.Incarnation == uuid.Nil || expected.Version == 0 || expected.Version > math.MaxInt64 || !validOrdinaryCut(cut) {
		return zero, ErrInvalidIntent
	}
	if err := o.Authority.CheckMaintenance(ctx, cut); err != nil {
		return zero, err
	}
	if err := o.checkSchema(ctx); err != nil {
		return zero, err
	}
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return zero, err
	}
	defer rollback(tx)
	// Native room first, then the unchanged identity/fence order inside mutateTx.
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1 FOR UPDATE`, expected.RoomID).Scan(&raw); err != nil {
		return zero, mapStoreError(err)
	}
	before, err := decodeRoom(raw, expected.RoomID)
	if err != nil {
		return zero, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2))`, int32(0x4f524443), expected.WorkID.String()); err != nil {
		return zero, err
	}
	request := cutoverRequest{OrdinaryCutoverOwnerProtocol, cut, expected}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return zero, err
	}
	old, err := readCutoverWork(ctx, tx, expected.WorkID)
	if err == nil {
		if !bytes.Equal(old.OwnerRequestBytes, requestBytes) || before.Intent.OperationID != expected.Incarnation || (before.Stage != StageClosing && before.Stage != StageAborting && before.Stage != StageClosed) {
			return zero, ErrConflict
		}
		return old.Ref, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	if before.Intent.OperationID != expected.Incarnation || before.Version != expected.Version {
		return zero, ErrConflict
	}
	canonicalStored, err := cutoverCanonical(raw)
	if err != nil {
		return zero, err
	}
	beforeBytes, err := json.Marshal(before)
	if err != nil {
		return zero, err
	}
	canonicalModel, err := cutoverCanonical(beforeBytes)
	// Unknown model fields may contain old issuance/history: never discard them.
	if err != nil || !bytes.Equal(canonicalStored, canonicalModel) {
		return zero, ErrConflict
	}
	if err := validateCutoverMembershipIndex(ctx, tx, before); err != nil {
		return zero, err
	}
	closed, err := o.Store.mutateTx(ctx, tx, expected.RoomID, func(r *Room) error {
		if r.Intent.OperationID != expected.Incarnation || r.Version != expected.Version {
			return ErrConflict
		}
		return r.terminate(uuid.Nil, "maintenance_retirement", r.TransactionTime)
	})
	if err != nil {
		return zero, err
	}
	closedBytes, err := json.Marshal(closed)
	if err != nil {
		return zero, err
	}
	closedBytes, err = cutoverCanonical(closedBytes)
	if err != nil {
		return zero, err
	}
	targets := []cutoverTarget{{"room", expected.RoomID.String(), canonicalStored}, {"closure", expected.RoomID.String(), closedBytes}}
	for _, source := range []struct {
		kind, query string
		arg         any
	}{
		{"membership", `SELECT profile_id::text,to_jsonb(t)::text FROM voice_ordinary_memberships t WHERE room_id=$1 ORDER BY profile_id`, expected.RoomID},
		{"fence", `SELECT account_id::text,to_jsonb(t)::text FROM voice_account_voice_fences t WHERE room_id=$1 ORDER BY account_id`, expected.RoomID.String()},
		{"outbox", `SELECT event_id::text,to_jsonb(t)::text FROM voice_ordinary_outbox t WHERE room_id=$1 ORDER BY event_id`, expected.RoomID},
	} {
		rows, err := tx.Query(ctx, source.query, source.arg)
		if err != nil {
			return zero, err
		}
		for rows.Next() {
			var key string
			var body []byte
			if err = rows.Scan(&key, &body); err != nil {
				rows.Close()
				return zero, err
			}
			body, err = cutoverCanonical(body)
			if err != nil {
				rows.Close()
				return zero, err
			}
			targets = append(targets, cutoverTarget{source.kind, key, body})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return zero, err
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Kind+"\x00"+targets[i].Key < targets[j].Kind+"\x00"+targets[j].Key
	})
	manifest, err := json.Marshal(targets)
	if err != nil {
		return zero, err
	}
	head, err := json.Marshal(before.Intent)
	if err != nil {
		return zero, err
	}
	work := retirementproof.OwnerWork{
		Ref:     retirementproof.OwnerWorkRef{Protocol: retirementproof.ProtocolV1, OwnerProtocol: OrdinaryCutoverOwnerProtocol, WorkID: expected.WorkID, RequestHash: retirementproof.Hash(retirementproof.OwnerRequestDomain, requestBytes)},
		Purpose: retirementproof.ColdCutDrain, SourceKind: "NATIVE_ORDINARY", Cut: cut,
		CapturedHead:       retirementproof.OwnerHeadBinding{State: retirementproof.HeadPresent, CanonicalBytes: head, Hash: retirementproof.Hash(retirementproof.HeadDomain, head)},
		OwnerRequestDomain: retirementproof.OwnerRequestDomain, OwnerRequestBytes: requestBytes, SourceManifestHash: OrdinaryCutoverContractHash(),
		SealedTargetManifestBytes: manifest, SealedTargetManifestHash: retirementproof.Hash(retirementproof.TargetDomain, manifest), SealedTargetCount: uint64(len(targets)), UnknownIssuanceMarkers: cutoverUnknownMarkers(targets),
	}
	if _, err = validateCutoverManifest(work); err != nil {
		return zero, err
	}
	encoded, err := json.Marshal(work)
	if err != nil {
		return zero, err
	}
	digest := retirementproof.Hash(retirementproof.WorkDomain, encoded)
	if _, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_cutover_works(work_id,room_id,incarnation,cut_operation_id,request_sha256,work_bytes,work_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`, expected.WorkID, expected.RoomID, expected.Incarnation, cut.CutoverOperationID, work.Ref.RequestHash[:], encoded, digest[:]); err != nil {
		return zero, mapStoreError(err)
	}
	for _, target := range targets {
		body, err := json.Marshal(target)
		if err != nil {
			return zero, err
		}
		hash := retirementproof.Hash(cutoverEntryDomain, body)
		if _, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_cutover_targets(work_id,target_kind,target_key,target_bytes,target_sha256) VALUES($1,$2,$3,$4,$5)`, expected.WorkID, target.Kind, target.Key, body, hash[:]); err != nil {
			return zero, err
		}
	}
	return work.Ref, tx.Commit(ctx)
}

// The native journal is the authority. Do not let syncMemberships overwrite a
// stale predecessor tuple before its evidence has been authoritatively resolved.
func validateCutoverMembershipIndex(ctx context.Context, q cutoverQuery, room *Room) error {
	rows, err := q.Query(ctx, `SELECT profile_id,account_id,membership_generation,fence_operation_id,media_identity,state,session_epoch
FROM voice_ordinary_memberships WHERE room_id=$1 ORDER BY profile_id`, room.Intent.RoomID)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var profile, account, generation, operation uuid.UUID
		var media, state string
		var epoch int64
		if err := rows.Scan(&profile, &account, &generation, &operation, &media, &state, &epoch); err != nil {
			return err
		}
		member, ok := room.Members[profile.String()]
		if !ok || member.ProfileID != profile || member.AccountID != account || member.Generation != generation || member.FenceOperationID != operation || member.MediaIdentity != media || string(member.State) != state || member.SessionEpoch != epoch {
			return ErrConflict
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(room.Members) {
		return ErrConflict
	}
	return nil
}

func cutoverCanonical(raw []byte) ([]byte, error) {
	if !json.Valid(raw) {
		return nil, ErrConflict
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

// Ordinary has no complete grant ledger. Every retained entry therefore needs
// protected historical source confinement; current endpoint/expiry is not proof.
func cutoverUnknownMarkers(targets []cutoverTarget) []retirementproof.Digest {
	markers := make([]retirementproof.Digest, 0, len(targets))
	for _, target := range targets {
		body, _ := json.Marshal(target)
		markers = append(markers, retirementproof.Hash(cutoverEntryDomain, body))
	}
	sort.Slice(markers, func(i, j int) bool { return bytes.Compare(markers[i][:], markers[j][:]) < 0 })
	return markers
}
func validateCutoverManifest(work retirementproof.OwnerWork) ([]cutoverTarget, error) {
	if work.Ref.Protocol != retirementproof.ProtocolV1 || work.Ref.OwnerProtocol != OrdinaryCutoverOwnerProtocol || work.Ref.WorkID == uuid.Nil || work.SourceKind != "NATIVE_ORDINARY" || work.Purpose != retirementproof.ColdCutDrain || work.SpaceID != uuid.Nil || work.ExistingP3 != nil || !validOrdinaryCut(work.Cut) || work.SourceManifestHash != OrdinaryCutoverContractHash() || work.OwnerRequestDomain != retirementproof.OwnerRequestDomain || retirementproof.Hash(work.OwnerRequestDomain, work.OwnerRequestBytes) != work.Ref.RequestHash {
		return nil, ErrConflict
	}
	var request cutoverRequest
	if json.Unmarshal(work.OwnerRequestBytes, &request) != nil || request.Protocol != OrdinaryCutoverOwnerProtocol || request.Cut != work.Cut || request.Expected.WorkID != work.Ref.WorkID {
		return nil, ErrConflict
	}
	requestBytes, _ := json.Marshal(request)
	if !bytes.Equal(requestBytes, work.OwnerRequestBytes) {
		return nil, ErrConflict
	}
	var targets []cutoverTarget
	if json.Unmarshal(work.SealedTargetManifestBytes, &targets) != nil || len(targets) < 2 || uint64(len(targets)) != work.SealedTargetCount || retirementproof.Hash(retirementproof.TargetDomain, work.SealedTargetManifestBytes) != work.SealedTargetManifestHash {
		return nil, ErrConflict
	}
	encoded, _ := json.Marshal(targets)
	if !bytes.Equal(encoded, work.SealedTargetManifestBytes) {
		return nil, ErrConflict
	}
	var before, closed *Room
	for i, target := range targets {
		if i > 0 && targets[i-1].Kind+"\x00"+targets[i-1].Key >= target.Kind+"\x00"+target.Key {
			return nil, ErrConflict
		}
		canonical, err := cutoverCanonical(target.Bytes)
		if err != nil || !bytes.Equal(canonical, target.Bytes) {
			return nil, ErrConflict
		}
		switch target.Kind {
		case "room", "closure":
			if target.Key != request.Expected.RoomID.String() {
				return nil, ErrConflict
			}
			r, err := decodeRoom(target.Bytes, request.Expected.RoomID)
			if err != nil || r.Intent.OperationID != request.Expected.Incarnation {
				return nil, ErrConflict
			}
			modelBytes, err := json.Marshal(r)
			if err != nil {
				return nil, ErrConflict
			}
			modelBytes, err = cutoverCanonical(modelBytes)
			if err != nil || !bytes.Equal(modelBytes, target.Bytes) {
				return nil, ErrConflict
			}
			if target.Kind == "room" {
				before = r
			} else {
				closed = r
			}
		case "membership", "fence", "outbox":
			field := map[string]string{"membership": "profile_id", "fence": "account_id", "outbox": "event_id"}[target.Kind]
			var row map[string]json.RawMessage
			var room, key string
			if json.Unmarshal(target.Bytes, &row) != nil || json.Unmarshal(row["room_id"], &room) != nil || json.Unmarshal(row[field], &key) != nil || room != request.Expected.RoomID.String() || key != target.Key {
				return nil, ErrConflict
			}
		default:
			return nil, ErrConflict
		}
	}
	if before == nil || closed == nil || before.Version != request.Expected.Version || !reflect.DeepEqual(before.Intent, closed.Intent) || closed.Version < before.Version || (closed.Stage != StageClosing && closed.Stage != StageAborting && closed.Stage != StageClosed) {
		return nil, ErrConflict
	}
	head, _ := json.Marshal(before.Intent)
	if work.CapturedHead.State != retirementproof.HeadPresent || !bytes.Equal(work.CapturedHead.CanonicalBytes, head) || work.CapturedHead.Hash != retirementproof.Hash(retirementproof.HeadDomain, head) {
		return nil, ErrConflict
	}
	// Re-extract every marker on EVERY authoritative resolution. Never trust a
	// serialized caller-selected subset, even with internally consistent hashes.
	if len(work.CapturedProviderBindings) != 0 || !reflect.DeepEqual(work.UnknownIssuanceMarkers, cutoverUnknownMarkers(targets)) {
		return nil, ErrConflict
	}
	return targets, nil
}
func readCutoverWork(ctx context.Context, q cutoverQuery, id uuid.UUID) (retirementproof.OwnerWork, error) {
	var work retirementproof.OwnerWork
	var encoded, digest []byte
	if err := q.QueryRow(ctx, `SELECT work_bytes,work_sha256 FROM voice_ordinary_cutover_works WHERE work_id=$1`, id).Scan(&encoded, &digest); err != nil {
		return work, err
	}
	hash := retirementproof.Hash(retirementproof.WorkDomain, encoded)
	if !bytes.Equal(hash[:], digest) || json.Unmarshal(encoded, &work) != nil || work.Ref.WorkID != id {
		return work, ErrConflict
	}
	canonical, _ := json.Marshal(work)
	if !bytes.Equal(canonical, encoded) {
		return work, ErrConflict
	}
	targets, err := validateCutoverManifest(work)
	if err != nil {
		return work, err
	}
	rows, err := q.Query(ctx, `SELECT target_kind,target_key,target_bytes,target_sha256 FROM voice_ordinary_cutover_targets WHERE work_id=$1 ORDER BY target_kind,target_key`, id)
	if err != nil {
		return work, err
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var kind, key string
		var body, digest []byte
		if err = rows.Scan(&kind, &key, &body, &digest); err != nil {
			return work, err
		}
		if index >= len(targets) {
			return work, ErrConflict
		}
		want, _ := json.Marshal(targets[index])
		hash := retirementproof.Hash(cutoverEntryDomain, body)
		if kind != targets[index].Kind || key != targets[index].Key || !bytes.Equal(body, want) || !bytes.Equal(digest, hash[:]) {
			return work, ErrConflict
		}
		index++
	}
	if err = rows.Err(); err != nil {
		return work, err
	}
	if index != len(targets) {
		return work, ErrConflict
	}
	return work, nil
}
func (o *CutoverDrainOwner) ReadOwnerWork(ctx context.Context, ref retirementproof.OwnerWorkRef) (retirementproof.OwnerWork, error) {
	if err := o.checkSchema(ctx); err != nil {
		return retirementproof.OwnerWork{}, err
	}
	work, err := readCutoverWork(ctx, o.Store.pool, ref.WorkID)
	if err != nil {
		return retirementproof.OwnerWork{}, err
	}
	if work.Ref != ref {
		return retirementproof.OwnerWork{}, ErrConflict
	}
	return work, nil
}
