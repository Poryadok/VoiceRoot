import type { MetricPoint } from "../api/analytics";

interface DashboardMetricsTableProps {
  title: string;
  metrics: MetricPoint[];
  error: string | null;
  loading?: boolean;
}

export function DashboardMetricsTable({
  title,
  metrics,
  error,
  loading,
}: DashboardMetricsTableProps) {
  return (
    <section>
      <h2>{title}</h2>
      {loading ? <p>Loading…</p> : null}
      {error ? <p className="error">{error}</p> : null}
      <table className="data-table">
        <thead>
          <tr>
            <th>Metric</th>
            <th>Value</th>
          </tr>
        </thead>
        <tbody>
          {metrics.map((m) => (
            <tr key={m.name}>
              <td>{m.name}</td>
              <td>{m.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
