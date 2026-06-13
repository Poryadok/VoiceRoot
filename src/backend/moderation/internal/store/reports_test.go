package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReportStore_NotConfigured(t *testing.T) {
	var s *ReportStore
	_, err := s.InsertReport(context.Background(), uuid.New(), "user", uuid.New(), "spam", nil, `{}`)
	require.ErrorIs(t, err, errStoreNotConfigured)

	_, err = s.CountReports24h(context.Background(), uuid.New())
	require.ErrorIs(t, err, errStoreNotConfigured)

	err = s.InsertAutoModLog(context.Background(), uuid.New(), "report_threshold", "shadow_ban", `{}`)
	require.ErrorIs(t, err, errStoreNotConfigured)

	_, err = s.ListReports(context.Background(), "pending", 10)
	require.ErrorIs(t, err, errStoreNotConfigured)
}
