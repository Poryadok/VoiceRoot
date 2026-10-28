// Package inventory pulls authenticated private protective evidence. It cannot
// activate the global reference ledger or supply release authority.
package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/file/internal/store"
	inv "voice/backend/pkg/referenceinventory"
)

type Source interface {
	BeginReferenceInventory(context.Context, *commonv1.BeginReferenceInventoryRequest) (*commonv1.BeginReferenceInventoryResponse, error)
	ReadReferenceInventoryPage(context.Context, *commonv1.ReadReferenceInventoryPageRequest) (*commonv1.ReadReferenceInventoryPageResponse, error)
	ReadReferenceInventoryChanges(context.Context, *commonv1.ReadReferenceInventoryChangesRequest) (*commonv1.ReadReferenceInventoryChangesResponse, error)
}

type Importer struct {
	Files  *store.FilesStore
	Source Source
}

// Run makes bounded progress on an ALREADY DURABLE operation. Every remote call
// is outside File transactions. A restart retries the same operation and page;
// it never changes identity, drops protection or treats source absence as zero.
func (i *Importer) Run(ctx context.Context, operation uuid.UUID) error {
	if i == nil || i.Files == nil || i.Source == nil {
		return inv.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	o, err := i.Files.LoadInventoryOperation(ctx, operation)
	if err != nil {
		return err
	}
	if o.State == "BLOCKED" {
		return inv.ErrConflict
	}
	if o.Descriptor == nil {
		response, e := i.Source.BeginReferenceInventory(ctx, &commonv1.BeginReferenceInventoryRequest{OperationId: operation.String(), Identity: o.Identity})
		if e != nil {
			return e
		}
		if inv.Strict(response) != nil {
			return i.Files.BlockInventoryOperation(ctx, operation, "invalid begin response")
		}
		if e = i.Files.SaveInventoryDescriptor(ctx, operation, response.GetDescriptor_()); e != nil {
			return e
		}
	}
	for pages := 0; pages < 4; pages++ {
		o, err = i.Files.LoadInventoryOperation(ctx, operation)
		if err != nil {
			return err
		}
		if o.State == "BLOCKED" {
			return inv.ErrConflict
		}
		if o.State != "SNAPSHOT" {
			break
		}
		response, e := i.Source.ReadReferenceInventoryPage(ctx, &commonv1.ReadReferenceInventoryPageRequest{Identity: o.Identity, PageIndex: o.NextPage})
		if e != nil {
			return e
		}
		if inv.Strict(response) != nil {
			return i.Files.BlockInventoryOperation(ctx, operation, "invalid page response")
		}
		if e = i.Files.ApplyInventoryPage(ctx, operation, response.Page); e != nil {
			return e
		}
	}
	o, err = i.Files.LoadInventoryOperation(ctx, operation)
	if err != nil {
		return err
	}
	if o.State != "CATCHING_UP" {
		return nil
	}
	response, err := i.Source.ReadReferenceInventoryChanges(ctx, &commonv1.ReadReferenceInventoryChangesRequest{Identity: o.Identity, AfterRevision: o.Cursor, Limit: 1000})
	if err != nil {
		return err
	}
	return i.Files.ApplyInventoryChanges(ctx, operation, o.Cursor, response)
}

// RunPendingOnce never creates a migration from configuration or startup IDs.
// Multiple workers may fetch concurrently; immutable receipts serialize apply.
func (i *Importer) RunPendingOnce(ctx context.Context, limit int) (int, error) {
	if i == nil || i.Files == nil || i.Source == nil {
		return 0, inv.ErrUnavailable
	}
	operations, err := i.Files.PendingInventoryOperations(ctx, limit)
	if err != nil {
		return 0, err
	}
	var result error
	var done int
	for _, op := range operations {
		if ctx.Err() != nil {
			return done, errors.Join(result, ctx.Err())
		}
		if err = i.Run(ctx, op); err != nil {
			result = errors.Join(result, err)
		} else {
			done++
		}
	}
	return done, result
}
