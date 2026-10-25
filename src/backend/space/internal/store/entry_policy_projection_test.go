package store

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSpaceProjectionNeverDisclosesLegacyConfiguredAnswers(t *testing.T) {
	raw := `[{"id":"rank","prompt":"Your rank?","required":true,"kind":"exact_text","expected_answer":"private-answer","answer_digest":"private-digest","provider":{"secret":"private-provider"}}]`
	row := &SpaceRow{EntryQuestionsJSON: &raw}
	public := row.ToProto().GetEntryQuestionsJson()
	require.NotContains(t, public, "private-answer")
	require.NotContains(t, public, "private-digest")
	require.NotContains(t, public, "private-provider")
	require.NotContains(t, public, "expected_answer")
	require.JSONEq(t, `[{"id":"rank","prompt":"Your rank?","required":true,"kind":"exact_text"}]`, public)
}
