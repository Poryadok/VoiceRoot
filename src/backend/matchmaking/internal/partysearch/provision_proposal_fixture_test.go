package partysearch

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
	"voice/backend/matchmaking/internal/store"
)

// Native provisioning fixtures enter through the same qualified reservation
// boundary as the matcher. The deliberate legacy-NULL negative stays separate.
func createProvisionProposalFixture(t *testing.T, pool *pgxpool.Pool, matches *store.MatchStore, params store.CreateProposalParams) (store.CreateProposalResult, error) {
	t.Helper()
	candidate, err := matches.CaptureProposalCandidate(context.Background(), params)
	if err != nil {
		return store.CreateProposalResult{}, err
	}
	proof := validateNativeProposal(t, pool, candidate)
	return matches.CreateValidatedProposal(context.Background(), candidate, proof)
}
