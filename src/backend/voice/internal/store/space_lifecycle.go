package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SpaceProjectionTicket is server-owned, per-attempt dispatch metadata. Never
// reconstruct its gate incarnation from a serialized Call or upgrade it on retry.
type SpaceProjectionTicket struct {
	AdmissionLive                                                                                  bool
	AdmissionIncarnation                                                                           string
	AdmissionOwnerEpoch                                                                            int64
	Incarnation                                                                                    string
	Generation                                                                                     int64
	SpaceID, RoomID, VoiceRoomID, OperationID, AdmissionGeneration, AccountID, ProfileID, Identity string
	RoomGeneration                                                                                 uint64
}
type SpaceLifecycleLoader = func(context.Context, string) (int64, bool, error)
type SpaceAdmissionGateLoader = func(context.Context, string) (int64, bool, error)
type SpaceAdmissionProjectionGates interface {
	EnsureAdmissionDispatch(context.Context, string, SpaceAdmissionGateLoader) (SpaceProjectionTicket, error)
}
type SpaceProjectionGates interface {
	EnsureSpaceLifecycle(context.Context, string, SpaceLifecycleLoader) error
	ReadReadySpaceGate(context.Context, string) (SpaceProjectionTicket, error)
}
type spaceTicketContext struct{}
type spaceCleanupContext struct{}

func WithSpaceCleanupTarget(ctx context.Context, t SpaceProjectionTicket) context.Context {
	return context.WithValue(ctx, spaceCleanupContext{}, t)
}

func (s *RedisCallStore) requireAdmissionDenied(ctx context.Context, tx *redis.Tx, t SpaceProjectionTicket) error {
	if t.OperationID == "" || t.AdmissionIncarnation == "" || t.AdmissionOwnerEpoch <= 0 || t.AdmissionLive {
		return ErrSpaceMediaStaleGrant
	}
	raw, err := tx.Get(ctx, s.spaceAdmissionKey(t.OperationID)).Result()
	if err != nil {
		return ErrSpaceMediaStaleGrant
	}
	g, err := decodeSpaceGate(raw)
	if err != nil || g.Phase != "READY" || g.Live || g.Incarnation != t.AdmissionIncarnation || g.Generation != t.AdmissionOwnerEpoch {
		return ErrSpaceMediaStaleGrant
	}
	return nil
}

func WithSpaceProjectionTicket(ctx context.Context, t SpaceProjectionTicket) context.Context {
	return context.WithValue(ctx, spaceTicketContext{}, t)
}

type spaceGate struct {
	Version            int
	Incarnation, Phase string
	Generation         int64
	Live               bool
}

func decodeSpaceGate(raw string) (spaceGate, error) {
	var g spaceGate
	if json.Unmarshal([]byte(raw), &g) != nil || g.Version != 1 || g.Generation < 0 || (g.Phase != "READY" && g.Phase != "RESTORING") {
		return g, ErrSpaceMediaStaleGrant
	}
	id, err := uuid.Parse(g.Incarnation)
	if err != nil || id == uuid.Nil || id.String() != g.Incarnation {
		return g, ErrSpaceMediaStaleGrant
	}
	return g, nil
}
func encodeSpaceGate(g spaceGate) string { raw, _ := json.Marshal(g); return string(raw) }
func (s *RedisCallStore) spaceLifecycleKey(space string) string {
	return s.prefix + "space_lifecycle:" + space
}

