package gameprovision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"sort"
	"strings"
	"voice/backend/voice/internal/retirementproof"
)

const GameConversionSource = "GAME_PROVISION_AND_CONVERSION"
const OwnedRoomSource = "OWNED_ROOM"

// CutoverMaintenanceAuthority must reobserve protected closed admission and
// external installation/DB identity. A request CutBinding is not authority.
// Production uses the issuer's distinct non-serving maintenance composition;
// there is deliberately no default implementation or serving-root registration.
type CutoverMaintenanceAuthority interface {
	CheckMaintenance(context.Context, retirementproof.CutBinding) error
}
type CutoverWorkStore struct {
	Pool       *pgxpool.Pool
	Authority  CutoverMaintenanceAuthority
	SourceKind string
}
type cutoverTarget struct {
	Source string
	Row    json.RawMessage
}
type cutoverSource struct{ Table, Predicate string }
type cutoverRequest struct {
	WorkID             uuid.UUID
	SourceKind         string
	Cut                retirementproof.CutBinding
	SourceContractHash retirementproof.Digest
}

// CutoverSourceContract returns recoverable protocol/table/predicate bytes for
// the protected SOURCE_CATALOG. It is not a list of caller-selected sources.
func CutoverSourceContract(kind string) (string, []byte, retirementproof.Digest, error) {
	sources, err := cutoverSources(kind)
	if err != nil {
		return "", nil, retirementproof.Digest{}, err
	}
	protocol := "voice.gameprovision.cutover.v1"
	if kind == OwnedRoomSource {
		protocol = "voice.matchsquad.cutover.v1"
	}
	raw, err := json.Marshal(struct {
		Protocol, SourceKind, IssuancePolicy string
		Sources                              []cutoverSource
	}{protocol, kind, "ALL_RETAINED_ROWS_AND_UNKNOWN_HISTORICAL_ISSUANCE", sources})
	return protocol, raw, retirementproof.Hash("voice.party.retirement.source-contract.v1", raw), err
}

func cutoverSources(kind string) ([]cutoverSource, error) {
	var rooms, identities string
	var sources []cutoverSource
	switch kind {
	case OwnedRoomSource:
		rooms = `SELECT room_id FROM voice_room_instances WHERE purpose='MATCH_SQUAD' UNION SELECT room_id FROM voice_match_squad_operations UNION SELECT room_id FROM voice_match_squad_member_operations UNION SELECT room_id FROM voice_match_squad_member_effects UNION SELECT room_id FROM voice_match_squad_member_grants`
		identities = `SELECT account_id FROM voice_match_squad_member_operations UNION SELECT account_id FROM voice_match_squad_member_grants`
		for _, table := range []string{"voice_match_squad_operations", "voice_match_squad_member_operations", "voice_match_squad_member_effects", "voice_match_squad_member_grants"} {
			sources = append(sources, cutoverSource{table, "true"})
		}
	case GameConversionSource:
		rooms = `SELECT room_id FROM voice_room_instances WHERE purpose='GAME_SESSION' UNION SELECT room_id FROM voice_game_session_operations UNION SELECT room_id FROM voice_game_session_closures UNION SELECT room_id FROM voice_game_session_roster_receipts`
		identities = `SELECT source_account_id FROM voice_sdk_conversion_fence_receipts UNION SELECT target_account_id FROM voice_sdk_conversion_fence_receipts`
		for _, table := range []string{"voice_game_session_operations", "voice_game_session_closures", "voice_game_session_roster_receipts", "voice_sdk_conversion_fence_receipts"} {
			sources = append(sources, cutoverSource{table, "true"})
		}
	default:
		return nil, retirementproof.ErrUnsupported
	}
	sources = append(sources, cutoverSource{"voice_room_instances", "room_id IN (" + rooms + ")"}, cutoverSource{"voice_room_memberships", "room_id IN (" + rooms + ")"})
	// A newer account fence is retained as contradictory evidence, never omitted
	// because it no longer points at the captured owner room. It is not releasable
	// by this snapshot alone.
	fencePredicate := "room_id IN (SELECT room_id::text FROM (" + rooms + ") captured_rooms) OR account_id IN (" + identities + ")"
	sources = append(sources, cutoverSource{"voice_account_voice_fences", fencePredicate}, cutoverSource{"voice_profile_account_mappings", "account_id IN (" + identities + ") OR profile_id IN (SELECT profile_id FROM voice_room_memberships WHERE room_id IN (" + rooms + ")) OR profile_id IN (SELECT profile_id FROM voice_account_voice_fences WHERE " + fencePredicate + ")"})
	sort.Slice(sources, func(i, j int) bool { return sources[i].Table < sources[j].Table })
	return sources, nil
}

