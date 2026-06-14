import type { AuditEntry } from "../api/types";

interface AuditLogTableProps {
  entries: AuditEntry[];
  loading?: boolean;
  error?: string;
}

export function AuditLogTable({ entries, loading, error }: AuditLogTableProps) {
  if (loading) {
    return <p className="status-message">Loading audit log…</p>;
  }

  if (error) {
    return (
      <p className="status-message error" role="alert">
        {error}
      </p>
    );
  }

  if (entries.length === 0) {
    return <p className="status-message">No audit entries yet.</p>;
  }

  return (
    <table className="data-table" data-testid="audit-log-table">
      <thead>
        <tr>
          <th>Time</th>
          <th>Actor</th>
          <th>Action</th>
          <th>Target</th>
          <th>Details</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((entry, index) => (
          <tr key={entry.id ?? `${entry.action}-${index}`}>
            <td>{entry.created_at ?? "—"}</td>
            <td>{entry.actor_profile_id ?? "—"}</td>
            <td>{entry.action ?? "—"}</td>
            <td>
              {entry.target_type ?? "—"}
              {entry.target_id ? ` / ${entry.target_id}` : ""}
            </td>
            <td>{entry.details ?? "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
