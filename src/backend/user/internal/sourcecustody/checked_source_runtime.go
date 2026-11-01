package sourcecustody

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"voice/backend/user/internal/identityassociation"
)

type CheckedSourceConfig struct {
	Recovery             RecoveryConfig
	Binding              Binding
	KeyManifest, KeysDir string
}

func LoadCheckedSourceConfig(lookup func(string) (string, bool)) (CheckedSourceConfig, error) {
	var empty CheckedSourceConfig
	recovery, err := LoadRecoveryConfig(lookup)
	if err != nil {
		return empty, ErrUnavailable
	}
	names := []string{"CUSTODY_HISTORY_GENERATION", "CUSTODY_USER_SOURCE_GENERATION", "CUSTODY_KEYSET_DIGEST", "CUSTODY_USER_KEY_MANIFEST", "CUSTODY_USER_KEYS_DIR"}
	values := make([]string, len(names))
	for i, name := range names {
		value, ok := lookup(name)
		if !ok || value == "" || strings.TrimSpace(value) != value {
			return empty, ErrUnavailable
		}
		values[i] = value
	}
	history, err := uuid.Parse(values[0])
	if err != nil || history.String() != values[0] {
		return empty, ErrUnavailable
	}
	generation, err := uuid.Parse(values[1])
	if err != nil || generation.String() != values[1] {
		return empty, ErrUnavailable
	}
	decoded, err := hex.DecodeString(values[2])
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != values[2] {
		return empty, ErrUnavailable
	}
	var digest [32]byte
	copy(digest[:], decoded)
	binding := Binding{Epoch: recovery.Request.Epoch, SourceID: recovery.Request.SourceID, HistoryGeneration: history, UserSourceGeneration: generation, KeysetDigest: digest}
	if !binding.valid() {
		return empty, ErrUnavailable
	}
	return CheckedSourceConfig{Recovery: recovery, Binding: binding, KeyManifest: values[3], KeysDir: values[4]}, nil
}

// RunCheckedSource publishes only a CLOSED control record for a quarantined
// existing identity source. A fresh signed opening remains mandatory.
func RunCheckedSource(ctx context.Context, cfg CheckedSourceConfig) error {
	if ctx == nil || ctx.Err() != nil || forbiddenPGEnvironment() || !validControlURL(cfg.Recovery.ControlURL) || !validControlURL(cfg.Recovery.SourceAdminURL) || !cfg.Binding.valid() {
		return ErrUnavailable
	}
	keys, err := identityassociation.LoadProductionKeyset(cfg.KeyManifest, cfg.KeysDir)
	if err != nil {
		return ErrUnavailable
	}
	digest, err := keys.CustodyDigest()
	if err != nil || digest != cfg.Binding.KeysetDigest {
		return ErrUnavailable
	}
	control, err := openPinnedPostgres(ctx, cfg.Recovery.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	admin, err := openPinnedPostgres(ctx, cfg.Recovery.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	var phase string
	if control.QueryRow(ctx, `SELECT phase FROM public.user_custody_creation WHERE singleton AND source_id=$1 AND epoch=$2`, cfg.Binding.SourceID, cfg.Binding.Epoch).Scan(&phase) != nil {
		return ErrUnavailable
	}
	if phase == "QUARANTINED_EXISTING" {
		if CheckQuarantinedIdentity(ctx, control, admin, cfg.Recovery.Request, cfg.Recovery.Physical, cfg.Binding, keys) != nil {
			return ErrUnavailable
		}
	} else if phase != "IDENTITY_CHECKED_CLOSED" {
		return ErrUnavailable
	}
	return MaterializeCheckedSource(ctx, control, admin, cfg.Recovery.Request, cfg.Recovery.Physical, cfg.Binding)
}
