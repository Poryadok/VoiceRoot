import type { Report } from "../api/types";

interface ReportDetailProps {
  report: Report | null;
  onAssignToMe: () => void;
  assignBusy: boolean;
  assignError?: string;
}

export function ReportDetail({
  report,
  onAssignToMe,
  assignBusy,
  assignError,
}: ReportDetailProps) {
  if (!report) {
    return (
      <section className="panel" aria-label="Report detail">
        <p className="status-message">Select a report to view details.</p>
      </section>
    );
  }

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
          disabled={assignBusy}
          data-testid="assign-to-me"
        >
          {assignBusy ? "Assigning…" : "Assign to me"}
        </button>
      </div>

      {assignError ? (
        <p className="status-message error" role="alert">
          {assignError}
        </p>
      ) : null}
    </section>
  );
}
