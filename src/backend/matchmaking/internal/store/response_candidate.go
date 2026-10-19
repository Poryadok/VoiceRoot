package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	admission "voice/backend/matchmaking/internal/mmadmission"
)

type ResponseParams struct {
	MatchID, AccountID, ProfileID, OperationID uuid.UUID
	SessionEpoch                               int64
	RequestHash                                [32]byte
	Accept                                     bool
}
type ResponseCandidate struct {
	cut    ProvisionCandidate
	params ResponseParams
	hash   [32]byte
}

func (c ResponseCandidate) Hash() [32]byte         { return c.hash }
func (c ResponseCandidate) Parties() []SourceParty { return c.cut.Parties() }
func (c ResponseCandidate) Accounts() []uuid.UUID  { return c.cut.Accounts() }
func (c ResponseCandidate) Params() ResponseParams { return c.params }
func (s *MatchStore) CaptureResponseCandidate(ctx context.Context, params ResponseParams) (ResponseCandidate, error) {
	if s == nil || s.Pool == nil || s.NativeAdmission == nil {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	binding := copyNativeBinding(s.NativeAdmission)
	if binding.Validate() != nil {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	defer rollbackPartySearch(tx)
	candidate, err := readResponseCandidate(ctx, tx, params, binding)
	if err != nil {
		return ResponseCandidate{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	return candidate, nil
}

func readResponseCandidate(ctx context.Context, tx pgx.Tx, params ResponseParams, binding *admission.Binding) (ResponseCandidate, error) {
	if binding == nil || params.MatchID == uuid.Nil || params.AccountID == uuid.Nil || params.ProfileID == uuid.Nil || params.OperationID == uuid.Nil || params.SessionEpoch <= 0 || params.RequestHash == ([32]byte{}) {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	mode := responseRead
	if !params.Accept {
		mode = terminalResponseRead
	}
	cut, err := readMatchPartyCandidate(ctx, tx, params.MatchID, binding, mode)
	if err != nil || len(cut.Accounts()) == 0 {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	bound := false
	for _, party := range cut.parties {
		for _, member := range party.MemberAuthorities {
			if member.ProfileID == params.ProfileID && member.AccountID == params.AccountID {
				bound = true
			}
		}
	}
	if !bound {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	// The live responding device epoch is independent of original admission
	// epochs. Account/profile equality comes only from the captured private tuple.
	raw, err := json.Marshal(struct {
		Cut   [32]byte
		Actor ResponseParams
	}{cut.Hash(), params})
	if err != nil {
		return ResponseCandidate{}, ErrSourcePartyUnavailable
	}
	return ResponseCandidate{cut: cut, params: params, hash: sha256.Sum256(append([]byte("voice.mm.response-local-cut.v1\x00"), raw...))}, nil
}

// VerifyResponseCandidateTx performs no locking, RPC or commit. The transition
// owner must first hold admission, all-account, match, parties, sessions and
// proposal/receipt locks, then fresh ban checks, this verification and COMMIT.
func VerifyResponseCandidateTx(ctx context.Context, tx pgx.Tx, candidate ResponseCandidate) error {
	if tx == nil || candidate.hash == ([32]byte{}) {
		return ErrProvisionCandidateChanged
	}
	current, err := readResponseCandidate(ctx, tx, candidate.params, candidate.cut.binding)
	if err != nil || current.hash != candidate.hash {
		return ErrProvisionCandidateChanged
	}
	return nil
}
