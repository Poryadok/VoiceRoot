// Package entrypolicy validates private Space admission policies and safe projections.
package entrypolicy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

var (
	ErrInvalid = errors.New("invalid entry policy or submission")
	ErrDenied  = errors.New("entry requirements not satisfied")
)

type Question struct {
	ID             uuid.UUID `json:"id"`
	Prompt         string    `json:"prompt"`
	Kind           string    `json:"kind"`
	ExpectedAnswer string    `json:"expected_answer"`
}
type Policy struct {
	SchemaVersion         int        `json:"schema_version"`
	RequirePhone          bool       `json:"require_phone"`
	RequireCaptcha        bool       `json:"require_captcha"`
	RequireManualApproval bool       `json:"require_manual_approval"`
	Questions             []Question `json:"questions"`
}
type Answer struct {
	QuestionID uuid.UUID
	Value      string
}

// Parse rejects ambiguous JSON rather than accepting encoding/json's last-key wins.
func Parse(raw []byte) (Policy, error) {
	var p Policy
	value, err := strictJSON(raw)
	if err != nil {
		return p, ErrInvalid
	}
	obj, ok := value.(map[string]any)
	if !ok || !exactKeys(obj, "schema_version", "require_phone", "require_captcha", "require_manual_approval", "questions") {
		return p, ErrInvalid
	}
	for _, name := range []string{"require_phone", "require_captcha", "require_manual_approval"} {
		if _, ok := obj[name].(bool); !ok {
			return p, ErrInvalid
		}
	}
	questions, ok := obj["questions"].([]any)
	if !ok {
		return p, ErrInvalid
	}
	for _, q := range questions {
		fields, ok := q.(map[string]any)
		if !ok || !exactKeys(fields, "id", "prompt", "kind", "expected_answer") {
			return p, ErrInvalid
		}
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Policy{}, ErrInvalid
	}
	if err := p.validate(); err != nil {
		return Policy{}, err
	}
	for i := range p.Questions {
		p.Questions[i].ExpectedAnswer, _ = normalizeAnswer(p.Questions[i].ExpectedAnswer)
	}
	return p, nil
}

func (p Policy) validate() error {
	if p.SchemaVersion != 1 || p.Questions == nil || len(p.Questions) > 10 {
		return ErrInvalid
	}
	ids := make(map[uuid.UUID]bool, len(p.Questions))
	for _, q := range p.Questions {
		if q.ID == uuid.Nil || ids[q.ID] || q.Kind != "exact_text" || !utf8.ValidString(q.Prompt) || len(q.Prompt) < 1 || len(q.Prompt) > 1024 {
			return ErrInvalid
		}
		ids[q.ID] = true
		if _, err := normalizeAnswer(q.ExpectedAnswer); err != nil {
			return err
		}
	}
	return nil
}

func normalizeAnswer(value string) (string, error) {
	if !utf8.ValidString(value) || len(value) > 4096 {
		return "", ErrInvalid
	}
	for _, r := range value {
		if r == 127 || (r < 32 && r != '\t' && r != '\n' && r != '\r') {
			return "", ErrInvalid
		}
	}
	value = strings.Trim(norm.NFC.String(value), "\t\n\r ")
	if len(value) == 0 || len(value) > 256 {
		return "", ErrInvalid
	}
	return value, nil
}

// CheckAnswers checks every configured answer without returning a correctness oracle.
func (p Policy) CheckAnswers(answers []Answer) error {
	if err := p.validate(); err != nil {
		return err
	}
	if len(answers) != len(p.Questions) {
		return ErrInvalid
	}
	values := make(map[uuid.UUID]string, len(answers))
	for _, a := range answers {
		if a.QuestionID == uuid.Nil {
			return ErrInvalid
		}
		if _, exists := values[a.QuestionID]; exists {
			return ErrInvalid
		}
		value, err := normalizeAnswer(a.Value)
		if err != nil {
			return err
		}
		values[a.QuestionID] = value
	}
	denied := false
	for _, q := range p.Questions {
		value, ok := values[q.ID]
		if !ok {
			return ErrInvalid
		}
		expected, err := normalizeAnswer(q.ExpectedAnswer)
		if err != nil {
			return err
		}
		denied = denied || value != expected
	}
	if denied {
		return ErrDenied
	}
	return nil
}

