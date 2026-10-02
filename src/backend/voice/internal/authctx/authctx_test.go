package authctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestAccountIDReadsOnlyGatewayMetadataAndRejectsMissingOrBlank(t *testing.T) {
	account := "  account-1  "
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderAccountID, account))
	id, ok := AccountID(ctx)
	require.True(t, ok)
	require.Equal(t, "account-1", id)

	id, ok = AccountID(metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderAccountID, " ")))
	require.False(t, ok)
	require.Empty(t, id)
	_, ok = AccountID(context.Background())
	require.False(t, ok)
}

func TestSessionEpochRequiresOnePositiveGatewayValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []string
		want   int64
		ok     bool
	}{
		{name: "positive", values: []string{"17"}, want: 17, ok: true},
		{name: "missing"},
		{name: "blank", values: []string{" "}},
		{name: "malformed", values: []string{"1x"}},
		{name: "zero", values: []string{"0"}},
		{name: "negative", values: []string{"-1"}},
		{name: "duplicate", values: []string{"17", "18"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if len(tc.values) == 1 {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(HeaderSessionEpoch, tc.values[0]))
			} else if len(tc.values) > 1 {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(HeaderSessionEpoch, tc.values[0], HeaderSessionEpoch, tc.values[1]))
			}
			got, ok := SessionEpoch(ctx)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
