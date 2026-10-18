package automodevidence

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func TestPattern_KeyedAccountScopeUnicodeFramingAndLinks(t *testing.T) {
	ring, err := NewRing(map[string][]byte{"current": bytes.Repeat([]byte{1}, 32), "next": bytes.Repeat([]byte{2}, 32)})
	require.NoError(t, err)
	subject := uuid.New()
	one, err := ring.Project(subject, " \u2003ПРИВЕТ\t ", "", false)
	require.NoError(t, err)
	same, err := ring.Project(subject, "привет", "", false)
	require.NoError(t, err)
	require.Equal(t, one, same)
	require.Len(t, one.Digests, 2)
	other, err := ring.Project(uuid.New(), "привет", "", false)
	require.NoError(t, err)
	require.NotEqual(t, one.Digests[0], other.Digests[0])
	shifted, err := ring.Project(subject, "", "привет", false)
	require.NoError(t, err)
	require.NotEqual(t, one.Digests[0], shifted.Digests[0], "field boundaries and order are part of the pattern")
	spaced, err := ring.Project(subject, "при вет", "", false)
	require.NoError(t, err)
	require.NotEqual(t, one.Digests[0], spaced.Digests[0], "internal whitespace is not collapsed")
	links, err := ring.Project(subject, "HTTP://one HTTPS://two", "https://three", false)
	require.NoError(t, err)
	require.EqualValues(t, 3, links.LinkCount)
	encrypted, err := ring.Project(subject, "https://one https://two https://three", "", true)
	require.NoError(t, err)
	require.Equal(t, automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_E2E, encrypted.Eligibility)
	require.Empty(t, encrypted.Digests)
	require.Zero(t, encrypted.LinkCount)
	empty, err := ring.Project(subject, " \t\n", "\u2003", false)
	require.NoError(t, err)
	require.Equal(t, automodv1.EvidenceEligibility_EVIDENCE_ELIGIBILITY_NO_TEXT, empty.Eligibility)
	require.Empty(t, empty.Digests)
}
func TestPattern_RotationCannotSilentlyResetLiveHistory(t *testing.T) {
	_, err := NewRing(map[string][]byte{"current": bytes.Repeat([]byte{1}, 32)})
	require.Error(t, err)
	_, err = NewRing(map[string][]byte{"current": bytes.Repeat([]byte{1}, 32), "next": bytes.Repeat([]byte{1}, 32)})
	require.Error(t, err)
	last := time.Now().UTC()
	require.False(t, CanRetire(last, last.Add(60*24*time.Hour-time.Nanosecond), false))
	require.True(t, CanRetire(last, last.Add(60*24*time.Hour), false))
	require.False(t, CanRetire(last, last.Add(90*24*time.Hour), true), "pending admitted old-key evidence must be delivered or expired first")
}