type PublicQuestion struct {
	ID     uuid.UUID `json:"id"`
	Prompt string    `json:"prompt"`
	Kind   string    `json:"kind"`
}
type PublicPolicy struct {
	SchemaVersion         int              `json:"schema_version"`
	RequirePhone          bool             `json:"require_phone"`
	RequireCaptcha        bool             `json:"require_captcha"`
	RequireManualApproval bool             `json:"require_manual_approval"`
	Questions             []PublicQuestion `json:"questions"`
}

func (p Policy) Public() PublicPolicy {
	result := PublicPolicy{p.SchemaVersion, p.RequirePhone, p.RequireCaptcha, p.RequireManualApproval, make([]PublicQuestion, 0, len(p.Questions))}
	for _, q := range p.Questions {
		result.Questions = append(result.Questions, PublicQuestion{q.ID, q.Prompt, q.Kind})
	}
	return result
}

// FromLegacy preserves restrictions; invalid legacy data requires explicit repair.
func FromLegacy(requirement string, questions []byte) (Policy, error) {
	p := Policy{SchemaVersion: 1, Questions: []Question{}}
	switch requirement {
	case "none":
	case "phone":
		p.RequirePhone = true
	case "captcha":
		p.RequireCaptcha = true
	case "manual":
		p.RequireManualApproval = true
	case "questions":
		value, err := strictJSON(questions)
		if err != nil {
			return Policy{}, ErrInvalid
		}
		items, ok := value.([]any)
		if !ok || len(items) == 0 {
			return Policy{}, ErrInvalid
		}
		for _, q := range items {
			fields, ok := q.(map[string]any)
			if !ok || !exactKeys(fields, "id", "prompt", "kind", "expected_answer") {
				return Policy{}, ErrInvalid
			}
		}
		if json.Unmarshal(questions, &p.Questions) != nil {
			return Policy{}, ErrInvalid
		}
	default:
		return Policy{}, ErrInvalid
	}
	if err := p.validate(); err != nil {
		return Policy{}, err
	}
	for i := range p.Questions {
		p.Questions[i].ExpectedAnswer, _ = normalizeAnswer(p.Questions[i].ExpectedAnswer)
	}
	return p, nil
}

// LegacyPublicQuestions preserves existing display fields without ever relaying
// an arbitrary settings JSON object through the public Space projection.
func LegacyPublicQuestions(raw string) string {
	value, err := strictJSON([]byte(raw))
	if err != nil {
		return ""
	}
	items, ok := value.([]any)
	if !ok || len(items) > 10 {
		return ""
	}
	public := make([]map[string]any, 0, len(items))
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			return ""
		}
		out := map[string]any{}
		for _, key := range []string{"id", "prompt", "kind"} {
			if value, exists := fields[key]; exists {
				text, ok := value.(string)
				if !ok {
					return ""
				}
				out[key] = text
			}
		}
		if value, exists := fields["required"]; exists {
			flag, ok := value.(bool)
			if !ok {
				return ""
			}
			out["required"] = flag
		}
		public = append(public, out)
	}
	encoded, err := json.Marshal(public)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func exactKeys(obj map[string]any, keys ...string) bool {
	if len(obj) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			return false
		}
	}
	return true
}

func strictJSON(raw []byte) (any, error) {
	if len(raw) > 32768 || !utf8.Valid(raw) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	value, err := readValue(d, 0)
	if err != nil {
		return nil, ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return value, nil
}

func readValue(d *json.Decoder, depth int) (any, error) {
	if depth > 8 {
		return nil, ErrInvalid
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	if delim == '{' {
		obj := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, ErrInvalid
			}
			if _, duplicate := obj[name]; duplicate {
				return nil, ErrInvalid
			}
			value, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			obj[name] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, ErrInvalid
		}
		return obj, nil
	}
	if delim == '[' {
		items := []any{}
		for d.More() {
			value, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, ErrInvalid
		}
		return items, nil
	}
	return nil, ErrInvalid
}
