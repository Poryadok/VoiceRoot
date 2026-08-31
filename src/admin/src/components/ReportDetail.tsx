import type { Report } from "../api/types";
import { useEffect, useState } from "react";

interface ReportDetailProps {
  report: Report | null;
  onAssignToMe: () => void;
  assignBusy: boolean;
  assignError?: string;
  onResolve: (note: string) => void;
  onDismiss: (note: string) => void;
  closeBusy: boolean;
  closeError?: string;
}

const CLOSED_STATUSES = new Set(["resolved", "dismissed"]);

export function ReportDetail({
  report,
  onAssignToMe,
  assignBusy,
  assignError,
  onResolve,
  onDismiss,
  closeBusy,
  closeError,
}: ReportDetailProps) {
  const [resolutionNote, setResolutionNote] = useState("");

  useEffect(() => {
    setResolutionNote("");
  }, [report?.id]);

  if (!report) {
    return (
      <section className="panel" aria-label="Report detail">
        <p className="status-message">Select a report to view details.</p>
      </section>
    );
  }

  const isClosed = CLOSED_STATUSES.has(report.status);
  const actionsBusy = assignBusy || closeBusy;

  return (
    <section className="panel" aria-label="Report detail" data-testid="report-detail">
      <div className="detail-section">
        <h3>Report</h3>
        <dl className="detail-dl">
          <dt>ID</dt>
          <dd>{report.id}</dd>
          <dt>Status</dt>
          <dd>{report.status}</dd>
          <dt>Category</dt>
          <dd>{report.category}</dd>
          <dt>Target</dt>
          <dd>
            {report.target_type} · {report.target_id}
          </dd>
          <dt>Reporter</dt>
          <dd>{report.reporter_profile_id}</dd>
          <dt>Assigned to</dt>
          <dd>{report.assigned_to_profile_id ?? "—"}</dd>
          <dt>Created</dt>
          <dd>{report.created_at}</dd>
        </dl>
      </div>

      {report.description ? (
        <div className="detail-section">
          <h3>Description</h3>
          <p>{report.description}</p>
        </div>
      ) : null}

      <div className="detail-section">
        <h3>Evidence</h3>
        <pre>{report.evidence_json || "{}"}</pre>
      </div>

      <div className="btn-row">
        <button
          type="button"
          className="btn btn-primary"
          onClick={onAssignToMe}
          disabled={actionsBusy || isClosed}
          data-testid="assign-to-me"
        >
          {assignBusy ? "Assigning…" : "Assign to me"}
        </button>
      </div>

      {!isClosed ? (
        <div className="detail-section">
          <h3>Close report</h3>
          <label>
            Resolution note (optional)
            <textarea
              value={resolutionNote}
              onChange={(event) => setResolutionNote(event.target.value)}
              disabled={closeBusy}
              rows={3}
              placeholder="Short note for the audit trail"
              data-testid="resolution-note"
            />
          </label>
          <div className="btn-row">
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => onResolve(resolutionNote)}
              disabled={closeBusy}
              data-testid="resolve-report"
            >
              {closeBusy ? "Closing…" : "Resolve"}
            </button>
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => onDismiss(resolutionNote)}
              disabled={closeBusy}
              data-testid="dismiss-report"
            >
              Dismiss
            </button>
          </div>
        </div>
      ) : null}

      {assignError ? (
        <p className="status-message error" role="alert">
          {assignError}
        </p>
      ) : null}
      {closeError ? (
        <p className="status-message error" role="alert">
          {closeError}
        </p>
      ) : null}
    </section>
  );
}
