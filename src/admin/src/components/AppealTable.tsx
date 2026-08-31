import type { Appeal } from "../api/types";

interface AppealTableProps {
  appeals: Appeal[];
  selectedId?: string;
  onSelect: (appeal: Appeal) => void;
}

export function AppealTable({
  appeals,
  selectedId,
  onSelect,
}: AppealTableProps) {
  if (appeals.length === 0) {
    return <p className="status-message">No appeals in this queue.</p>;
  }

  return (
    <table className="data-table" data-testid="appeal-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Account</th>
          <th>Sanction</th>
          <th>Status</th>
          <th>Created</th>
        </tr>
      </thead>
      <tbody>
        {appeals.map((appeal) => (
          <tr
            key={appeal.id}
            className={appeal.id === selectedId ? "selected" : undefined}
            onClick={() => onSelect(appeal)}
            data-testid={`appeal-row-${appeal.id}`}
          >
            <td>{appeal.id.slice(0, 8)}…</td>
            <td>{appeal.appellant_account_id.slice(0, 8)}…</td>
            <td>{appeal.sanction_id.slice(0, 8)}…</td>
            <td>{appeal.status}</td>
            <td>{formatTimestamp(appeal.created_at)}</td>
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
