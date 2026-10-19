// Command report-journal-export captures finite opaque report journal bytes.
// It does not fence writers, assert a FINAL source cut or open service readiness.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/moderation/internal/reportrecovery"
	"voice/backend/moderation/internal/store"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	incarnationFlag := flag.String("store-incarnation", "", "independently verified moderation store incarnation UUID")
	operationFlag := flag.String("operation", "", "trusted offline capture operation UUID")
	parent := flag.String("output-parent", "", "existing private operator-owned output directory")
	timeout := flag.Duration("timeout", 5*time.Minute, "finite capture timeout (1s to 30m)")
	flag.Parse()
	incarnation, incarnationErr := uuid.Parse(*incarnationFlag)
	operation, operationErr := uuid.Parse(*operationFlag)
	if flag.NArg() != 0 || incarnationErr != nil || operationErr != nil || incarnation == uuid.Nil || operation == uuid.Nil || strings.TrimSpace(*parent) == "" || *timeout < time.Second || *timeout > 30*time.Minute {
		return errors.New("valid store-incarnation, operation, private output-parent and bounded timeout are required")
	}
	// Never print the database URL or its parse/connection error: it may contain
	// credentials. Only the fixed environment variable is accepted, not argv.
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	config.MaxConns, config.MinConns = 2, 0
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("database configuration failed")
	}
	defer pool.Close()
	reports := &store.ReportStore{Pool: pool}
	snapshot, err := reports.ExportReportRecoverySnapshot(ctx, incarnation, operation)
	if err != nil {
		return errors.New("finite report journal capture failed; no publication")
	}
	directory, err := reportrecovery.PublishDirectory(*parent, snapshot.Manifest, snapshot.Chunks, snapshot.Cut)
	if err != nil {
		return errors.New("report journal publication failed; verify the private directory and exact existing snapshot")
	}
	if err := json.NewEncoder(os.Stdout).Encode(struct {
		Directory string             `json:"directory"`
		Cut       reportrecovery.Cut `json:"cut"`
	}{Directory: directory, Cut: snapshot.Cut}); err != nil {
		return errors.New("capture metadata output failed")
	}
	return nil
}
