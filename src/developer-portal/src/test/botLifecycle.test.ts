import { describe, expect, it, vi } from 'vitest';
import {
  botResourcePath,
  buildUpdateBotBody,
  deleteBot,
  fetchBot,
  parseBotDetail,
  parseUpdatedBot,
  updateBot,
} from '../botLifecycle';

describe('bot lifecycle REST helpers', () => {
  it('builds PATCH path for a bot id', () => {
    const botId = '00000000-0000-0000-0000-000000000001';
    expect(botResourcePath(botId)).toBe(
      '/api/v1/bots/00000000-0000-0000-0000-000000000001',
    );
  });

  it('builds update body with only provided fields', () => {
    expect(
      buildUpdateBotBody({
        name: 'Renamed',
        description: 'New desc',
        scopesJson: '["TEXT_CHAT_SEND_MESSAGES"]',
      }),
    ).toEqual({
      name: 'Renamed',
      description: 'New desc',
      scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
    });
    expect(buildUpdateBotBody({ name: 'Only name' })).toEqual({ name: 'Only name' });
  });

  it('parses bot detail from GET response', () => {
    const bot = parseBotDetail({
      bot: {
        id: 'bot-1',
        name: 'StatsBot',
        description: 'Stats',
        scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
      },
    });
    expect(bot).toEqual({
      id: 'bot-1',
      name: 'StatsBot',
      description: 'Stats',
      scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
    });
  });

  it('GETs bot detail via apiFetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          bot: {
            id: 'bot-1',
            name: 'StatsBot',
            scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
          },
        }),
        { status: 200 },
      ),
    );
    const result = await fetchBot('bot-1', fetchMock);
    expect(result).toEqual({
      ok: true,
      bot: {
        id: 'bot-1',
        name: 'StatsBot',
        scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
      },
    });
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/bots/bot-1');
  });

  it('parses updated bot from PATCH response', () => {
    const bot = parseUpdatedBot({
      bot: { id: 'bot-1', name: 'StatsBot', description: 'Stats' },
    });
    expect(bot).toEqual({ id: 'bot-1', name: 'StatsBot', description: 'Stats' });
  });

  it('PATCHes bot metadata via apiFetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({ bot: { id: 'bot-1', name: 'Renamed' } }),
        { status: 200 },
      ),
    );
    const result = await updateBot(
      'bot-1',
      { name: 'Renamed', description: 'New desc' },
      fetchMock,
    );
    expect(result).toEqual({ ok: true, bot: { id: 'bot-1', name: 'Renamed' } });
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/bots/bot-1', {
      method: 'PATCH',
      body: JSON.stringify({ name: 'Renamed', description: 'New desc' }),
    });
  });

  it('DELETEs bot via apiFetch', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    const result = await deleteBot('bot-1', fetchMock);
    expect(result).toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/bots/bot-1', { method: 'DELETE' });
  });

  it('surfaces PATCH errors', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'forbidden' }), { status: 403 }),
    );
    const result = await updateBot('bot-1', { name: 'X' }, fetchMock);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('forbidden');
    }
  });
});
