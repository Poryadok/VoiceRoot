package entrypolicy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func questionPolicy(t *testing.T, expected string) Policy {
	t.Helper()
	return Policy{SchemaVersion: 1, Questions: []Question{{ID: uuid.MustParse("16ed34cb-9298-4733-9b06-b0204699f6df"), Prompt: "2x2", Kind: "exact_text", ExpectedAnswer: expected}}}
}

func TestConfiguredAnswersExactComparison(t *testing.T) {
	p := questionPolicy(t, "4")
	for _, value := range []string{"4", " \t4\r\n"} {
		require.NoError(t, p.CheckAnswers([]Answer{{QuestionID: p.Questions[0].ID, Value: value}}))
	}
	for _, value := range []string{"5", "04", "4.0", "+4", "four", "４", "4 4", "\u00a04\u00a0"} {
		t.Run(value, func(t *testing.T) {
			require.ErrorIs(t, p.CheckAnswers([]Answer{{QuestionID: p.Questions[0].ID, Value: value}}), ErrDenied)
		})
	}
	p = questionPolicy(t, "é")
	require.NoError(t, p.CheckAnswers([]Answer{{QuestionID: p.Questions[0].ID, Value: "e\u0301"}}))
	require.ErrorIs(t, p.CheckAnswers([]Answer{{QuestionID: p.Questions[0].ID, Value: "É"}}), ErrDenied)
}

func TestAllAnswersAndExactIDsRequired(t *testing.T) {
	p := questionPolicy(t, "4")
	p.Questions = append(p.Questions, Question{ID: uuid.New(), Prompt: "Sky", Kind: "exact_text", ExpectedAnswer: "blue"})
	valid := []Answer{{p.Questions[1].ID, "blue"}, {p.Questions[0].ID, "4"}}
	require.NoError(t, p.CheckAnswers(valid))
	for _, answers := range [][]Answer{nil, valid[:1], {valid[0], valid[0]}, {valid[0], {uuid.New(), "4"}}, {valid[0], valid[1], {uuid.New(), "extra"}}} {
		require.Error(t, p.CheckAnswers(answers))
	}
	valid[0].Value = "red"
	require.ErrorIs(t, p.CheckAnswers(valid), ErrDenied)
}

func TestStrictPolicyJSONAndLimits(t *testing.T) {
	p := questionPolicy(t, "4")
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	parsed, err := Parse(raw)
	require.NoError(t, err)
	require.Equal(t, p, parsed)
	require.False(t, parsed.RequireManualApproval, "questions must not silently enable manual review")
	for _, raw := range []string{
		`{}`, `null`, `{"schema_version":1,"require_phone":false,"require_captcha":false,"questions":[]}`,
		`{"schema_version":1,"require_phone":false,"require_captcha":false,"require_manual_approval":false,"questions":null}`,
		strings.Replace(string(raw), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		strings.Replace(string(raw), `"expected_answer":"4"`, `"expected_answer":"4","expected_answer":"5"`, 1),
		strings.Replace(string(raw), `"exact_text"`, `"regex"`, 1),
		strings.Replace(string(raw), `"expected_answer":"4"`, `"expected_answer":"4","extra":true`, 1),
		string(raw) + `{}`, string([]byte{0xff}),
	} {
		_, err := Parse([]byte(raw))
		require.ErrorIs(t, err, ErrInvalid, raw)
	}
	for _, answer := range []string{"", " \t\n", "4\u0000", "4\x7f", strings.Repeat("a", 257)} {
		p := questionPolicy(t, answer)
		b, err := json.Marshal(p)
		require.NoError(t, err)
		_, err = Parse(b)
		require.ErrorIs(t, err, ErrInvalid)
	}
}

func TestPublicPolicyNeverContainsConfiguredAnswers(t *testing.T) {
	p := questionPolicy(t, "private-configured-answer")
	b, err := json.Marshal(p.Public())
	require.NoError(t, err)
	require.NotContains(t, string(b), "private-configured-answer")
	require.NotContains(t, string(b), "expected_answer")
	require.Contains(t, string(b), "2x2")
}

func TestLegacyPolicyMigrationNeverWeakensRequirements(t *testing.T) {
	for _, legacy := range []string{"none", "phone", "captcha", "manual"} {
		p, err := FromLegacy(legacy, nil)
		require.NoError(t, err)
		require.Equal(t, legacy == "phone", p.RequirePhone)
		require.Equal(t, legacy == "captcha", p.RequireCaptcha)
		require.Equal(t, legacy == "manual", p.RequireManualApproval)
	}
	for _, legacy := range []string{"", "unknown", "questions"} {
		_, err := FromLegacy(legacy, []byte(`[{"id":"rank","prompt":"Rank?","required":true}]`))
		require.ErrorIs(t, err, ErrInvalid)
	}
	p := questionPolicy(t, "4")
	b, err := json.Marshal(p.Questions)
	require.NoError(t, err)
	migrated, err := FromLegacy("questions", b)
	require.NoError(t, err)
	require.NoError(t, migrated.CheckAnswers([]Answer{{p.Questions[0].ID, "4"}}))
}
