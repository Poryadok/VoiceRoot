package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

var (
	ErrBotGlobalTerminalInvalid  = errors.New("invalid Bot global terminal request")
	ErrBotGlobalTerminalDenied   = errors.New("verified Bot global terminal authority required")
	ErrBotGlobalTerminalConflict = errors.New("Bot global terminal binding conflicts")
)

func botGlobalTerminalAuthority(ctx context.Context, r *rolev1.RegisterBotGlobalTerminalRequest) (uuid.UUID, uuid.UUID, error) {
	if r == nil || r.ProtocolVersion != 1 || len(r.ProtoReflect().GetUnknown()) != 0 {
		return uuid.Nil, uuid.Nil, ErrBotGlobalTerminalInvalid
	}
	bot, err := botRemovalUUID(r.BotId)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrBotGlobalTerminalInvalid
	}
	decision, err := botRemovalUUID(r.DecisionId)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrBotGlobalTerminalInvalid
	}
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(r)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "role" || p.RPC != rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName || p.RequestID != r.DecisionId || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return uuid.Nil, uuid.Nil, ErrBotGlobalTerminalDenied
	}
	return bot, decision, nil
}

// RegisterBotGlobalTerminal records only Role's own terminal fact. It never
// derives a Space, actor, cleanup count or current human authorization.
func (s *RoleStore) RegisterBotGlobalTerminal(ctx context.Context, req *rolev1.RegisterBotGlobalTerminalRequest) (*rolev1.RegisterBotGlobalTerminalResponse, error) {
	if req == nil {
		return nil, ErrBotGlobalTerminalInvalid
	}
	req = proto.Clone(req).(*rolev1.RegisterBotGlobalTerminalRequest)
	bot, decision, err := botGlobalTerminalAuthority(ctx, req)
	if err != nil {
		return nil, err
	}
	var result *rolev1.RegisterBotGlobalTerminalResponse
	err = s.withBotGlobalOperation(ctx, decision, decision, nil, func(scoped *RoleStore) error {
		if err := scoped.checkBotGlobalTerminalCatalog(ctx); err != nil {
			return err
		}
		if _, err := scoped.db().Exec(ctx, "SELECT pg_advisory_xact_lock(1380074034,hashtext($1))", bot.String()); err != nil {
			return err
		}
		var err error
		result, err = scoped.botGlobalTerminalResult(ctx, bot, decision)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// withBotGlobalOperation changes only the initial operation stage for global
// Bot writers. No nested entry may append locks after Space/Bot acquisition.
func (s *RoleStore) withBotGlobalOperation(ctx context.Context, operation, decision uuid.UUID, space *uuid.UUID, fn func(*RoleStore) error) error {
	if s == nil || s.Pool == nil || s.tx != nil || operation == uuid.Nil || decision == uuid.Nil || fn == nil || (space != nil && *space == uuid.Nil) || (space == nil && operation != decision) {
		return ErrScopeUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("%w: global operation begin", ErrScopeUnavailable)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var isolation string
	if err = tx.QueryRow(ctx, "SHOW transaction_isolation").Scan(&isolation); err != nil || isolation != "read committed" {
		return ErrScopeUnavailable
	}
	keys, err := botGlobalOperationKeys(ctx, tx, []uuid.UUID{operation, decision})
	if err != nil {
		return err
	}
	for _, key := range keys {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1::integer,$2::integer)", ownershipOperationLockNamespace, key); err != nil {
			return err
		}
	}
	scoped := &RoleStore{Pool: s.Pool, tx: tx, scopeOperation: operation}
	if space != nil {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", space.String()); err != nil {
			return err
		}
		scoped.scopeSpaces = map[uuid.UUID]struct{}{*space: {}}
	}
	if err = fn(scoped); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: global operation commit", ErrScopeUnavailable)
	}
	return nil
}

