import { describe, expect, it } from 'vitest';
import { defaultManifest } from '../manifestDefaults';

describe('manifest defaults', () => {
  it('includes ping command', () => {
    expect(defaultManifest).toContain('ping');
    expect(defaultManifest).toContain('TEXT_CHAT_SEND_MESSAGES');
  });
});
