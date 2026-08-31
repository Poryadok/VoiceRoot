import { useCallback, useEffect, useMemo, useState } from "react";
import {
  createGame,
  findDuplicateName,
  searchGames,
  type CatalogGame,
} from "../api/gameCatalog";

const DEFAULT_CONFIG = JSON.stringify(
  {
    regions: ["global"],
    modes: [
      {
        name: "Default",
        slots: 1,
        party_size_min: 1,
        party_size_max: 1,
      },
    ],
  },
  null,
  2,
);

export function CreateGamePage() {
  const [name, setName] = useState("");
  const [iconUrl, setIconUrl] = useState("");
  const [configJson, setConfigJson] = useState(DEFAULT_CONFIG);
  const [similar, setSimilar] = useState<CatalogGame[]>([]);
  const [duplicate, setDuplicate] = useState<CatalogGame | undefined>();
  const [status, setStatus] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);

  const lookupSimilar = useCallback(async (query: string) => {
    const trimmed = query.trim();
    if (trimmed.length < 2) {
      setSimilar([]);
      setDuplicate(undefined);
      return;
    }
    try {
      const response = await searchGames(trimmed);
      const games = response.games ?? [];
      setSimilar(games);
      setDuplicate(findDuplicateName(trimmed, games));
    } catch {
      setSimilar([]);
      setDuplicate(undefined);
    }
  }, []);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      void lookupSimilar(name);
    }, 300);
    return () => window.clearTimeout(handle);
  }, [lookupSimilar, name]);

  const canSubmit = useMemo(
    () => name.trim().length > 0 && !duplicate && !busy,
    [busy, duplicate, name],
  );

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    setBusy(true);
    setStatus(undefined);
    try {
      JSON.parse(configJson);
    } catch {
      setStatus("Config must be valid JSON.");
      setBusy(false);
      return;
    }
    try {
      const response = await createGame({
        name: name.trim(),
        config_json: configJson,
        icon_url: iconUrl.trim() || undefined,
      });
      setStatus(`Created game "${response.game?.name ?? name}" (${response.game?.id ?? "ok"}).`);
      setName("");
      setIconUrl("");
      setConfigJson(DEFAULT_CONFIG);
      setSimilar([]);
      setDuplicate(undefined);
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Create failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section>
      <h2>Add game to catalog</h2>
      <p>Staff publish via Gateway <code>POST /api/v1/matchmaking/games</code>.</p>
      <form onSubmit={(e) => void onSubmit(e)}>
        <label>
          Name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            autoComplete="off"
          />
        </label>
        <label>
          Icon URL (optional)
          <input
            value={iconUrl}
            onChange={(e) => setIconUrl(e.target.value)}
            autoComplete="off"
          />
        </label>
        <label>
          Config JSON
          <textarea
            value={configJson}
            onChange={(e) => setConfigJson(e.target.value)}
            rows={12}
            spellCheck={false}
          />
        </label>
        {duplicate ? (
          <p role="alert" className="error">
            Duplicate: &quot;{duplicate.name}&quot; already exists (id {duplicate.id}).
          </p>
        ) : null}
        {similar.length > 0 && !duplicate ? (
          <aside>
            <h3>Similar games</h3>
            <ul>
              {similar.map((game) => (
                <li key={game.id}>
                  {game.name} <span>({game.status ?? "unknown"})</span>
                </li>
              ))}
            </ul>
          </aside>
        ) : null}
        <button type="submit" disabled={!canSubmit}>
          {busy ? "Creating…" : "Create game"}
        </button>
      </form>
      {status ? <p role="status">{status}</p> : null}
    </section>
  );
}
