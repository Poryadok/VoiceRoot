package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/manifest"
)

const pingManifest = `
name: PingBot
description: Replies pong
scopes:
  - TEXT_CHAT_SEND_MESSAGES
commands:
  - name: ping
    description: Health check
`

func TestParseYAML_pingBotValid(t *testing.T) {
	doc, errs, err := manifest.ParseYAML(pingManifest)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Equal(t, "PingBot", doc.Name)
	require.Len(t, doc.Commands, 1)
	require.Equal(t, "ping", doc.Commands[0].Name)
}

func TestValidate_unknownScopeRejected(t *testing.T) {
	doc := manifest.Document{
		Name:   "Bad",
		Scopes: []string{"READ_ALL_MESSAGES"},
		Commands: []manifest.Command{
			{Name: "ping", Description: "x"},
		},
	}
	errs := manifest.Validate(doc)
	require.NotEmpty(t, errs)
}
