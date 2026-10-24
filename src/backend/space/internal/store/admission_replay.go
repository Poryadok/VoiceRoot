package store

import (
	"context"
	"time"
)

// ClaimAdmissionPrincipalReplay is the explicit admission ingress nonce API.
// Gateway issuer/JTI remains globally single-use across Space purposes; sharing
// the existing committed ledger does not allow a Creator credential to pass
// the admission verifier's independent exact method/body/type/epoch checks.
func (s *SpaceStore) ClaimAdmissionPrincipalReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	return s.claimGatewayPrincipalReplay(ctx, issuer, jti, expiry)
}

// CleanupAdmissionPrincipalReplays supports an admission-only runtime without
// making cleanup contingent on enabling the Creator ingress. Both entrypoints
// use the same bounded expiry policy and SKIP LOCKED maintenance operation.
func (s *SpaceStore) CleanupAdmissionPrincipalReplays(ctx context.Context, limit int) (int64, error) {
	return s.cleanupGatewayPrincipalReplays(ctx, limit)
}
