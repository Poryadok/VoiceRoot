package store

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/pkg/principal"
)

var ErrMembershipConflict = errors.New("bot membership operation conflict")
var ErrMembershipSource = errors.New("bot membership source unavailable")

type MembershipRecord struct {
	SpaceID, BotID, InstallationID, ActorProfileID uuid.UUID
	AddOperationID, CancelOperationID              uuid.UUID
	Generation, Revision, CompletionRevision       int64
	Desired, Phase                                 string
	AddBytes, CancelBytes, CompletionBytes         []byte
	CompletionHash                                 string
	AdministratorAccount, AdministratorProfile     uuid.UUID
	CompletionDone, CleanupSettled                 bool
}

type boundMembershipCall struct {
	found, succeeded bool
	add              uuid.UUID
	completion       int64
	result           uuid.UUID
}

func readBoundMembershipCall(ctx context.Context, tx pgx.Tx, admin installprincipal.Administrator, method string) (boundMembershipCall, error) {
	var c boundMembershipCall
	var hash, kind, state string
	var profile uuid.UUID
	var result *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT administrator_profile_id,method,request_hash,add_operation_id,completion_revision,state,result_installation_id FROM bot_space_membership_calls WHERE administrator_account_id=$1 AND request_id=$2`, admin.AccountID(), admin.RequestID()).Scan(&profile, &kind, &hash, &c.add, &c.completion, &state, &result)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if profile != admin.ProfileID() || kind != method || hash != admin.RequestHash() {
		return c, ErrMembershipConflict
	}
	c.found = true
	c.succeeded = state == "SUCCEEDED"
	if result != nil {
		c.result = *result
	}
	return c, nil
}
func bindMembershipCall(ctx context.Context, tx pgx.Tx, admin installprincipal.Administrator, method string, r MembershipRecord, done bool) error {
	c, err := readBoundMembershipCall(ctx, tx, admin, method)
	if err != nil {
		return err
	}
	completion := r.CompletionRevision
	if method == "CANCEL" {
		completion = 0
	}
	if c.found {
		if c.add != r.AddOperationID || c.completion != completion {
			return ErrMembershipConflict
		}
		return nil
	}
	state := "PENDING"
	var result any
	if done {
		state = "SUCCEEDED"
		if method == "INSTALL" {
			result = r.InstallationID
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_calls(administrator_account_id,request_id,administrator_profile_id,method,request_hash,add_operation_id,completion_revision,state,result_installation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, admin.AccountID(), admin.RequestID(), admin.ProfileID(), method, admin.RequestHash(), r.AddOperationID, completion, state, result)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE bot_space_membership_source SET issued_calls=issued_calls+1 WHERE singleton`)
	return err
}

func CanonicalInstallInputs(req *botv1.InstallBotInSpaceRequest) ([]byte, string, error) {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, "", ErrMembershipConflict
	}
	copy := proto.Clone(req).(*botv1.InstallBotInSpaceRequest)
	if _, err := lifecycleID(copy.GetBotId()); err != nil {
		return nil, "", ErrMembershipConflict
	}
	if _, err := lifecycleID(copy.GetSpaceId()); err != nil {
		return nil, "", ErrMembershipConflict
	}
	sort.Slice(copy.AllowedChats, func(i, j int) bool { return copy.AllowedChats[i].GetId() < copy.AllowedChats[j].GetId() })
	previous := ""
	for _, ref := range copy.AllowedChats {
		if ref == nil || len(ref.ProtoReflect().GetUnknown()) != 0 || ref.GetId() == previous {
			return nil, "", ErrMembershipConflict
		}
		if _, err := lifecycleID(ref.GetId()); err != nil {
			return nil, "", ErrMembershipConflict
		}
		previous = ref.GetId()
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(copy)
	if err != nil {
		return nil, "", err
	}
	hash, err := principal.RequestHash(copy)
	return raw, hash, err
}

