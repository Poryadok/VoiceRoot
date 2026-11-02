package sourcecustody

import (
	"context"

	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

// CaptureClient holds a stream across one CaptureProfiles operation. It is not
// source qualification or a fence for an independently committing transaction.
type CaptureClient struct {
	client  *Client
	binding Binding
	store   *identityassociation.Store
}

func NewCaptureClient(client *Client, binding Binding, store *identityassociation.Store) (*CaptureClient, error) {
	if client == nil || store == nil || !binding.valid() {
		return nil, ErrUnavailable
	}
	return &CaptureClient{client: client, binding: binding, store: store}, nil
}

func (c *CaptureClient) CaptureProfiles(ctx context.Context, ids []uuid.UUID) (identityassociation.CaptureBatchResult, error) {
	if c == nil || ctx == nil || ctx.Err() != nil || c.client == nil || c.store == nil {
		return identityassociation.CaptureBatchResult{}, ErrUnavailable
	}
	grant, err := c.client.AcquireQualified(ctx, c.binding)
	if err != nil || grant == nil {
		return identityassociation.CaptureBatchResult{}, ErrUnavailable
	}
	defer grant.Release()
	physical := grant.Physical()
	out, err := c.store.CaptureProfilesForPhysicalSource(grant.Context(), ids, c.binding.HistoryGeneration, c.binding.UserSourceGeneration, c.binding.KeysetDigest, identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID})
	if err != nil {
		return identityassociation.CaptureBatchResult{}, err
	}
	if grant.Context().Err() != nil {
		return identityassociation.CaptureBatchResult{}, ErrUnavailable
	}
	return out, nil
}
