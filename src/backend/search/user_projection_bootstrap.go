package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/search/internal/profileprojection"
)

// runUserProjectionBootstrap applies a point-in-time snapshot first, then the
// journal after its high watermark. Replays are safe because StoreAdapter
// enforces source revisions and exact payload hashes.
func runUserProjectionBootstrap(ctx context.Context, client userv1.UserServiceClient, adapter *profileprojection.StoreAdapter) error {
	if client == nil || adapter == nil {
		return fmt.Errorf("user projection client and store are required")
	}
	checkpoint, err := adapter.Checkpoint(ctx)
	if err != nil {
		return err
	}
	snapshot, err := adapter.SnapshotState(ctx)
	if err != nil {
		return err
	}
	if checkpoint == 0 || snapshot.Phase == "snapshot" {
		if snapshot.Phase != "snapshot" {
			begin, beginErr := client.BeginSearchProfileSnapshot(ctx, &userv1.BeginSearchProfileSnapshotRequest{})
			if beginErr != nil {
				return beginErr
			}
			if err := adapter.StartSnapshot(ctx, begin.GetHighWatermark()); err != nil {
				return err
			}
			snapshot = profileprojection.SnapshotState{Phase: "snapshot", HighWatermark: begin.GetHighWatermark()}
		}
		for cursor := snapshot.Cursor; ; {
			page, err := client.ListSearchProfileSnapshot(ctx, &userv1.ListSearchProfileSnapshotRequest{HighWatermark: snapshot.HighWatermark, PageSize: 100, Cursor: cursor})
			if err != nil {
				if cursor != "" && status.Code(err) == codes.InvalidArgument {
					if resetErr := adapter.ResetSnapshot(ctx); resetErr != nil {
						return resetErr
					}
					return runUserProjectionBootstrap(ctx, client, adapter)
				}
				return err
			}
			for _, event := range page.GetEvents() {
				// Snapshot pages contain durable journal events. Their projection,
				// inbox fence, and progress cursor must commit as one fact too.
				if _, err := adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset()); err != nil {
					return err
				}
				if event.GetJournalOffset() > checkpoint {
					checkpoint = event.GetJournalOffset()
				}
			}
			if page.GetNextCursor() == "" {
				if err := adapter.FinishSnapshot(ctx); err != nil {
					return err
				}
				break
			}
			if err := adapter.AdvanceSnapshotCursor(ctx, page.GetNextCursor()); err != nil {
				return err
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
		if len(page.GetEvents()) < 100 {
			return nil
		}
	}
}
