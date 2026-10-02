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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/federation/protocol"
)

//go:embed migrations/000001_authority.sql
var migrationSQL string

//go:embed migrations/000002_q11_http_audit.sql
var auditMigrationSQL string

//go:embed migrations/000003_hosted_resources.sql
var hostedResourcesMigrationSQL string

//go:embed migrations/000004_snapshot_revisions.sql
var snapshotRevisionsMigrationSQL string

//go:embed migrations/000005_voice_room_bindings.sql
var voiceRoomBindingsMigrationSQL string

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
	for version, migration := range []string{migrationSQL, auditMigrationSQL, hostedResourcesMigrationSQL, snapshotRevisionsMigrationSQL, voiceRoomBindingsMigrationSQL} {
		version++
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM federation_schema_versions WHERE version=$1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err = tx.Exec(ctx, migration); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO federation_schema_versions(version) VALUES($1)`, version); err != nil {
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

type q11AuditRecord struct {
	ActorClass, ActorFingerprint, NodeID, SpaceID string
	Action, Result, ReasonCode, RequestID         string
	HTTPStatus                                    int
}

func (s *authorityStore) appendQ11Audit(ctx context.Context, record q11AuditRecord) error {
	if s == nil || s.Pool == nil {
		return errors.New("federation audit store unavailable")
	}
	var nodeID, spaceID any
	if record.NodeID != "" {
		nodeID = record.NodeID
	}
	if record.SpaceID != "" {
		spaceID = record.SpaceID
	}
	return s.transaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO federation_http_audit
			(id,actor_class,actor_fingerprint_sha256,target_node_id,target_space_id,action,result,http_status,reason_code,request_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, uuid.NewString(), record.ActorClass,
			record.ActorFingerprint, nodeID, spaceID, record.Action, record.Result,
			record.HTTPStatus, record.ReasonCode, record.RequestID)
		return err
	})
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
		if action == "approve" && n.Status != "pending" {
			if n.Status == "active" {
				return errQ11ApprovalConflict
			}
			return errConflict
		}
		if action == "rotate" && n.Status != "active" {
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
		if action == "approve" && isUniqueViolation(err) {
			return errQ11ApprovalConflict
		}
		return uniqueViolationAsConflict(err)
	})
	if err != nil {
		return credential{}, err
	}
	return result, err
}

func uniqueViolationAsConflict(err error) error {
	if isUniqueViolation(err) {
		return errConflict
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
		if err = snap.Validate(now); err != nil {
			return err
		}
		if err = validateRoutedPolicy(ctx, tx, node, space, snap); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if snap.Validate(now) != nil {
			return errForbidden
		}
		hash := digest(raw)
		_, err = tx.Exec(ctx, `UPDATE federation_placements SET revision=$3,snapshot=$4,snapshot_hash=$5,valid_until=$6,updated_at=clock_timestamp() WHERE space_id=$1 AND node_id=$2`, space, node, snap.Revision, raw, hash, time.UnixMilli(snap.ValidUntil))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO federation_snapshot_revision_events(space_id,node_id,revision,snapshot_hash,valid_until) VALUES($1,$2,$3,$4,$5)`, space, node, snap.Revision, hash, time.UnixMilli(snap.ValidUntil))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM federation_snapshot_revision_events WHERE space_id=$1 AND node_id=$2 AND revision <= $3-102`, space, node, snap.Revision)
		return err
	})
}

type leaseRequest struct {
	Revision int64  `json:"revision"`
	Hash     string `json:"hash"`
	Nonce    string `json:"nonce"`
}

func (s *authorityStore) issue(ctx context.Context, node, space, pin, secret string, ack *leaseRequest) (Envelope, error) {
	return s.issueSnapshot(ctx, node, space, pin, secret, ack, nil)
}

func (s *authorityStore) issuePage(ctx context.Context, node, space, pin, secret string, pageIndex int) (Envelope, error) {
	return s.issueSnapshot(ctx, node, space, pin, secret, nil, &pageIndex)
}

func (s *authorityStore) issueSnapshot(ctx context.Context, node, space, pin, secret string, ack *leaseRequest, pageIndex *int) (Envelope, error) {
	var envelope Envelope
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		n, now, err := authorizeNodeTx(ctx, tx, node, s.Environment, pin, secret)
		if err != nil {
			return err
		}
		var generation, revision int64
		var raw []byte
		var hash *string
		var until *time.Time
		if err = tx.QueryRow(ctx, `SELECT generation,revision,snapshot,snapshot_hash,valid_until FROM federation_placements WHERE space_id=$1 AND node_id=$2 FOR UPDATE`, space, node).Scan(&generation, &revision, &raw, &hash, &until); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errQ11ScopeMismatch
			}
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if !now.Before(n.Expiry) {
			return errQ11CredentialRevoked
		}
		if until == nil || hash == nil || !now.Before(*until) || revision < 1 {
			return errForbidden
		}
		var snap Snapshot
		if strictJSON(raw, &snap) != nil || snap.Validate(now) != nil || digest(raw) != *hash || snap.Revision != revision || snap.ValidUntil != until.UnixMilli() {
			return errForbidden
		}
		if err = validateRoutedPolicy(ctx, tx, node, space, snap); err != nil {
			return err
		}
		if err = refreshAuthorityClock(ctx, tx, &now, n.Expiry, *until); err != nil {
			return err
		}
		if ack == nil {
			scope := protocol.Scope{Issuer: s.Issuer, Environment: s.Environment, NodeID: node, SpaceID: space, Generation: generation, Epoch: n.Epoch}
			if pageIndex == nil {
				envelope, err = protocol.SignSnapshotManifest(s.Key, s.KeyID, scope, protocol.ManifestFor(snap), now)
				return err
			}
			pages, pageErr := protocol.SignSnapshotPages(s.Key, s.KeyID, scope, snap, now)
			if pageErr != nil || *pageIndex < 0 || *pageIndex >= len(pages) {
				return errInvalid
			}
			envelope = pages[*pageIndex]
			return nil
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
		}
		if err = refreshAuthorityClock(ctx, tx, &now, n.Expiry, *until); err != nil {
			return err
		}
		c := Claims{Version: 1, Kind: "lease", Issuer: s.Issuer, Audience: "voice-node", Environment: s.Environment, NodeID: node, SpaceID: space, Generation: generation, Epoch: n.Epoch, Revision: revision, IssuedAt: now.UnixMilli(), ExpiresAt: min(until.UnixMilli(), now.Add(2*time.Second).UnixMilli(), n.Expiry.UnixMilli()), Hash: *hash}
		envelope, err = signEnvelope(s.Key, s.KeyID, c)
		return err
	})
	return envelope, err
}

func (s *authorityStore) issueRevisionStream(ctx context.Context, node, space, pin, secret string, after int64) (Envelope, error) {
	if after < 0 {
		return Envelope{}, errInvalid
	}
	var envelope Envelope
	err := s.transaction(ctx, func(tx pgx.Tx) error {
		n, now, err := authorizeNodeTx(ctx, tx, node, s.Environment, pin, secret)
		if err != nil {
			return err
		}
		var generation, revision int64
		var hash *string
		var validUntil *time.Time
		var raw []byte
		if err = tx.QueryRow(ctx, `SELECT generation,revision,snapshot_hash,valid_until,snapshot FROM federation_placements WHERE space_id=$1 AND node_id=$2 FOR UPDATE`, space, node).Scan(&generation, &revision, &hash, &validUntil, &raw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errQ11ScopeMismatch
			}
			return err
		}
		if revision < 1 || hash == nil || validUntil == nil || !now.Before(*validUntil) {
			return errForbidden
		}
		if err = refreshAuthorityClock(ctx, tx, &now, n.Expiry, *validUntil); err != nil {
			return err
		}
		var snap Snapshot
		if strictJSON(raw, &snap) != nil || snap.Validate(now) != nil || digest(raw) != *hash || snap.Revision != revision || snap.ValidUntil != validUntil.UnixMilli() {
			return errForbidden
		}
		if err = validateRoutedPolicy(ctx, tx, node, space, snap); err != nil {
			return err
		}
		stream := protocol.RevisionStream{AfterRevision: after, CurrentRevision: revision, Events: []protocol.RevisionEvent{}}
		if after > revision {
			stream.ResnapshotRequired = true
		}
		var oldest int64
		if err = tx.QueryRow(ctx, `SELECT coalesce(min(revision),0) FROM federation_snapshot_revision_events WHERE space_id=$1 AND node_id=$2`, space, node).Scan(&oldest); err != nil {
			return err
		}
		if after > revision {
			stream.ResnapshotRequired = true
		} else if after < revision && (oldest == 0 || after < oldest-1) {
			stream.ResnapshotRequired = true
		} else if after < revision {
			rows, queryErr := tx.Query(ctx, `SELECT revision,snapshot_hash,(extract(epoch FROM valid_until)*1000)::bigint FROM federation_snapshot_revision_events WHERE space_id=$1 AND node_id=$2 AND revision>$3 ORDER BY revision LIMIT 101`, space, node, after)
			if queryErr != nil {
				return queryErr
			}
			for rows.Next() {
				var event protocol.RevisionEvent
				if err = rows.Scan(&event.Revision, &event.Hash, &event.ValidUntil); err != nil {
					rows.Close()
					return err
				}
				stream.Events = append(stream.Events, event)
			}
			rowsErr := rows.Err()
			rows.Close()
			if rowsErr != nil {
				return rowsErr
			}
			if len(stream.Events) > 100 || !contiguousRevisionEvents(after, revision, stream.Events) {
				stream.Events = []protocol.RevisionEvent{}
				stream.ResnapshotRequired = true
			}
		}
		// Placement and event queries can wait past the source or credential TTL.
		if err = refreshAuthorityClock(ctx, tx, &now, n.Expiry, *validUntil); err != nil {
			return err
		}
		expiresAt := min(validUntil.UnixMilli(), now.Add(2*time.Second).UnixMilli(), n.Expiry.UnixMilli())
		scope := protocol.Scope{Issuer: s.Issuer, Environment: s.Environment, NodeID: node, SpaceID: space, Generation: generation, Epoch: n.Epoch}
		envelope, err = protocol.SignRevisionStream(s.Key, s.KeyID, scope, stream, expiresAt, now)
		return err
	})
	return envelope, err
}

func refreshAuthorityClock(ctx context.Context, tx pgx.Tx, now *time.Time, credentialExpiry, sourceExpiry time.Time) error {
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(now); err != nil {
		return err
	}
	if !now.Before(credentialExpiry) {
		return errQ11CredentialRevoked
	}
	if !now.Before(sourceExpiry) {
		return errForbidden
	}
	return nil
}

func contiguousRevisionEvents(after, current int64, events []protocol.RevisionEvent) bool {
	if len(events) == 0 {
		return after == current
	}
	for i, event := range events {
		if event.Revision != after+int64(i)+1 || event.ValidUntil <= 0 {
			return false
		}
	}
	return events[len(events)-1].Revision == current
}

func authorizeNodeTx(ctx context.Context, tx pgx.Tx, node, environment, pin, secret string) (nodeState, time.Time, error) {
	n, err := lockNode(ctx, tx, node, environment)
	if err != nil {
		return nodeState{}, time.Time{}, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nodeState{}, time.Time{}, err
	}
	if n.Status == "suspended" || n.Status == "defederated" {
		return nodeState{}, time.Time{}, errQ11CredentialRevoked
	}
	if n.Status != "active" {
		return nodeState{}, time.Time{}, errForbidden
	}
	if n.Pin != pin {
		var belongsToOtherNode bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM federation_nodes WHERE environment=$1 AND status='active' AND id<>$2 AND certificate_sha256=$3 AND credential_hash=$4)`, environment, node, pin, digest([]byte(secret))).Scan(&belongsToOtherNode); err != nil {
			return nodeState{}, time.Time{}, err
		}
		if belongsToOtherNode {
			return nodeState{}, time.Time{}, errQ11ScopeMismatch
		}
		return nodeState{}, time.Time{}, errQ11NodeCertificateMismatch
	}
	if !now.Before(n.Expiry) {
		return nodeState{}, time.Time{}, errQ11CredentialRevoked
	}
	if subtle.ConstantTimeCompare([]byte(n.Hash), []byte(digest([]byte(secret)))) != 1 {
		return nodeState{}, time.Time{}, errForbidden
	}
	return n, now, nil
}
