package ordinarycall

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/legacycut"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/store"
)

func TestOrdinaryMemberOwnershipBindsCompleteNativeTargetsButNotForeignOrStructure(t *testing.T) {
	native, first, _ := pendingGroup(t)
	op := native.Intent.OperationID
	require.NoError(t, native.RecordEnsureResult(op, true, testNow))
	require.NoError(t, native.CommitAdmission(op, testNow))
	survivor := identity()
	join := uuid.New()
	require.NoError(t, native.BeginJoin(join, survivor, testNow))
	require.NoError(t, native.CommitAdmission(join, testNow))
	oldMember := native.Members[first.ProfileID.String()]
	oldVersion := native.Version
	require.NoError(t, native.DecideLeave(first.ProfileID, testNow))
	require.NoError(t, native.CompleteMemberCleanup(first.ProfileID, oldMember.Generation, testNow))
	rejoin := uuid.New()
	require.NoError(t, native.BeginJoin(rejoin, first, testNow))
	require.NoError(t, native.CommitAdmission(rejoin, testNow))
	work := ordinaryOwnershipWorkFixture(t, native)
	foreign := uuid.NewString()
	profile := first.ProfileID.String()
	call := store.Call{RoomID: native.Intent.RoomID.String(), ChatID: native.Intent.ChatID.String(), LivekitRoomName: native.LiveKitRoomName,
		OrdinaryOperationID: op.String(), OrdinaryVersion: native.Version, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		States:                  map[string]store.ParticipantState{profile: {ProfileID: profile}, foreign: {ProfileID: foreign}},
		OrdinaryMediaIdentities: map[string]string{profile: native.Members[profile].MediaIdentity},
		ScreenShares:            []store.ScreenShareEntry{{ProfileID: profile, StreamID: "native"}, {ProfileID: foreign, StreamID: "foreign"}}}
	captured := ordinaryLegacyWorkFixture(t, work.Cut, call, profile)
	plan, err := ordinaryMemberOwnership(work, captured)
	require.NoError(t, err)
	require.Len(t, plan.AdditionalObligations, 2+len(native.Members), "native obligations remain even when Redis is absent")
	raw, err := legacycut.DeriveObligations(captured)
	require.NoError(t, err)
	byID := map[string]legacycut.Obligation{}
	for _, o := range append(raw, plan.AdditionalObligations...) {
		byID[o.ID] = o
	}
	claimed := map[string]bool{}
	for _, a := range plan.Assignments {
		o, ok := byID[a.ObligationID]
		require.True(t, ok)
		require.Equal(t, work.Ref, a.Owner)
		require.Equal(t, o.ValueHash, a.ValueHash)
		require.NotZero(t, a.TargetHash)
		require.False(t, claimed[o.ID])
		claimed[o.ID] = true
	}
	for _, o := range raw {
		if strings.HasPrefix(o.Path, "json/states/"+profile+"/") && !strings.HasSuffix(o.Path, "/@object") || o.Path == "json/ordinary_media_identities/"+profile || strings.HasPrefix(o.Path, "json/screen_shares/share:"+profile+":") && !strings.HasSuffix(o.Path, "/@object") {
			require.True(t, claimed[o.ID], o.Path)
			for _, a := range plan.Assignments {
				if a.ObligationID == o.ID {
					require.Equal(t, ordinaryTargetHashFixture(t, work, "membership", profile), a.TargetHash, "current tuple binds its exact entry, not the whole target manifest")
				}
			}
		} else {
			require.False(t, claimed[o.ID], "foreign, structural, locator or room metadata requires its own temporal/custody resolution: "+o.Path)
		}
	}
	for _, o := range plan.AdditionalObligations {
		require.True(t, claimed[o.ID])
	}
	oldCall := call
	oldCall.OrdinaryVersion = oldVersion
	oldCall.OrdinaryMediaIdentities = map[string]string{profile: oldMember.MediaIdentity}
	oldCaptured := ordinaryLegacyWorkFixture(t, work.Cut, oldCall, profile)
	oldPlan, err := ordinaryMemberOwnership(work, oldCaptured)
	require.NoError(t, err)
	oldObligations, err := legacycut.DeriveObligations(oldCaptured)
	require.NoError(t, err)
	for _, o := range oldObligations {
		if o.Path == "json/ordinary_media_identities/"+profile {
			found := false
			for _, a := range oldPlan.Assignments {
				if a.ObligationID == o.ID {
					found = true
					require.Equal(t, ordinaryTargetHashFixture(t, work, "room", native.Intent.RoomID.String()), a.TargetHash, "retired generation binds retained room history, never current membership")
				}
			}
			require.True(t, found)
		}
	}
	var changedTargets []cutoverTarget
	require.NoError(t, json.Unmarshal(work.SealedTargetManifestBytes, &changedTargets))
	for i := range changedTargets {
		if changedTargets[i].Kind == "membership" && changedTargets[i].Key == profile {
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(changedTargets[i].Bytes, &fields))
			fields["account_id"] = json.RawMessage(`"` + uuid.NewString() + `"`)
			changedTargets[i].Bytes, err = json.Marshal(fields)
			require.NoError(t, err)
		}
	}
	wrongAccount := work
	wrongAccount.SealedTargetManifestBytes, err = json.Marshal(changedTargets)
	require.NoError(t, err)
	wrongAccount.SealedTargetManifestHash = retirementproof.Hash(retirementproof.TargetDomain, wrongAccount.SealedTargetManifestBytes)
	wrongAccount.UnknownIssuanceMarkers = cutoverUnknownMarkers(changedTargets)
	_, err = ordinaryMemberOwnership(wrongAccount, captured)
	require.ErrorIs(t, err, retirementproof.ErrConflict, "self-consistent hashes cannot transfer a captured generation to another account")
	changed := captured
	changed.Cut.CoverageEpoch++
	_, err = ordinaryMemberOwnership(work, changed)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	changedNative := work
	changedNative.UnknownIssuanceMarkers = changedNative.UnknownIssuanceMarkers[:1]
	_, err = ordinaryMemberOwnership(changedNative, captured)
	require.Error(t, err)
	markerless := call
	markerless.OrdinaryOperationID = ""
	_, err = ordinaryMemberOwnership(work, ordinaryLegacyWorkFixture(t, work.Cut, markerless, profile))
	require.ErrorIs(t, err, retirementproof.ErrUnsupported, "matching current head never owns an unversioned pointer")
}

