import { useCallback, useEffect, useState } from "react";
import {
  approveGameRequest,
  listGameRequests,
  rejectGameRequest,
  type CatalogGame,
} from "../api/gameRequests";

export function GameRequestsPage() {
  const [games, setGames] = useState<CatalogGame[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [busyId, setBusyId] = useState<string | undefined>();

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const response = await listGameRequests();
      setGames(response.game_list?.games ?? []);
    } catch (err) {
      setGames([]);
      setError(err instanceof Error ? err.message : "Failed to load game requests");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onApprove(id: string) {
    setBusyId(id);
    try {
      await approveGameRequest(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Approve failed");
    } finally {
      setBusyId(undefined);
    }
  }

  async function onReject(id: string) {
    setBusyId(id);
    try {
      await rejectGameRequest(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reject failed");
    } finally {
      setBusyId(undefined);
    }
  }

  return (
    <section>
      <h2>Game catalog requests</h2>
      <p>Pending user submissions awaiting moderation (П.4 / GC-03).</p>
      {loading ? <p>Loading…</p> : null}
      {error ? <p role="alert">{error}</p> : null}
      {games.length === 0 && !loading ? <p>No pending requests.</p> : null}
      <ul>
        {games.map((game) => (
          <li key={game.id} data-testid={`game-request-${game.id}`}>
            <strong>{game.name}</strong>{" "}
            <span>({game.status})</span>{" "}
            <button
              type="button"
              disabled={busyId === game.id}
              onClick={() => void onApprove(game.id)}
            >
              Approve
            </button>{" "}
            <button
              type="button"
              disabled={busyId === game.id}
              onClick={() => void onReject(game.id)}
            >
              Reject
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
