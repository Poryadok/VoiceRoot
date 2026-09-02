import { describe, expect, it } from 'vitest';
import {
  privilegedScopesInJson,
  privilegedScopesInManifest,
  warningsForPrivilegedScopes,
} from '../scopeWarnings';

describe('privileged scope warnings', () => {
  it('detects privileged scopes in JSON', () => {
    expect(privilegedScopesInJson('["TEXT_CHAT_SEND_MESSAGES"]')).toEqual([]);
    expect(
      privilegedScopesInJson('["TEXT_CHAT_READ_HISTORY","TEXT_CHAT_SEND_MESSAGES"]'),
    ).toEqual(['TEXT_CHAT_READ_HISTORY']);
    expect(privilegedScopesInJson('["space_manage_roles"]')).toEqual(['SPACE_MANAGE_ROLES']);
  });

  it('returns empty for invalid JSON', () => {
    expect(privilegedScopesInJson('not-json')).toEqual([]);
    expect(privilegedScopesInJson('{"scopes":[]}').length).toBe(0);
  });

  it('detects privileged scopes in manifest YAML', () => {
    const manifest = `name: ModBot
scopes:
  - TEXT_CHAT_SEND_MESSAGES
  - TEXT_CHAT_READ_HISTORY
commands: []`;
    expect(privilegedScopesInManifest(manifest)).toEqual(['TEXT_CHAT_READ_HISTORY']);
  });

  it('builds human-readable warnings', () => {
    const warnings = warningsForPrivilegedScopes(['TEXT_CHAT_READ_HISTORY', 'SPACE_MANAGE_ROLES']);
    expect(warnings).toHaveLength(2);
    expect(warnings[0]).toContain('TEXT_CHAT_READ_HISTORY');
    expect(warnings[1]).toContain('SPACE_MANAGE_ROLES');
  });
});
