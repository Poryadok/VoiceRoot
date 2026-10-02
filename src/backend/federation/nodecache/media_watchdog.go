package nodecache

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var ErrMediaWatchdogUnavailable = errors.New("federated media watchdog unavailable")

// MediaParticipant is the minimum verified identity needed to recheck an
// active LiveKit participant against the current node authority snapshot.
type MediaParticipant struct {
	RoomName     string
	Identity     string
	AccountID    string
	ProfileID    string
	ResourceID   string
	SessionEpoch int64
}

type MediaParticipantLister interface {
	ListFederatedParticipants(context.Context) ([]MediaParticipant, error)
}

type MediaParticipantEjector interface {
	EjectFederatedParticipant(context.Context, string, string) error
}

// MediaWatchdog provides a bounded, independent sweep for already-connected
// media participants. Admission checks must also call Cache.Authorize; this
// sweep handles permission expiry and revocation after LiveKit admission.
type MediaWatchdog struct {
	Policy       *Cache
	Participants MediaParticipantLister
	Ejector      MediaParticipantEjector
	Interval     time.Duration
	ClockSkew    time.Duration
	OperationTTL time.Duration
	Now          func() time.Time
	OnError      func(error)
}

func (w *MediaWatchdog) validate() error {
	if w == nil || w.Policy == nil || w.Participants == nil || w.Ejector == nil || w.Now == nil ||
		w.Interval <= 0 || w.Interval > 250*time.Millisecond ||
		w.ClockSkew < 0 || w.ClockSkew > MaxClockUncertainty ||
		w.OperationTTL <= 0 || w.OperationTTL > time.Second {
		return ErrMediaWatchdogUnavailable
	}
	return nil
}

func (w *MediaWatchdog) Sweep(ctx context.Context) error {
	if err := w.validate(); err != nil || ctx == nil {
		return ErrMediaWatchdogUnavailable
	}
	participants, err := w.Participants.ListFederatedParticipants(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, participant := range participants {
		if participant.RoomName == "" || participant.Identity == "" {
			errs = append(errs, ErrPermissionDenied)
			continue
		}
		if err := w.Policy.Authorize(participant.AccountID, participant.ProfileID, participant.ResourceID, participant.SessionEpoch, "media", w.Now(), w.ClockSkew); err == nil {
			continue
		}
		ejectCtx, cancel := context.WithTimeout(ctx, w.OperationTTL)
		err := w.Ejector.EjectFederatedParticipant(ejectCtx, participant.RoomName, participant.Identity)
		cancel()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (w *MediaWatchdog) Run(ctx context.Context) error {
	if err := w.validate(); err != nil || ctx == nil {
		return ErrMediaWatchdogUnavailable
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		if err := w.Sweep(ctx); err != nil && ctx.Err() == nil {
			report := w.OnError
			if report == nil {
				report = func(err error) { slog.Default().Error("federated media watchdog sweep failed", "error", err) }
			}
			report(err)
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
		}
	}
}
