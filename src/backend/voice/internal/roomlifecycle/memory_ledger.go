package roomlifecycle

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ledgerState uint8

const (
	ledgerPending ledgerState = iota + 1
	ledgerCompleted
)

type ledgerRecord struct {
	actorProfileID uuid.UUID
	operationID    uuid.UUID
	method         Method
	fingerprint    string
	ownerToken     string
	state          ledgerState
	receipt        Receipt
	completedAt    time.Time
}

type MemoryLedger struct {
	mu      sync.Mutex
	clock   func() time.Time
	records map[string]ledgerRecord
}

func NewMemoryLedger(clock func() time.Time) *MemoryLedger {
	if clock == nil {
		clock = time.Now
	}
	return &MemoryLedger{clock: clock, records: make(map[string]ledgerRecord)}
}

func (l *MemoryLedger) Begin(_ context.Context, binding Binding) (Reservation, *Receipt, error) {
	if l == nil || !validBinding(binding) {
		return Reservation{}, nil, status.Error(codes.FailedPrecondition, "valid operation binding required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	key := memoryLedgerKey(binding)
	record, exists := l.records[key]
	if exists && record.state == ledgerCompleted && l.clock().UTC().After(record.completedAt.Add(receiptRetention)) {
		delete(l.records, key)
		exists = false
	}
	if exists {
		if !sameBoundOperation(record.method, record.fingerprint, binding) {
			return Reservation{}, nil, status.Error(codes.AlreadyExists, "operation id is bound to another request")
		}
		switch record.state {
		case ledgerPending:
			return Reservation{}, nil, status.Error(codes.FailedPrecondition, "operation is pending reconciliation")
		case ledgerCompleted:
			receipt := record.receipt
			return Reservation{}, &receipt, nil
		default:
			return Reservation{}, nil, status.Error(codes.Unavailable, "operation ledger state is invalid")
		}
	}

	ownerToken, err := newOwnerToken()
	if err != nil {
		return Reservation{}, nil, status.Error(codes.Unavailable, "operation reservation unavailable")
	}
	record = ledgerRecord{
		actorProfileID: binding.actorProfileID,
		operationID:    binding.operationID,
		method:         binding.method,
		fingerprint:    binding.fingerprint,
		ownerToken:     ownerToken,
		state:          ledgerPending,
	}
	l.records[key] = record
	return reservationFor(binding, ownerToken), nil, nil
}

func (l *MemoryLedger) Complete(_ context.Context, reservation Reservation, receipt Receipt) error {
	if l == nil || !validReservation(reservation) {
		return status.Error(codes.FailedPrecondition, "valid operation reservation required")
	}
	if !validReceipt(receipt) {
		return status.Error(codes.InvalidArgument, "valid canonical operation receipt required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	key := memoryReservationKey(reservation)
	record, exists := l.records[key]
	if !exists || !sameReservation(record, reservation) {
		return status.Error(codes.FailedPrecondition, "operation reservation fence mismatch")
	}
	switch record.state {
	case ledgerPending:
		record.state = ledgerCompleted
		record.receipt = receipt
		record.completedAt = l.clock().UTC()
		l.records[key] = record
		return nil
	case ledgerCompleted:
		if record.receipt != receipt {
			return status.Error(codes.AlreadyExists, "operation already completed with another result")
		}
		return nil
	default:
		return status.Error(codes.Unavailable, "operation ledger state is invalid")
	}
}

func memoryLedgerKey(binding Binding) string {
	return binding.actorProfileID.String() + ":" + binding.operationID.String()
}

func memoryReservationKey(reservation Reservation) string {
	return reservation.actorProfileID.String() + ":" + reservation.operationID.String()
}
