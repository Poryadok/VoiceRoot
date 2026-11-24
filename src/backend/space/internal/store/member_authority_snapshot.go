package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"math"
	"sort"
	"strconv"
	"time"
	rolev1 "voice.app/voice/role/v1"
	"voice.app/voice/space/v1"
	"voice.app/voice/user/v1"
	"voice/backend/pkg/memberadmission"
)

// MemberAuthoritySnapshot is one consistent snapshot read per relation for a
// whole profile set inside the owner transaction. It only replaces the Ban
// capture/begin/finish loop round-trips; the batch and single-profile routes
// share the per-entry validation cores below, so no decision changes. The
// rare native witness validators stay live per-entry DB reads.
type MemberAuthoritySnapshot struct {
	ordinary          map[uuid.UUID]ordinaryAuthorityRow
	ordinaryPresent   map[uuid.UUID]bool
	ordinaryPublished map[string]bool
	qualified         map[string]qualifiedAuthorityRow
	banned            map[uuid.UUID]bool
	community         map[uuid.UUID]communityAuthorityRow
	rosterExists      map[uuid.UUID]bool
	rosterRemaining   map[string]rosterRemainingRow
	// intents and nativePubs are grouped by exactly-one-row keys.
	intents    map[string][]intentAuthorityRow
	nativePubs map[string][]nativePublicationRow
	// version gates the qualification relation like readQualifiedSourceAccount.
	version int64
}

// found marks a head row present; found=false reproduces the ErrNoRows shape
// and the single wrapper fails closed on any other Scan error immediately.
type ordinaryAuthorityRow struct {
	found      bool
	generation uint64
	// retiredThrough is the head retirement watermark.
	retiredThrough uint64
	operation      uuid.UUID
	state          string
	legacy         bool
	// receipt, binding and native keep their exact bytea shapes.
	receipt []byte
	binding []byte
	native  []byte
	kind    int32
	// account is the head qualified source account fact.
	account *uuid.UUID
}

type qualifiedAuthorityRow struct {
	found     bool
	account   uuid.UUID
	duplicate bool
}

type communityAuthorityRow struct {
	found           bool
	generation      uint64
	retiredThrough  uint64
	ownerGeneration uint64
	episode         uuid.UUID
	state           string
}

type intentAuthorityRow struct {
	role  uuid.UUID
	saved []byte
	state string
}

type nativePublicationRow struct {
	raw         []byte
	digest      []byte
	state       string
	publishedAt time.Time
}

type rosterRemainingRow struct {
	found          bool
	sourceRevision uint64
	remaining      time.Duration
	duplicate      bool
}

func ordinaryCaptureEntry(profile, operation uuid.UUID, generation uint64) *userv1.SpaceAccountCaptureEntry {
	return &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{Generation: generation, PublicationId: operation.String()}}}
}

func communityCaptureEntry(profile, episode uuid.UUID, ownerGeneration, generation uint64) *userv1.SpaceAccountCaptureEntry {
	return &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Community{Community: &userv1.SpaceCommunityPublicationIdentity{OwnerGeneration: ownerGeneration, MemberGeneration: generation, EpisodeId: episode.String()}}}
}

func ordinaryPublishedKey(profile, operation uuid.UUID, generation uint64) string {
	return profile.String() + "|" + strconv.FormatUint(generation, 10) + "|" + operation.String()
}

func intentKey(operation, profile uuid.UUID, generation uint64, binding []byte) string {
	return operation.String() + "|" + profile.String() + "|" + strconv.FormatUint(generation, 10) + "|" + hex.EncodeToString(binding)
}

func nativePubKey(operation, profile uuid.UUID, generation uint64, kind int32) string {
	return operation.String() + "|" + profile.String() + "|" + strconv.FormatInt(int64(kind), 10)
}

func rosterKey(profile uuid.UUID, ownerGeneration uint64) string {
	return profile.String() + "|" + strconv.FormatUint(ownerGeneration, 10)
}

