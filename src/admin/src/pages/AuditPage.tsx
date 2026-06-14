import { useCallback, useEffect, useState } from "react";
import { downloadAuditExport, fetchAuditExport } from "../api/moderation";
import type { AuditEntry } from "../api/types";
import { AuditLogTable } from "../components/AuditLogTable";

export function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | undefined>();

  const loadEntries = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const response = await fetchAuditExport();
      setEntries(response.entries ?? []);
    } catch (err) {
      setEntries([]);
      setError(err instanceof Error ? err.message : "Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadEntries();
  }, [loadEntries]);

  async function handleExport() {
    setExporting(true);
    setError(undefined);
    try {
      await downloadAuditExport();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Export failed");
    } finally {
      setExporting(false);
    }
  }

  return (
    <section className="panel" aria-label="Moderation audit log">
      <div className="audit-toolbar">
        <h2>Audit log</h2>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => void handleExport()}
          disabled={exporting}
          data-testid="audit-export"
        >
          {exporting ? "Exporting…" : "Export JSON"}
        </button>
      </div>
      <AuditLogTable entries={entries} loading={loading} error={error} />
    </section>
  );
}