func (r MembershipRecord) AddRequest() (*spacev1.AddBotMemberRequest, error) {
	var req spacev1.AddBotMemberRequest
	if err := proto.Unmarshal(r.AddBytes, &req); err != nil {
		return nil, ErrMembershipSource
	}
	if r.AddOperationID == uuid.Nil || len(req.ProtoReflect().GetUnknown()) != 0 || !r.matches(req.GetInstallationIntent()) || req.GetSpaceId() != r.SpaceID.String() || req.GetProfileId() != r.ActorProfileID.String() || req.GetInstallationIntent().GetOperationId() != r.AddOperationID.String() {
		return nil, ErrMembershipSource
	}
	return &req, nil
}
func (r MembershipRecord) RemoveRequest() (*spacev1.RemoveBotMemberRequest, error) {
	var req spacev1.RemoveBotMemberRequest
	if err := proto.Unmarshal(r.CancelBytes, &req); err != nil {
		return nil, ErrMembershipSource
	}
	if r.AddOperationID == uuid.Nil || r.CancelOperationID == uuid.Nil || r.AddOperationID == r.CancelOperationID || len(req.ProtoReflect().GetUnknown()) != 0 || !r.matches(req.GetInstallationIntent()) || req.GetSpaceId() != r.SpaceID.String() || req.GetProfileId() != r.ActorProfileID.String() || req.GetInstallationIntent().GetOperationId() != r.CancelOperationID.String() || req.GetInstallOperationId() != r.AddOperationID.String() {
		return nil, ErrMembershipSource
	}
	return &req, nil
}
func (r MembershipRecord) matches(i *spacev1.BotInstallationIntent) bool {
	return r.SpaceID != uuid.Nil && r.BotID != uuid.Nil && r.InstallationID != uuid.Nil && r.ActorProfileID != uuid.Nil && i != nil && len(i.ProtoReflect().GetUnknown()) == 0 && i.GetBotId() == r.BotID.String() && i.GetInstallationId() == r.InstallationID.String() && i.GetSpaceId() == r.SpaceID.String() && i.GetActorProfileId() == r.ActorProfileID.String() && r.Generation > 0 && i.GetGeneration() == uint64(r.Generation)
}
func (r MembershipRecord) intent(operation uuid.UUID) *spacev1.BotInstallationIntent {
	return &spacev1.BotInstallationIntent{OperationId: operation.String(), BotId: r.BotID.String(), InstallationId: r.InstallationID.String(), SpaceId: r.SpaceID.String(), ActorProfileId: r.ActorProfileID.String(), Generation: uint64(r.Generation)}
}

// Every public transition uses the same guard order. SQL transactions end
// before the coordinator receives the record and starts any network operation.
func (s *BotStore) membershipTx(ctx context.Context, source, space, bot uuid.UUID) (pgx.Tx, error) {
	if s == nil || s.Pool == nil || source == uuid.Nil || space == uuid.Nil || bot == uuid.Nil {
		return nil, ErrMembershipSource
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	fail := func(e error) (pgx.Tx, error) { _ = tx.Rollback(ctx); return nil, e }
	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout='2s'; SET LOCAL statement_timeout='5s'`); err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,71431))`, space.String()); err != nil {
		return fail(err)
	}
	var id uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM bots WHERE id=$1 FOR UPDATE`, bot).Scan(&id); err != nil {
		return fail(err)
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT count(*)=1 AND coalesce(bool_and(m.version=$2 AND NOT m.dirty),false)
 AND EXISTS(SELECT 1 FROM bot_space_membership_source s WHERE s.singleton AND s.source_generation=$1
 AND s.issued_pairs=(SELECT count(*) FROM bot_space_membership_heads)
 AND s.issued_operations=(SELECT count(*) FROM bot_space_membership_operations)
	 AND s.issued_calls=(SELECT count(*) FROM bot_space_membership_calls)) FROM schema_migrations m`, source, BotSchemaVersion).Scan(&valid)
	if err != nil || !valid {
		return fail(ErrMembershipSource)
	}
	return tx, nil
}