// PrepareCutoverDrain seals raw owner sources without forwarding provisioning,
// allocating a public close/cancel operation or changing a denial source. It is
// only preparation; neither this ref nor its existence is a drain receipt.
func (s *CutoverWorkStore) PrepareCutoverDrain(ctx context.Context, id uuid.UUID, cut retirementproof.CutBinding) (retirementproof.OwnerWorkRef, error) {
	var zero retirementproof.OwnerWorkRef
	if s == nil || s.Pool == nil || s.Authority == nil {
		return zero, retirementproof.ErrUnavailable
	}
	if id == uuid.Nil || !validCutoverBinding(cut) {
		return zero, retirementproof.ErrConflict
	}
	protocol, _, contract, err := CutoverSourceContract(s.SourceKind)
	if err != nil {
		return zero, err
	}
	if err = s.Authority.CheckMaintenance(ctx, cut); err != nil {
		return zero, err
	}
	request, err := json.Marshal(cutoverRequest{id, s.SourceKind, cut, contract})
	if err != nil {
		return zero, err
	}
	ref := retirementproof.OwnerWorkRef{Protocol: 1, OwnerProtocol: protocol, WorkID: id, RequestHash: retirementproof.Hash(retirementproof.OwnerRequestDomain, request)}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// No native row locks or native mutations: the snapshot cannot invert the
	// existing room/account identity barriers. External fences remain mandatory.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "voice.owned-game.work/"+id.String()); err != nil {
		return zero, err
	}
	var stored, storedHash []byte
	err = tx.QueryRow(ctx, `SELECT work_bytes,work_sha256 FROM voice_owned_game_cutover_work WHERE work_id=$1`, id).Scan(&stored, &storedHash)
	if err == nil {
		digest := retirementproof.Hash(retirementproof.WorkDomain, stored)
		if !bytes.Equal(storedHash, digest[:]) {
			return zero, retirementproof.ErrConflict
		}
		if _, err = decodeCutoverWork(stored, ref, s.SourceKind); err != nil {
			return zero, err
		}
		return ref, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	targets, err := captureCutoverTargets(ctx, tx, s.SourceKind)
	if err != nil {
		return zero, err
	}
	providers, unknown, err := deriveCutoverIssuance(targets)
	if err != nil {
		return zero, err
	}
	var rows []cutoverTarget
	if err = json.Unmarshal(targets, &rows); err != nil {
		return zero, err
	}
	work := retirementproof.OwnerWork{Ref: ref, Purpose: retirementproof.ColdCutDrain, SourceKind: s.SourceKind, Cut: cut, CapturedHead: retirementproof.OwnerHeadBinding{State: retirementproof.HeadAbsent, Hash: retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent))}, OwnerRequestDomain: retirementproof.OwnerRequestDomain, OwnerRequestBytes: request, SourceManifestHash: contract, SealedTargetManifestBytes: targets, SealedTargetManifestHash: retirementproof.Hash(retirementproof.TargetDomain, targets), SealedTargetCount: uint64(len(rows)), CapturedProviderBindings: providers, UnknownIssuanceMarkers: unknown}
	raw, err := json.Marshal(work)
	if err != nil {
		return zero, err
	}
	hash := retirementproof.Hash(retirementproof.WorkDomain, raw)
	_, err = tx.Exec(ctx, `INSERT INTO voice_owned_game_cutover_work(work_id,source_kind,owner_protocol,request_sha256,work_bytes,work_sha256) VALUES($1,$2,$3,$4,$5,$6)`, id, s.SourceKind, protocol, ref.RequestHash[:], raw, hash[:])
	if err != nil {
		return zero, err
	}
	return ref, tx.Commit(ctx)
}

