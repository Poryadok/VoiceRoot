package main

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/sourcecustody"
	"voice/backend/user/internal/store"
)

type spaceCaptureRuntimeConfig struct {
	client            sourcecustody.ClientConfig
	jwks              sourcecustody.UserJWKSConfig
	auth              identityassociation.AuthParentRuntimeConfig
	binding           sourcecustody.Binding
	manifest, keysDir string
}

// loadSpaceCaptureRuntimeConfig accepts only a complete, explicitly pinned
// capture configuration. None of these settings asserts source custody.
func loadSpaceCaptureRuntimeConfig(lookup func(string) (string, bool)) (spaceCaptureRuntimeConfig, error) {
	var cfg spaceCaptureRuntimeConfig
	client, enabled, err := sourcecustody.LoadClientConfig(lookup)
	if err != nil || !enabled {
		return cfg, errors.New("capture custody client configuration required")
	}
	jwks, enabled, err := sourcecustody.LoadUserJWKSConfig(lookup)
	if err != nil || !enabled {
		return cfg, errors.New("capture User JWKS configuration required")
	}
	auth, enabled, err := identityassociation.LoadAuthParentRuntimeConfig(lookup)
	if err != nil || !enabled {
		return cfg, errors.New("capture Auth parent configuration required")
	}
	names := []string{"USER_CUSTODY_EPOCH", "USER_CUSTODY_SOURCE_ID", "USER_CUSTODY_HISTORY_GENERATION", "USER_CUSTODY_USER_SOURCE_GENERATION", "USER_CUSTODY_KEYSET_DIGEST", "USER_IDENTITY_KEY_MANIFEST", "USER_IDENTITY_KEYS_DIR"}
	values := make([]string, len(names))
	for i, name := range names {
		value, ok := lookup(name)
		if !ok || value == "" || strings.TrimSpace(value) != value {
			return cfg, errors.New("capture source configuration incomplete")
		}
		values[i] = value
	}
	epoch, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || epoch <= 0 || strconv.FormatInt(epoch, 10) != values[0] {
		return cfg, errors.New("capture source epoch invalid")
	}
	history, err := uuid.Parse(values[2])
	if err != nil || history == uuid.Nil || history.String() != values[2] {
		return cfg, errors.New("capture history generation invalid")
	}
	generation, err := uuid.Parse(values[3])
	if err != nil || generation == uuid.Nil || generation == history || generation.String() != values[3] {
		return cfg, errors.New("capture source generation invalid")
	}
	digest, err := hex.DecodeString(values[4])
	if err != nil || len(digest) != 32 || hex.EncodeToString(digest) != values[4] {
		return cfg, errors.New("capture keyset digest invalid")
	}
	var pinned [32]byte
	copy(pinned[:], digest)
	if pinned == [32]byte{} {
		return cfg, errors.New("capture keyset digest invalid")
	}
	binding := sourcecustody.Binding{Epoch: epoch, SourceID: values[1], HistoryGeneration: history, UserSourceGeneration: generation, KeysetDigest: pinned}
	if !binding.Valid() {
		return cfg, errors.New("capture source binding invalid")
	}
	return spaceCaptureRuntimeConfig{client: client, jwks: jwks, auth: auth, binding: binding, manifest: values[5], keysDir: values[6]}, nil
}

type spaceCaptureRuntime struct {
	client  *sourcecustody.ClientRuntime
	jwks    *sourcecustody.UserJWKSRuntime
	auth    *identityassociation.AuthParentRuntime
	keys    *identityassociation.Keyset
	binding sourcecustody.Binding
}

