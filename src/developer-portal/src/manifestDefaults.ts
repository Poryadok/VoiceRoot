export const defaultManifest = `name: MyBot
description: Example bot with autocomplete and subcommands
scopes:
  - TEXT_CHAT_SEND_MESSAGES
commands:
  - name: ping
    description: Health check
  - name: stats
    description: Show player stats
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
  - name: queue
    description: Queue group
    subcommands:
      - name: join
        description: Join queue
      - name: leave
        description: Leave queue
`;