func qualifiedAuthorityFromSnapshot(snapshot *MemberAuthoritySnapshot, entry *userv1.SpaceAccountCaptureEntry, known *uuid.UUID) (*uuid.UUID, bool) {
	if snapshot.version != 50 && snapshot.version != 51 {
		return known, true
	}
	qualified := snapshot.qualified[hex.EncodeToString(deterministicBanBytes(entry))]
	if !qualified.found {
		return known, true
	}
	if qualified.duplicate || qualified.account == uuid.Nil || (known != nil && *known != qualified.account) {
		return nil, false
	}
	return &qualified.account, true
}

func ordinaryAuthorityFromRow(ctx context.Context, tx pgx.Tx, space, profile uuid.UUID, row ordinaryAuthorityRow, snapshot *MemberAuthoritySnapshot) (*spacev1.MemberAdmissionOrdinaryAuthority, error) {
	r := &spacev1.MemberAdmissionOrdinaryAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE}
	operation := row.operation
	state := row.state
	receipt := row.receipt
	binding := row.binding
	native := row.native
	kind := row.kind
	account := row.account
	if !row.found {
		if snapshot == nil {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space, profile).Scan(&exists); err != nil || exists {
				return nil, ErrMemberAuthorityUnavailable
			}
		} else if snapshot.ordinaryPresent[profile] {
			return nil, ErrMemberAuthorityUnavailable
		}
		return r, nil
	}
	r.HeadGeneration = row.generation
	r.RetiredThrough = row.retiredThrough
	r.ExplicitLegacy = row.legacy
	if operation == uuid.Nil || r.HeadGeneration > math.MaxInt64 || r.RetiredThrough > r.HeadGeneration {
		return nil, ErrMemberAuthorityUnavailable
	}
	if state == "ACTIVE" || state == "PREPARING" {
		entry := ordinaryCaptureEntry(profile, operation, r.HeadGeneration)
		var qualified *uuid.UUID
		var qualifiedErr error
		var ok bool
		if snapshot == nil {
			qualified, qualifiedErr = readQualifiedSourceAccount(ctx, tx, space, entry, account)
		} else {
			qualified, ok = qualifiedAuthorityFromSnapshot(snapshot, entry, account)
			if !ok {
				qualifiedErr = ErrMemberAuthorityUnavailable
			}
		}
		if qualifiedErr != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		account = qualified
	}
	if account != nil {
		var banned bool
		if snapshot == nil {
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bans WHERE space_id=$1 AND account_id=$2)`, space, *account).Scan(&banned); err != nil {
				return nil, err
			}
		} else {
			banned = snapshot.banned[*account]
		}
		if banned {
			r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
			return r, nil
		}
	}
	switch state {
	case "RETIRED":
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
		return r, nil
	case "PREPARING":
		if r.HeadGeneration == 0 || r.ExplicitLegacy || len(binding) != 32 || len(receipt) != 0 || len(native) != 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING
		r.PreparingBinding = ordinaryMembershipBinding(r.HeadGeneration, operation, false)
		r.PreparingAdmissionBindingSha256 = binding
		return r, nil
	case "ACTIVE":
		var published bool
		if snapshot == nil {
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2 AND membership_generation=$3 AND admission_operation_id=$4)`, space, profile, r.HeadGeneration, operation).Scan(&published); err != nil || !published {
				return nil, ErrMemberAuthorityUnavailable
			}
		} else {
			published = snapshot.ordinaryPublished[ordinaryPublishedKey(profile, operation, r.HeadGeneration)]
			if !published {
				return nil, ErrMemberAuthorityUnavailable
			}
		}
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE
		r.PublicationKind = spacev1.MemberAdmissionPublicationKind(kind)
		r.PublishedBinding = ordinaryMembershipBinding(r.HeadGeneration, operation, r.ExplicitLegacy)
		if r.ExplicitLegacy {
			if r.HeadGeneration != 0 || kind != 2 || len(receipt) != 0 || len(native) != 0 {
				return nil, ErrMemberAuthorityUnavailable
			}
			return r, nil
		}
		if r.HeadGeneration == 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		if kind == 1 {
			if len(receipt) != 32 || len(native) != 0 || len(binding) != 32 {
				return nil, ErrMemberAuthorityUnavailable
			}
			var role uuid.UUID
			var saved []byte
			var savedState string
			if snapshot == nil {
				if err := tx.QueryRow(ctx, `SELECT role_id,role_receipt_sha256,state FROM space_member_admission_intents WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3 AND generation=$4 AND binding_sha256=$5`, operation, space, profile, r.HeadGeneration, binding).Scan(&role, &saved, &savedState); err != nil || savedState != "PUBLISHED" || !bytes.Equal(saved, receipt) {
					return nil, ErrMemberAuthorityUnavailable
				}
			} else {
				rows := snapshot.intents[intentKey(operation, profile, r.HeadGeneration, binding)]
				if len(rows) != 1 {
					return nil, ErrMemberAuthorityUnavailable
				}
				savedState = rows[0].state
				saved = rows[0].saved
				role = rows[0].role
				if savedState != "PUBLISHED" || !bytes.Equal(saved, receipt) {
					return nil, ErrMemberAuthorityUnavailable
				}
			}
			expected, err := memberadmission.EnsureReceiptFingerprint(memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: space.String(), ProfileID: profile.String(), AdmissionOperationID: operation.String(), MembershipGeneration: r.HeadGeneration, AdmissionBindingSHA256: binding}, role.String())
			if err != nil || !bytes.Equal(expected[:], receipt) {
				return nil, ErrMemberAuthorityUnavailable
			}
			r.RoleReceiptSha256 = receipt
			return r, nil
		}
		if kind < 3 || kind > 6 || len(receipt) != 0 || len(native) != 32 {
			return nil, ErrMemberAuthorityUnavailable
		}
		var raw, digest []byte
		var nativeState string
		var nativePublishedAt time.Time
		if snapshot == nil {
			if err := tx.QueryRow(ctx, `SELECT binding_bytes,binding_sha256,state,published_at FROM space_native_member_publications WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3 AND generation=$4 AND kind=$5`, operation, space, profile, r.HeadGeneration, kind).Scan(&raw, &digest, &nativeState, &nativePublishedAt); err != nil || nativeState != "PUBLISHED" || !bytes.Equal(digest, native) {
				return nil, ErrMemberAuthorityUnavailable
			}
		} else {
			rows := snapshot.nativePubs[nativePubKey(operation, profile, r.HeadGeneration, kind)]
			if len(rows) != 1 {
				return nil, ErrMemberAuthorityUnavailable
			}
			raw = rows[0].raw
			digest = rows[0].digest
			nativeState = rows[0].state
			nativePublishedAt = rows[0].publishedAt
			if nativeState != "PUBLISHED" || !bytes.Equal(digest, native) {
				return nil, ErrMemberAuthorityUnavailable
			}
		}
		message := &spacev1.NativeMembershipPublicationBinding{}
		if err := proto.Unmarshal(raw, message); err != nil || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
		if err != nil || !bytes.Equal(canonical, raw) {
			return nil, ErrMemberAuthorityUnavailable
		}
		expected := sha256.Sum256(append([]byte(nativeMemberPublicationDomain), raw...))
		if !bytes.Equal(expected[:], native) || message.GetProtocolVersion() != 1 || message.GetSpaceId() != space.String() || message.GetProfileId() != profile.String() || message.GetPublicationOperationId() != operation.String() || message.GetOrdinaryGeneration() != r.HeadGeneration || int32(message.GetKind()) != kind {
			return nil, ErrMemberAuthorityUnavailable
		}
		switch kind {
		case 3:
			if err := validateCreatorNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		case 4:
			if err := validateCommunityBootstrapNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		case 5:
			if err := validateCommunityRecoveryNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		case 6:
			if err := validateBotNativeProof(ctx, tx, message); err != nil {
				return nil, err
			}
		default:
			return nil, ErrMemberAuthorityUnavailable
		}
		if !message.PublishedAt.AsTime().Equal(nativePublishedAt) {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: r.PublicationKind, OperationId: operation.String(), BindingSha256: native}
		return r, nil
	default:
		return nil, ErrMemberAuthorityUnavailable
	}
}