// EnsureSpaceLifecycle is maintenance, never a projection authorization. Missing
// installs RESTORING before fresh PG authority. Publish CAS binds the exact nonce.
func (s *RedisCallStore) EnsureSpaceLifecycle(ctx context.Context, space string, load SpaceLifecycleLoader) error {
	if space == "" || load == nil {
		return ErrInvalidState
	}
	key := s.spaceLifecycleKey(space)
	for range redisTransitionAttempts {
		restoring := spaceGate{Version: 1, Incarnation: uuid.NewString(), Phase: "RESTORING"}
		if err := s.client.SetNX(ctx, key, encodeSpaceGate(restoring), 0).Err(); err != nil {
			return err
		}
		observed, err := s.client.Get(ctx, key).Result()
		if err != nil {
			return err
		}
		g, err := decodeSpaceGate(observed)
		if err != nil {
			return err
		}
		if g.Phase == "READY" {
			return nil
		}
		generation, live, err := load(ctx, space)
		if err != nil {
			return err
		}
		if generation < 0 {
			return ErrInvalidState
		}
		ready := g
		ready.Phase = "READY"
		ready.Generation = generation
		ready.Live = live
		err = s.client.Watch(ctx, func(tx *redis.Tx) error {
			current, err := tx.Get(ctx, key).Result()
			if err != nil {
				return err
			}
			if current != observed {
				return ErrSpaceMediaStaleGrant
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, encodeSpaceGate(ready), 0); return nil })
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) || errors.Is(err, ErrSpaceMediaStaleGrant) || errors.Is(err, redis.Nil) {
			continue
		}
		return err
	}
	return ErrMoveContention
}
func (s *RedisCallStore) ReadReadySpaceGate(ctx context.Context, space string) (SpaceProjectionTicket, error) {
	raw, err := s.client.Get(ctx, s.spaceLifecycleKey(space)).Result()
	if err != nil {
		return SpaceProjectionTicket{}, ErrSpaceMediaStaleGrant
	}
	g, err := decodeSpaceGate(raw)
	if err != nil || g.Phase != "READY" || !g.Live {
		return SpaceProjectionTicket{}, ErrSpaceMediaStaleGrant
	}
	return SpaceProjectionTicket{Incarnation: g.Incarnation, Generation: g.Generation, SpaceID: space}, nil
}

// SetSpaceLifecycle is ONLY InstallDeny. Projection callers cannot publish LIVE.
func (s *RedisCallStore) SetSpaceLifecycle(ctx context.Context, space string, generation int64, live bool) error {
	if space == "" || generation < 0 || live {
		return ErrInvalidState
	}
	key := s.spaceLifecycleKey(space)
	for range redisTransitionAttempts {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			old, err := tx.Get(ctx, key).Result()
			g := spaceGate{Version: 1, Incarnation: uuid.NewString()}
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if err == nil {
				g, err = decodeSpaceGate(old)
				if err != nil {
					return err
				}
				if g.Generation > generation {
					return ErrSpaceMediaStaleGrant
				}
			}
			g.Phase = "READY"
			g.Generation = generation
			g.Live = false
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, encodeSpaceGate(g), 0); return nil })
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		return err
	}
	return ErrMoveContention
}

// ReinitializeSpaceLifecycle is only for a current PG-authorized lifecycle or
// fenced infrastructure recovery. Every candidate gets a fresh gate incarnation.
func (s *RedisCallStore) ReinitializeSpaceLifecycle(ctx context.Context, space string, load SpaceLifecycleLoader) error {
	key := s.spaceLifecycleKey(space)
	for range redisTransitionAttempts {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			if _, err := tx.Get(ctx, key).Result(); err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			g := spaceGate{Version: 1, Incarnation: uuid.NewString(), Phase: "RESTORING"}
			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, encodeSpaceGate(g), 0); return nil })
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return err
		}
		return s.EnsureSpaceLifecycle(ctx, space, load)
	}
	return ErrMoveContention
}
func (s *RedisCallStore) requireSpaceLive(ctx context.Context, tx *redis.Tx, space string, generation int64, tickets ...SpaceProjectionTicket) error {
	t, _ := ctx.Value(spaceTicketContext{}).(SpaceProjectionTicket)
	if len(tickets) > 0 {
		t = tickets[0]
	}
	if t.Incarnation == "" || t.SpaceID != space || t.Generation != generation {
		return ErrSpaceMediaStaleGrant
	}
	raw, err := tx.Get(ctx, s.spaceLifecycleKey(space)).Result()
	if err != nil {
		return ErrSpaceMediaStaleGrant
	}
	g, err := decodeSpaceGate(raw)
	if err != nil || g.Phase != "READY" || !g.Live || g.Incarnation != t.Incarnation || g.Generation != generation {
		return ErrSpaceMediaStaleGrant
	}
	if t.OperationID == "" || t.AdmissionIncarnation == "" || t.AdmissionOwnerEpoch <= 0 {
		return ErrSpaceMediaStaleGrant
	}
	raw, err = tx.Get(ctx, s.spaceAdmissionKey(t.OperationID)).Result()
	if err != nil {
		return ErrSpaceMediaStaleGrant
	}
	g, err = decodeSpaceGate(raw)
	if err != nil || g.Phase != "READY" || !g.Live || g.Incarnation != t.AdmissionIncarnation || g.Generation != t.AdmissionOwnerEpoch {
		return ErrSpaceMediaStaleGrant
	}
	return nil
}

