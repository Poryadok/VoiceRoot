import { useCallback, useEffect, useState } from "react";
import {
  fetchAccountSanctions,
  revokeSanction,
} from "../api/moderation";
import type { Sanction } from "../api/types";
import { ConfirmDialog } from "./ConfirmDialog";

interface AccountSanctionsProps {
  accountId?: string;
  onChanged?: () => void;
}

export function AccountSanctions({
  accountId,
  onChanged,
}: AccountSanctionsProps) {
  const [sanctions, setSanctions] = useState<Sanction[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [pendingRevoke, setPendingRevoke] = useState<Sanction | null>(null);
  const [busy, setBusy] = useState(false);

  const loadSanctions = useCallback(async () => {
    if (!accountId) {
      setSanctions([]);
      return;
    }
    setLoading(true);
    setError(undefined);
    try {
      const response = await fetchAccountSanctions(accountId);
      setSanctions(response.sanction_list?.sanctions ?? []);
    } catch (err) {
      setSanctions([]);
      setError(err instanceof Error ? err.message : "Failed to load sanctions");
    } finally {
      setLoading(false);
    }
  }, [accountId]);

  useEffect(() => {
    void loadSanctions();
  }, [loadSanctions]);

  if (!accountId) {
    return null;
  }

  async function confirmRevoke() {
    if (!pendingRevoke) {
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await revokeSanction(pendingRevoke.id);
      setPendingRevoke(null);
      await loadSanctions();
      onChanged?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke sanction");
    } finally {
      setBusy(false);
    }
  }

  const activeSanctions = sanctions.filter((row) => !row.revoked_at);

  return (
    <section
      className="panel"
      aria-label="Account sanctions"
      data-testid="account-sanctions"
    >
      <div className="detail-section">
        <h3>Account sanctions</h3>
        {loading ? <p className="status-message">Loading sanctions…</p> : null}
        {!loading && activeSanctions.length === 0 ? (
          <p className="status-message">No active sanctions for this account.</p>
        ) : null}
        {activeSanctions.length > 0 ? (
          <ul className="sanction-list">
            {activeSanctions.map((row) => (
              <li key={row.id} data-testid={`sanction-row-${row.id}`}>
                <span>{row.type.replace(/_/g, " ")}</span>
                {row.expires_at ? (
                  <span className="sanction-meta">until {row.expires_at}</span>
                ) : null}
                <button
                  type="button"
                  className="btn btn-danger"
                  onClick={() => setPendingRevoke(row)}
                  disabled={busy}
                  data-testid={`revoke-sanction-${row.id}`}
                >
                  Revoke
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      {error ? (
        <p className="status-message error" role="alert">
          {error}
        </p>
      ) : null}

      <ConfirmDialog
        open={pendingRevoke !== null}
        title="Revoke sanction?"
        description={
          pendingRevoke
            ? `This will revoke the ${pendingRevoke.type.replace(/_/g, " ")} on account ${accountId}.`
            : ""
        }
        confirmLabel="Revoke sanction"
        destructive
        busy={busy}
        onCancel={() => setPendingRevoke(null)}
        onConfirm={() => void confirmRevoke()}
      />
    </section>
  );
}
