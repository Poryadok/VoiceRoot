package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAuditLogStore_NotConfigured(t *testing.T) {
	var s *AuditLogStore
	err := s.InsertAudit(context.Background(), uuid.New(), "action", "type", uuid.New(), `{}`)
	require.ErrorIs(t, err, errStoreNotConfigured)

	_, err = s.ListAuditLog(context.Background(), 10)
	require.ErrorIs(t, err, errStoreNotConfigured)
}
