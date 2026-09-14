import { useEffect, useState } from "react";
import { fetchFunnel, type FunnelStep } from "../api/analytics";
import { AnalyticsSubnav } from "../components/AnalyticsSubnav";
import { AnalyticsTimeRangeFilter, AnalyticsTimeRangeScope, useAnalyticsTimeRange } from "../components/AnalyticsTimeRange";

function FunnelsContent() {
  const [steps, setSteps] = useState<FunnelStep[]>([]);
  const [error, setError] = useState<string | null>(null);
  const { range, isRangeValid } = useAnalyticsTimeRange();

  useEffect(() => {
    if (!isRangeValid) {
      return;
    }
    let active = true;
    setError(null);
    const request = range ? fetchFunnel("registration", range) : fetchFunnel("registration");
    void request
      .then((r) => { if (active) setSteps(r.steps ?? []); })
      .catch((e: Error) => { if (active) setError(e.message); });
    return () => { active = false; };
  }, [isRangeValid, range]);

  return (
    <>
      <AnalyticsSubnav />
      <AnalyticsTimeRangeFilter />
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
    </>
  );
}

export function FunnelsPage() {
  return <AnalyticsTimeRangeScope><FunnelsContent /></AnalyticsTimeRangeScope>;
}
