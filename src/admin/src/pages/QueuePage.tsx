import { useCallback, useEffect, useState } from "react";
import { listReports, resolveReport } from "../api/moderation";
import { resolveAccountIdForProfile } from "../api/users";
import type { ModerationQueue, Report } from "../api/types";
import {
  QueueFilters,
  filterReportsByCategory,
  type QueueFiltersValue,
} from "../components/QueueFilters";
import { ReportDetail } from "../components/ReportDetail";
import { ReportTable } from "../components/ReportTable";
import { SanctionActions } from "../components/SanctionActions";
import { staffProfileIdFromToken } from "../lib/jwt";

const QUEUE_TABS: { id: ModerationQueue; label: string }[] = [
  { id: "content", label: "Content" },
  { id: "spaces", label: "Spaces" },
];

export function QueuePage() {
  const [queue, setQueue] = useState<ModerationQueue>("content");
  const [filters, setFilters] = useState<QueueFiltersValue>({
    status: "pending",
    category: "",
  });
  const [reports, setReports] = useState<Report[]>([]);
  const [selected, setSelected] = useState<Report | null>(null);
  const [targetAccountId, setTargetAccountId] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [assignBusy, setAssignBusy] = useState(false);
  const [assignError, setAssignError] = useState<string | undefined>();

  const loadReports = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const response = await listReports({
        queue,
        status: filters.status || undefined,
      });
      setReports(response.report_list?.reports ?? []);
    } catch (err) {
      setReports([]);
      setError(err instanceof Error ? err.message : "Failed to load reports");
    } finally {
      setLoading(false);
    }
  }, [filters.status, queue]);

  useEffect(() => {
    void loadReports();
  }, [loadReports]);

  useEffect(() => {
    setSelected(null);
    setTargetAccountId(undefined);
  }, [queue, filters.status, filters.category]);

  useEffect(() => {
    if (!selected) {
      setTargetAccountId(undefined);
      return;
    }
    if (selected.target_type !== "user") {
      setTargetAccountId(undefined);
      return;
    }
    let cancelled = false;
    void resolveAccountIdForProfile(selected.target_id).then((accountId) => {
      if (!cancelled) {
        setTargetAccountId(accountId);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [selected]);

  const visibleReports = filterReportsByCategory(reports, filters.category);

  async function handleAssignToMe() {
    if (!selected) {
      return;
    }
    const profileId = staffProfileIdFromToken();
    if (!profileId) {
      setAssignError(
        "Staff profile id not found in VITE_STAFF_TOKEN JWT (profile_id claim).",
      );
      return;
    }
    setAssignBusy(true);
    setAssignError(undefined);
    try {
      const response = await resolveReport(selected.id, {
        new_status: "reviewing",
        assigned_to_profile_id: profileId,
        resolution_json: "{}",
      });
      const updated = response.report;
      setSelected(updated);
      setReports((current) =>
        current.map((report) => (report.id === updated.id ? updated : report)),
      );
    } catch (err) {
      setAssignError(
        err instanceof Error ? err.message : "Failed to assign report",
      );
    } finally {
      setAssignBusy(false);
    }
  }

  return (
    <div>
      <div className="tabs" role="tablist" aria-label="Moderation queue">
        {QUEUE_TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={queue === tab.id}
            className={queue === tab.id ? "tab active" : "tab"}
            onClick={() => setQueue(tab.id)}
            data-testid={`queue-tab-${tab.id}`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <QueueFilters value={filters} onChange={setFilters} />

      {loading ? <p className="status-message">Loading queue…</p> : null}
      {error ? (
        <p className="status-message error" role="alert">
          {error}
        </p>
      ) : null}

      <div className="queue-layout">
        <section className="panel" aria-label="Report queue">
          <ReportTable
            reports={visibleReports}
            selectedId={selected?.id}
            onSelect={setSelected}
          />
        </section>

        <div>
          <ReportDetail
            report={selected}
            onAssignToMe={() => void handleAssignToMe()}
            assignBusy={assignBusy}
            assignError={assignError}
          />
          <SanctionActions
            report={selected}
            targetAccountId={targetAccountId}
            onApplied={() => void loadReports()}
          />
        </div>
      </div>
    </div>
  );
}
