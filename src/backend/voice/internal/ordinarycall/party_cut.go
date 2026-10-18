package ordinarycall

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NativePartyCut is only the ordinary component of an A3 native cut. A nil Room
// says nothing about other sources or global absence. Witness includes the full
// native document and exact materialized identity ownership evidence.
type NativePartyCut struct {
	Room    *Room
	Witness []byte
}

// ReadPartyCut uses the caller's existing read-only repeatable-read transaction.
// It never takes locks, opens another transaction, or performs network IO.
func ReadPartyCut(ctx context.Context, tx pgx.Tx, account, profile uuid.UUID) (NativePartyCut, error) {
	if tx == nil || account == uuid.Nil || profile == uuid.Nil {
		return NativePartyCut{}, ErrUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT r.room_id FROM voice_ordinary_rooms r
WHERE r.state<>'CLOSED' AND (
EXISTS (SELECT 1 FROM voice_ordinary_memberships m WHERE m.room_id=r.room_id
 AND (m.account_id=$1 OR m.profile_id=$2) AND m.state IN ('PENDING','RESERVED','JOINED','LEAVING'))
OR EXISTS (SELECT 1 FROM jsonb_each(r.document->'members') member
 WHERE (member.value->>'account_id'=$1::text OR member.value->>'profile_id'=$2::text)
 AND member.value->>'state' IN ('PENDING','RESERVED','JOINED','LEAVING')))
ORDER BY r.room_id LIMIT 2`, account, profile)
	if err != nil {
		return NativePartyCut{}, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return NativePartyCut{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return NativePartyCut{}, err
	}
	if len(ids) == 0 {
		return NativePartyCut{Witness: []byte(`{"ordinary_rooms":[]}`)}, nil
	}
	if len(ids) != 1 {
		return NativePartyCut{}, ErrUnavailable
	}
	var body []byte
	var operation uuid.UUID
	var version, roster uint64
	var state string
	if err := tx.QueryRow(ctx, `SELECT operation_id,version,roster_version,state,document FROM voice_ordinary_rooms WHERE room_id=$1`, ids[0]).Scan(&operation, &version, &roster, &state, &body); err != nil {
		return NativePartyCut{}, err
	}
	room, err := decodeRoom(body, ids[0])
	if err != nil || room.Intent.OperationID != operation || room.Version != version || room.RosterVersion != roster || string(room.Stage) != state {
		return NativePartyCut{}, ErrUnavailable
	}
	rows, err = tx.Query(ctx, `SELECT m.profile_id,m.account_id,m.membership_generation,m.fence_operation_id,m.media_identity,m.session_epoch,m.state,
EXISTS(SELECT 1 FROM voice_account_voice_fences f WHERE f.account_id=m.account_id AND f.profile_id=m.profile_id
 AND f.room_id=m.room_id::text AND f.admission_operation_id=m.fence_operation_id
 AND f.admission_generation=m.membership_generation::text AND f.state='active' AND f.reservation_expires_at IS NULL),
EXISTS(SELECT 1 FROM voice_profile_account_mappings p WHERE p.profile_id=m.profile_id AND p.account_id=m.account_id)
FROM voice_ordinary_memberships m WHERE m.room_id=$1 ORDER BY m.profile_id`, ids[0])
	if err != nil {
		return NativePartyCut{}, err
	}
	type memberWitness struct {
		Member             Member
		OwnsFence, Mapping bool
	}
	witness := struct {
		Room    json.RawMessage
		Members []memberWitness
	}{Room: body, Members: []memberWitness{}}
	for rows.Next() {
		var member Member
		var ownsFence, mapping bool
		if err := rows.Scan(&member.ProfileID, &member.AccountID, &member.Generation, &member.FenceOperationID, &member.MediaIdentity, &member.SessionEpoch, &member.State, &ownsFence, &mapping); err != nil {
			rows.Close()
			return NativePartyCut{}, err
		}
		document, ok := room.Members[member.ProfileID.String()]
		if !ok || document.AccountID != member.AccountID || document.Generation != member.Generation || document.FenceOperationID != member.FenceOperationID || document.MediaIdentity != member.MediaIdentity || document.SessionEpoch != member.SessionEpoch || document.State != member.State {
			rows.Close()
			return NativePartyCut{}, ErrUnavailable
		}
		if member.State != MemberLeft && member.State != MemberAborted && (!ownsFence || !mapping) {
			rows.Close()
			return NativePartyCut{}, ErrUnavailable
		}
		witness.Members = append(witness.Members, memberWitness{Member: document, OwnsFence: ownsFence, Mapping: mapping})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return NativePartyCut{}, err
	}
	if len(witness.Members) != len(room.Members) {
		return NativePartyCut{}, ErrUnavailable
	}
	encoded, err := json.Marshal(witness)
	if err != nil {
		return NativePartyCut{}, err
	}
	return NativePartyCut{Room: room, Witness: encoded}, nil
}
