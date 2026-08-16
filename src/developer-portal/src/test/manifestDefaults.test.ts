import { describe, expect, it } from 'vitest';
import { defaultManifest } from '../manifestDefaults';

describe('manifest defaults', () => {
  it('includes ping, autocomplete stats, and queue subcommands', () => {
    expect(defaultManifest).toContain('ping');
    expect(defaultManifest).toContain('TEXT_CHAT_SEND_MESSAGES');
    expect(defaultManifest).toContain('autocomplete: true');
    expect(defaultManifest).toContain('subcommands:');
    expect(defaultManifest).toContain('queue');
  });
});
