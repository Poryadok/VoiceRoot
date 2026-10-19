package ordinarycall

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"voice/backend/voice/internal/legacycut"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

// ReadCutoverMemberOwnership resolves protected work references and contributes
// exact marked member/share claims to the composite temporal resolver. This is
// a partial semantic plan, never a total partition ACK or execution authority.
// Structural custody, room metadata and unversioned locators remain separate.
func (o *CutoverDrainOwner) ReadCutoverMemberOwnership(ctx context.Context, native, captured retirementproof.OwnerWorkRef) (legacycut.OwnershipPlan, error) {
	if o == nil || o.Authority == nil || o.LegacyWorks == nil {
		return legacycut.OwnershipPlan{}, ErrUnavailable
	}
	work, err := o.ReadOwnerWork(ctx, native)
	if err != nil {
		return legacycut.OwnershipPlan{}, err
	}
	if err = o.Authority.CheckMaintenance(ctx, work.Cut); err != nil {
		return legacycut.OwnershipPlan{}, err
	}
	legacy, err := o.LegacyWorks.ReadOwnerWork(ctx, captured)
	if err != nil {
		return legacycut.OwnershipPlan{}, err
	}
	if legacy.Ref != captured {
		return legacycut.OwnershipPlan{}, ErrConflict
	}
	return ordinaryMemberOwnership(work, legacy)
}

func ordinaryMemberOwnership(native, captured retirementproof.OwnerWork) (legacycut.OwnershipPlan, error) {
	var plan legacycut.OwnershipPlan
	targets, err := validateCutoverManifest(native)
	if err != nil {
		return plan, err
	}
	if captured.Ref.Protocol != retirementproof.ProtocolV1 || captured.Ref.OwnerProtocol != legacycut.Protocol || captured.Cut != native.Cut {
		return plan, retirementproof.ErrConflict
	}
	obligations, err := legacycut.DeriveObligations(captured)
	if err != nil {
		return plan, err
	}
	encoded, err := json.Marshal(native)
	if err != nil {
		return plan, err
	}
	workHash := retirementproof.Hash(retirementproof.WorkDomain, encoded)
	var request cutoverRequest
	if json.Unmarshal(native.OwnerRequestBytes, &request) != nil {
		return plan, retirementproof.ErrConflict
	}
	var before, closure *Room
	var roomTarget retirementproof.Digest
	memberTargets := map[string]struct {
		body []byte
		hash retirementproof.Digest
	}{}
	for _, target := range targets {
		body, err := json.Marshal(target)
		if err != nil {
			return plan, err
		}
		hash := retirementproof.Hash(cutoverEntryDomain, body)
		id := "native-ordinary/" + native.Ref.WorkID.String() + "/" + hex.EncodeToString(hash[:])
		retained := target.Kind == "outbox"
		disposition := "DRAIN"
		if retained {
			disposition = "RETAIN"
		}
		plan.AdditionalObligations = append(plan.AdditionalObligations, legacycut.Obligation{ID: id, Path: "native/" + target.Kind + "/" + target.Key, ValueHash: hash, Retain: retained, SourceBytes: body})
		plan.Assignments = append(plan.Assignments, legacycut.OwnershipAssignment{ObligationID: id, ValueHash: hash, Owner: native.Ref, OwnerWorkHash: workHash, TargetHash: hash, Disposition: disposition})
		switch target.Kind {
		case "room":
			before, err = decodeRoom(target.Bytes, request.Expected.RoomID)
			roomTarget = hash
		case "closure":
			closure, err = decodeRoom(target.Bytes, request.Expected.RoomID)
		case "membership":
			memberTargets[target.Key] = struct {
				body []byte
				hash retirementproof.Digest
			}{target.Bytes, hash}
		}
		if err != nil {
			return plan, err
		}
	}
	if before == nil || closure == nil {
		return plan, retirementproof.ErrConflict
	}
	// DeriveObligations already validates the complete closed capture, including
	// all raw/retained segments. This view selects only matching raw call entries.
	var segments []struct {
		Kind     string
		Endpoint *legacycut.Endpoint
		Snapshot *store.LegacyRedisSnapshot
	}
	if json.Unmarshal(captured.SealedTargetManifestBytes, &segments) != nil {
		return plan, retirementproof.ErrConflict
	}
	for _, segment := range segments {
		if segment.Kind != "REDIS" || segment.Endpoint == nil || segment.Snapshot == nil {
			continue
		}
		for _, entry := range segment.Snapshot.Entries {
			if entry.Key != segment.Endpoint.Prefix+"call:"+before.Intent.RoomID.String() {
				continue
			}
			footprint, err := inspectOrdinaryCapturedCall(before, []byte(entry.Value), closure.Version)
			if err != nil {
				return plan, err
			}
			for profile, member := range footprint.Members {
				targetHash := roomTarget // exact retained generation is in the room entry
				if row, exists := memberTargets[profile]; exists {
					var tuple struct {
						Account    string `json:"account_id"`
						Generation string `json:"membership_generation"`
						Operation  string `json:"fence_operation_id"`
						Identity   string `json:"media_identity"`
					}
					if json.Unmarshal(row.body, &tuple) != nil {
						return plan, retirementproof.ErrConflict
					}
					if tuple.Generation == member.Generation.String() {
						if tuple.Account != member.AccountID.String() || tuple.Operation != member.FenceOperationID.String() || tuple.Identity != member.MediaIdentity {
							return plan, retirementproof.ErrConflict
						}
						targetHash = row.hash
					}
				}
				escaped := strings.ReplaceAll(strings.ReplaceAll(profile, "~", "~0"), "/", "~1")
				for _, obligation := range obligations {
					if obligation.Resource != segment.Endpoint.ID || obligation.Key != entry.Key || strings.HasSuffix(obligation.Path, "/@object") || strings.HasSuffix(obligation.Path, "/@array") {
						continue
					}
					path := obligation.Path
					if !strings.HasPrefix(path, "json/states/"+escaped+"/") && path != "json/ordinary_media_identities/"+escaped && !strings.HasPrefix(path, "json/screen_shares/share:"+escaped+":") {
						continue
					}
					plan.Assignments = append(plan.Assignments, legacycut.OwnershipAssignment{ObligationID: obligation.ID, ValueHash: obligation.ValueHash, Owner: native.Ref, OwnerWorkHash: workHash, TargetHash: targetHash, Disposition: "DRAIN"})
				}
			}
		}
	}
	sort.Slice(plan.AdditionalObligations, func(i, j int) bool { return plan.AdditionalObligations[i].ID < plan.AdditionalObligations[j].ID })
	sort.Slice(plan.Assignments, func(i, j int) bool { return plan.Assignments[i].ObligationID < plan.Assignments[j].ObligationID })
	for i := 1; i < len(plan.Assignments); i++ {
		if plan.Assignments[i-1].ObligationID == plan.Assignments[i].ObligationID {
			return legacycut.OwnershipPlan{}, retirementproof.ErrConflict
		}
	}
	return plan, nil
}
