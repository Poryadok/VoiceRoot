package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/000001_authority.sql
var migrationSQL string

func migrate(ctx context.Context, p *pgxpool.Pool) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(76392911)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS federation_schema_versions(version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM federation_schema_versions WHERE version=1)`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err = tx.Exec(ctx, migrationSQL); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO federation_schema_versions(version) VALUES(1)`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type authorityStore struct {
	Pool                       *pgxpool.Pool
	Key                        ed25519.PrivateKey
	KeyID, Issuer, Environment string
}

const nodeCredentialEntropyBytes = 32

type nodeState struct {
	Status, Pin, Hash string
	Epoch             int64
	Expiry            time.Time
}

func lockNode(ctx context.Context, tx pgx.Tx, id, env string) (nodeState, error) {
	var n nodeState
	err := tx.QueryRow(ctx, `SELECT status,certificate_sha256,coalesce(credential_hash,''),epoch,coalesce(credential_expires_at,'epoch'::timestamptz) FROM federation_nodes WHERE id=$1 AND environment=$2 FOR UPDATE`, id, env).Scan(&n.Status, &n.Pin, &n.Hash, &n.Epoch, &n.Expiry)
	if errors.Is(err, pgx.ErrNoRows) {
		err = errForbidden
	}
	return n, err
}
func (s *authorityStore) transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *authorityStore) enroll(ctx context.Context, id, operator, endpoint, pin string) error {
	tag, err := s.Pool.Exec(ctx, `INSERT INTO federation_nodes(id,operator_id,environment,endpoint,certificate_sha256,status) VALUES($1,$2,$3,$4,$5,'pending') ON CONFLICT DO NOTHING`, id, operator, s.Environment, endpoint, pin)
	if err == nil && tag.RowsAffected() != 1 {
		return errConflict
	}
	return err
}

type credential struct {
	Secret    string `json:"credential"`
	ExpiresAt int64  `json:"expires_at"`
}

func (s *authorityStore) changeNode(ctx context.Context, id, action, pin, actor string) (credential, error) {
	if action == "rotate" && !validPin(pin) {
		return credential{}, errInvalid
	}
	var result credential
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		n, err := lockNode(ctx, tx, id, s.Environment)
		if err != nil {
			return err
		}
		if action == "suspend" || action == "defederate" {
			status := "suspended"
			if action == "defederate" {
				status = "defederated"
			}
			if n.Status == "defederated" && status != "defederated" {
				return errConflict
			}
			_, err = tx.Exec(ctx, `UPDATE federation_nodes SET status=$2,epoch=epoch+1,credential_hash=NULL,credential_expires_at=NULL,updated_at=clock_timestamp() WHERE id=$1`, id, status)
			return err
		}
		if (action == "approve" && n.Status != "pending") || (action == "rotate" && n.Status != "active") {
			return errConflict
		}
		if pin == "" {
			pin = n.Pin
		}
		secret := make([]byte, nodeCredentialEntropyBytes)
		if _, err = rand.Read(secret); err != nil {
			return err
		}
		result.Secret = base64.RawURLEncoding.EncodeToString(secret)
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		expiry := now.Add(24 * time.Hour)
		result.ExpiresAt = expiry.UnixMilli()
		_, err = tx.Exec(ctx, `UPDATE federation_nodes SET status='active',certificate_sha256=$2,credential_hash=$3,credential_expires_at=$4,approved_by=$5,epoch=epoch+1,updated_at=clock_timestamp() WHERE id=$1`, id, pin, digest([]byte(result.Secret)), expiry, actor)
		return uniqueViolationAsConflict(err)
	})
	if err != nil {
		return credential{}, err
	}
	return result, err
}

func uniqueViolationAsConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errConflict
	}
	return err
}
func (s *authorityStore) place(ctx context.Context, node, space string) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		n, err := lockNode(ctx, tx, node, s.Environment)
		if err != nil {
			return err
		}
		if n.Status != "active" {
			return errForbidden
		}
		tag, err := tx.Exec(ctx, `INSERT INTO federation_placements(space_id,node_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, space, node)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			return nil
		}
		var owner string
		if err = tx.QueryRow(ctx, `SELECT node_id::text FROM federation_placements WHERE space_id=$1`, space).Scan(&owner); err != nil {
			return err
		}
		if owner != node {
			return errConflict
		}
		return nil
	})
}
func (s *authorityStore) publish(ctx context.Context, node, space string, snap Snapshot) error {
	return s.transaction(ctx, func(tx pgx.Tx) error {
		n, err := lockNode(ctx, tx, node, s.Environment)
		if err != nil {
			return err
		}
		if n.Status != "active" {
			return errForbidden
		}
		var revision int64
		var previous []byte
		if err = tx.QueryRow(ctx, `SELECT revision,snapshot FROM federation_placements WHERE space_id=$1 AND node_id=$2 FOR UPDATE`, space, node).Scan(&revision, &previous); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errForbidden
			}
			return err
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if revision == snap.Revision && digest(previous) == digest(raw) {
			return nil
		}
		if snap.Revision != revision+1 {
			return errConflict
		}
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if err = snap.validate(now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE federation_placements SET revision=$3,snapshot=$4,snapshot_hash=$5,valid_until=$6,updated_at=clock_timestamp() WHERE space_id=$1 AND node_id=$2`, space, node, snap.Revision, raw, digest(raw), time.UnixMilli(snap.ValidUntil))
		return err
	})
}

type leaseRequest struct {
	Revision int64  `json:"revision"`
	Hash     string `json:"hash"`
	Nonce    string `json:"nonce"`
}

func (s *authorityStore) issue(ctx context.Context, node, space, pin, secret string, ack *leaseRequest) (Envelope, error) {
	var envelope Envelope
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		n, err := lockNode(ctx, tx, node, s.Environment)
		if err != nil {
			return err
		}
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if n.Status != "active" || n.Pin != pin || !now.Before(n.Expiry) || subtle.ConstantTimeCompare([]byte(n.Hash), []byte(digest([]byte(secret)))) != 1 {
			return errForbidden
		}
		var generation, revision int64
		var raw []byte
		var hash *string
		var until *time.Time
		if err = tx.QueryRow(ctx, `SELECT generation,revision,snapshot,snapshot_hash,valid_until FROM federation_placements WHERE space_id=$1 AND node_id=$2 FOR UPDATE`, space, node).Scan(&generation, &revision, &raw, &hash, &until); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errForbidden
			}
			return err
		}
		if until == nil || hash == nil || !now.Before(*until) || revision < 1 {
			return errForbidden
		}
		c := Claims{Version: 1, Kind: "snapshot", Issuer: s.Issuer, Audience: "voice-node", Environment: s.Environment, NodeID: node, SpaceID: space, Generation: generation, Epoch: n.Epoch, Revision: revision, IssuedAt: now.UnixMilli(), ExpiresAt: until.UnixMilli(), Hash: *hash}
		if ack == nil {
			var snap Snapshot
			if err = strictJSON(raw, &snap); err != nil {
				return err
			}
			c.Snapshot = &snap
		} else {
			if ack.Revision != revision || ack.Hash != *hash || !canonicalID(ack.Nonce) {
				return errConflict
			}
			// Nonces survive credential lifetime, so replay cannot renew after a lease expires.
			if _, err = tx.Exec(ctx, `DELETE FROM federation_lease_nonces WHERE node_id=$1 AND expires_at<clock_timestamp()`, node); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `INSERT INTO federation_lease_nonces(node_id,nonce,expires_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, node, ack.Nonce, n.Expiry)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return errConflict
			}
			c.Kind = "lease"
			c.ExpiresAt = min(c.ExpiresAt, now.Add(2*time.Second).UnixMilli(), n.Expiry.UnixMilli())
		}
		envelope, err = signEnvelope(s.Key, s.KeyID, c)
		return err
	})
	return envelope, err
}
