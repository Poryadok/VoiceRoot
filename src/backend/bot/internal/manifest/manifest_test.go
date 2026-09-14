package manifest_test

import (
	"fmt"
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

const queueManifest = `
name: QueueBot
description: Queue
scopes:
  - TEXT_CHAT_SEND_MESSAGES
commands:
  - name: queue
    description: Queue management
    subcommands:
      - name: join
        description: Join queue
      - name: leave
        description: Leave queue
`

func TestParseYAML_subcommandsValid(t *testing.T) {
	doc, errs, err := manifest.ParseYAML(queueManifest)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, doc.Commands[0].Subcommands, 2)
}

func TestFlattenCommands_expandsSubcommands(t *testing.T) {
	doc, _, err := manifest.ParseYAML(queueManifest)
	require.NoError(t, err)
	flat := manifest.FlattenCommands(doc.Commands)
	require.Len(t, flat, 2)
	require.Equal(t, "queue", flat[0].GroupName)
	require.Equal(t, "join", flat[0].Name)
	require.Equal(t, "queue join", flat[0].GroupName+" "+flat[0].Name)
}

func TestValidate_rejectsOptionsAndSubcommandsTogether(t *testing.T) {
	doc := manifest.Document{
		Name:   "Bad",
		Scopes: []string{"TEXT_CHAT_SEND_MESSAGES"},
		Commands: []manifest.Command{
			{
				Name: "stats",
				Options: []manifest.Option{
					{Name: "game", Type: "string"},
				},
				Subcommands: []manifest.Subcommand{
					{Name: "join"},
				},
			},
		},
	}
	errs := manifest.Validate(doc)
	require.NotEmpty(t, errs)
}

func TestParseYAML_acceptsCanonicalOptionTypes(t *testing.T) {
	manifestYAML := `
name: TypedBot
commands:
  - name: configure
    description: Configure a game
    options:
      - name: title
        type: string
      - name: max_players
        type: integer
      - name: ranked
        type: boolean
      - name: captain
        type: user
      - name: destination
        type: channel
      - name: role
        type: role
      - name: rules
        type: attachment
`

	doc, errs, err := manifest.ParseYAML(manifestYAML)
	require.NoError(t, err)
	require.Empty(t, errs)
	require.Len(t, doc.Commands[0].Options, 7)
	require.Equal(t, []string{"string", "integer", "boolean", "user", "channel", "role", "attachment"}, []string{
		doc.Commands[0].Options[0].Type,
		doc.Commands[0].Options[1].Type,
		doc.Commands[0].Options[2].Type,
		doc.Commands[0].Options[3].Type,
		doc.Commands[0].Options[4].Type,
		doc.Commands[0].Options[5].Type,
		doc.Commands[0].Options[6].Type,
	})
}

func TestParseYAML_rejectsUnknownOptionType(t *testing.T) {
	for _, optionType := range []string{"number", "String", " string"} {
		t.Run(optionType, func(t *testing.T) {
			manifestYAML := fmt.Sprintf(`
name: TypedBot
commands:
  - name: configure
    description: Configure a game
    options:
      - name: max_players
        type: %q
`, optionType)

			doc, errs, err := manifest.ParseYAML(manifestYAML)
			require.Equal(t, manifest.Document{}, doc)
			require.EqualError(t, err, "manifest invalid")
			require.Equal(t, []string{"command configure: unknown option type: " + optionType}, errs)
		})
	}
}
