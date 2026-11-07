package sourcecustody

import (
	"context"
	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

// ReadAuthProfiles holds qualified custody through the read transaction's commit.
// It never invokes CaptureProfiles or repairs missing historical associations.
func (c *CaptureClient) ReadAuthProfiles(ctx context.Context, account uuid.UUID, ids []uuid.UUID) (identityassociation.AuthProfileBatch, error) {
	if c == nil || ctx == nil || ctx.Err() != nil || c.client == nil || c.store == nil {
		return identityassociation.AuthProfileBatch{}, ErrUnavailable
	}
	grant, err := c.client.AcquireQualified(ctx, c.binding)
	if err != nil || grant == nil {
		return identityassociation.AuthProfileBatch{}, ErrUnavailable
	}
	defer grant.Release()
	physical := grant.Physical()
	out, err := c.store.ReadAuthProfiles(grant.Context(), account, ids, identityassociation.AuthProfileSource{HistoryGeneration: c.binding.HistoryGeneration, UserSourceGeneration: c.binding.UserSourceGeneration, KeysetDigest: c.binding.KeysetDigest, Physical: identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID}})
	if err != nil {
		return identityassociation.AuthProfileBatch{}, err
	}
	if grant.Context().Err() != nil {
		return identityassociation.AuthProfileBatch{}, ErrUnavailable
	}
	return out, nil
}
