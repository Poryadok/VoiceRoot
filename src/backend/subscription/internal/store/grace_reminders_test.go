package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGraceDay_Days1_3_7(t *testing.T) {
	graceEnd := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	graceStart := graceEnd.Add(-7 * 24 * time.Hour)

	require.EqualValues(t, 1, GraceDay(graceStart.Add(time.Minute), graceEnd))
	require.EqualValues(t, 3, GraceDay(graceStart.Add(2*24*time.Hour+time.Hour), graceEnd))
	require.EqualValues(t, 7, GraceDay(graceStart.Add(6*24*time.Hour+time.Hour), graceEnd))
	require.EqualValues(t, 0, GraceDay(graceStart.Add(-time.Hour), graceEnd))
	require.EqualValues(t, 0, GraceDay(graceEnd.Add(time.Hour), graceEnd))
}

func TestShouldEmitGraceReminder(t *testing.T) {
	require.True(t, ShouldEmitGraceReminder(1, nil))
	require.False(t, ShouldEmitGraceReminder(2, nil))
	require.False(t, ShouldEmitGraceReminder(1, []int32{1}))
	require.True(t, ShouldEmitGraceReminder(3, []int32{1}))
}