func communityAuthorityFromRow(ctx context.Context, tx pgx.Tx, space, profile uuid.UUID, sampled time.Time, row communityAuthorityRow, snapshot *MemberAuthoritySnapshot) (*spacev1.MemberAdmissionCommunityAuthority, error) {
	r := &spacev1.MemberAdmissionCommunityAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE}
	generation := row.generation
	owner := row.ownerGeneration
	episode := row.episode
	state := row.state
	if !row.found {
		if snapshot == nil {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_roster_members m JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation WHERE m.space_id=$1 AND m.profile_id=$2 AND a.status='active' AND m.revoked_at IS NULL AND m.lease_expires_at>$3 AND a.roster_lease_expires_at>$3)`, space, profile, sampled).Scan(&exists); err != nil || exists {
				return nil, ErrMemberAuthorityUnavailable
			}
		} else if snapshot.rosterExists[profile] {
			return nil, ErrMemberAuthorityUnavailable
		}
		return r, nil
	}
	r.RetiredThrough = row.retiredThrough
	if generation == 0 || generation > math.MaxInt64 || owner == 0 || owner > math.MaxInt64 || r.RetiredThrough > generation || episode == uuid.Nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if state == "RETIRED" {
		r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
		return r, nil
	}
	if state != "ACTIVE" {
		return nil, ErrMemberAuthorityUnavailable
	}
	entry := communityCaptureEntry(profile, episode, owner, generation)
	var account *uuid.UUID
	var qualifiedErr error
	if snapshot == nil {
		account, qualifiedErr = readQualifiedSourceAccount(ctx, tx, space, entry, nil)
	} else {
		a2, ok := qualifiedAuthorityFromSnapshot(snapshot, entry, nil)
		if !ok {
			qualifiedErr = ErrMemberAuthorityUnavailable
		}
		account = a2
	}
	if qualifiedErr != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if account != nil {
		var banned bool
		if snapshot == nil {
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bans WHERE space_id=$1 AND account_id=$2)`, space, *account).Scan(&banned); err != nil {
				return nil, ErrMemberAuthorityUnavailable
			}
		} else {
			banned = snapshot.banned[*account]
		}
		if banned {
			r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
			return r, nil
		}
	}
	var remaining time.Duration
	if snapshot == nil {
		err := tx.QueryRow(ctx, `SELECT m.source_revision,(EXTRACT(EPOCH FROM (LEAST(m.lease_expires_at,a.roster_lease_expires_at)-$4::timestamptz))*1000000000)::bigint
	 FROM community_roster_members m JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation
	 WHERE m.space_id=$1 AND m.profile_id=$2 AND m.owner_generation=$3 AND a.status='active' AND m.revoked_at IS NULL
	 AND m.lease_expires_at>$4 AND a.roster_lease_expires_at>$4`, space, profile, owner, sampled).Scan(&r.RosterSourceRevision, &remaining)
		if errors.Is(err, pgx.ErrNoRows) {
			r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
			return r, nil
		}
		if err != nil || remaining <= 0 || r.RosterSourceRevision == 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
	} else {
		rr := snapshot.rosterRemaining[rosterKey(profile, owner)]
		if !rr.found {
			r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_RETIRED
			return r, nil
		}
		if rr.duplicate || rr.remaining <= 0 || rr.sourceRevision == 0 {
			return nil, ErrMemberAuthorityUnavailable
		}
		r.RosterSourceRevision = rr.sourceRevision
		remaining = rr.remaining
	}
	r.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE
	r.Binding = &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Community{Community: &rolev1.CommunityMembershipBinding{OwnerGeneration: owner, MembershipGeneration: generation, MembershipEpisodeId: episode.String()}}}
	r.ValidityRemaining = durationpb.New(remaining)
	return r, nil
}

