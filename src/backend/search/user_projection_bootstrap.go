package main

import (
	"context"
	"fmt"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/search/internal/profileprojection"
)

type projectionSource interface {
	BeginSearchProfileSnapshot(context.Context, *userv1.BeginSearchProfileSnapshotRequest, ...any) (*userv1.BeginSearchProfileSnapshotResponse, error)
}

// runUserProjectionBootstrap applies a point-in-time snapshot first, then the
// journal after its high watermark. Replays are safe because StoreAdapter
// enforces source revisions and exact payload hashes.
func runUserProjectionBootstrap(ctx context.Context, client userv1.UserServiceClient, adapter *profileprojection.StoreAdapter) error {
	if client == nil || adapter == nil {
		return fmt.Errorf("User projection client and store are required")
	}
	checkpoint, err := adapter.Checkpoint(ctx)
	if err != nil {
		return err
	}
	if checkpoint == 0 {
		begin, err := client.BeginSearchProfileSnapshot(ctx, &userv1.BeginSearchProfileSnapshotRequest{})
		if err != nil {
			return err
		}
		for cursor := ""; ; {
			page, err := client.ListSearchProfileSnapshot(ctx, &userv1.ListSearchProfileSnapshotRequest{HighWatermark: begin.GetHighWatermark(), PageSize: 100, Cursor: cursor})
			if err != nil {
				return err
			}
			for _, event := range page.GetEvents() {
				if _, err := adapter.Apply(ctx, event); err != nil {
					return err
				}
			}
			if page.GetNextCursor() == "" {
				checkpoint = begin.GetHighWatermark()
				break
			}
			cursor = page.GetNextCursor()
		}
	}
	for {
		page, err := client.ListSearchProfileJournal(ctx, &userv1.ListSearchProfileJournalRequest{AfterOffset: checkpoint, PageSize: 100})
		if err != nil {
			return err
		}
		for _, event := range page.GetEvents() {
			// Never advance the replay cursor after a separately committed
			// projection write. The adapter commits both facts together.
			if _, err := adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset()); err != nil {
				return err
			}
			if event.GetJournalOffset() > checkpoint {
				checkpoint = event.GetJournalOffset()
			}
		}
		// The initial empty snapshot and an empty journal page have no event to
		// atomically apply. Persisting their high watermark is safe because no
		// projection mutation is being acknowledged here.
		if len(page.GetEvents()) == 0 {
			if err := adapter.StoreCheckpoint(ctx, checkpoint); err != nil {
				return err
			}
		}
		if len(page.GetEvents()) < 100 {
			return nil
		}
	}
}