func readMembership(ctx context.Context, tx pgx.Tx, space, bot uuid.UUID) (MembershipRecord, error) {
	r := MembershipRecord{SpaceID: space, BotID: bot}
	var install, actor, add, cancel *uuid.UUID
	var generation *int64
	err := tx.QueryRow(ctx, `SELECT installation_id,actor_profile_id,generation,add_operation_id,cancel_operation_id,desired_state,phase,revision,completion_revision,cleanup_settled FROM bot_space_membership_heads WHERE space_id=$1 AND bot_id=$2 FOR UPDATE`, space, bot).Scan(&install, &actor, &generation, &add, &cancel, &r.Desired, &r.Phase, &r.Revision, &r.CompletionRevision, &r.CleanupSettled)
	if err != nil {
		return r, err
	}
	if r.Phase == "LEGACY_UNKNOWN" {
		return r, ErrMembershipSource
	}
	if install == nil || actor == nil || generation == nil || add == nil {
		return r, ErrMembershipSource
	}
	r.InstallationID, r.ActorProfileID, r.Generation, r.AddOperationID = *install, *actor, *generation, *add
	if cancel != nil {
		r.CancelOperationID = *cancel
	}
	var hash string
	if err = tx.QueryRow(ctx, `SELECT request_bytes,request_hash FROM bot_space_membership_operations WHERE operation_id=$1 AND kind='ADD' AND space_id=$2 AND bot_id=$3 AND generation=$4`, r.AddOperationID, space, bot, r.Generation).Scan(&r.AddBytes, &hash); err != nil {
		return r, ErrMembershipSource
	}
	req, err := r.AddRequest()
	if err != nil {
		return r, err
	}
	actual, err := principal.RequestHash(req)
	if err != nil || hash != actual {
		return r, ErrMembershipSource
	}
	if r.CancelOperationID != uuid.Nil {
		if err = tx.QueryRow(ctx, `SELECT request_bytes,request_hash FROM bot_space_membership_operations WHERE operation_id=$1 AND kind='CANCEL' AND original_add_operation_id=$2`, r.CancelOperationID, r.AddOperationID).Scan(&r.CancelBytes, &hash); err != nil {
			return r, ErrMembershipSource
		}
		req, err := r.RemoveRequest()
		if err != nil {
			return r, err
		}
		actual, err := principal.RequestHash(req)
		if err != nil || hash != actual {
			return r, ErrMembershipSource
		}
	}
	if r.CompletionRevision > 0 {
		err = tx.QueryRow(ctx, `SELECT inputs,inputs_hash,administrator_account_id,administrator_profile_id,completed FROM bot_space_membership_completions WHERE add_operation_id=$1 AND completion_revision=$2`, r.AddOperationID, r.CompletionRevision).Scan(&r.CompletionBytes, &r.CompletionHash, &r.AdministratorAccount, &r.AdministratorProfile, &r.CompletionDone)
		if err != nil {
			return r, ErrMembershipSource
		}
	}
	return r, nil
}

func membershipLive(ctx context.Context, tx pgx.Tx, space uuid.UUID) error {
	var state string
	err := tx.QueryRow(ctx, `SELECT desired_state FROM bot_space_lifecycle_heads WHERE space_id=$1`, space).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != "LIVE" {
		return ErrSpaceLifecycleConflict
	}
	return nil
}

