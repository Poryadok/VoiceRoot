import type { BotSummary } from './botLifecycle';
import { formatCommandLabel, type CatalogCommand } from './commandCatalog';

// Match the public install page's English BotScopeLabels.
const scopeLabels: Record<string, string> = {
  TEXT_CHAT_SEND_MESSAGES: 'Send messages in allowed text chats',
  DM_SEND: 'Send direct messages (reply only)',
  SPACE_VIEW_MEMBER_LIST: 'View space member list',
  MEMBER_ASSIGN_ROLES: 'Assign roles below the bot',
  TEXT_CHAT_CREATE_IN_SPACE: 'Create text chats in the space',
  TEXT_CHAT_READ_HISTORY: 'Read message history (privileged)',
  SPACE_MANAGE_ROLES: 'Create and manage roles below the bot (privileged)',
};

function scopeLabel(scope: string): string {
  return Object.prototype.hasOwnProperty.call(scopeLabels, scope) ? scopeLabels[scope] : scope;
}

function requestedScopes(value: string | undefined): string[] | null {
  try {
    const scopes: unknown = JSON.parse(value ?? '');
    return Array.isArray(scopes) && scopes.every((scope) => typeof scope === 'string')
      ? scopes
      : null;
  } catch {
    return null;
  }
}

export function PublicBotPreview({ bot, commands, commandStatus }: {
  bot: BotSummary;
  commands: CatalogCommand[];
  commandStatus: 'loading' | 'ready' | 'unavailable';
}) {
  const scopes = requestedScopes(bot.scopes_json);
  const slug = bot.slug ?? '';
  return (
    <section data-testid="public-bot-preview" aria-labelledby="public-bot-preview-title">
      <h3 id="public-bot-preview-title">Public bot preview</h3>
      <p className="hint">Saved public details. Save changes to update this preview.</p>
      <label>
        Public bot slug
        <input value={slug} readOnly />
      </label>
      {slug.trim() ? (
        <a href={`https://voice.gg/bots/${encodeURIComponent(slug)}`} target="_blank" rel="noopener noreferrer">
          Open public bot page
        </a>
      ) : <p className="hint">Public slug unavailable</p>}
      <h4>{bot.name}</h4>
      {bot.description && <p>{bot.description}</p>}
      <h4>Requested scopes</h4>
      {scopes === null ? <p>Requested scopes unavailable</p> : scopes.length === 0 ? (
        <p>No requested scopes</p>
      ) : (
        <ul>{scopes.map((scope, index) => <li key={`${scope}:${index}`}>{scopeLabel(scope)}</li>)}</ul>
      )}
      <h4>Commands</h4>
      {commandStatus === 'loading' ? <p>Loading command preview…</p> : commandStatus === 'unavailable' ? (
        <p>Command preview unavailable</p>
      ) : commands.length === 0 ? <p>No commands registered</p> : (
        <ul>{commands.map((command, index) => (
          <li key={`${command.name}:${index}`}>
            <strong>{formatCommandLabel(command.name)}</strong>
            {command.description ? ` — ${command.description}` : ''}
          </li>
        ))}</ul>
      )}
    </section>
  );
}