func botGlobalOperationKeys(ctx context.Context, db scopeExecutor, ids []uuid.UUID) ([]int32, error) {
	values := make([]string, len(ids))
	for i, id := range ids {
		if id == uuid.Nil {
			return nil, ErrScopeUnavailable
		}
		values[i] = id.String()
	}
	rows, err := db.Query(ctx, "SELECT hashtext(v) FROM unnest($1::text[]) AS v", values)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	unique := map[int32]struct{}{}
	for rows.Next() {
		var key int32
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		unique[key] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	keys := make([]int32, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys, nil
}

func (s *RoleStore) botGlobalTerminalResult(ctx context.Context, bot, decision uuid.UUID) (*rolev1.RegisterBotGlobalTerminalResponse, error) {
	// c110 requires a fresh qualified UNION check even before saved success.
	if err := s.checkBotGlobalBindingUnion(ctx, bot, decision, true); err != nil {
		return nil, err
	}
	var savedBot, savedDecision, receipt uuid.UUID
	var at time.Time
	err := s.db().QueryRow(ctx, "SELECT bot_id,decision_id,receipt_id,registered_at FROM public.role_bot_global_terminals WHERE bot_id=$1 OR decision_id=$2 ORDER BY bot_id LIMIT 1", bot, decision).Scan(&savedBot, &savedDecision, &receipt, &at)
	if err == nil {
		if savedBot != bot || savedDecision != decision {
			return nil, ErrBotGlobalTerminalConflict
		}
		return botGlobalTerminalResponse(bot, decision, receipt, at)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	err = s.db().QueryRow(ctx, "INSERT INTO public.role_bot_global_terminals(bot_id,decision_id,receipt_id) VALUES($1,$2,$3) RETURNING receipt_id,registered_at", bot, decision, uuid.New()).Scan(&receipt, &at)
	if err != nil {
		return nil, err
	}
	return botGlobalTerminalResponse(bot, decision, receipt, at)
}

func botGlobalTerminalResponse(bot, decision, receipt uuid.UUID, at time.Time) (*rolev1.RegisterBotGlobalTerminalResponse, error) {
	stamp := timestamppb.New(at.UTC())
	if bot == uuid.Nil || decision == uuid.Nil || receipt == uuid.Nil || stamp.CheckValid() != nil {
		return nil, ErrScopeUnavailable
	}
	return &rolev1.RegisterBotGlobalTerminalResponse{Receipt: &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: bot.String(), DecisionId: decision.String(), ReceiptId: receipt.String(), RegisteredAt: stamp, Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}}, nil
}

type oldBotGlobalBinding struct {
	bot, actor, decision uuid.UUID
	hash                 []byte
}

// checkBotGlobalBindingUnion never guesses that a legacy row is a qualified
// root. Even a blocked Owner cleanup needs its original accepted request link.
func (s *RoleStore) checkBotGlobalBindingUnion(ctx context.Context, bot, decision uuid.UUID, minimal bool) error {
	rows, err := s.db().Query(ctx, "SELECT bot_id,actor_profile_id,decision_id,decision_sha256 FROM role_bot_global_removals WHERE bot_id=$1 OR decision_id=$2 ORDER BY bot_id", bot, decision)
	if err != nil {
		return err
	}
	old := []oldBotGlobalBinding{}
	for rows.Next() {
		var b oldBotGlobalBinding
		if err = rows.Scan(&b.bot, &b.actor, &b.decision, &b.hash); err != nil {
			rows.Close()
			return err
		}
		old = append(old, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	type tuple struct{ bot, root uuid.UUID }
	bindings := []tuple{}
	for _, b := range old {
		if err = s.qualifyOldBotGlobalRequest(ctx, b); err != nil {
			return err
		}
		bindings = append(bindings, tuple{b.bot, b.decision})
	}
	if minimal {
		rows, err := s.db().Query(ctx, "SELECT bot_id,decision_id FROM public.role_bot_global_terminals WHERE bot_id=$1 OR decision_id=$2 ORDER BY bot_id", bot, decision)
		if err != nil {
			return err
		}
		for rows.Next() {
			var b tuple
			if err = rows.Scan(&b.bot, &b.root); err != nil {
				rows.Close()
				return err
			}
			bindings = append(bindings, b)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	byBot := map[uuid.UUID]uuid.UUID{}
	byRoot := map[uuid.UUID]uuid.UUID{}
	for _, b := range bindings {
		if b.bot == uuid.Nil || b.root == uuid.Nil {
			return ErrScopeUnavailable
		}
		if root, ok := byBot[b.bot]; ok && root != b.root {
			return fmt.Errorf("%w: inconsistent retained Bot roots", ErrScopeUnavailable)
		}
		if bot, ok := byRoot[b.root]; ok && bot != b.bot {
			return fmt.Errorf("%w: inconsistent retained root Bots", ErrScopeUnavailable)
		}
		byBot[b.bot] = b.root
		byRoot[b.root] = b.bot
	}
	for _, b := range bindings {
		if b.bot != bot || b.root != decision {
			return ErrBotGlobalTerminalConflict
		}
	}
	return nil
}

func (s *RoleStore) qualifyOldBotGlobalRequest(ctx context.Context, b oldBotGlobalBinding) error {
	var actor uuid.UUID
	if err := s.db().QueryRow(ctx, "SELECT actor_profile_id FROM role_bot_actor_bindings WHERE bot_id=$1", b.bot).Scan(&actor); err != nil || actor != b.actor {
		return fmt.Errorf("%w: old actor provenance unavailable", ErrScopeUnavailable)
	}
	rows, err := s.db().Query(ctx, "SELECT operation_id,bot_id,actor_profile_id,space_id,installation_id,request_bytes,request_sha256 FROM role_bot_removal_operations WHERE bot_id=$1 ORDER BY operation_id", b.bot)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var op, bot, actor, space uuid.UUID
		var installation *uuid.UUID
		var wire, hash []byte
		if err = rows.Scan(&op, &bot, &actor, &space, &installation, &wire, &hash); err != nil {
			return err
		}
		r := &rolev1.ApplyBotInstallationRemovalRequest{}
		if err = proto.Unmarshal(wire, r); err != nil {
			return fmt.Errorf("%w: old request bytes unavailable", ErrScopeUnavailable)
		}
		if r.Purpose != "global_delete" || r.DecisionId != b.decision.String() {
			continue
		}
		parsed, err := validateBotRemoval(r)
		digest := sha256.Sum256(wire)
		if err != nil || !bytes.Equal(parsed.requestBytes, wire) || !bytes.Equal(hash, digest[:]) || len(b.hash) != 32 || !bytes.Equal(r.DecisionSha256, b.hash) || parsed.bot != b.bot || parsed.actor != b.actor || parsed.decision != b.decision || parsed.operation != op || parsed.bot != bot || parsed.actor != actor || parsed.space != space || (installation == nil) != (parsed.installation == uuid.Nil) || (installation != nil && *installation != parsed.installation) {
			return fmt.Errorf("%w: old global request provenance conflicts", ErrScopeUnavailable)
		}
		found = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: accepted old global request missing", ErrScopeUnavailable)
	}
	return nil
}