func insertMembershipOperation(ctx context.Context, tx pgx.Tx, r MembershipRecord, kind string, operation uuid.UUID, request proto.Message) error {
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_operations(operation_id,kind,installation_id,space_id,bot_id,actor_profile_id,generation,original_add_operation_id,request_bytes,request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, operation, kind, r.InstallationID, r.SpaceID, r.BotID, r.ActorProfileID, r.Generation, r.AddOperationID, raw, hash)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE bot_space_membership_source SET issued_operations=issued_operations+1 WHERE singleton`)
	return err
}

func (s *BotStore) PrepareInstall(ctx context.Context, source uuid.UUID, admin installprincipal.Administrator, request *botv1.InstallBotInSpaceRequest) (MembershipRecord, error) {
	raw, hash, err := CanonicalInstallInputs(request)
	if err != nil {
		return MembershipRecord{}, err
	}
	if admin.AccountID() == uuid.Nil || admin.ProfileID() == uuid.Nil {
		return MembershipRecord{}, ErrMembershipConflict
	}
	space, _ := uuid.Parse(request.GetSpaceId())
	bot, _ := uuid.Parse(request.GetBotId())
	tx, err := s.membershipTx(ctx, source, space, bot)
	if err != nil {
		return MembershipRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	call, callErr := readBoundMembershipCall(ctx, tx, admin, "INSTALL")
	if callErr != nil {
		return MembershipRecord{}, callErr
	}
	if call.succeeded {
		if call.result == uuid.Nil {
			return MembershipRecord{}, ErrMembershipSource
		}
		if err = tx.Commit(ctx); err != nil {
			return MembershipRecord{}, err
		}
		return MembershipRecord{InstallationID: call.result, Phase: "RESULT_REPLAY", CompletionDone: true}, nil
	}
	if !time.Now().Before(admin.ExpiresAt()) {
		return MembershipRecord{}, ErrMembershipConflict
	}
	if err = membershipLive(ctx, tx, space); err != nil {
		return MembershipRecord{}, err
	}
	var actor uuid.UUID
	var state, scopes string
	if err = tx.QueryRow(ctx, `SELECT actor_profile_id,status,scopes::text FROM bots WHERE id=$1`, bot).Scan(&actor, &state, &scopes); err != nil {
		return MembershipRecord{}, err
	}
	if actor == uuid.Nil || state != "live" || (HasPrivilegedScope(scopes) && !request.GetAcknowledgePrivilegedScopes()) {
		return MembershipRecord{}, ErrMembershipConflict
	}
	r, err := readMembership(ctx, tx, space, bot)
	newPair := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !newPair {
		return r, err
	}
	newGeneration := newPair || r.Phase == "CANCELED"
	if newGeneration {
		generation := int64(1)
		if !newPair {
			if !r.CleanupSettled || r.Generation == math.MaxInt64 {
				return r, ErrMembershipConflict
			}
			generation = r.Generation + 1
		}
		var legacy bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_space_installations WHERE space_id=$1 AND bot_id=$2)`, space, bot).Scan(&legacy); err != nil || legacy {
			return r, ErrMembershipSource
		}
		r = MembershipRecord{SpaceID: space, BotID: bot, ActorProfileID: actor, InstallationID: uuid.New(), AddOperationID: uuid.New(), Generation: generation, Revision: r.Revision + 1, CompletionRevision: 1, Desired: "INSTALL", Phase: "PREPARED"}
		add := &spacev1.AddBotMemberRequest{SpaceId: space.String(), ProfileId: actor.String(), InstallationIntent: r.intent(r.AddOperationID)}
		if err = insertMembershipOperation(ctx, tx, r, "ADD", r.AddOperationID, add); err != nil {
			return r, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_heads(space_id,bot_id,installation_id,actor_profile_id,generation,add_operation_id,desired_state,phase,revision,completion_revision) VALUES($1,$2,$3,$4,$5,$6,'INSTALL','PREPARED',$7,1)
 ON CONFLICT(space_id,bot_id) DO UPDATE SET installation_id=EXCLUDED.installation_id,actor_profile_id=EXCLUDED.actor_profile_id,generation=EXCLUDED.generation,add_operation_id=EXCLUDED.add_operation_id,cancel_operation_id=NULL,desired_state='INSTALL',phase='PREPARED',revision=EXCLUDED.revision,completion_revision=1,lease_until=NULL,cleanup_settled=false`, space, bot, r.InstallationID, actor, r.Generation, r.AddOperationID, r.Revision)
		if err != nil {
			return r, err
		}
		if newPair {
			if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_source SET issued_pairs=issued_pairs+1 WHERE singleton`); err != nil {
				return r, err
			}
		}
	} else {
		if r.Desired != "INSTALL" || r.ActorProfileID != actor || (r.Phase != "PREPARED" && r.Phase != "ADD_ACK" && r.Phase != "ACTIVE") {
			return r, ErrMembershipConflict
		}
		if r.CompletionHash == hash && bytes.Equal(r.CompletionBytes, raw) && r.AdministratorProfile == admin.ProfileID() && r.AdministratorAccount == admin.AccountID() {
			if err = bindMembershipCall(ctx, tx, admin, "INSTALL", r, r.CompletionDone); err != nil {
				return r, err
			}
			if err = tx.Commit(ctx); err != nil {
				return r, err
			}
			return r, nil
		}
		if r.Phase != "ACTIVE" || !r.CompletionDone {
			return r, ErrMembershipConflict
		}
		r.CompletionRevision++
		r.Revision++
		if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_heads SET completion_revision=$3,revision=$4,lease_until=NULL WHERE space_id=$1 AND bot_id=$2`, space, bot, r.CompletionRevision, r.Revision); err != nil {
			return r, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_completions(add_operation_id,completion_revision,administrator_account_id,administrator_profile_id,initiating_request_id,initiating_request_hash,inputs,inputs_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, r.AddOperationID, r.CompletionRevision, admin.AccountID(), admin.ProfileID(), admin.RequestID(), admin.RequestHash(), raw, hash)
	if err != nil {
		return r, err
	}
	r, err = readMembership(ctx, tx, space, bot)
	if err != nil {
		return r, err
	}
	if err = bindMembershipCall(ctx, tx, admin, "INSTALL", r, false); err != nil {
		return r, err
	}
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	return r, nil
}

// ClaimMembershipAttempt returns a revision token, not an assertion that a
// previous timed-out remote operation cannot still execute.
func (s *BotStore) ClaimMembershipAttempt(ctx context.Context, source uuid.UUID, expected MembershipRecord, lease time.Duration) (MembershipRecord, error) {
	if lease <= 0 || lease > 30*time.Second {
		return MembershipRecord{}, ErrMembershipConflict
	}
	tx, err := s.membershipTx(ctx, source, expected.SpaceID, expected.BotID)
	if err != nil {
		return MembershipRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, expected.SpaceID, expected.BotID)
	if err != nil {
		return r, err
	}
	if !sameAttempt(r, expected) || r.Desired != expected.Desired || r.Phase == "PURGED" || r.Phase == "CANCELED" {
		return r, ErrMembershipConflict
	}
	if r.Desired == "INSTALL" {
		if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
			return r, err
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE bot_space_membership_heads SET revision=revision+1,lease_until=clock_timestamp()+$3::double precision*interval '1 second' WHERE space_id=$1 AND bot_id=$2 AND (lease_until IS NULL OR lease_until<=clock_timestamp())`, r.SpaceID, r.BotID, lease.Seconds())
	if err != nil {
		return r, err
	}
	if tag.RowsAffected() != 1 {
		return r, ErrMembershipConflict
	}
	r.Revision++
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	return r, nil
}
func sameAttempt(a, b MembershipRecord) bool {
	return a.SpaceID == b.SpaceID && a.BotID == b.BotID && a.AddOperationID == b.AddOperationID && a.InstallationID == b.InstallationID && a.Generation == b.Generation && a.CompletionRevision == b.CompletionRevision && a.CancelOperationID == b.CancelOperationID
}

