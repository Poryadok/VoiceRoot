package outboxdelivery

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const ownershipOutboxAlertName = "SpaceOwnershipOutboxDeliveryFailures"

type prometheusAlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type prometheusRuleGroup struct {
	Name  string                `yaml:"name"`
	Rules []prometheusAlertRule `yaml:"rules"`
}

type prometheusRuleDocument struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
	Spec   struct {
		Groups []prometheusRuleGroup `yaml:"groups"`
	} `yaml:"spec"`
}

func ownershipOutboxRuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func readOwnershipOutboxAlertRule(t *testing.T, path string) prometheusAlertRule {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	var matches []prometheusAlertRule
	for {
		var document prometheusRuleDocument
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		groups := append(document.Groups, document.Spec.Groups...)
		for _, group := range groups {
			for _, rule := range group.Rules {
				if rule.Alert == ownershipOutboxAlertName {
					rule.Expr = strings.TrimSpace(rule.Expr)
					matches = append(matches, rule)
				}
			}
		}
	}
	require.Len(t, matches, 1, "%s must define the ownership outbox alert exactly once", path)
	return matches[0]
}

func TestOwnershipOutboxDeliveryAlertRule_IsImmediateAndMirrored(t *testing.T) {
	root := ownershipOutboxRuleRepoRoot(t)
	var standalone, fullProfile prometheusAlertRule
	t.Run("standalone rules", func(t *testing.T) {
		standalone = readOwnershipOutboxAlertRule(t, filepath.Join(root,
			"deploy", "observability", "prometheus", "rules", "alerts-p2.yaml"))
	})
	t.Run("full profile mirror", func(t *testing.T) {
		fullProfile = readOwnershipOutboxAlertRule(t, filepath.Join(root,
			"deploy", "observability", "profiles", "full", "prometheus-rules.yaml"))
	})
	if t.Failed() {
		return
	}

	require.Equal(t, prometheusAlertRule{
		Alert: ownershipOutboxAlertName,
		Expr:  "space_ownership_outbox_delivery_alerting_events > 0",
		For:   "0m",
		Labels: map[string]string{
			"severity": "warning",
			"cluster":  "voice-staging",
		},
		Annotations: map[string]string{
			"summary":     "Space ownership outbox delivery failures persist",
			"description": "One or more Space ownership outbox events have failed delivery at least 10 consecutive times and remain retryable.",
		},
	}, standalone)
	require.Equal(t, standalone, fullProfile,
		"standalone and full-profile Prometheus rules must remain semantically identical")
}
