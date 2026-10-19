package partysnapshot

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type foreignCut struct {
	Witness    []byte
	Unresolved bool
	Purpose    string
	RoomID     uuid.UUID
}

type foreignEntry struct {
	Kind, Key  string
	Document   json.RawMessage
	Unresolved bool
}

// readForeignCut is an exclusion read, never a legacy membership adoption path.
// Retained work without its qualified owner terminal classification remains
// unavailable; timestamps and an empty current room do not prove retirement.
func readForeignCut(ctx context.Context, tx pgx.Tx, subject Subject, native Candidate) (foreignCut, error) {
	if tx == nil || subject.AccountID == uuid.Nil || subject.ProfileID == uuid.Nil {
		return foreignCut{}, ErrUnavailable
	}
	var readonly, isolation string
	if err := tx.QueryRow(ctx, `SELECT current_setting('transaction_read_only'),current_setting('transaction_isolation')`).Scan(&readonly, &isolation); err != nil || readonly != "on" || isolation != "repeatable read" {
		return foreignCut{}, ErrUnavailable
	}
	accounts := map[uuid.UUID]bool{subject.AccountID: true}
	profiles := map[uuid.UUID]bool{subject.ProfileID: true}
	bindings := map[uuid.UUID]uuid.UUID{subject.ProfileID: subject.AccountID}
	expected := make(map[uuid.UUID][]Member)
	requiredFences := make(map[uuid.UUID]bool)
	for _, m := range native.Members {
		if m.State == Left {
			continue
		}
		if m.AccountID == uuid.Nil || m.ProfileID == uuid.Nil {
			return foreignCut{}, ErrUnavailable
		}
		if prior, exists := bindings[m.ProfileID]; exists && prior != m.AccountID {
			return foreignCut{}, ErrUnavailable
		}
		bindings[m.ProfileID] = m.AccountID
		accounts[m.AccountID], profiles[m.ProfileID] = true, true
		expected[m.AccountID] = append(expected[m.AccountID], m)
		if m.State == Joined || m.State == Reconnecting {
			requiredFences[m.AccountID] = true
		}
	}
	directProfiles := sortedForeignIDs(profiles)
	accountIDs := sortedForeignIDs(accounts)
	out := foreignCut{}
	entries := []foreignEntry{}
	rows, err := tx.Query(ctx, `SELECT profile_id,account_id,to_jsonb(m)::text FROM voice_profile_account_mappings m
 WHERE account_id=ANY($1::uuid[]) OR profile_id=ANY($2::uuid[]) ORDER BY profile_id`, accountIDs, directProfiles)
	if err != nil {
		return out, ErrUnavailable
	}
	mappings := make(map[uuid.UUID]uuid.UUID)
	for rows.Next() {
		var profile, account uuid.UUID
		var raw string
		if err := rows.Scan(&profile, &account, &raw); err != nil {
			rows.Close()
			return out, ErrUnavailable
		}
		mappings[profile] = account
		profiles[profile] = true
		bad := bindings[profile] != uuid.Nil && bindings[profile] != account
		out.Unresolved = out.Unresolved || bad
		entries = append(entries, foreignEntry{"mapping", profile.String(), json.RawMessage(raw), bad})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, ErrUnavailable
	}
	profileIDs := sortedForeignIDs(profiles)
	type ownedMember struct{ profile, account, room uuid.UUID }
	owned := make(map[uuid.UUID]ownedMember)
	known := 0
	rows, err = tx.Query(ctx, `SELECT m.profile_id,m.account_id,m.session_epoch,m.membership_state,m.room_id,
 COALESCE(r.purpose,''),COALESCE(r.state,''),
 (r.purpose='MATCH_SQUAD' AND EXISTS(SELECT 1 FROM voice_match_squad_operations o
 WHERE o.room_id=r.room_id AND o.state='active' AND o.owner_id=r.owner_id AND o.match_id=r.owner_id
 AND o.operation_id=r.creation_operation_id AND o.creation_receipt_id=r.creation_receipt_id
 AND o.participant_manifest_sha256=r.creation_manifest_hash))
 OR (r.purpose='GAME_SESSION' AND EXISTS(SELECT 1 FROM voice_game_session_operations o
 WHERE o.room_id=r.room_id AND o.operation_id=r.creation_operation_id
 AND o.voice_creation_receipt_id=r.creation_receipt_id AND m.profile_id=ANY(o.roster_profile_ids)
 AND o.lease_fenced_at IS NULL AND o.lease_expires_at>clock_timestamp())),
 jsonb_build_object('membership',to_jsonb(m),'room',to_jsonb(r))::text
 FROM voice_room_memberships m LEFT JOIN voice_room_instances r ON r.room_id=m.room_id
 WHERE m.account_id=ANY($1::uuid[]) OR m.profile_id=ANY($2::uuid[]) ORDER BY m.profile_id`, accountIDs, profileIDs)
	if err != nil {
		return out, ErrUnavailable
	}
	for rows.Next() {
		var profile, room uuid.UUID
		var account *uuid.UUID
		var epoch *int64
		var state *string
		var purpose, roomState, raw string
		var ownerReady bool
		if err := rows.Scan(&profile, &account, &epoch, &state, &room, &purpose, &roomState, &ownerReady, &raw); err != nil {
			rows.Close()
			return out, ErrUnavailable
		}
		current := state == nil || (*state != "LEFT" && *state != "EJECTED")
		qualified := current && state != nil && *state == "JOINED" && account != nil && *account != uuid.Nil && epoch != nil && *epoch > 0 && mappings[profile] == *account && roomState == "active" && ownerReady
		if current {
			if qualified && (purpose == "MATCH_SQUAD" || purpose == "GAME_SESSION") {
				known++
				out.Purpose, out.RoomID = purpose, room
				if purpose == "GAME_SESSION" {
					out.Purpose = "GAME"
				}
				owned[*account] = ownedMember{profile, *account, room}
			} else {
				out.Unresolved = true
			}
		}
		entries = append(entries, foreignEntry{"membership", profile.String(), json.RawMessage(raw), current && !qualified})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, ErrUnavailable
	}
	if known > 1 || (known > 0 && native.Disposition != Absent) {
		out.Unresolved = true
	}
	seenFences := make(map[uuid.UUID]bool)
	rows, err = tx.Query(ctx, `SELECT account_id,profile_id,room_id,state,reservation_expires_at,admission_operation_id,admission_generation,to_jsonb(f)::text
 FROM voice_account_voice_fences f WHERE account_id=ANY($1::uuid[]) OR profile_id=ANY($2::uuid[]) ORDER BY account_id`, accountIDs, profileIDs)
	if err != nil {
		return out, ErrUnavailable
	}
	for rows.Next() {
		var account, profile uuid.UUID
		var room, state, raw string
		var expiry *time.Time
		var operation *uuid.UUID
		var generation *string
		if err := rows.Scan(&account, &profile, &room, &state, &expiry, &operation, &generation, &raw); err != nil {
			rows.Close()
			return out, ErrUnavailable
		}
		valid := false
		if members, ok := expected[account]; ok {
			for _, m := range members {
				exact := profile == m.ProfileID && room == native.RoomID.String() && operation != nil && *operation == m.OperationID && generation != nil && *generation == m.Generation
				active := state == "active" && expiry == nil
				reserved := m.State == Joining && state == "reserving" && expiry != nil
				valid = valid || (exact && (active || reserved))
			}
		} else if m, ok := owned[account]; ok {
			valid = profile == m.profile && room == m.room.String() && operation == nil && generation == nil && state == "active" && expiry == nil
		}
		valid = valid && mappings[profile] == account
		seenFences[account] = true
		out.Unresolved = out.Unresolved || !valid
		entries = append(entries, foreignEntry{"fence", account.String(), json.RawMessage(raw), !valid})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, ErrUnavailable
	}
	for account := range requiredFences {
		if !seenFences[account] {
			out.Unresolved = true
		}
	}
	for account := range owned {
		if !seenFences[account] {
			out.Unresolved = true
		}
	}
	for _, statement := range foreignStatements {
		args := []any{accountIDs, profileIDs}
		if statement.kind == "conversion" {
			args = append(args, directProfiles)
		}
		rows, err := tx.Query(ctx, statement.query, args...)
		if err != nil {
			return out, ErrUnavailable
		}
		for rows.Next() {
			var key, raw string
			var blocked bool
			if err := rows.Scan(&key, &raw, &blocked); err != nil {
				rows.Close()
				return out, ErrUnavailable
			}
			out.Unresolved = out.Unresolved || blocked
			entries = append(entries, foreignEntry{statement.kind, key, json.RawMessage(raw), blocked})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, ErrUnavailable
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Key < entries[j].Key
	})
	out.Witness, err = json.Marshal(entries)
	if err != nil {
		return foreignCut{}, ErrUnavailable
	}
	return out, nil
}

