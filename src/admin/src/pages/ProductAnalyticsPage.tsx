import { useEffect, useState } from "react";
import { fetchDashboard, type MetricPoint } from "../api/analytics";

export function ProductAnalyticsPage() {
  const [metrics, setMetrics] = useState<MetricPoint[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchDashboard("product")
      .then((r) => setMetrics(r.metrics ?? []))
      .catch((e: Error) => setError(e.message));
  }, []);

  return (
    <section>
      <h2>Product analytics</h2>
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