func (s *BotStore) RecordAddAcknowledged(ctx context.Context, source uuid.UUID, claim MembershipRecord) (MembershipRecord, error) {
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return MembershipRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return r, err
	}
	if !sameAttempt(r, claim) || r.Revision != claim.Revision || r.Desired != "INSTALL" || (r.Phase != "PREPARED" && r.Phase != "ADD_ACK" && r.Phase != "ACTIVE") {
		return r, ErrMembershipConflict
	}
	if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
		return r, err
	}
	if r.Phase != "ACTIVE" {
		r.Phase = "ADD_ACK"
	}
	r.Revision++
	_, err = tx.Exec(ctx, `UPDATE bot_space_membership_heads SET phase=$3,revision=$4 WHERE space_id=$1 AND bot_id=$2`, r.SpaceID, r.BotID, r.Phase, r.Revision)
	if err != nil {
		return r, err
	}
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	return r, nil
}

func (s *BotStore) CompleteInstallation(ctx context.Context, source uuid.UUID, claim MembershipRecord) (uuid.UUID, error) {
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return uuid.Nil, err
	}
	if !sameAttempt(r, claim) || r.Revision != claim.Revision || r.Desired != "INSTALL" || (r.Phase != "ADD_ACK" && r.Phase != "ACTIVE") {
		return uuid.Nil, ErrMembershipConflict
	}
	if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
		return uuid.Nil, err
	}
	var actor uuid.UUID
	var state string
	if err = tx.QueryRow(ctx, `SELECT actor_profile_id,status FROM bots WHERE id=$1`, r.BotID).Scan(&actor, &state); err != nil {
		return uuid.Nil, err
	}
	if actor != r.ActorProfileID || state != "live" {
		return uuid.Nil, ErrMembershipConflict
	}
	var input botv1.InstallBotInSpaceRequest
	if proto.Unmarshal(r.CompletionBytes, &input) != nil {
		return uuid.Nil, ErrMembershipSource
	}
	raw, hash, err := CanonicalInstallInputs(&input)
	if err != nil || hash != r.CompletionHash || !bytes.Equal(raw, r.CompletionBytes) || input.GetBotId() != r.BotID.String() || input.GetSpaceId() != r.SpaceID.String() {
		return uuid.Nil, ErrMembershipSource
	}
	// The actual persisted identity is returned on whitelist updates too.
	var install uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO bot_space_installations(id,bot_id,space_id,installed_by_profile_id) VALUES($1,$2,$3,$4)
 ON CONFLICT(bot_id,space_id) DO UPDATE SET installed_by_profile_id=EXCLUDED.installed_by_profile_id WHERE bot_space_installations.id=EXCLUDED.id RETURNING id`, r.InstallationID, r.BotID, r.SpaceID, r.AdministratorProfile).Scan(&install)
	if err != nil {
		return uuid.Nil, err
	}
	if install != r.InstallationID {
		return uuid.Nil, ErrMembershipConflict
	}
	var chats []uuid.UUID
	for _, ref := range input.GetAllowedChats() {
		id, err := lifecycleID(ref.GetId())
		if err != nil {
			return uuid.Nil, err
		}
		chats = append(chats, id)
	}
	var qualified int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM bot_space_membership_chat_sources WHERE add_operation_id=$1 AND completion_revision=$2 AND response_bytes IS NOT NULL AND chat_id=ANY($3::uuid[])`, r.AddOperationID, r.CompletionRevision, chats).Scan(&qualified); err != nil {
		return uuid.Nil, err
	}
	if qualified != len(chats) {
		return uuid.Nil, ErrMembershipSource
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_message_deliveries SET status='canceled',claimed_until=NULL WHERE bot_id=$1 AND status='pending' AND chat_id IN(SELECT chat_id FROM bot_chat_whitelist WHERE bot_id=$1 AND space_id=$2) AND NOT(chat_id=ANY($3::uuid[]))`, r.BotID, r.SpaceID, chats); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM bot_chat_whitelist WHERE bot_id=$1 AND space_id=$2`, r.BotID, r.SpaceID); err != nil {
		return uuid.Nil, err
	}
	for _, chat := range chats {
		if _, err = tx.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,true,$4)`, r.BotID, chat, r.SpaceID, r.AdministratorProfile); err != nil {
			return uuid.Nil, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_completions SET completed=true WHERE add_operation_id=$1 AND completion_revision=$2`, r.AddOperationID, r.CompletionRevision); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_heads SET phase='ACTIVE',revision=revision+1,lease_until=NULL WHERE space_id=$1 AND bot_id=$2`, r.SpaceID, r.BotID); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_calls SET state='SUCCEEDED',result_installation_id=$3 WHERE add_operation_id=$1 AND completion_revision=$2 AND method='INSTALL'`, r.AddOperationID, r.CompletionRevision, r.InstallationID); err != nil {
		return uuid.Nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return install, nil
}

