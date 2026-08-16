export type CatalogCommandOption = {
  name: string;
  type?: string;
  required?: boolean;
  autocomplete?: boolean;
};

export type CatalogCommand = {
  name: string;
  description: string;
  options: CatalogCommandOption[];
};

/** Parse Gateway GetCommands `command_list.commands_json` for portal catalog UX. */
export function parseCommandCatalog(commandsJson: string): CatalogCommand[] {
  const raw = JSON.parse(commandsJson) as Array<{
    name?: string;
    description?: string;
    options?: CatalogCommandOption[] | null;
  }>;
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.map((row) => ({
    name: String(row.name ?? ''),
    description: String(row.description ?? ''),
    options: Array.isArray(row.options) ? row.options : [],
  }));
}

export function catalogHasAutocomplete(commands: CatalogCommand[]): boolean {
  return commands.some((c) => c.options.some((o) => o.autocomplete === true));
}

export function formatCommandLabel(name: string): string {
  return name.startsWith('/') ? name : `/${name}`;
}
