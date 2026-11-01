package sourcecustody

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

type FenceConfig struct {
	Recovery RecoveryConfig
	Binding  Binding
}

func LoadFenceConfig(lookup func(string) (string, bool)) (FenceConfig, error) {
	var empty FenceConfig
	recovery, err := LoadRecoveryConfig(lookup)
	if err != nil {
		return empty, ErrUnavailable
	}
	historyText, okH := lookup("CUSTODY_HISTORY_GENERATION")
	sourceText, okU := lookup("CUSTODY_USER_SOURCE_GENERATION")
	digestText, okD := lookup("CUSTODY_KEYSET_DIGEST")
	history, errH := uuid.Parse(historyText)
	source, errU := uuid.Parse(sourceText)
	digest, errD := hex.DecodeString(digestText)
	if !okH || !okU || !okD || errH != nil || errU != nil || errD != nil || history == source || history.String() != historyText || source.String() != sourceText || len(digest) != 32 || hex.EncodeToString(digest) != digestText {
		return empty, ErrUnavailable
	}
	var bytes [32]byte
	copy(bytes[:], digest)
	binding := Binding{Epoch: recovery.Request.Epoch, SourceID: recovery.Request.SourceID, HistoryGeneration: history, UserSourceGeneration: source, KeysetDigest: bytes}
	if !binding.valid() {
		return empty, ErrUnavailable
	}
	return FenceConfig{Recovery: recovery, Binding: binding}, nil
}

// RunFence closes admission and fences the pinned physical source. It never
// qualifies or opens the source and assumes exclusive administrative custody.
func RunFence(parent context.Context, cfg FenceConfig) (result error) {
	if parent == nil || parent.Err() != nil || !cfg.Binding.valid() || cfg.Binding.Epoch != cfg.Recovery.Request.Epoch || cfg.Binding.SourceID != cfg.Recovery.Request.SourceID || cfg.Recovery.Physical.SystemIdentifier != cfg.Recovery.Request.SourceSystemID || cfg.Recovery.Physical.Database != cfg.Recovery.Request.Database || cfg.Recovery.Physical.DatabaseOID == 0 || !systemID.MatchString(cfg.Recovery.Request.ControlSystemID) || !systemID.MatchString(cfg.Recovery.Request.SourceSystemID) || cfg.Recovery.Request.ControlSystemID == cfg.Recovery.Request.SourceSystemID || !validControlURL(cfg.Recovery.ControlURL) || !validControlURL(cfg.Recovery.SourceAdminURL) || forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	control, err := openPinnedPostgres(ctx, cfg.Recovery.ControlURL)
	if err != nil {
		return ErrUnavailable
	}
	defer control.Close()
	var system string
	if control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&system) != nil || system != cfg.Recovery.Request.ControlSystemID {
		return ErrUnavailable
	}
	admin, err := openPinnedPostgres(ctx, cfg.Recovery.SourceAdminURL)
	if err != nil {
		return ErrUnavailable
	}
	defer admin.Close()
	controller, err := Start(ctx, control, cfg.Binding)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if controller.Shutdown(cleanup) != nil {
			result = ErrUnavailable
		}
	}()
	controller.mu.Lock()
	if controller.stopped || controller.leader == nil {
		err = ErrUnavailable
	} else {
		err = controller.leader.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&system)
	}
	controller.mu.Unlock()
	if err != nil || system != cfg.Recovery.Request.ControlSystemID {
		return ErrUnavailable
	}
	return controller.FenceSource(ctx, admin, cfg.Recovery.Physical)
}
