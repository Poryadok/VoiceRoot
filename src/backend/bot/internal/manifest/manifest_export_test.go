package manifest_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/manifest"
)

func TestToYAML_roundTripName(t *testing.T) {
	doc := manifest.Document{
		Name:        "PingBot",
		Description: "pong",
		Scopes:      []string{"TEXT_CHAT_SEND_MESSAGES"},
	}
	out, err := manifest.ToYAML(doc)
	require.NoError(t, err)
	require.Contains(t, out, "name: PingBot")
	require.Contains(t, out, "TEXT_CHAT_SEND_MESSAGES")
}

func TestCommandsFromStoredRows_rebuildsGroups(t *testing.T) {
	rows := []manifest.StoredCommandRow{
		{Name: "queue join", Description: "join queue"},
		{Name: "ping", Description: "ping", Parameters: "[]"},
	}
	commands := manifest.CommandsFromStoredRows(rows)
	require.Len(t, commands, 2)
	var foundGroup bool
	for _, cmd := range commands {
		if cmd.Name == "queue" {
			foundGroup = true
			require.Len(t, cmd.Subcommands, 1)
			require.Equal(t, "join", cmd.Subcommands[0].Name)
		}
		if cmd.Name == "ping" {
			require.Equal(t, "ping", cmd.Description)
		}
	}
	require.True(t, foundGroup)
}

func TestToYAML_includesCommands(t *testing.T) {
	doc := manifest.Document{
		Name: "Bot",
		Commands: manifest.CommandsFromStoredRows([]manifest.StoredCommandRow{
			{Name: "ping", Description: "Ping"},
		}),
	}
	out, err := manifest.ToYAML(doc)
	require.NoError(t, err)
	require.True(t, strings.Contains(out, "ping"))
}
