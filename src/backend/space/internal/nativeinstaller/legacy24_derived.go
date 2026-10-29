package nativeinstaller

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
)

// These private identities belong in protected operation custody, never in
// installer stdout. They capture only source-owned 24 revision consequences.
type legacy24Floor struct {
	Epoch   int64     `json:"epoch"`
	Updated time.Time `json:"updated"`
	Members int64     `json:"members"`
}
type legacy24OldEvent struct {
	Hash string `json:"hash"`
}
type legacy24Derived struct {
	Floors map[string]legacy24Floor    `json:"floors"`
	Events map[string]legacy24OldEvent `json:"events"`
}

func captureLegacy24DerivedTx(ctx context.Context, tx pgx.Tx) (*legacy24Derived, error) {
	const limit = 1000000
	floors := make(map[string]legacy24Floor)
	rows, err := tx.Query(ctx, `SELECT e.space_id::text,e.access_epoch,e.updated_at,(SELECT count(*) FROM public.space_members m WHERE m.space_id=e.space_id) FROM public.space_voice_access_epochs e ORDER BY e.space_id LIMIT 1000001`)
	if err != nil {
		return nil, errors.New("legacy24 revision source unavailable")
	}
	for rows.Next() {
		var id string
		var item legacy24Floor
		if rows.Scan(&id, &item.Epoch, &item.Updated, &item.Members) != nil || id == "" || item.Epoch < 1 || item.Members < 0 || len(floors) >= limit {
			rows.Close()
			return nil, errors.New("legacy24 revision source unavailable")
		}
		if _, exists := floors[id]; exists {
			rows.Close()
			return nil, errors.New("legacy24 revision source unavailable")
		}
		floors[id] = item
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, errors.New("legacy24 revision source unavailable")
	}
	rows.Close()
	var missing int64
	if tx.QueryRow(ctx, `SELECT count(*) FROM public.space_members m LEFT JOIN public.space_voice_access_epochs e ON e.space_id=m.space_id WHERE e.space_id IS NULL`).Scan(&missing) != nil || missing != 0 {
		return nil, errors.New("legacy24 member revision missing")
	}
	events := make(map[string]legacy24OldEvent)
	rows, err = tx.Query(ctx, `SELECT event_id::text,encode(sha256(convert_to(to_jsonb(o)::text,'UTF8')),'hex') FROM public.space_voice_access_outbox o ORDER BY event_id LIMIT 1000001`)
	if err != nil {
		return nil, errors.New("legacy24 outbox source unavailable")
	}
	for rows.Next() {
		var id, hash string
		if rows.Scan(&id, &hash) != nil || id == "" || len(hash) != 64 || len(events) >= limit {
			rows.Close()
			return nil, errors.New("legacy24 outbox source unavailable")
		}
		if _, exists := events[id]; exists {
			rows.Close()
			return nil, errors.New("legacy24 outbox source unavailable")
		}
		events[id] = legacy24OldEvent{Hash: hash}
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, errors.New("legacy24 outbox source unavailable")
	}
	rows.Close()
	return &legacy24Derived{Floors: floors, Events: events}, nil
}

// Verify the exact M39 effects on original source rows immediately after44.
// This does not qualify source authority, migration provenance or Role rights.
func verifyLegacy24DerivedTx(ctx context.Context, tx pgx.Tx, before map[string]legacy25TableDigest, derived *legacy24Derived) error {
	if derived == nil || derived.Floors == nil || derived.Events == nil {
		return errors.New("legacy24 derived evidence unavailable")
	}
	after, err := captureLegacy25ProjectionTx(ctx, tx, before)
	if err != nil {
		return err
	}
	for table, old := range before {
		if table == "space_voice_access_epochs" || table == "space_voice_access_outbox" {
			continue
		}
		if !reflect.DeepEqual(old, after[table]) {
			return errors.New("legacy24 original rows changed")
		}
	}
	current, err := captureLegacy24DerivedTx(ctx, tx)
	if err != nil || len(current.Floors) != len(derived.Floors) {
		return errors.New("legacy24 revision floors changed")
	}
	expectedNew := int64(0)
	for id, old := range derived.Floors {
		now, ok := current.Floors[id]
		if !ok || now.Members != old.Members || old.Members > int64(^uint64(0)>>1)-old.Epoch || now.Epoch != old.Epoch+old.Members ||
			(old.Members == 0 && !now.Updated.Equal(old.Updated)) || (old.Members > 0 && now.Updated.Before(old.Updated)) {
			return errors.New("legacy24 revision floors changed")
		}
		expectedNew += old.Members
	}
	for id, old := range derived.Events {
		now, ok := current.Events[id]
		if !ok || now != old {
			return errors.New("legacy24 original outbox changed")
		}
	}
	if int64(len(current.Events)-len(derived.Events)) != expectedNew {
		return errors.New("legacy24 derived outbox changed")
	}
	rows, err := tx.Query(ctx, `SELECT event_id::text,space_id::text,access_epoch,event_kind,voice_room_id IS NULL,profile_id IS NULL,payload=jsonb_build_object('space_id',space_id::text,'access_epoch',access_epoch),created_at FROM public.space_voice_access_outbox`)
	if err != nil {
		return errors.New("legacy24 derived outbox unavailable")
	}
	seen := make(map[string]map[int64]bool)
	var updateTime time.Time
	for rows.Next() {
		var id, space, kind string
		var epoch int64
		var roomNull, profileNull, payloadOK bool
		var created time.Time
		if rows.Scan(&id, &space, &epoch, &kind, &roomNull, &profileNull, &payloadOK, &created) != nil {
			rows.Close()
			return errors.New("legacy24 derived outbox unavailable")
		}
		if _, old := derived.Events[id]; old {
			continue
		}
		floor, ok := derived.Floors[space]
		if !ok || epoch <= floor.Epoch || epoch > floor.Epoch+floor.Members || kind != "voice_room_access_invalidated" || !roomNull || !profileNull || !payloadOK || !created.Equal(current.Floors[space].Updated) || (!updateTime.IsZero() && !created.Equal(updateTime)) {
			rows.Close()
			return errors.New("legacy24 derived outbox changed")
		}
		updateTime = created
		if seen[space] == nil {
			seen[space] = make(map[int64]bool)
		}
		if seen[space][epoch] {
			rows.Close()
			return errors.New("legacy24 derived outbox changed")
		}
		seen[space][epoch] = true
	}
	if rows.Err() != nil {
		rows.Close()
		return errors.New("legacy24 derived outbox unavailable")
	}
	rows.Close()
	for id, floor := range derived.Floors {
		if int64(len(seen[id])) != floor.Members {
			return errors.New("legacy24 derived outbox incomplete")
		}
	}
	return nil
}