func ordinaryOwnershipWorkFixture(t *testing.T, before *Room) retirementproof.OwnerWork {
	t.Helper()
	raw, err := json.Marshal(before)
	require.NoError(t, err)
	closed, err := decodeRoom(raw, before.Intent.RoomID)
	require.NoError(t, err)
	require.NoError(t, closed.terminate(uuid.Nil, "maintenance_retirement", testNow))
	closeBytes, err := json.Marshal(closed)
	require.NoError(t, err)
	raw, err = cutoverCanonical(raw)
	require.NoError(t, err)
	closeBytes, err = cutoverCanonical(closeBytes)
	require.NoError(t, err)
	targets := []cutoverTarget{{"closure", before.Intent.RoomID.String(), closeBytes}, {"room", before.Intent.RoomID.String(), raw}}
	for profile, member := range closed.Members {
		body, e := json.Marshal(map[string]any{"room_id": before.Intent.RoomID.String(), "profile_id": profile, "account_id": member.AccountID.String(), "membership_generation": member.Generation.String(), "fence_operation_id": member.FenceOperationID.String(), "media_identity": member.MediaIdentity, "state": string(member.State), "session_epoch": member.SessionEpoch})
		require.NoError(t, e)
		targets = append(targets, cutoverTarget{"membership", profile, body})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Kind+"\x00"+targets[i].Key < targets[j].Kind+"\x00"+targets[j].Key
	})
	manifest, err := json.Marshal(targets)
	require.NoError(t, err)
	cut := ordinaryCutFixture()
	id := uuid.New()
	request, err := json.Marshal(cutoverRequest{OrdinaryCutoverOwnerProtocol, cut, CutoverExpectedRoom{WorkID: id, RoomID: before.Intent.RoomID, Incarnation: before.Intent.OperationID, Version: before.Version}})
	require.NoError(t, err)
	head, err := json.Marshal(before.Intent)
	require.NoError(t, err)
	return retirementproof.OwnerWork{Ref: retirementproof.OwnerWorkRef{Protocol: 1, OwnerProtocol: OrdinaryCutoverOwnerProtocol, WorkID: id, RequestHash: retirementproof.Hash(retirementproof.OwnerRequestDomain, request)}, Purpose: retirementproof.ColdCutDrain, SourceKind: "NATIVE_ORDINARY", Cut: cut,
		CapturedHead:       retirementproof.OwnerHeadBinding{State: retirementproof.HeadPresent, CanonicalBytes: head, Hash: retirementproof.Hash(retirementproof.HeadDomain, head)},
		OwnerRequestDomain: retirementproof.OwnerRequestDomain, OwnerRequestBytes: request, SourceManifestHash: OrdinaryCutoverContractHash(),
		SealedTargetManifestBytes: manifest, SealedTargetManifestHash: retirementproof.Hash(retirementproof.TargetDomain, manifest), SealedTargetCount: uint64(len(targets)), UnknownIssuanceMarkers: cutoverUnknownMarkers(targets)}
}

