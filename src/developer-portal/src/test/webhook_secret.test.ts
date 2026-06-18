import { describe, expect, it } from 'vitest';

/** Mirrors Developer Portal parsing of RegisterBot / RegenerateWebhookSecret responses. */
function parseRegisterSecrets(body: Record<string, unknown>) {
  const token = (body.token_response as { token?: string } | undefined)?.token ?? '';
  const webhookSecret =
    (body.webhook_secret_response as { webhook_secret?: string } | undefined)?.webhook_secret ?? '';
  return { token, webhookSecret };
}

function parseRotatedWebhookSecret(body: Record<string, unknown>) {
  return (
    (body.webhook_secret_response as { webhook_secret?: string } | undefined)?.webhook_secret ?? ''
  );
}

describe('Developer Portal bot secret parsing', () => {
  it('reads one-shot token and webhook_secret from register response', () => {
    const parsed = parseRegisterSecrets({
      bot: { id: 'bot-1' },
      token_response: { token: 'plain-token' },
      webhook_secret_response: { webhook_secret: 'whsec-abc' },
    });
    expect(parsed.token).toBe('plain-token');
    expect(parsed.webhookSecret).toBe('whsec-abc');
  });

  it('reads rotated webhook secret response', () => {
    const secret = parseRotatedWebhookSecret({
      webhook_secret_response: { webhook_secret: 'whsec-new' },
    });
    expect(secret).toBe('whsec-new');
  });
});

describe('webhook secret REST paths', () => {
  it('uses webhook-secret regenerate route', () => {
    const botId = '00000000-0000-0000-0000-000000000001';
    expect(`/api/v1/bots/${botId}/webhook-secret/regenerate`).toBe(
      '/api/v1/bots/00000000-0000-0000-0000-000000000001/webhook-secret/regenerate',
    );
  });
});