func captureCutoverTargets(ctx context.Context, tx pgx.Tx, kind string) ([]byte, error) {
	sources, err := cutoverSources(kind)
	if err != nil {
		return nil, err
	}
	targets := []cutoverTarget{}
	for _, source := range sources {
		// Table/predicate come exclusively from the compiled contract above.
		rows, err := tx.Query(ctx, `SELECT to_jsonb(source)::text FROM `+source.Table+` source WHERE `+source.Predicate)
		if err != nil {
			return nil, fmt.Errorf("capture %s: %w", source.Table, err)
		}
		var captured []json.RawMessage
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return nil, err
			}
			canonical, err := canonicalCutoverJSON(raw)
			if err != nil {
				rows.Close()
				return nil, err
			}
			captured = append(captured, canonical)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		sort.Slice(captured, func(i, j int) bool { return bytes.Compare(captured[i], captured[j]) < 0 })
		descriptor, _ := json.Marshal(struct {
			Table, Predicate string
			Rows             int
		}{source.Table, source.Predicate, len(captured)})
		targets = append(targets, cutoverTarget{Source: "SOURCE_DESCRIPTOR", Row: descriptor})
		for _, raw := range captured {
			targets = append(targets, cutoverTarget{Source: source.Table, Row: raw})
		}
	}
	// Neither owner has complete authenticated historical credential-domain
	// evidence in its current schema. Empty memberships or expired grants must
	// therefore still resolve through protected source-specific history.
	marker, _ := json.Marshal(struct{ SourceKind, Reason string }{kind, "UNKNOWN_HISTORICAL_ISSUANCE_DOMAIN"})
	targets = append(targets, cutoverTarget{Source: "UNKNOWN_ISSUANCE_SOURCE", Row: marker})
	return json.Marshal(targets)
}

func (s *CutoverWorkStore) ReadOwnerWork(ctx context.Context, ref retirementproof.OwnerWorkRef) (retirementproof.OwnerWork, error) {
	var zero retirementproof.OwnerWork
	if s == nil || s.Pool == nil || s.Authority == nil {
		return zero, retirementproof.ErrUnavailable
	}
	var raw, hash, request []byte
	var kind, protocol string
	err := s.Pool.QueryRow(ctx, `SELECT source_kind,owner_protocol,request_sha256,work_bytes,work_sha256 FROM voice_owned_game_cutover_work WHERE work_id=$1`, ref.WorkID).Scan(&kind, &protocol, &request, &raw, &hash)
	if err != nil {
		return zero, err
	}
	digest := retirementproof.Hash(retirementproof.WorkDomain, raw)
	if kind != s.SourceKind || protocol != ref.OwnerProtocol || !bytes.Equal(request, ref.RequestHash[:]) || !bytes.Equal(hash, digest[:]) {
		return zero, retirementproof.ErrConflict
	}
	work, err := decodeCutoverWork(raw, ref, s.SourceKind)
	if err != nil {
		return zero, err
	}
	if err = s.Authority.CheckMaintenance(ctx, work.Cut); err != nil {
		return zero, err
	}
	return work, nil
}

func decodeCutoverWork(raw []byte, ref retirementproof.OwnerWorkRef, kind string) (retirementproof.OwnerWork, error) {
	var work retirementproof.OwnerWork
	if json.Unmarshal(raw, &work) != nil {
		return work, retirementproof.ErrConflict
	}
	canonical, err := json.Marshal(work)
	if err != nil || !bytes.Equal(raw, canonical) {
		return work, retirementproof.ErrConflict
	}
	protocol, _, contract, err := CutoverSourceContract(kind)
	if err != nil {
		return work, err
	}
	expectedRequest, _ := json.Marshal(cutoverRequest{ref.WorkID, kind, work.Cut, contract})
	var targets []cutoverTarget
	if work.Ref != ref || ref.Protocol != 1 || ref.OwnerProtocol != protocol || ref.WorkID == uuid.Nil || !validCutoverBinding(work.Cut) || work.SourceKind != kind || work.SourceManifestHash != contract || work.Purpose != retirementproof.ColdCutDrain || work.ExistingP3 != nil || work.SpaceID != uuid.Nil || work.OwnerRequestDomain != retirementproof.OwnerRequestDomain || !bytes.Equal(expectedRequest, work.OwnerRequestBytes) || retirementproof.Hash(retirementproof.OwnerRequestDomain, expectedRequest) != ref.RequestHash || work.CapturedHead.State != retirementproof.HeadAbsent || len(work.CapturedHead.CanonicalBytes) != 0 || work.CapturedHead.Hash != retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent)) || retirementproof.Hash(retirementproof.TargetDomain, work.SealedTargetManifestBytes) != work.SealedTargetManifestHash || json.Unmarshal(work.SealedTargetManifestBytes, &targets) != nil || uint64(len(targets)) != work.SealedTargetCount {
		return work, retirementproof.ErrConflict
	}
	// Re-derive every binding and UNKNOWN marker from authoritative retained raw
	// target bytes. Serialized/caller lists never become verifier authority.
	providers, unknown, err := deriveCutoverIssuance(work.SealedTargetManifestBytes)
	if err != nil {
		return work, err
	}
	if !digestListsEqual(providers, work.CapturedProviderBindings) || !digestListsEqual(unknown, work.UnknownIssuanceMarkers) {
		return work, retirementproof.ErrConflict
	}
	work.CapturedProviderBindings = providers
	work.UnknownIssuanceMarkers = unknown
	return work, nil
}

