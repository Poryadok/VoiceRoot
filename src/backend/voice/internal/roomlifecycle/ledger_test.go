package roomlifecycle

import (
	"context"
	"encoding/hex"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const completedReceiptRetention = 24 * time.Hour

type ledgerUnderTest interface {
	Begin(context.Context, Binding) (Reservation, *Receipt, error)
	Complete(context.Context, Reservation, Receipt) error
}

func TestMemoryLedger_BeginKeysByActorAndOperationAndBindsMethodAndRequest(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	ledger := NewMemoryLedger(func() time.Time { return now })
	binding := newLedgerBinding(t, now)

	reservation, replay, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Nil(t, replay)
	require.Equal(t, binding.ActorProfileID(), reservation.ActorProfileID())
	require.Equal(t, binding.OperationID(), reservation.OperationID())
	require.Equal(t, binding.Method(), reservation.Method())
	require.Equal(t, binding.Fingerprint(), reservation.Fingerprint())
	requireOwnerToken(t, reservation.OwnerToken())

	_, _, err = ledger.Begin(context.Background(), binding)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "identical pending operation must not gain a second owner")

	changedRequest := binding
	changedRequest.fingerprint = "sha256:" + strings.Repeat("f", 64)
	_, _, err = ledger.Begin(context.Background(), changedRequest)
	require.Equal(t, codes.AlreadyExists, status.Code(err))

	changedMethod := binding
	changedMethod.method = Method("LEAVE")
	_, _, err = ledger.Begin(context.Background(), changedMethod)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "method must be a bound value, not part of the key")

	sameActorOtherOperation := binding
	sameActorOtherOperation.operationID = uuid.New()
	otherOperationReservation, _, err := ledger.Begin(context.Background(), sameActorOtherOperation)
	require.NoError(t, err)
	require.NotEqual(t, reservation.OwnerToken(), otherOperationReservation.OwnerToken())

	sameOperationOtherActor := newLedgerBinding(t, now)
	sameOperationOtherActor.operationID = binding.operationID
	otherActorReservation, _, err := ledger.Begin(context.Background(), sameOperationOtherActor)
	require.NoError(t, err)
	require.NotEqual(t, reservation.OwnerToken(), otherActorReservation.OwnerToken())
}

func TestMemoryLedger_CompleteUsesOwnerFenceAndIsIdempotent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	ledger := NewMemoryLedger(func() time.Time { return now })
	binding := newLedgerBinding(t, now)
	reservation, _, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	receipt := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}

	wrongOwner := reservation
	wrongOwner.ownerToken = strings.Repeat("0", len(reservation.OwnerToken()))
	require.Equal(t, codes.FailedPrecondition, status.Code(ledger.Complete(context.Background(), wrongOwner, receipt)))

	wrongBinding := reservation
	wrongBinding.fingerprint = "sha256:" + strings.Repeat("e", 64)
	require.Equal(t, codes.FailedPrecondition, status.Code(ledger.Complete(context.Background(), wrongBinding, receipt)))

	require.NoError(t, ledger.Complete(context.Background(), reservation, receipt))
	require.NoError(t, ledger.Complete(context.Background(), reservation, receipt), "same completion must be idempotent")

	different := receipt
	different.VoiceSessionID = uuid.NewString()
	require.Equal(t, codes.AlreadyExists, status.Code(ledger.Complete(context.Background(), reservation, different)))

	secondReservation, replay, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Zero(t, secondReservation)
	require.Equal(t, &receipt, replay)
}

