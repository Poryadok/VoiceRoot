package legacycut

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"strings"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

type PartitionRef struct {
	ID   uuid.UUID
	Hash retirementproof.Digest
}
type Obligation struct {
	ID, Resource, Key, Path string
	ValueHash               retirementproof.Digest
	Retain                  bool
}
type OwnershipAssignment struct {
	ObligationID              string
	ValueHash                 retirementproof.Digest
	Owner                     retirementproof.OwnerWorkRef
	OwnerWorkHash, TargetHash retirementproof.Digest
	Disposition               string
}
type Partition struct {
	ID          uuid.UUID
	Cut         retirementproof.CutBinding
	LegacyWork  retirementproof.OwnerWorkRef
	CaptureHash retirementproof.Digest
	Proof       retirementproof.ProofRef
	Assignments []OwnershipAssignment
}

// ValidatePartitionShape checks exact coverage only. It grants no execution
// authority; protected owner-journal acceptance and fresh proof remain required.
func ValidatePartitionShape(p Partition, obligations []Obligation) error {
	zero := retirementproof.Digest{}
	if p.ID == uuid.Nil || p.LegacyWork.Protocol != 1 || p.LegacyWork.WorkID == uuid.Nil || p.CaptureHash == zero || p.Proof.Protocol != 1 || p.Proof.ProofID == uuid.Nil || p.Proof.ReceiptHash == zero || len(p.Assignments) != len(obligations) {
		return retirementproof.ErrConflict
	}
	expected := map[string]Obligation{}
	for _, o := range obligations {
		if o.ID == "" || o.ValueHash == zero {
			return retirementproof.ErrConflict
		}
		if _, exists := expected[o.ID]; exists {
			return retirementproof.ErrConflict
		}
		expected[o.ID] = o
	}
	seen := map[string]bool{}
	owners := map[retirementproof.OwnerWorkRef]retirementproof.Digest{}
	for i, a := range p.Assignments {
		o, exists := expected[a.ObligationID]
		if !exists || seen[a.ObligationID] || o.ValueHash != a.ValueHash || i > 0 && p.Assignments[i-1].ObligationID >= a.ObligationID || a.Owner.Protocol != 1 || a.Owner.OwnerProtocol == "" || a.Owner.WorkID == uuid.Nil || a.Owner.RequestHash == zero || a.OwnerWorkHash == zero || a.TargetHash == zero {
			return retirementproof.ErrConflict
		}
		seen[a.ObligationID] = true
		if o.Retain && a.Disposition != "RETAIN" || !o.Retain && a.Disposition != "DRAIN" {
			return retirementproof.ErrConflict
		}
		if prior, ok := owners[a.Owner]; ok && prior != a.OwnerWorkHash {
			return retirementproof.ErrConflict
		}
		owners[a.Owner] = a.OwnerWorkHash
	}
	return nil
}

func PartitionHash(p Partition) (retirementproof.Digest, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return retirementproof.Digest{}, err
	}
	return retirementproof.Hash("voice.legacy.partition.v1", raw), nil
}

// ProjectionLinkage is the stable internal two-owner seam. No target tuple,
// before/after JSON, bool or mutable global binding substitutes for these refs.
type ProjectionLinkage struct {
	NativeOwnerWorkRef retirementproof.OwnerWorkRef
	LegacyWorkRef      retirementproof.OwnerWorkRef
	PartitionRef       PartitionRef
	ResourceProofRef   retirementproof.ProofRef
	NativeTargetHash   retirementproof.Digest
	NativeCommandRef   uuid.UUID
}
type ProjectionProgress struct {
	Linkage          ProjectionLinkage
	Step             store.LegacyProjectionStepRef
	CanonicalReceipt []byte
	ReceiptHash      retirementproof.Digest
}
type GuardedProjection interface {
	PrepareProjection(context.Context, ProjectionLinkage) (store.LegacyProjectionStepRef, error)
	ResumeProjection(context.Context, ProjectionLinkage, store.LegacyProjectionStepRef) (ProjectionProgress, error)
	ReadProjection(context.Context, ProjectionLinkage, store.LegacyProjectionStepRef) (ProjectionProgress, error)
}

