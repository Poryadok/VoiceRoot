package spacemedia

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PartyRoomHead is the native Space room incarnation, not a coverage certificate.
type PartyRoomHead struct {
	VoiceRoomID, SpaceID, RoomID, CreatorOperationID uuid.UUID
	RoomGeneration, RosterVersion                    uint64
	State                                            string
}

type PartyMember struct {
	OperationID, AccountID, ProfileID uuid.UUID
	Generation                        string
	SessionEpoch                      int64
	State, AdmissionState             string
	ProjectionApplied                 bool
}

// NativePartyCut is only the Space component. Absence proves nothing about
// ordinary, foreign or legacy sources. Witness retains complete native rows.
type NativePartyCut struct {
	Head    *PartyRoomHead
	Members []PartyMember
	Witness []byte
}

// ReadPartyCut must use the caller's existing READ ONLY REPEATABLE READ tx.
func ReadPartyCut(ctx context.Context, tx pgx.Tx, account, profile uuid.UUID) (NativePartyCut, error) {
	if tx == nil || account == uuid.Nil || profile == uuid.Nil {
		return NativePartyCut{}, ErrAdmissionUnavailable
	}
	var readonly, isolation string
	if err := tx.QueryRow(ctx, `SELECT current_setting('transaction_read_only'),current_setting('transaction_isolation')`).Scan(&readonly, &isolation); err != nil || readonly != "on" || isolation != "repeatable read" {
		return NativePartyCut{}, ErrAdmissionUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT voice_room_id,room_generation FROM voice_space_media_admissions
WHERE (account_id=$1 OR profile_id=$2) AND (
 participant_state<>'ENDED' OR (state IN ('PREPARED','FENCED','ABORTING') AND cleanup_completed_at IS NULL)
 OR (state='COMMITTED' AND projection_applied_at IS NULL))
ORDER BY voice_room_id,room_generation LIMIT 2`, account, profile)
	if err != nil {
		return NativePartyCut{}, err
	}
	type locator struct {
		room       uuid.UUID
		generation int64
	}
	var locators []locator
	for rows.Next() {
		var l locator
		if err := rows.Scan(&l.room, &l.generation); err != nil {
			rows.Close()
			return NativePartyCut{}, err
		}
		locators = append(locators, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return NativePartyCut{}, err
	}
	if len(locators) == 0 {
		return NativePartyCut{Witness: []byte(`{"space_rooms":[]}`)}, nil
	}
	if len(locators) != 1 {
		return NativePartyCut{}, ErrAdmissionUnavailable
	}
	l := locators[0]
	var head PartyRoomHead
	var roomText, headJSON string
	var generation, version int64
	err = tx.QueryRow(ctx, `SELECT voice_room_id,space_id,call_room_id,creator_operation_id,room_generation,roster_version,state,to_jsonb(h)::text
FROM voice_space_media_room_heads h WHERE voice_room_id=$1 AND room_generation=$2`, l.room, l.generation).Scan(&head.VoiceRoomID, &head.SpaceID, &roomText, &head.CreatorOperationID, &generation, &version, &head.State, &headJSON)
	if err != nil || generation <= 0 || version < 0 {
		return NativePartyCut{}, ErrAdmissionUnavailable
	}
	if head.State != "OPEN" {
		return NativePartyCut{}, ErrRoomHeadPending
	}
	head.RoomID, err = uuid.Parse(roomText)
	if err != nil || head.RoomID == uuid.Nil || head.RoomID.String() != roomText {
		return NativePartyCut{}, ErrAdmissionUnavailable
	}
	head.RoomGeneration = uint64(generation)
	head.RosterVersion = uint64(version)
	var creatorFinalized bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_space_media_admissions
WHERE operation_id=$1 AND voice_room_id=$2 AND room_generation=$3 AND room_id=$4 AND space_id=$5
 AND created_room AND state='COMMITTED' AND projection_applied_at IS NOT NULL)`, head.CreatorOperationID, head.VoiceRoomID, generation, roomText, head.SpaceID).Scan(&creatorFinalized)
	if err != nil {
		return NativePartyCut{}, err
	}
	// Creator membership may be ENDED; the finalized creation witness is retained.
	if !creatorFinalized {
		return NativePartyCut{}, ErrRoomHeadPending
	}
	if version == 0 {
		return NativePartyCut{}, ErrAdmissionUnavailable
	}
	rows, err = tx.Query(ctx, `SELECT a.operation_id,a.generation,a.account_id,a.profile_id,a.session_epoch,a.participant_state,a.state,
a.projection_applied_at IS NOT NULL,to_jsonb(a)::text,
EXISTS(SELECT 1 FROM voice_account_voice_fences f WHERE f.account_id=a.account_id AND f.profile_id=a.profile_id
 AND f.room_id=a.room_id AND f.admission_operation_id=a.operation_id AND f.admission_generation=a.generation
 AND f.state='active' AND f.reservation_expires_at IS NULL),
EXISTS(SELECT 1 FROM voice_profile_account_mappings m WHERE m.profile_id=a.profile_id AND m.account_id=a.account_id),
a.room_id=$3 AND a.space_id=$4 AND a.created_room=(a.operation_id=$5)
FROM voice_space_media_admissions a WHERE a.voice_room_id=$1 AND a.room_generation=$2
ORDER BY a.profile_id,a.operation_id`, head.VoiceRoomID, generation, roomText, head.SpaceID, head.CreatorOperationID)
	if err != nil {
		return NativePartyCut{}, err
	}
	type memberWitness struct {
		Admission                      json.RawMessage
		OwnsFence, Mapping, Consistent bool
	}
	witness := struct {
		Head    json.RawMessage
		Members []memberWitness
	}{Head: json.RawMessage(headJSON), Members: []memberWitness{}}
	cut := NativePartyCut{Head: &head, Members: []PartyMember{}}
	activeProfiles, activeAccounts := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for rows.Next() {
		var member PartyMember
		var raw string
		var ownsFence, mapping, consistent bool
		if err := rows.Scan(&member.OperationID, &member.Generation, &member.AccountID, &member.ProfileID, &member.SessionEpoch, &member.State, &member.AdmissionState, &member.ProjectionApplied, &raw, &ownsFence, &mapping, &consistent); err != nil {
			rows.Close()
			return NativePartyCut{}, err
		}
		if !consistent || member.SessionEpoch <= 0 || member.Generation == "" {
			rows.Close()
			return NativePartyCut{}, ErrAdmissionUnavailable
		}
		if member.State == "ACTIVE" {
			if member.AdmissionState != "COMMITTED" || !member.ProjectionApplied || !ownsFence || !mapping || activeProfiles[member.ProfileID] || activeAccounts[member.AccountID] {
				rows.Close()
				return NativePartyCut{}, ErrAdmissionUnavailable
			}
			activeProfiles[member.ProfileID] = true
			activeAccounts[member.AccountID] = true
		}
		cut.Members = append(cut.Members, member)
		witness.Members = append(witness.Members, memberWitness{Admission: json.RawMessage(raw), OwnsFence: ownsFence, Mapping: mapping, Consistent: consistent})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return NativePartyCut{}, err
	}
	cut.Witness, err = json.Marshal(witness)
	if err != nil {
		return NativePartyCut{}, err
	}
	return cut, nil
}
