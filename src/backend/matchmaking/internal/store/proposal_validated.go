package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ProposalValidationProof interface{ Matches(ProposalCandidate) bool }

func (s *MatchStore) CreateValidatedProposal(ctx context.Context, candidate ProposalCandidate, proof ProposalValidationProof) (CreateProposalResult, error) {
	if candidate.Hash() == [32]byte{} || proof == nil || !proof.Matches(candidate) || len(candidate.Accounts()) == 0 {
		return CreateProposalResult{}, ErrSourcePartyUnavailable
	}
	return s.createProposal(ctx, candidate.params, &candidate, proof)
}

// CreateProposal preserves the pre-cut legacy path. Typed captured parties
// require CreateValidatedProposal and cannot select this path by omitting proof.
func (s *MatchStore) CreateProposal(ctx context.Context, p CreateProposalParams) (CreateProposalResult, error) {
	return s.createProposal(ctx, p, nil, nil)
}

func requireUntypedProposalTx(ctx context.Context, tx pgx.Tx, parties []uuid.UUID) error {
	var typed bool
	// to_jsonb also works against the real pre-cut schema without native columns.
	// SQL NULL (not JSON null) denotes an absent authority field here.
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM parties p WHERE id=ANY($1::uuid[]) AND
 ((to_jsonb(p)->>'search_operation_id') IS NOT NULL OR
 (to_jsonb(p)->>'voice_snapshot_protocol_version') IS NOT NULL OR
 (to_jsonb(p)->>'voice_source_kind') IS NOT NULL OR
 (to_jsonb(p)->>'initiator_account_id') IS NOT NULL OR
 (to_jsonb(p)->>'voice_member_authority_version') IS NOT NULL))`, parties).Scan(&typed)
	if err != nil || typed {
		return ErrSourcePartyUnavailable
	}
	return nil
}
