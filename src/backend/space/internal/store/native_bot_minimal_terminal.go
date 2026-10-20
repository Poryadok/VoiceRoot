package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

// RegisterBotGlobalTerminal records only the receiver-owned all-Add fence.
// Bot asserts its already committed accepted root; no Space, human identity,
// cleanup receipt or personal-derived root digest belongs in this effect.
func (s *SpaceStore) RegisterBotGlobalTerminal(ctx context.Context, proof *botmembershipprincipal.VerifiedGlobalTerminal, request *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.BotGlobalTerminalReceipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || request == nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	r := proto.Clone(request).(*spacev1.RegisterBotGlobalTerminalRequest)
	binding, err := proof.Check(r)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockBotInstallationOperations(ctx, tx, []uuid.UUID{binding.DecisionID}); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 48 {
		return nil, ErrMemberAuthorityUnavailable
	}
	var receipt uuid.UUID
	var registered time.Time
	if err = tx.QueryRow(ctx, `SELECT receipt_id,registered_at FROM public.space_bot_global_terminal_register($1,$2)`, binding.BotID, binding.DecisionID).Scan(&receipt, &registered); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrMemberAdmissionConflict
		}
		return nil, ErrMemberAuthorityUnavailable
	}
	if receipt == uuid.Nil || registered.IsZero() {
		return nil, ErrMemberAuthorityUnavailable
	}
	out := &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: binding.BotID.String(), DecisionId: binding.DecisionID.String(), ReceiptId: receipt.String(), RegisteredAt: timestamppb.New(registered), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}
	if err = out.RegisteredAt.CheckValid(); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	return out, nil
}
