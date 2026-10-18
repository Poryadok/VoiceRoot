package grpcsvc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"os"
	"testing"
	"time"
	"voice/backend/pkg/reporttargetprincipal"
	"voice/backend/user/internal/store"
)

type reportTargetProofDeletion struct{}

func (reportTargetProofDeletion) DeletedAmong(context.Context, []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	return map[uuid.UUID]struct{}{}, nil
}

// TestReportTargetProofServer is a bounded Linux-only subprocess fixture for
// Moderation's four-owner integration test. It uses the real handler/store/TLS.
func TestReportTargetProofServer(t *testing.T) {
	if os.Getenv("REPORT_TARGET_PROOF_SERVER") != "1" {
		t.Skip("four-owner subprocess fixture only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("REPORT_TARGET_PROOF_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	svc := &UserGRPC{Profiles: store.NewProfileStore(pool), DeletedAccounts: reportTargetProofDeletion{}}
	stop, err := reporttargetprincipal.Start(ctx, "user", func(server grpc.ServiceRegistrar) { RegisterReportTargetServer(server, svc) }, func(err error) { fmt.Fprintln(os.Stderr, err) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	fmt.Println("REPORT_TARGET_READY")
	<-ctx.Done()
}