func (s *BotStore) BeginCancel(ctx context.Context, source uuid.UUID, admin installprincipal.Administrator, space, bot uuid.UUID) (MembershipRecord, error) {
	if admin.AccountID() == uuid.Nil || admin.ProfileID() == uuid.Nil {
		return MembershipRecord{}, ErrMembershipConflict
	}
	tx, err := s.membershipTx(ctx, source, space, bot)
	if err != nil {
		return MembershipRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	call, err := readBoundMembershipCall(ctx, tx, admin, "CANCEL")
	if err != nil {
		return MembershipRecord{}, err
	}
	if call.succeeded {
		if err = tx.Commit(ctx); err != nil {
			return MembershipRecord{}, err
		}
		return MembershipRecord{Phase: "CANCEL_RESULT_REPLAY", CleanupSettled: true}, nil
	}
	if !time.Now().Before(admin.ExpiresAt()) {
		return MembershipRecord{}, ErrMembershipConflict
	}
	r, err := readMembership(ctx, tx, space, bot)
	if err != nil {
		return r, err
	}
	if r.Phase == "PURGED" || (call.found && call.add != r.AddOperationID) {
		return r, ErrMembershipConflict
	}
	if r.CancelOperationID == uuid.Nil {
		r.CancelOperationID = uuid.New()
		remove := &spacev1.RemoveBotMemberRequest{SpaceId: r.SpaceID.String(), ProfileId: r.ActorProfileID.String(), InstallationIntent: r.intent(r.CancelOperationID), InstallOperationId: r.AddOperationID.String()}
		if err = insertMembershipOperation(ctx, tx, r, "CANCEL", r.CancelOperationID, remove); err != nil {
			return r, err
		}
		if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_heads SET cancel_operation_id=$3,desired_state='CANCEL',phase='CANCEL_PENDING',revision=revision+1,lease_until=NULL WHERE space_id=$1 AND bot_id=$2`, space, bot, r.CancelOperationID); err != nil {
			return r, err
		}
	}
	// Snapshot actual whitelist AND every durably prepared completion before
	// revocation. A lost reply could have applied a prepared chat effect already.
	if _, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_cleanup_chats(add_operation_id,chat_id) SELECT $3,chat_id FROM bot_chat_whitelist WHERE space_id=$1 AND bot_id=$2 ON CONFLICT DO NOTHING`, space, bot, r.AddOperationID); err != nil {
		return r, err
	}
	rows, err := tx.Query(ctx, `SELECT inputs FROM bot_space_membership_completions WHERE add_operation_id=$1 ORDER BY completion_revision`, r.AddOperationID)
	if err != nil {
		return r, err
	}
	var snapshots [][]byte
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return r, err
		}
		snapshots = append(snapshots, raw)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return r, err
	}
	for _, raw := range snapshots {
		var input botv1.InstallBotInSpaceRequest
		if proto.Unmarshal(raw, &input) != nil {
			return r, ErrMembershipSource
		}
		for _, ref := range input.GetAllowedChats() {
			id, err := lifecycleID(ref.GetId())
			if err != nil {
				return r, ErrMembershipSource
			}
			if _, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_cleanup_chats(add_operation_id,chat_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, r.AddOperationID, id); err != nil {
				return r, err
			}
		}
	}
	if err = revokeMembershipVisibility(ctx, tx, space, bot); err != nil {
		return r, err
	}
	r, err = readMembership(ctx, tx, space, bot)
	if err != nil {
		return r, err
	}
	if err = persistMembershipCleanupDecision(ctx, tx, r, admin); err != nil {
		return r, err
	}
	if err = bindMembershipCall(ctx, tx, admin, "CANCEL", r, r.Phase == "CANCELED" && r.CleanupSettled); err != nil {
		return r, err
	}
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	return r, nil
}

