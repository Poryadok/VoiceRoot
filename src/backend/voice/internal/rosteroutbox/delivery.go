package rosteroutbox

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var ErrStaleLease = errors.New("roster outbox lease changed")

type Item struct {
	ID, LeaseToken uuid.UUID
	Payload        []byte
}
type Store struct{ Pool *pgxpool.Pool }

func (s *Store) Claim(ctx context.Context) (*Item, error) {
	var item Item
	err := s.Pool.QueryRow(ctx, `UPDATE voice_roster_outbox SET lease_token=$1,lease_until=clock_timestamp()+interval '15 seconds'
WHERE event_id=(SELECT event_id FROM voice_roster_outbox WHERE delivered_at IS NULL AND next_attempt_at<=clock_timestamp()
AND (lease_until IS NULL OR lease_until<=clock_timestamp()) ORDER BY occurred_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1)
RETURNING event_id,lease_token,payload`, uuid.New()).Scan(&item.ID, &item.LeaseToken, &item.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &item, err
}
func (s *Store) Delivered(ctx context.Context, item Item) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE voice_roster_outbox SET delivered_at=clock_timestamp(),lease_token=NULL,lease_until=NULL WHERE event_id=$1 AND lease_token=$2 AND delivered_at IS NULL AND lease_until>clock_timestamp()`, item.ID, item.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}
func (s *Store) Retry(ctx context.Context, item Item, after time.Duration) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE voice_roster_outbox SET lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp()+$3::interval WHERE event_id=$1 AND lease_token=$2 AND delivered_at IS NULL AND lease_until>clock_timestamp()`, item.ID, item.LeaseToken, after.String())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

type Publisher interface {
	PublishAdmissionEvent(context.Context, uuid.UUID, string, []byte) error
}
type Relay struct {
	Store     *Store
	Publisher Publisher
	OnError   func(error)
}

func (r *Relay) DispatchOnce(ctx context.Context) (bool, error) {
	item, err := r.Store.Claim(ctx)
	if err != nil || item == nil {
		return false, err
	}
	send, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = r.Publisher.PublishAdmissionEvent(send, item.ID, Subject, item.Payload)
	cancel()
	// Completion survives caller cancellation, still bounded and exact-lease.
	finish, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	if err != nil {
		return true, errors.Join(err, r.Store.Retry(finish, *item, 5*time.Second))
	}
	return true, r.Store.Delivered(finish, *item)
}
func (r *Relay) Run(ctx context.Context) error {
	if r == nil || r.Store == nil || r.Store.Pool == nil || r.Publisher == nil {
		return errors.New("roster outbox relay unavailable")
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if _, err := r.DispatchOnce(ctx); err != nil && r.OnError != nil {
				r.OnError(err)
			}
		}
	}
}
