package grpcsvc

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/space/internal/store"
)

// acquire waits only for the supplied space's ownership transition. It keeps a
// reference while waiting so a canceled waiter cannot delete a live lock.
func (l *ownershipTransferLocker) acquire(ctx context.Context, spaceID string) (func(), error) {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[string]*ownershipTransferLock)
	}
	entry := l.locks[spaceID]
	if entry == nil {
		entry = &ownershipTransferLock{semaphore: make(chan struct{}, 1)}
		l.locks[spaceID] = entry
	}
	entry.refs++
	l.mu.Unlock()

	select {
	case entry.semaphore <- struct{}{}:
		return func() { l.release(spaceID, entry) }, nil
	case <-ctx.Done():
		l.releaseReference(spaceID, entry)
		return nil, ctx.Err()
	}
}

func (l *ownershipTransferLocker) release(spaceID string, entry *ownershipTransferLock) {
	<-entry.semaphore
	l.releaseReference(spaceID, entry)
}

func (l *ownershipTransferLocker) releaseReference(spaceID string, entry *ownershipTransferLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.refs--
	if entry.refs == 0 && l.locks[spaceID] == entry {
		delete(l.locks, spaceID)
	}
}

// lockSpaceMutation holds the per-space mutation gate. It keeps every write
// out of the temporary owner window between TransferOwnership's database leg
// and its Role/audit legs. Waiting remains bounded by the request context.
func (s *SpaceGRPC) lockSpaceMutation(ctx context.Context, spaceID uuid.UUID) (func(), error) {
	releaseLocal, err := s.ownershipTransfers.acquire(ctx, spaceID.String())
	if err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if s.MutationLocker == nil {
		return releaseLocal, nil
	}

	releaseDistributed, err := s.MutationLocker.Acquire(ctx, spaceID)
	if err != nil {
		releaseLocal()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, status.FromContextError(err).Err()
		}
		return nil, status.Error(codes.Unavailable, "space mutation lock unavailable: "+err.Error())
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			releaseDistributed()
			releaseLocal()
		})
	}, nil
}

// ownershipTransferCleanupContext preserves request values while detaching
// cancellation/deadline and bounding compensation work.
func ownershipTransferCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return store.BoundedCleanupContext(ctx)
}
