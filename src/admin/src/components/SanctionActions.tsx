import { useState } from "react";
import { applySanction } from "../api/moderation";
import type { Report, SanctionType } from "../api/types";
import { DESTRUCTIVE_SANCTIONS, SANCTION_TYPES } from "../api/types";
import {
  TEMP_BAN_DEFAULT_DAYS,
  TEMP_BAN_MAX_DAYS,
  TEMP_BAN_MIN_DAYS,
  tempBanExpiresAtIso,
} from "../lib/tempBanExpiry";
import { ConfirmDialog } from "./ConfirmDialog";

interface SanctionActionsProps {
  report: Report | null;
  targetAccountId?: string;
  onApplied?: () => void;
}

interface PendingSanction {
  type: SanctionType;
  reason: string;
}

export function SanctionActions({
  report,
  targetAccountId,
  onApplied,
}: SanctionActionsProps) {
  const [reason, setReason] = useState("");
  const [tempBanDays, setTempBanDays] = useState(TEMP_BAN_DEFAULT_DAYS);
  const [pending, setPending] = useState<PendingSanction | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | undefined>();

  if (!report) {
    return null;
  }

  if (!targetAccountId) {
    return (
      <section className="panel" aria-label="Sanctions" data-testid="sanction-actions">
        <p className="status-message">
          Resolve the sanction target account before applying sanctions.
        </p>
      </section>
    );
  }

  const resolvedAccountId = targetAccountId;

  async function submitSanction(type: SanctionType) {
    if (!report) {
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await applySanction({
        target_account_id: resolvedAccountId,
        type,
        reason: reason.trim() || `Sanction from report ${report.id}`,
        report_id: report.id,
        expires_at:
          type === "temp_ban" ? tempBanExpiresAtIso(tempBanDays) : undefined,
      });
      setPending(null);
      setReason("");
      onApplied?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to apply sanction");
    } finally {
      setBusy(false);
    }
  }

  function requestSanction(type: SanctionType) {
    const sanctionReason =
      reason.trim() || `Sanction from report ${report?.id ?? ""}`;
    if (DESTRUCTIVE_SANCTIONS.has(type)) {
      setPending({ type, reason: sanctionReason });
      return;
    }
    void submitSanction(type);
  }

  return (
    <section className="panel" aria-label="Sanctions" data-testid="sanction-actions">
      <div className="detail-section">
        <h3>Sanctions</h3>
        <label>
          Reason
          <input
            type="text"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="Reason shown in audit trail"
            data-testid="sanction-reason"
          />
        </label>
        <label>
          Temp ban duration (days)
          <input
            type="number"
            min={TEMP_BAN_MIN_DAYS}
            max={TEMP_BAN_MAX_DAYS}
            value={tempBanDays}
            onChange={(event) => setTempBanDays(Number(event.target.value))}
            data-testid="temp-ban-days"
          />
        </label>
      </div>

      <div className="btn-row">
        {SANCTION_TYPES.map((type) => (
          <button
            key={type}
            type="button"
            className={
              DESTRUCTIVE_SANCTIONS.has(type) ? "btn btn-danger" : "btn"
            }
            onClick={() => requestSanction(type)}
            disabled={busy}
            data-testid={`sanction-${type}`}
          >
            {formatSanctionLabel(type)}
          </button>
        ))}
      </div>

      {error ? (
        <p className="status-message error" role="alert">
          {error}
        </p>
      ) : null}

      <ConfirmDialog
        open={pending !== null}
        title="Apply destructive sanction?"
        description={
          pending
            ? pending.type === "temp_ban"
              ? `This will apply a ${formatSanctionLabel(pending.type)} (${tempBanDays} days) to account ${resolvedAccountId}. This action is audited and may restrict the user immediately.`
              : `This will apply a ${formatSanctionLabel(pending.type)} to account ${resolvedAccountId}. This action is audited and may restrict the user immediately.`
            : ""
        }
        confirmLabel="Apply sanction"
        destructive
        busy={busy}
        onCancel={() => setPending(null)}
        onConfirm={() => {
          if (pending) {
            void submitSanction(pending.type);
          }
        }}
      />
    </section>
  );
}

function formatSanctionLabel(type: SanctionType): string {
  return type.replace(/_/g, " ");
}
