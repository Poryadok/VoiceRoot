import type { Report } from "../api/types";

interface ReportTableProps {
  reports: Report[];
  selectedId?: string;
  onSelect: (report: Report) => void;
}

export function ReportTable({
  reports,
  selectedId,
  onSelect,
}: ReportTableProps) {
  if (reports.length === 0) {
    return <p className="status-message">No reports in this queue.</p>;
  }

  return (
    <table className="data-table" data-testid="report-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Target</th>
          <th>Category</th>
          <th>Status</th>
          <th>Created</th>
        </tr>
      </thead>
      <tbody>
        {reports.map((report) => (
          <tr
            key={report.id}
            className={report.id === selectedId ? "selected" : undefined}
            onClick={() => onSelect(report)}
            data-testid={`report-row-${report.id}`}
          >
            <td>{report.id.slice(0, 8)}…</td>
            <td>
              {report.target_type} / {report.target_id.slice(0, 8)}…
            </td>
            <td>{report.category}</td>
            <td>{report.status}</td>
            <td>{formatTimestamp(report.created_at)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}