func ordinaryTargetHashFixture(t *testing.T, work retirementproof.OwnerWork, kind, key string) retirementproof.Digest {
	t.Helper()
	var targets []cutoverTarget
	require.NoError(t, json.Unmarshal(work.SealedTargetManifestBytes, &targets))
	for _, target := range targets {
		if target.Kind == kind && target.Key == key {
			body, e := json.Marshal(target)
			require.NoError(t, e)
			return retirementproof.Hash(cutoverEntryDomain, body)
		}
	}
	t.Fatalf("missing target %s/%s", kind, key)
	return retirementproof.Digest{}
}

// Complete closed legacy schema fixture, not installed source provenance.
func ordinaryLegacyWorkFixture(t *testing.T, cut retirementproof.CutBinding, call store.Call, profile string) retirementproof.OwnerWork {
	t.Helper()
	type target struct {
		Kind      string
		Inventory *legacycut.Inventory       `json:",omitempty"`
		Endpoint  *legacycut.Endpoint        `json:",omitempty"`
		Snapshot  *store.LegacyRedisSnapshot `json:",omitempty"`
		Table     string                     `json:",omitempty"`
		Rows      []json.RawMessage
	}
	d := retirementproof.Hash("fixture", []byte("component only"))
	e := legacycut.Endpoint{ID: "fixture", ResourceIdentity: d, StorageLineageHash: d, Prefix: "voice:", EvidencePrefix: "receipts:"}
	i := legacycut.Inventory{Cut: cut, Endpoints: []legacycut.Endpoint{e}, ExecutionCohorts: []legacycut.ExecutionCohort{}, CanonicalDeploymentBytes: json.RawMessage(`{"fixture":true}`)}
	b, err := json.Marshal(call)
	require.NoError(t, err)
	entry := func(key, value string) store.LegacyRedisEntry {
		return store.LegacyRedisEntry{Key: key, Kind: "string", Value: value, RawKey: []byte(key), RawValue: []byte(value), Expiry: "PERSISTENT"}
	}
	s := store.LegacyRedisSnapshot{Prefix: e.Prefix, ObservedAtUnixMilli: 1, Entries: []store.LegacyRedisEntry{entry("voice:call:"+call.RoomID, string(b)), entry("voice:session:"+profile, call.RoomID)}}
	targets := []target{{Kind: "CLOSED_INVENTORY", Inventory: &i}, {Kind: "REDIS", Endpoint: &e, Snapshot: &s}}
	for _, table := range []string{"voice_room_instances", "voice_room_memberships", "voice_lifecycle_operations", "voice_lifecycle_effects", "voice_media_epoch_denials", "voice_event_outbox", "voice_lifecycle_redis_divergences"} {
		targets = append(targets, target{Kind: "R22_RETAINED", Table: table, Rows: []json.RawMessage{}})
	}
	markers := []retirementproof.Digest{}
	for _, v := range targets {
		raw, e := json.Marshal(v)
		require.NoError(t, e)
		markers = append(markers, retirementproof.Hash("voice.legacy.unknown-issuance.v1", raw))
	}
	sort.Slice(markers, func(i, j int) bool { return bytes.Compare(markers[i][:], markers[j][:]) < 0 })
	id := uuid.New()
	request, err := json.Marshal(struct {
		Protocol  string
		ID        uuid.UUID
		Inventory legacycut.Inventory
	}{legacycut.Protocol, id, i})
	require.NoError(t, err)
	manifest, err := json.Marshal(targets)
	require.NoError(t, err)
	w := retirementproof.OwnerWork{Ref: retirementproof.OwnerWorkRef{Protocol: 1, OwnerProtocol: legacycut.Protocol, WorkID: id, RequestHash: retirementproof.Hash(retirementproof.OwnerRequestDomain, request)}, Purpose: retirementproof.ColdCutDrain, SourceKind: legacycut.SourceKind, Cut: cut,
		CapturedHead: retirementproof.OwnerHeadBinding{State: retirementproof.HeadAbsent, Hash: retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent))}, OwnerRequestDomain: retirementproof.OwnerRequestDomain, OwnerRequestBytes: request, SourceManifestHash: legacycut.ContractHash(),
		SealedTargetManifestBytes: manifest, SealedTargetManifestHash: retirementproof.Hash(retirementproof.TargetDomain, manifest), SealedTargetCount: uint64(len(targets)), UnknownIssuanceMarkers: markers}
	_, err = legacycut.DeriveObligations(w)
	require.NoError(t, err)
	return w
}