func revokeMembershipVisibility(ctx context.Context, tx pgx.Tx, space, bot uuid.UUID) error {
	if _, err := tx.Exec(ctx, `UPDATE bot_message_deliveries SET status='canceled',claimed_until=NULL WHERE bot_id=$1 AND status IN('pending','claimed') AND chat_id IN(SELECT chat_id FROM bot_chat_whitelist WHERE space_id=$2 AND bot_id=$1)`, bot, space); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE bot_event_log SET delivery_status='canceled',claimed_until=NULL WHERE bot_id=$1 AND space_id=$2 AND delivery_status IN('pending','deferred')`, bot, space); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM bot_chat_whitelist WHERE space_id=$1 AND bot_id=$2`, space, bot); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM bot_space_installations WHERE space_id=$1 AND bot_id=$2`, space, bot)
	return err
}

func (s *BotStore) RecordCancelAcknowledged(ctx context.Context, source uuid.UUID, claim MembershipRecord) (MembershipRecord, error) {
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return MembershipRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return r, err
	}
	if !sameAttempt(r, claim) || r.Revision != claim.Revision || r.Desired != "CANCEL" || (r.Phase != "CANCEL_PENDING" && r.Phase != "REMOVE_ACK" && r.Phase != "CLEANUP_PENDING") {
		return r, ErrMembershipConflict
	}
	r.Phase = "CLEANUP_PENDING"
	r.Revision++
	if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_heads SET phase=$3,revision=$4 WHERE space_id=$1 AND bot_id=$2`, r.SpaceID, r.BotID, r.Phase, r.Revision); err != nil {
		return r, err
	}
	if err = tx.Commit(ctx); err != nil {
		return r, err
	}
	return r, nil
}