func TestLedger_CompleteFencesEveryBindingAndReceiptFieldWithoutChangingState(t *testing.T) {
	backends := []struct {
		name string
		new  func(*testing.T) ledgerUnderTest
	}{
		{"memory", func(t *testing.T) ledgerUnderTest { return NewMemoryLedger(time.Now) }},
		{"redis", newRedisTestLedger},
	}
	reservationChanges := []struct {
		name   string
		change func(*Reservation)
	}{
		{"actor key", func(r *Reservation) { r.actorProfileID = uuid.New() }},
		{"operation key", func(r *Reservation) { r.operationID = uuid.New() }},
		{"method binding", func(r *Reservation) { r.method = Method("LEAVE") }},
		{"request fingerprint", func(r *Reservation) { r.fingerprint = "sha256:" + strings.Repeat("c", 64) }},
		{"owner token", func(r *Reservation) { r.ownerToken = strings.Repeat("0", len(r.ownerToken)) }},
	}
	receiptChanges := []struct {
		name   string
		change func(*Receipt)
	}{
		{"voice session id", func(r *Receipt) { r.VoiceSessionID = uuid.NewString() }},
	}

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			for _, tc := range reservationChanges {
				t.Run("reservation "+tc.name, func(t *testing.T) {
					ledger := backend.new(t)
					binding := newLedgerBinding(t, time.Now().UTC())
					reservation, _, err := ledger.Begin(context.Background(), binding)
					require.NoError(t, err)
					receipt := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}
					altered := reservation
					tc.change(&altered)
					require.Equal(t, codes.FailedPrecondition, status.Code(ledger.Complete(context.Background(), altered, receipt)))
					require.NoError(t, ledger.Complete(context.Background(), reservation, receipt), "rejected fence must leave the original pending owner intact")
					_, replay, err := ledger.Begin(context.Background(), binding)
					require.NoError(t, err)
					require.Equal(t, &receipt, replay)
				})
			}
			for _, tc := range receiptChanges {
				t.Run("receipt "+tc.name, func(t *testing.T) {
					ledger := backend.new(t)
					binding := newLedgerBinding(t, time.Now().UTC())
					reservation, _, err := ledger.Begin(context.Background(), binding)
					require.NoError(t, err)
					original := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}
					require.NoError(t, ledger.Complete(context.Background(), reservation, original))
					altered := original
					tc.change(&altered)
					require.Equal(t, codes.AlreadyExists, status.Code(ledger.Complete(context.Background(), reservation, altered)))
					_, replay, err := ledger.Begin(context.Background(), binding)
					require.NoError(t, err)
					require.Equal(t, &original, replay, "conflicting completion must not overwrite the durable receipt")
				})
			}
		})
	}
}

func TestLedger_InvalidReceiptFailsBeforeMutation(t *testing.T) {
	backends := []struct {
		name string
		new  func(*testing.T) ledgerUnderTest
	}{
		{"memory", func(t *testing.T) ledgerUnderTest { return NewMemoryLedger(time.Now) }},
		{"redis", newRedisTestLedger},
	}
	validSessionID := uuid.NewString()
	invalidReceipts := []struct {
		name    string
		receipt Receipt
	}{
		{"empty outcome", Receipt{VoiceSessionID: validSessionID}},
		{"arbitrary outcome", Receipt{Outcome: OperationOutcome("joined"), VoiceSessionID: validSessionID}},
		{"credential shaped outcome", Receipt{Outcome: OperationOutcome("eyJhbGciOiJSUzI1NiJ9.payload.signature"), VoiceSessionID: validSessionID}},
		{"empty session", Receipt{Outcome: JoinSucceeded}},
		{"malformed session", Receipt{Outcome: JoinSucceeded, VoiceSessionID: "not-a-uuid"}},
		{"nil session", Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.Nil.String()}},
		{"credential shaped session", Receipt{Outcome: JoinSucceeded, VoiceSessionID: "eyJhbGciOiJSUzI1NiJ9.payload.signature"}},
		{"noncanonical session", Receipt{Outcome: JoinSucceeded, VoiceSessionID: bytesToUpperUUID(validSessionID)}},
	}

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			for _, tc := range invalidReceipts {
				t.Run(tc.name, func(t *testing.T) {
					ledger := backend.new(t)
					binding := newLedgerBinding(t, time.Now().UTC())
					reservation, _, err := ledger.Begin(context.Background(), binding)
					require.NoError(t, err)
					require.Equal(t, codes.InvalidArgument, status.Code(ledger.Complete(context.Background(), reservation, tc.receipt)))
					_, replay, err := ledger.Begin(context.Background(), binding)
					require.Equal(t, codes.FailedPrecondition, status.Code(err), "invalid receipt must leave the reservation pending")
					require.Nil(t, replay)
					valid := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}
					require.NoError(t, ledger.Complete(context.Background(), reservation, valid))
				})
			}
		})
	}
}

