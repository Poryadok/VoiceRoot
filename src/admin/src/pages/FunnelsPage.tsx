import { useEffect, useState } from "react";
import { fetchFunnel, type FunnelStep } from "../api/analytics";

export function FunnelsPage() {
  const [steps, setSteps] = useState<FunnelStep[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchFunnel("registration")
      .then((r) => setSteps(r.steps ?? []))
      .catch((e: Error) => setError(e.message));
  }, []);

  return (
    <section>
      <h2>Registration funnel</h2>
      {error ? <p className="error">{error}</p> : null}
      <table className="data-table">
        <thead>
          <tr>
            <th>Step</th>
            <th>Count</th>
          </tr>
        </thead>
        <tbody>
          {steps.map((s) => (
            <tr key={s.step}>
              <td>{s.step}</td>
              <td>{s.count}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