// startSpaceCaptureRuntime exposes the User public keys before waiting for
// custody, breaking the authenticated custody/JWKS startup dependency cycle.
func startSpaceCaptureRuntime(ctx context.Context, cfg spaceCaptureRuntimeConfig) (*spaceCaptureRuntime, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.New("capture startup unavailable")
	}
	keys, err := identityassociation.LoadProductionKeyset(cfg.manifest, cfg.keysDir)
	if err != nil {
		return nil, errors.New("capture keyset unavailable")
	}
	digest, err := keys.CustodyDigest()
	if err != nil || digest != cfg.binding.KeysetDigest {
		return nil, errors.New("capture keyset binding mismatch")
	}
	client, err := sourcecustody.OpenClient(ctx, cfg.client)
	if err != nil {
		return nil, errors.New("capture custody client unavailable")
	}
	jwks, err := sourcecustody.StartUserJWKS(ctx, cfg.jwks.Address, cfg.jwks.CertFile, cfg.jwks.KeyFile, client.JWKS())
	if err != nil {
		_ = client.Close()
		return nil, errors.New("capture User JWKS unavailable")
	}
	runtime := &spaceCaptureRuntime{client: client, jwks: jwks, keys: keys, binding: cfg.binding}
	auth, err := identityassociation.OpenAuthParentRuntime(ctx, cfg.auth, client.Issuer())
	if err != nil {
		runtime.Close()
		return nil, errors.New("capture Auth parent unavailable")
	}
	runtime.auth = auth
	return runtime, nil
}

func (r *spaceCaptureRuntime) Close() {
	if r == nil {
		return
	}
	if r.auth != nil {
		_ = r.auth.Close()
	}
	if r.jwks != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = r.jwks.Close(ctx)
		cancel()
	}
	if r.client != nil {
		_ = r.client.Close()
	}
}

// waitAdmission keeps the JWKS listener available while the independently
// operated custody service starts and opens its source. No User RPC is ready
// until a qualified stream has actually been received.
func (r *spaceCaptureRuntime) waitAdmission(ctx context.Context) error {
	if r == nil || r.client == nil || r.jwks == nil || ctx == nil {
		return errors.New("capture startup unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	for {
		if ctx.Err() != nil {
			return errors.New("capture startup unavailable")
		}
		grant, err := r.client.Client().AcquireQualified(ctx, r.binding)
		if err == nil {
			grant.Release()
			if ctx.Err() != nil {
				return errors.New("capture startup unavailable")
			}
			select {
			case <-r.jwks.Done():
				return errors.New("capture User JWKS stopped")
			default:
				return nil
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("capture startup unavailable")
		case <-r.jwks.Done():
			timer.Stop()
			return errors.New("capture User JWKS stopped")
		case <-timer.C:
		}
	}
}

// bind checks the actual database and identity state under a live qualified
// custody grant before installing the same guarded profile writer everywhere.
func (r *spaceCaptureRuntime) bind(ctx context.Context, pool *pgxpool.Pool) (*store.ProfileStore, *sourcecustody.CaptureClient, error) {
	if r == nil || ctx == nil || ctx.Err() != nil || pool == nil || r.client == nil || r.auth == nil || r.keys == nil {
		return nil, nil, errors.New("capture startup unavailable")
	}
	identities, err := identityassociation.NewStore(pool, r.keys, r.binding.HistoryGeneration)
	if err != nil {
		return nil, nil, errors.New("capture identity store unavailable")
	}
	grant, err := r.client.Client().AcquireQualified(ctx, r.binding)
	if err != nil {
		return nil, nil, errors.New("qualified User capture source unavailable")
	}
	defer grant.Release()
	physical := grant.Physical()
	tx, err := identities.Begin(grant.Context())
	if err != nil {
		return nil, nil, errors.New("qualified User capture source unavailable")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	actual := identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID}
	if identities.ValidateForActivationTx(grant.Context(), tx, r.binding.HistoryGeneration, r.binding.UserSourceGeneration, r.binding.KeysetDigest, actual) != nil || tx.Commit(grant.Context()) != nil || grant.Context().Err() != nil {
		return nil, nil, errors.New("qualified User capture source unavailable")
	}
	parents, err := identityassociation.NewParentVerifier(r.auth.Client)
	if err != nil {
		return nil, nil, errors.New("capture Auth parent unavailable")
	}
	profiles, err := store.NewProfileStoreWithBirthCapture(pool, identities, parents)
	if err != nil {
		return nil, nil, errors.New("guarded profile store unavailable")
	}
	capture, err := sourcecustody.NewCaptureClient(r.client.Client(), r.binding, identities)
	if err != nil {
		return nil, nil, errors.New("capture client unavailable")
	}
	return profiles, capture, nil
}
