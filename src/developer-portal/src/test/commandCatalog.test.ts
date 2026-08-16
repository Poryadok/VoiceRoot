import { describe, expect, it } from 'vitest';
import {
  catalogHasAutocomplete,
  formatCommandLabel,
  parseCommandCatalog,
} from '../commandCatalog';

describe('commandCatalog', () => {
  it('parses GetCommands JSON with autocomplete + subcommands', () => {
    const json = JSON.stringify([
      {
        name: 'stats',
        description: 'Show player stats',
        options: [{ name: 'game', type: 'string', required: true, autocomplete: true }],
      },
      { name: 'queue join', description: 'Join queue', options: [] },
      { name: 'queue leave', description: 'Leave queue', options: [] },
    ]);
    const catalog = parseCommandCatalog(json);
    expect(catalog).toHaveLength(3);
    expect(catalog[0].name).toBe('stats');
    expect(catalog[0].options[0].autocomplete).toBe(true);
    expect(catalogHasAutocomplete(catalog)).toBe(true);
    expect(formatCommandLabel('queue join')).toBe('/queue join');
  });

  it('tolerates missing options arrays', () => {
    const catalog = parseCommandCatalog(JSON.stringify([{ name: 'ping', description: 'pong' }]));
    expect(catalog[0].options).toEqual([]);
    expect(catalogHasAutocomplete(catalog)).toBe(false);
  });
});
