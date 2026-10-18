package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"voice/backend/moderation/internal/reportintake"
	"voice/backend/moderation/internal/reporttargetclient"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/principal"
)

type reportValidationAuthority struct {
	Issuer   *principal.Issuer
	Subjects store.ReportSubjectResolver
	Privacy  store.ReportPrivacyAcquire
}

func reportTargetsConfigured() bool {
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "MODERATION_REPORT_TARGETS_") {
			return true
		}
	}
	return false
}

// The caller supplies Moderation's one canonical signer. JWKS publication is
// independently owned by the shared private mTLS authority server.
func newReportTargetRuntime(_ context.Context, _ *slog.Logger, issuer *principal.Issuer) (*reporttargetclient.Client, func(), error) {
	if !reportTargetsConfigured() {
		return nil, func() {}, nil
	}
	if issuer == nil {
		return nil, nil, errors.New("report target signing identity required")
	}
	return reporttargetclient.DialFromEnv(issuer)
}

// Compatibility for the pre-composition main. An enabled worker without the
// shared privacy gate remains closed; it cannot create a second ready flag.
func startReportValidation(ctx context.Context, reports *store.ReportStore, logger *slog.Logger) (func(), error) {
	if !reportTargetsConfigured() {
		return func() {}, nil
	}
	issuer, _, err := loadModerationIssuerFromEnv()
	if err != nil {
		return nil, err
	}
	return startReportValidationWithAuthority(ctx, reports, logger, nil, reportValidationAuthority{Issuer: issuer})
}

// The shared main passes the SAME issuer, subject store, generation gate and
// sole BE-046 validated hook. No reciprocal call is made under a SQL lock.
func startReportValidationWithAuthority(ctx context.Context, reports *store.ReportStore, logger *slog.Logger, validated store.ReportValidatedHook, authority reportValidationAuthority) (func(), error) {
	targets, closeTargets, err := newReportTargetRuntime(ctx, logger, authority.Issuer)
	if err != nil {
		return nil, err
	}
	if targets == nil {
		return closeTargets, nil
	}
	if reports == nil || reports.Subjects == nil || authority.Subjects == nil || authority.Privacy == nil {
		closeTargets()
		return nil, errors.New("shared report privacy authority required")
	}
	reports.Privacy = authority.Privacy
	publisher, err := reportintake.NewPublisher(strings.TrimSpace(os.Getenv("NATS_URL")))
	if err != nil {
		closeTargets()
		return nil, err
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	worker := &reportintake.Worker{Reports: reports, Targets: targets, Subjects: authority.Subjects, Privacy: authority.Privacy, Validated: validated, Publish: publisher.Publish}
	go func() { defer close(done); worker.Run(workerCtx, logger) }()
	return func() { cancel(); <-done; publisher.Close(); closeTargets() }, nil
}