func newMemberAuthoritySnapshot(ctx context.Context, tx pgx.Tx, space uuid.UUID, profiles []uuid.UUID, sampled time.Time) (*MemberAuthoritySnapshot, error) {
	snapshot := &MemberAuthoritySnapshot{
		ordinary:          map[uuid.UUID]ordinaryAuthorityRow{},
		ordinaryPresent:   map[uuid.UUID]bool{},
		ordinaryPublished: map[string]bool{},
		qualified:         map[string]qualifiedAuthorityRow{},
		banned:            map[uuid.UUID]bool{},
		community:         map[uuid.UUID]communityAuthorityRow{},
		rosterExists:      map[uuid.UUID]bool{},
		rosterRemaining:   map[string]rosterRemainingRow{},
		intents:           map[string][]intentAuthorityRow{},
		nativePubs:        map[string][]nativePublicationRow{},
	}
	var ids []uuid.UUID
	for _, profile := range profiles {
		if profile != uuid.Nil {
			ids = append(ids, profile)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	var unique []uuid.UUID
	for _, profile := range ids {
		if len(unique) > 0 && unique[len(unique)-1] == profile {
			continue
		}
		unique = append(unique, profile)
	}
	ids = unique
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT profile_id,generation,retired_through,publication_id,state,legacy,role_receipt_sha256,admission_binding_sha256,publication_kind,native_publication_sha256,account_id FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id = ANY($2)`, space, ids)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var profile uuid.UUID
			var generation, retiredThrough uint64
			var operation uuid.UUID
			var state string
			var legacy bool
			var receipt, binding, native []byte
			var kind int32
			var account *uuid.UUID
			if err = rows.Scan(&profile, &generation, &retiredThrough, &operation, &state, &legacy, &receipt, &binding, &kind, &native, &account); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			if snapshot.ordinary[profile].found {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			snapshot.ordinary[profile] = ordinaryAuthorityRow{found: true, generation: generation, retiredThrough: retiredThrough, operation: operation, state: state, legacy: legacy, receipt: receipt, binding: binding, native: native, kind: kind, account: account}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT profile_id,generation,retired_through,owner_generation,episode_id,state FROM space_community_member_heads WHERE space_id=$1 AND profile_id = ANY($2)`, space, ids)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var profile uuid.UUID
			var generation, retiredThrough, owner uint64
			var episode uuid.UUID
			var state string
			if err = rows.Scan(&profile, &generation, &retiredThrough, &owner, &episode, &state); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			if snapshot.community[profile].found {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			snapshot.community[profile] = communityAuthorityRow{found: true, generation: generation, retiredThrough: retiredThrough, ownerGeneration: owner, episode: episode, state: state}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	if err := tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&snapshot.version); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT profile_id FROM space_members WHERE space_id=$1 AND profile_id = ANY($2)`, space, ids)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var profile uuid.UUID
			if err = rows.Scan(&profile); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			snapshot.ordinaryPresent[profile] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT profile_id FROM community_roster_members m JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation WHERE m.space_id=$1 AND m.profile_id = ANY($2) AND a.status='active' AND m.revoked_at IS NULL AND m.lease_expires_at>$3 AND a.roster_lease_expires_at>$3`, space, ids, sampled)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var profile uuid.UUID
			if err = rows.Scan(&profile); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			snapshot.rosterExists[profile] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	var activeProfiles []uuid.UUID
	for profile, row := range snapshot.ordinary {
		if row.state == "ACTIVE" {
			activeProfiles = append(activeProfiles, profile)
		}
	}
	if len(activeProfiles) > 0 {
		rows, err := tx.Query(ctx, `SELECT profile_id,membership_generation,admission_operation_id FROM space_members WHERE space_id=$1 AND profile_id = ANY($2)`, space, activeProfiles)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var profile uuid.UUID
			var generation uint64
			var operation uuid.UUID
			if err = rows.Scan(&profile, &generation, &operation); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			snapshot.ordinaryPublished[ordinaryPublishedKey(profile, operation, generation)] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	var qualificationBytes [][]byte
	for profile, row := range snapshot.ordinary {
		if row.state == "ACTIVE" || row.state == "PREPARING" {
			qualificationBytes = append(qualificationBytes, deterministicBanBytes(ordinaryCaptureEntry(profile, row.operation, row.generation)))
		}
	}
	for profile, row := range snapshot.community {
		if row.state == "ACTIVE" {
			qualificationBytes = append(qualificationBytes, deterministicBanBytes(communityCaptureEntry(profile, row.episode, row.ownerGeneration, row.generation)))
		}
	}
	if snapshot.version == 50 || snapshot.version == 51 {
		if len(qualificationBytes) > 0 {
			rows, err := tx.Query(ctx, `SELECT source_bytes,account_id FROM space_account_qualifications WHERE space_id=$1 AND source_bytes = ANY($2)`, space, qualificationBytes)
			if err != nil {
				return nil, ErrMemberAuthorityUnavailable
			}
			for rows.Next() {
				var source []byte
				var account uuid.UUID
				if err = rows.Scan(&source, &account); err != nil {
					rows.Close()
					return nil, ErrMemberAuthorityUnavailable
				}
				key := hex.EncodeToString(source)
				existing := snapshot.qualified[key]
				if existing.found {
					existing.duplicate = true
				} else {
					existing = qualifiedAuthorityRow{found: true, account: account}
				}
				snapshot.qualified[key] = existing
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, ErrMemberAuthorityUnavailable
			}
		}
	}
	var candidates []uuid.UUID
	for _, row := range snapshot.ordinary {
		if row.account != nil && *row.account != uuid.Nil {
			candidates = append(candidates, *row.account)
		}
	}
	for profile, row := range snapshot.ordinary {
		if row.state == "ACTIVE" || row.state == "PREPARING" {
			qualified := snapshot.qualified[hex.EncodeToString(deterministicBanBytes(ordinaryCaptureEntry(profile, row.operation, row.generation)))]
			if qualified.found && qualified.account != uuid.Nil {
				candidates = append(candidates, qualified.account)
			}
		}
	}
	for profile, row := range snapshot.community {
		if row.state == "ACTIVE" {
			qualified := snapshot.qualified[hex.EncodeToString(deterministicBanBytes(communityCaptureEntry(profile, row.episode, row.ownerGeneration, row.generation)))]
			if qualified.found && qualified.account != uuid.Nil {
				candidates = append(candidates, qualified.account)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].String() < candidates[j].String() })
	var uniqueCandidates []uuid.UUID
	for _, candidate := range candidates {
		if candidate != uuid.Nil && (len(uniqueCandidates) == 0 || uniqueCandidates[len(uniqueCandidates)-1] != candidate) {
			uniqueCandidates = append(uniqueCandidates, candidate)
		}
	}
	if len(uniqueCandidates) > 0 {
		rows, err := tx.Query(ctx, `SELECT account_id FROM space_bans WHERE space_id=$1 AND account_id = ANY($2)`, space, uniqueCandidates)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var account uuid.UUID
			if err = rows.Scan(&account); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			snapshot.banned[account] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	var intentProfiles []uuid.UUID
	var intentOperations []uuid.UUID
	for profile, row := range snapshot.ordinary {
		if row.state == "ACTIVE" && row.kind == 1 {
			intentProfiles = append(intentProfiles, profile)
			intentOperations = append(intentOperations, row.operation)
		}
	}
	if len(intentProfiles) > 0 {
		// The intents relation stores the admission generation as generation,
		// matching the single witness read filter generation=$4.
		rows, err := tx.Query(ctx, `SELECT operation_id,profile_id,generation,binding_sha256,role_id,role_receipt_sha256,state FROM space_member_admission_intents WHERE space_id=$1 AND profile_id = ANY($2) AND operation_id = ANY($3)`, space, intentProfiles, intentOperations)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var operation uuid.UUID
			var profile uuid.UUID
			var generation uint64
			var binding []byte
			var role uuid.UUID
			var saved []byte
			var state string
			if err = rows.Scan(&operation, &profile, &generation, &binding, &role, &saved, &state); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			key := intentKey(operation, profile, generation, binding)
			snapshot.intents[key] = append(snapshot.intents[key], intentAuthorityRow{role: role, saved: saved, state: state})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	var nativeProfiles []uuid.UUID
	var nativeOperations []uuid.UUID
	for profile, row := range snapshot.ordinary {
		if row.state == "ACTIVE" && row.kind >= 3 && row.kind <= 6 {
			nativeProfiles = append(nativeProfiles, profile)
			nativeOperations = append(nativeOperations, row.operation)
		}
	}
	if len(nativeProfiles) > 0 {
		rows, err := tx.Query(ctx, `SELECT operation_id,profile_id,generation,kind,binding_bytes,binding_sha256,state,published_at FROM space_native_member_publications WHERE space_id=$1 AND profile_id = ANY($2) AND operation_id = ANY($3)`, space, nativeProfiles, nativeOperations)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var operation uuid.UUID
			var profile uuid.UUID
			var generation uint64
			var kind int32
			var raw []byte
			var digest []byte
			var state string
			var publishedAt time.Time
			if err = rows.Scan(&operation, &profile, &generation, &kind, &raw, &digest, &state, &publishedAt); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			key := nativePubKey(operation, profile, generation, kind)
			snapshot.nativePubs[key] = append(snapshot.nativePubs[key], nativePublicationRow{raw: raw, digest: digest, state: state, publishedAt: publishedAt})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	var rosterProfiles []uuid.UUID
	for profile, row := range snapshot.community {
		if row.state == "ACTIVE" {
			rosterProfiles = append(rosterProfiles, profile)
		}
	}
	if len(rosterProfiles) > 0 {
		rows, err := tx.Query(ctx, `SELECT m.profile_id,m.owner_generation,m.source_revision,(EXTRACT(EPOCH FROM (LEAST(m.lease_expires_at,a.roster_lease_expires_at)-$3::timestamptz))*1000000000)::bigint FROM community_roster_members m JOIN community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation WHERE m.space_id=$1 AND m.profile_id = ANY($2) AND a.status='active' AND m.revoked_at IS NULL AND m.lease_expires_at>$3 AND a.roster_lease_expires_at>$3`, space, rosterProfiles, sampled)
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		for rows.Next() {
			var profile uuid.UUID
			var owner uint64
			var sourceRevision uint64
			var remaining int64
			if err = rows.Scan(&profile, &owner, &sourceRevision, &remaining); err != nil {
				rows.Close()
				return nil, ErrMemberAuthorityUnavailable
			}
			key := rosterKey(profile, owner)
			existing := snapshot.rosterRemaining[key]
			if existing.found {
				existing.duplicate = true
			} else {
				existing = rosterRemainingRow{found: true, sourceRevision: sourceRevision, remaining: time.Duration(remaining)}
			}
			snapshot.rosterRemaining[key] = existing
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	return snapshot, nil
}