func sortedForeignIDs(ids map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sortIDs(out)
	return out
}

var foreignStatements = []struct{ kind, query string }{
	{"legacy_operation", `SELECT actor_profile_id::text||':'||operation_id::text,to_jsonb(o)::text,
 state<>'completed' OR EXISTS(SELECT 1 FROM voice_lifecycle_effects e WHERE e.actor_profile_id=o.actor_profile_id AND e.operation_id=o.operation_id AND e.state<>'applied')
 FROM voice_lifecycle_operations o WHERE actor_account_id=ANY($1::uuid[]) OR actor_profile_id=ANY($2::uuid[]) OR subject_profile_id=ANY($2::uuid[]) ORDER BY actor_profile_id,operation_id`},
	{"legacy_effect", `SELECT e.effect_id::text,to_jsonb(e)::text,e.state<>'applied'
 FROM voice_lifecycle_effects e JOIN voice_lifecycle_operations o USING(actor_profile_id,operation_id)
 WHERE o.actor_account_id=ANY($1::uuid[]) OR o.actor_profile_id=ANY($2::uuid[]) OR o.subject_profile_id=ANY($2::uuid[]) OR e.target_profile_id=ANY($2::uuid[]) ORDER BY e.effect_id`},
	{"owned_member_operation", `SELECT operation_id::text,to_jsonb(o)::text,state='pending'
 FROM voice_match_squad_member_operations o WHERE account_id=ANY($1::uuid[]) OR profile_id=ANY($2::uuid[]) ORDER BY operation_id`},
	{"owned_member_effect", `SELECT e.operation_id::text||':'||e.effect_kind,to_jsonb(e)::text,
 e.state<>'confirmed' OR e.profile_id<>o.profile_id OR e.room_id<>o.room_id OR e.media_epoch<>o.media_epoch
 FROM voice_match_squad_member_effects e JOIN voice_match_squad_member_operations o USING(operation_id)
 WHERE o.account_id=ANY($1::uuid[]) OR e.profile_id=ANY($2::uuid[]) OR o.profile_id=ANY($2::uuid[]) ORDER BY e.operation_id,effect_kind`},
	{"owned_grant", `SELECT request_id::text,to_jsonb(g)::text,NOT EXISTS(SELECT 1 FROM voice_room_memberships m
 WHERE m.profile_id=g.profile_id AND m.account_id=g.account_id AND m.room_id=g.room_id AND m.media_epoch=g.media_epoch)
 FROM voice_match_squad_member_grants g WHERE account_id=ANY($1::uuid[]) OR profile_id=ANY($2::uuid[]) ORDER BY request_id`},
	{"conversion", `SELECT operation_id::text,to_jsonb(c)::text,
 (state IN ('pending','fenced') AND (source_profile_id=ANY($3::uuid[]) OR target_profile_id=ANY($3::uuid[]) OR target_account_id=ANY($1::uuid[])))
 OR (state='activated' AND source_profile_id=ANY($3::uuid[]))
 FROM voice_sdk_conversion_fence_receipts c WHERE source_account_id=ANY($1::uuid[]) OR target_account_id=ANY($1::uuid[]) OR source_profile_id=ANY($2::uuid[]) OR target_profile_id=ANY($2::uuid[]) ORDER BY operation_id`},
}