func (s *BotStore) MembershipCleanupChatIDs(ctx context.Context, record MembershipRecord) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil || record.AddOperationID == uuid.Nil {
		return nil, ErrMembershipSource
	}
	rows, err := s.Pool.Query(ctx, `SELECT chat_id FROM bot_space_membership_cleanup_chats WHERE add_operation_id=$1 ORDER BY chat_id`, record.AddOperationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CompleteCancellation is called only after all protected receiver effects are
// actually settled. A lease expiry or generic fake cleanup result is not proof.
func (s *BotStore) CompleteCancellation(ctx context.Context, source uuid.UUID, claim MembershipRecord) error {
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return err
	}
	if !sameAttempt(r, claim) || r.Revision != claim.Revision || r.Desired != "CANCEL" || r.Phase != "CLEANUP_PENDING" {
		return ErrMembershipConflict
	}
	var settled bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bot_space_membership_effects WHERE add_operation_id=$1 AND step_key='role' AND method='/voice.role.v1.RoleService/ApplyBotInstallationRemoval' AND acknowledged AND response_bytes IS NOT NULL)
 AND NOT EXISTS(SELECT 1 FROM bot_space_membership_effects WHERE add_operation_id=$1 AND (NOT acknowledged OR response_bytes IS NULL))
 AND NOT EXISTS(SELECT 1 FROM bot_space_membership_chat_sources s WHERE s.add_operation_id=$1 AND s.response_bytes IS NOT NULL AND NOT EXISTS(SELECT 1 FROM bot_space_membership_effects e WHERE e.add_operation_id=$1 AND e.step_key='messaging:'||s.chat_id::text AND e.method='/voice.messaging.v1.MessagingService/ApplyBotChatPinRemoval' AND e.acknowledged AND e.response_bytes IS NOT NULL))`, r.AddOperationID).Scan(&settled); err != nil {
		return err
	}
	if !settled {
		return ErrMembershipConflict
	}
	if err = revokeMembershipVisibility(ctx, tx, r.SpaceID, r.BotID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_heads SET phase='CANCELED',cleanup_settled=true,revision=revision+1,lease_until=NULL WHERE space_id=$1 AND bot_id=$2`, r.SpaceID, r.BotID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bot_space_membership_calls SET state='SUCCEEDED' WHERE add_operation_id=$1 AND method='CANCEL'`, r.AddOperationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type MembershipEffect struct {
	Key, Method, Hash           string
	OperationID                 uuid.UUID
	RequestBytes, ResponseBytes []byte
	Acknowledged                bool
}

func validEffectClaim(current, claim MembershipRecord) bool {
	return sameAttempt(current, claim) && current.Revision == claim.Revision && current.Desired == claim.Desired &&
		((current.Desired == "INSTALL" && (current.Phase == "ADD_ACK" || current.Phase == "ACTIVE")) || (current.Desired == "CANCEL" && current.Phase == "CLEANUP_PENDING"))
}

func (s *BotStore) PrepareMembershipEffect(ctx context.Context, source uuid.UUID, claim MembershipRecord, key, method string, request proto.Message) (MembershipEffect, error) {
	var effect MembershipEffect
	if key == "" || len(key) > 256 || method == "" || request == nil || !request.ProtoReflect().IsValid() || len(request.ProtoReflect().GetUnknown()) != 0 {
		return effect, ErrMembershipConflict
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return effect, err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return effect, err
	}
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return effect, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return effect, err
	}
	if !validEffectClaim(r, claim) {
		return effect, ErrMembershipConflict
	}
	if r.Desired == "INSTALL" {
		if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
			return effect, err
		}
	}
	err = tx.QueryRow(ctx, `SELECT operation_id,method,request_bytes,request_hash,acknowledged,response_bytes FROM bot_space_membership_effects WHERE add_operation_id=$1 AND step_key=$2`, r.AddOperationID, key).Scan(&effect.OperationID, &effect.Method, &effect.RequestBytes, &effect.Hash, &effect.Acknowledged, &effect.ResponseBytes)
	if err == nil {
		if effect.Method != method || effect.Hash != hash || !bytes.Equal(effect.RequestBytes, raw) {
			return effect, ErrMembershipConflict
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		bound, ok := request.(interface{ GetOperationId() string })
		if !ok {
			return effect, ErrMembershipConflict
		}
		operation, parseErr := lifecycleID(bound.GetOperationId())
		if parseErr != nil {
			return effect, ErrMembershipConflict
		}
		effect = MembershipEffect{Key: key, Method: method, OperationID: operation, RequestBytes: raw, Hash: hash}
		_, err = tx.Exec(ctx, `INSERT INTO bot_space_membership_effects(add_operation_id,step_key,operation_id,method,request_bytes,request_hash) VALUES($1,$2,$3,$4,$5,$6)`, r.AddOperationID, key, effect.OperationID, method, raw, hash)
		if err != nil {
			return effect, err
		}
	} else {
		return effect, err
	}
	effect.Key = key
	if err = tx.Commit(ctx); err != nil {
		return effect, err
	}
	return effect, nil
}

func (s *BotStore) RecordMembershipEffectAcknowledged(ctx context.Context, source uuid.UUID, claim MembershipRecord, effect MembershipEffect) error {
	if len(effect.ResponseBytes) == 0 {
		return ErrMembershipConflict
	}
	tx, err := s.membershipTx(ctx, source, claim.SpaceID, claim.BotID)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	r, err := readMembership(ctx, tx, claim.SpaceID, claim.BotID)
	if err != nil {
		return err
	}
	if !validEffectClaim(r, claim) {
		return ErrMembershipConflict
	}
	if r.Desired == "INSTALL" {
		if err = membershipLive(ctx, tx, r.SpaceID); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE bot_space_membership_effects SET acknowledged=true,response_bytes=$7 WHERE add_operation_id=$1 AND step_key=$2 AND operation_id=$3 AND method=$4 AND request_hash=$5 AND request_bytes=$6 AND (response_bytes IS NULL OR response_bytes=$7)`, r.AddOperationID, effect.Key, effect.OperationID, effect.Method, effect.Hash, effect.RequestBytes, effect.ResponseBytes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMembershipConflict
	}
	return tx.Commit(ctx)
}
