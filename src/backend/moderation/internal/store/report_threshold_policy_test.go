package store

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportThresholdUsesSafeIntegerCeiling(t *testing.T) {
	for _, test := range []struct{ audience, threshold int64 }{{1, 10}, {999, 10}, {1000, 10}, {1001, 11}, {1100, 11}, {1101, 12}, {math.MaxInt64, 92233720368547759}} {
		threshold, err := ReportThreshold(test.audience)
		require.NoError(t, err)
		require.Equal(t, test.threshold, threshold)
	}
	for _, invalid := range []int64{0, -1, math.MinInt64} {
		threshold, err := ReportThreshold(invalid)
		require.Error(t, err)
		require.Zero(t, threshold)
	}
}
