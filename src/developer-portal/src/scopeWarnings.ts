export const PRIVILEGED_BOT_SCOPES = [
  'TEXT_CHAT_READ_HISTORY',
  'SPACE_MANAGE_ROLES',
] as const;

export type PrivilegedBotScope = (typeof PRIVILEGED_BOT_SCOPES)[number];

const PRIVILEGED_SCOPE_MESSAGES: Record<PrivilegedBotScope, string> = {
  TEXT_CHAT_READ_HISTORY:
    'TEXT_CHAT_READ_HISTORY is privileged — intended for moderation bots; users see an explicit warning when installing.',
  SPACE_MANAGE_ROLES:
    'SPACE_MANAGE_ROLES is privileged — allows creating/managing roles below the bot; users see an explicit warning when installing.',
};

export function privilegedScopesInJson(scopesJson: string): PrivilegedBotScope[] {
  const trimmed = scopesJson.trim();
  if (!trimmed) {
    return [];
  }
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    return PRIVILEGED_BOT_SCOPES.filter((scope) =>
      parsed.some((entry) => String(entry).toUpperCase() === scope),
    );
  } catch {
    return [];
  }
}

export function privilegedScopesInManifest(manifestYaml: string): PrivilegedBotScope[] {
  const found = new Set<PrivilegedBotScope>();
  let inScopes = false;
  for (const line of manifestYaml.split('\n')) {
    const trimmed = line.trim();
    if (/^scopes\s*:/i.test(trimmed)) {
      inScopes = true;
      continue;
    }
    if (inScopes) {
      if (trimmed && !trimmed.startsWith('-') && !/^\s/.test(line)) {
        break;
      }
      for (const scope of PRIVILEGED_BOT_SCOPES) {
        if (trimmed === `- ${scope}` || trimmed === scope) {
          found.add(scope);
        }
      }
    }
  }
  return PRIVILEGED_BOT_SCOPES.filter((scope) => found.has(scope));
}

export function warningsForPrivilegedScopes(scopes: PrivilegedBotScope[]): string[] {
  return scopes.map((scope) => PRIVILEGED_SCOPE_MESSAGES[scope]);
}