// DeriveObligations rederives every raw entry AND embedded logical value from C.
// It deliberately does not assign owners from a prefix or absent native marker.
// Protected native/history resolvers and owner acceptances decide ownership.
func DeriveObligations(w retirementproof.OwnerWork) ([]Obligation, error) {
	targets, err := decodeTargets(w)
	if err != nil {
		return nil, err
	}
	result := []Obligation{}
	appendValue := func(id, resource, key, path string, value any, retain bool) {
		raw, _ := json.Marshal(value)
		result = append(result, Obligation{ID: id, Resource: resource, Key: key, Path: path, ValueHash: retirementproof.Hash("voice.legacy.obligation-value.v1", raw), Retain: retain})
	}
	for i, t := range targets {
		if t.Kind != "REDIS" {
			appendValue("segment/"+strconv.Itoa(i), "", "", "", t, true)
			if t.Kind == "CLOSED_INVENTORY" {
				for _, cohort := range t.Inventory.ExecutionCohorts {
					appendValue("memory/"+cohort.ID.String(), "", "", "", cohort, false)
				}
			}
			if t.Kind == "R22_RETAINED" {
				for _, raw := range t.Rows {
					rowHash := retirementproof.Hash("voice.legacy.r22-row.v1", raw)
					var value any
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if decoder.Decode(&value) != nil {
						return nil, retirementproof.ErrConflict
					}
					id := "r22/" + t.Table + "/" + hex.EncodeToString(rowHash[:])
					if err := walkObligations(value, "", func(path string, v any) { appendValue(id+path, "", "", path, v, false) }); err != nil {
						return nil, err
					}
				}
			}
			continue
		}
		for _, e := range t.Snapshot.Entries {
			id := t.Endpoint.ID + "/" + hex.EncodeToString([]byte(e.Key))
			local := strings.TrimPrefix(e.Key, t.Endpoint.Prefix)
			retained := strings.HasPrefix(local, "voice_move_operation:") || strings.HasPrefix(local, "space_lifecycle:") || strings.HasPrefix(local, "space_media_") || strings.HasPrefix(local, "match_squad_epoch:") || strings.HasPrefix(local, "match_squad_terminal_epoch:")
			appendValue(id+"/@physical", t.Endpoint.ID, e.Key, "@physical", struct {
				Kind, Expiry string
				Deadline     int64
			}{e.Kind, e.Expiry, e.ExpiresAtUnixMilli}, retained)
			if retained {
				appendValue(id+"/@retained", t.Endpoint.ID, e.Key, "@retained", e, true)
				if strings.HasPrefix(local, "voice_move_operation:") {
					var move map[string]json.RawMessage
					if json.Unmarshal([]byte(e.Value), &move) != nil {
						return nil, retirementproof.ErrConflict
					}
					var value map[string]json.RawMessage
					raw := move["result"]
					if raw == nil {
						raw = move["Result"]
					}
					if json.Unmarshal(raw, &value) != nil {
						return nil, retirementproof.ErrConflict
					}
					for _, side := range []string{"Source", "Destination"} {
						raw = value[side]
						if raw == nil {
							raw = value[strings.ToLower(side)]
						}
						if !json.Valid(raw) {
							return nil, retirementproof.ErrConflict
						}
						appendValue(id+"/@history/"+side, t.Endpoint.ID, e.Key, "@history/"+side, json.RawMessage(raw), false)
					}
				}
				continue
			}
			if e.Kind == "set" {
				for _, m := range e.Members {
					appendValue(id+"/member/"+hex.EncodeToString([]byte(m)), t.Endpoint.ID, e.Key, "member/"+hex.EncodeToString([]byte(m)), m, false)
				}
				continue
			}
			if strings.HasPrefix(local, "call:") {
				var value any
				decoder := json.NewDecoder(strings.NewReader(e.Value))
				decoder.UseNumber()
				if decoder.Decode(&value) != nil {
					return nil, retirementproof.ErrConflict
				}
				if err := walkObligations(value, "", func(path string, v any) { appendValue(id+"/json"+path, t.Endpoint.ID, e.Key, "json"+path, v, false) }); err != nil {
					return nil, err
				}
			} else {
				appendValue(id+"/value", t.Endpoint.ID, e.Key, "value", e.RawValue, false)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	for i := 1; i < len(result); i++ {
		if result[i].ID == result[i-1].ID {
			return nil, retirementproof.ErrConflict
		}
	}
	return result, nil
}

func walkObligations(value any, path string, emit func(string, any)) error {
	escape := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
	switch v := value.(type) {
	case map[string]any:
		emit(path+"/@object", "object")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if err := walkObligations(v[k], path+"/"+escape(k), emit); err != nil {
				return err
			}
		}
	case []any:
		emit(path+"/@array", "array")
		seen := map[string]bool{}
		for i, item := range v {
			key := strconv.Itoa(i)
			if m, ok := item.(map[string]any); ok {
				profile, p := m["profile_id"].(string)
				stream, s := m["stream_id"].(string)
				if p && s {
					key = "share:" + escape(profile) + ":" + escape(stream)
				}
			}
			if seen[key] {
				return retirementproof.ErrConflict
			}
			seen[key] = true
			if err := walkObligations(item, path+"/"+key, emit); err != nil {
				return err
			}
		}
	default:
		emit(path, value)
	}
	return nil
}

func sameCanonical(a, b any) bool {
	x, e := json.Marshal(a)
	y, f := json.Marshal(b)
	return e == nil && f == nil && bytes.Equal(x, y)
}