func deriveCutoverIssuance(raw []byte) ([]retirementproof.Digest, []retirementproof.Digest, error) {
	var targets []cutoverTarget
	if json.Unmarshal(raw, &targets) != nil || len(targets) == 0 {
		return nil, nil, retirementproof.ErrConflict
	}
	providers, unknown := map[retirementproof.Digest]bool{}, map[retirementproof.Digest]bool{}
	var visit func(any) error
	visit = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				if strings.EqualFold(strings.ReplaceAll(key, "_", ""), "providerbinding") && item != nil {
					binding, err := parseCutoverProvider(item)
					if err != nil {
						return err
					}
					providers[binding] = true
				}
				if err := visit(item); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range v {
				if err := visit(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, target := range targets {
		if target.Source == "" || len(target.Row) == 0 {
			return nil, nil, retirementproof.ErrConflict
		}
		decoder := json.NewDecoder(bytes.NewReader(target.Row))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) != nil {
			return nil, nil, retirementproof.ErrConflict
		}
		if err := visit(value); err != nil {
			return nil, nil, err
		}
		if target.Source != "SOURCE_DESCRIPTOR" {
			canonical, err := json.Marshal(target)
			if err != nil {
				return nil, nil, err
			}
			unknown[retirementproof.Hash("voice.party.retirement.unknown-owner-issuance.v1", canonical)] = true
		}
	}
	return sortedCutoverDigests(providers), sortedCutoverDigests(unknown), nil
}

func parseCutoverProvider(value any) (retirementproof.Digest, error) {
	var digest retirementproof.Digest
	var raw []byte
	var err error
	switch v := value.(type) {
	case string:
		raw, err = hex.DecodeString(strings.TrimPrefix(v, `\x`))
		if err != nil || len(raw) != 32 {
			raw, err = base64.StdEncoding.DecodeString(v)
		}
	case []any:
		for _, n := range v {
			number, ok := n.(json.Number)
			if !ok {
				return digest, retirementproof.ErrConflict
			}
			i, e := number.Int64()
			if e != nil || i < 0 || i > 255 {
				return digest, retirementproof.ErrConflict
			}
			raw = append(raw, byte(i))
		}
	default:
		return digest, retirementproof.ErrConflict
	}
	if err != nil || len(raw) != 32 {
		return digest, retirementproof.ErrConflict
	}
	copy(digest[:], raw)
	if digest == (retirementproof.Digest{}) {
		return digest, retirementproof.ErrConflict
	}
	return digest, nil
}
func sortedCutoverDigests(values map[retirementproof.Digest]bool) []retirementproof.Digest {
	result := []retirementproof.Digest{}
	for d := range values {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return bytes.Compare(result[i][:], result[j][:]) < 0 })
	return result
}
func digestListsEqual(a, b []retirementproof.Digest) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func canonicalCutoverJSON(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
func validCutoverBinding(c retirementproof.CutBinding) bool {
	return c.EnvironmentID != uuid.Nil && c.AuthorityIncarnation != uuid.Nil && c.CutoverOperationID != uuid.Nil && c.CoverageEpoch > 0 && c.CoverageEpoch <= math.MaxInt64 && c.PlanHash != (retirementproof.Digest{}) && c.DatabaseIdentity != (retirementproof.Digest{}) && c.SourceCatalogHash != (retirementproof.Digest{}) && c.SchemaContractHash != (retirementproof.Digest{}) && c.WriterManifestHash != (retirementproof.Digest{}) && c.ProviderManifestHash != (retirementproof.Digest{}) && c.CredentialManifestHash != (retirementproof.Digest{}) && c.ClosedAdmissionBarrierID != uuid.Nil && c.ClosedAdmissionBarrierHash != (retirementproof.Digest{}) && c.IssuerImplementationManifestHash != (retirementproof.Digest{})
}
