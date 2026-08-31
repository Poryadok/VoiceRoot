import { useCallback, useEffect, useState } from "react";
import { listReports, resolveReport } from "../api/moderation";
import type { ModerationQueue, Report } from "../api/types";
import { AccountSanctions } from "../components/AccountSanctions";
import {
  QueueFilters,
  filterReportsByCategory,
  type QueueFiltersValue,
} from "../components/QueueFilters";
import { ReportDetail } from "../components/ReportDetail";
import { ReportTable } from "../components/ReportTable";
import { SanctionActions } from "../components/SanctionActions";
import { staffProfileIdFromToken } from "../lib/jwt";
import { buildResolutionJson } from "../lib/resolutionJson";
import { resolveTargetAccountId } from "../lib/resolveTargetAccount";

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
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [selected, setSelected] = useState<Report | null>(null);
  const [targetAccountId, setTargetAccountId] = useState<string | undefined>();
  const [targetResolveError, setTargetResolveError] = useState<
    string | undefined
  >();
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [assignBusy, setAssignBusy] = useState(false);
  const [assignError, setAssignError] = useState<string | undefined>();
  const [closeBusy, setCloseBusy] = useState(false);
  const [closeError, setCloseError] = useState<string | undefined>();
  const [sanctionsVersion, setSanctionsVersion] = useState(0);

  const loadReports = useCallback(
    async (cursor?: string) => {
      const append = Boolean(cursor);
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
      }
      setError(undefined);
      try {
        const response = await listReports({
          queue,
          status: filters.status || undefined,
          cursor,
        });
        const pageReports = response.report_list?.reports ?? [];
        setReports((current) =>
          append ? [...current, ...pageReports] : pageReports,
        );
        setNextCursor(response.report_list?.next_cursor || undefined);
      } catch (err) {
        if (!append) {
          setReports([]);
        }
        setError(err instanceof Error ? err.message : "Failed to load reports");
      } finally {
        if (append) {
          setLoadingMore(false);
        } else {
          setLoading(false);
        }
      }
    },
    [filters.status, queue],
  );

  useEffect(() => {
    void loadReports();
  }, [loadReports]);

  useEffect(() => {
    setSelected(null);
    setTargetAccountId(undefined);
    setTargetResolveError(undefined);
    setAssignError(undefined);
    setCloseError(undefined);
  }, [queue, filters.status, filters.category]);

  useEffect(() => {
    if (!selected) {
      setTargetAccountId(undefined);
      setTargetResolveError(undefined);
      return;
    }
    let cancelled = false;
    setTargetAccountId(undefined);
    setTargetResolveError(undefined);
    void resolveTargetAccountId(selected)
      .then((accountId) => {
        if (cancelled) {
          return;
        }
        if (!accountId) {
          setTargetResolveError(
            `Could not resolve account for ${selected.target_type} target.`,
          );
          return;
        }
        setTargetAccountId(accountId);
      })
      .catch(() => {
        if (!cancelled) {
          setTargetResolveError("Failed to resolve sanction target account.");
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
        "Staff profile id not found in session or VITE_STAFF_TOKEN JWT (profile_id claim).",
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

  async function handleCloseReport(
    newStatus: "resolved" | "dismissed",
    note: string,
  ) {
    if (!selected) {
      return;
    }
    setCloseBusy(true);
    setCloseError(undefined);
    try {
      await resolveReport(selected.id, {
        new_status: newStatus,
        resolution_json: buildResolutionJson(note),
      });
      setSelected(null);
      setTargetAccountId(undefined);
      await loadReports();
    } catch (err) {
      setCloseError(
        err instanceof Error
          ? err.message
          : `Failed to ${newStatus === "resolved" ? "resolve" : "dismiss"} report`,
      );
    } finally {
      setCloseBusy(false);
    }
  }

  function handleSanctionsChanged() {
    setSanctionsVersion((value) => value + 1);
    void loadReports();
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
          {nextCursor ? (
            <div className="btn-row">
              <button
                type="button"
                className="btn"
                onClick={() => void loadReports(nextCursor)}
                disabled={loadingMore}
                data-testid="load-more-reports"
              >
                {loadingMore ? "Loading…" : "Load more"}
              </button>
            </div>
          ) : null}
        </section>

        <div>
          <ReportDetail
            report={selected}
            onAssignToMe={() => void handleAssignToMe()}
            assignBusy={assignBusy}
            assignError={assignError}
            onResolve={(note) => void handleCloseReport("resolved", note)}
            onDismiss={(note) => void handleCloseReport("dismissed", note)}
            closeBusy={closeBusy}
            closeError={closeError}
          />
          {targetResolveError ? (
            <p className="status-message error" role="alert">
              {targetResolveError}
            </p>
          ) : null}
          <SanctionActions
            report={selected}
            targetAccountId={targetAccountId}
            onApplied={handleSanctionsChanged}
          />
          <AccountSanctions
            key={`${targetAccountId ?? "none"}-${sanctionsVersion}`}
            accountId={targetAccountId}
            onChanged={handleSanctionsChanged}
          />
        </div>
      </div>
    </div>
  );
}
