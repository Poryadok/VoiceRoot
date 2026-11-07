package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/store"
)

type Legacy24VerifyConfig struct {
	Config
	PriorLegacy24      *Legacy24Receipt
	OriginEvidencePath string
}

// This is the actual committed initializer result, read without invoking any
// repair or producer. Every admitted original Space must appear exactly once.
func legacy24Initialization(ctx context.Context, conn *pgx.Conn, origin *legacy24OriginObservation, final *store.NativeInstallerSourceFacts) (_ string, err error) {
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return "", errors.New("HOLD: initializer result snapshot unavailable")
	}
	defer finishLegacy24Read(tx, &err)
	rows, err := tx.Query(ctx, `SELECT space_id::text,epoch::text,state,to_jsonb(s)::text FROM native_membership_guard.spaces s ORDER BY space_id::text COLLATE "C"`)
	if err != nil {
		return "", errors.New("HOLD: initializer result unavailable")
	}
	manifest := make([]json.RawMessage, 0, len(origin.Spaces))
	for rows.Next() {
		var id, epoch, state, raw string
		if err = rows.Scan(&id, &epoch, &state, &raw); err != nil || len(manifest) >= len(origin.Spaces) || id != origin.Spaces[len(manifest)] || epoch != final.Epoch || state != "COMPLETE" {
			rows.Close()
			return "", errors.New("HOLD: initializer does not cover the exact original Spaces")
		}
		manifest = append(manifest, json.RawMessage(raw))
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(manifest) != len(origin.Spaces) {
		return "", errors.New("HOLD: initializer result incomplete")
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", errors.New("HOLD: initializer result encoding failed")
	}
	if err = tx.Commit(ctx); err != nil {
		return "", errors.New("HOLD: initializer snapshot completion unknown")
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func loadLegacy24Origin(path string, prior *Legacy24Receipt) (_ *legacy24OriginObservation, err error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm() != 0600 {
		return nil, errors.New("HOLD: private original legacy24 evidence required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("HOLD: original legacy24 evidence unavailable")
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, errors.New("original legacy24 evidence close failed"))
		}
	}()
	actual, err := f.Stat()
	if err != nil || !os.SameFile(before, actual) {
		return nil, errors.New("HOLD: original legacy24 evidence identity changed")
	}
	return readLegacy24Origin(f, prior)
}

// VerifyLegacy24Upgrade uses the original installer image and privately retained
// original observation. It cannot create/adopt an origin, migrate, initialize or
// resume an incomplete prefix. Candidate compatibility is a separate contract.
func VerifyLegacy24Upgrade(ctx context.Context, c Legacy24VerifyConfig) (err error) {
	prior := c.PriorLegacy24
	if c.Output == nil || c.OriginEvidencePath == "" || !validLegacy24Completion(prior) {
		return errors.New("HOLD: actual completed legacy24 evidence and original sidecar required")
	}
	_, err = loadLegacy24Origin(c.OriginEvidencePath, prior)
	if err != nil {
		return err
	}
	s, err := openLegacy24Session(ctx, c.Config)
	if err != nil {
		return err
	}
	defer func() { err = s.close(err) }()
	current := s.receipt
	if prior.SourceRevision != current.SourceRevision || prior.SourceSHA256 != current.SourceSHA256 || prior.ExecutableSHA256 != current.ExecutableSHA256 || prior.MigratorSHA256 != current.MigratorSHA256 || prior.InventorySHA256 != current.InventorySHA256 {
		return errors.New("HOLD: original legacy24 installer identity changed")
	}
	facts, err := s.store.InspectNativeInstallerSource(ctx, 51)
	if err != nil || !sameTarget(prior.Final, facts) || prior.Final.SealSHA256 != facts.SealSHA256 {
		return errors.New("HOLD: retained legacy24 target or native51 seal changed")
	}
	// Preservation and initialization are historical completed-upgrade proofs.
	// Their original sidecar remains bound above, but legitimate serving writes
	// may change business rows, floors and guard state after that completion.
	// Later verification authenticates the original installation and current
	// full native51 catalog; it never reruns migration or initialization.
	if err = s.checkWindow(ctx); err != nil {
		return err
	}
	return emitLegacy24(c.Output, "verified-legacy24", prior)
}