func TestMemoryLedger_ConcurrentBeginHasExactlyOneOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	ledger := NewMemoryLedger(func() time.Time { return now })
	binding := newLedgerBinding(t, now)

	const callers = 64
	start := make(chan struct{})
	results := make(chan error, callers)
	owners := make(chan string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			reservation, _, err := ledger.Begin(context.Background(), binding)
			if err == nil {
				owners <- reservation.OwnerToken()
			}
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(owners)

	var successes, pendingConflicts int
	for err := range results {
		switch status.Code(err) {
		case codes.OK:
			successes++
		case codes.FailedPrecondition:
			pendingConflicts++
		default:
			t.Fatalf("unexpected Begin error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, callers-1, pendingConflicts)
	require.Len(t, owners, 1)
}

func TestMemoryLedger_CompletedReceiptRetentionBoundaryAndReplayDoesNotResetIt(t *testing.T) {
	completedAt := time.Unix(1_700_000_000, 0).UTC()
	now := completedAt
	ledger := NewMemoryLedger(func() time.Time { return now })
	binding := newLedgerBinding(t, completedAt)
	reservation, _, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	receipt := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}
	require.NoError(t, ledger.Complete(context.Background(), reservation, receipt))

	now = completedAt.Add(23 * time.Hour)
	_, replay, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, &receipt, replay)

	now = completedAt.Add(completedReceiptRetention)
	_, replay, err = ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, &receipt, replay, "receipt must remain available for the full 24-hour boundary")

	now = completedAt.Add(completedReceiptRetention + time.Nanosecond)
	newReservation, replay, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)
	require.Nil(t, replay)
	require.NotEmpty(t, newReservation.OwnerToken(), "earlier replay must not extend retention")
}

func TestMemoryLedger_PendingReservationNeverExpiresOrAllowsTakeover(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	ledger := NewMemoryLedger(func() time.Time { return now })
	binding := newLedgerBinding(t, now)
	reservation, _, err := ledger.Begin(context.Background(), binding)
	require.NoError(t, err)

	now = now.Add(100 * 365 * 24 * time.Hour)
	_, _, err = ledger.Begin(context.Background(), binding)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	receipt := Receipt{Outcome: JoinSucceeded, VoiceSessionID: uuid.NewString()}
	require.NoError(t, ledger.Complete(context.Background(), reservation, receipt), "time alone must not invalidate the original owner fence")
}

func TestLedgerReceiptHasNoCredentialOrGrantSurface(t *testing.T) {
	typ := reflect.TypeOf(Receipt{})
	for i := range typ.NumField() {
		name := strings.ToLower(typ.Field(i).Name)
		for _, forbidden := range []string{"jwt", "token", "grant", "permission", "authority"} {
			require.NotContains(t, name, forbidden)
		}
	}
	require.Equal(t, 2, typ.NumField(), "receipt is limited to a stable outcome and voice session ID")
}

func newLedgerBinding(t *testing.T, now time.Time) Binding {
	t.Helper()
	decision := authorizeBindingDecision(t, validPrincipal(uuid.New(), uuid.New(), now.Add(time.Hour)), uuid.New(), uuid.New(), now)
	binding, err := BindJoin(decision, uuid.NewString())
	require.NoError(t, err)
	return binding
}

func requireOwnerToken(t *testing.T, token string) {
	t.Helper()
	decoded, err := hex.DecodeString(token)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(decoded), 16, "owner token must contain at least 128 bits of randomness")
}
