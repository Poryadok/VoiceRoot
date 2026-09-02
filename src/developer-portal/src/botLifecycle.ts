import { apiFetch } from './oauth/api';

export type BotSummary = {
  id?: string;
  name?: string;
  description?: string;
  slug?: string;
  scopes_json?: string;
};

export type BotUpdateFields = {
  name?: string;
  description?: string;
  scopesJson?: string;
  avatarUrl?: string;
};

export function botResourcePath(botId: string): string {
  return `/api/v1/bots/${botId}`;
}

/** Builds PATCH /api/v1/bots/{id} JSON body; omits unset fields. */
export function buildUpdateBotBody(fields: BotUpdateFields): Record<string, string> {
  const body: Record<string, string> = {};
  if (fields.name !== undefined) {
    body.name = fields.name;
  }
  if (fields.description !== undefined) {
    body.description = fields.description;
  }
  if (fields.scopesJson !== undefined) {
    body.scopes_json = fields.scopesJson;
  }
  if (fields.avatarUrl !== undefined) {
    body.avatar_url = fields.avatarUrl;
  }
  return body;
}

export function parseBotDetail(body: Record<string, unknown>): BotSummary | undefined {
  return (body.bot as BotSummary | undefined) ?? undefined;
}

export function parseUpdatedBot(body: Record<string, unknown>): BotSummary | undefined {
  return parseBotDetail(body);
}

export async function fetchBot(
  botId: string,
  fetchFn: typeof apiFetch = apiFetch,
): Promise<{ ok: true; bot: BotSummary } | { ok: false; error: string }> {
  const res = await fetchFn(botResourcePath(botId));
  const body = (await res.json()) as Record<string, unknown>;
  if (!res.ok) {
    return { ok: false, error: JSON.stringify(body) };
  }
  const bot = parseBotDetail(body);
  if (!bot?.id) {
    return { ok: false, error: 'missing bot in response' };
  }
  return { ok: true, bot };
}

export async function updateBot(
  botId: string,
  fields: BotUpdateFields,
  fetchFn: typeof apiFetch = apiFetch,
): Promise<{ ok: true; bot: BotSummary } | { ok: false; error: string }> {
  const res = await fetchFn(botResourcePath(botId), {
    method: 'PATCH',
    body: JSON.stringify(buildUpdateBotBody(fields)),
  });
  const body = (await res.json()) as Record<string, unknown>;
  if (!res.ok) {
    return { ok: false, error: JSON.stringify(body) };
  }
  const bot = parseUpdatedBot(body);
  if (!bot?.id) {
    return { ok: false, error: 'missing bot in response' };
  }
  return { ok: true, bot };
}

export async function deleteBot(
  botId: string,
  fetchFn: typeof apiFetch = apiFetch,
): Promise<{ ok: true } | { ok: false; error: string }> {
  const res = await fetchFn(botResourcePath(botId), { method: 'DELETE' });
  if (res.status === 204 || res.ok) {
    return { ok: true };
  }
  let error = `delete failed: ${res.status}`;
  try {
    const body = await res.json();
    error = JSON.stringify(body);
  } catch {
    // empty body on error
  }
  return { ok: false, error };
}