func (s *RedisCallStore) spaceAdmissionKey(operation string) string {
	return s.prefix + "space_admission_dispatch:" + operation
}

// EnsureAdmissionDispatch samples PG after the observed gate. A changed epoch
// or lost gate first installs a fresh RESTORING nonce, then samples PG again.
// A paused old publisher cannot overwrite a replacement gate, and a delayed
// dispatch keeps its old ticket rather than fetching the replacement nonce.
func (s *RedisCallStore) EnsureAdmissionDispatch(ctx context.Context, operation string, load SpaceAdmissionGateLoader) (SpaceProjectionTicket, error) {
	if operation == "" || load == nil {
		return SpaceProjectionTicket{}, ErrInvalidState
	}
	key := s.spaceAdmissionKey(operation)
	for range redisTransitionAttempts {
		restoring := spaceGate{Version: 1, Incarnation: uuid.NewString(), Phase: "RESTORING"}
		if err := s.client.SetNX(ctx, key, encodeSpaceGate(restoring), 0).Err(); err != nil {
			return SpaceProjectionTicket{}, err
		}
		observed, err := s.client.Get(ctx, key).Result()
		if err != nil {
			return SpaceProjectionTicket{}, err
		}
		g, err := decodeSpaceGate(observed)
		if err != nil {
			return SpaceProjectionTicket{}, err
		}
		epoch, live, err := load(ctx, operation)
		if err != nil || epoch <= 0 {
			if err == nil {
				err = ErrSpaceMediaStaleGrant
			}
			return SpaceProjectionTicket{}, err
		}
		if g.Phase == "READY" && g.Generation == epoch && g.Live == live {
			return SpaceProjectionTicket{OperationID: operation, AdmissionIncarnation: g.Incarnation, AdmissionOwnerEpoch: epoch, AdmissionLive: live}, nil
		}
		if g.Generation > epoch {
			return SpaceProjectionTicket{}, ErrSpaceMediaStaleGrant
		}
		if g.Phase != "RESTORING" {
			err = s.client.Watch(ctx, func(tx *redis.Tx) error {
				current, err := tx.Get(ctx, key).Result()
				if err != nil {
					return err
				}
				if current != observed {
					return redis.TxFailedErr
				}
				_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, encodeSpaceGate(restoring), 0); return nil })
				return err
			}, key)
			if errors.Is(err, redis.TxFailedErr) {
				continue
			}
			if err != nil {
				return SpaceProjectionTicket{}, err
			}
			observed = encodeSpaceGate(restoring)
			g = restoring
		}
		// This read is after RESTORING became visible, never authority cached
		// before a gate was lost or a takeover began.
		epoch, live, err = load(ctx, operation)
		if err != nil || epoch <= 0 {
			if err == nil {
				err = ErrSpaceMediaStaleGrant
			}
			return SpaceProjectionTicket{}, err
		}
		ready := g
		ready.Phase = "READY"
		ready.Generation = epoch
		ready.Live = live
		err = s.client.Watch(ctx, func(tx *redis.Tx) error {
			current, err := tx.Get(ctx, key).Result()
			if err != nil {
				return err
			}
			if current != observed {
				return redis.TxFailedErr
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, key, encodeSpaceGate(ready), 0); return nil })
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return SpaceProjectionTicket{}, err
		}
		return SpaceProjectionTicket{OperationID: operation, AdmissionIncarnation: ready.Incarnation, AdmissionOwnerEpoch: epoch, AdmissionLive: live}, nil
	}
	return SpaceProjectionTicket{}, ErrMoveContention
}
