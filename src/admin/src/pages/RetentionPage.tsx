import { useEffect, useState } from "react";
import { fetchRetention, type RetentionCohort } from "../api/analytics";
import { AnalyticsSubnav } from "../components/AnalyticsSubnav";
import { AnalyticsTimeRangeFilter, AnalyticsTimeRangeScope, useAnalyticsTimeRange } from "../components/AnalyticsTimeRange";

function RetentionContent() {
  const [cohorts, setCohorts] = useState<RetentionCohort[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const { range } = useAnalyticsTimeRange();

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    const request = range ? fetchRetention(range) : fetchRetention();
    void request
      .then((r) => { if (active) setCohorts(r.cohorts ?? []); })
      .catch((e: Error) => { if (active) setError(e.message); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [range]);

  return (
    <>
      <AnalyticsSubnav />
      <AnalyticsTimeRangeFilter />
      <section>
        <h2>Retention (D1 / D7 / D30)</h2>
        {loading ? <p>Loading…</p> : null}
        {error ? <p className="error">{error}</p> : null}
        {cohorts.length === 0 && !loading && !error ? (
          <p>No cohort data for the selected range.</p>
        ) : null}
        <table className="data-table">
          <thead>
            <tr>
              <th>Cohort date</th>
              <th>Size</th>
              <th>D1</th>
              <th>D7</th>
              <th>D30</th>
            </tr>
          </thead>
          <tbody>
            {cohorts.map((c) => (
              <tr key={c.cohort_date}>
                <td>{c.cohort_date}</td>
                <td>{c.cohort_size}</td>
                <td>{c.d1}</td>
                <td>{c.d7}</td>
                <td>{c.d30}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </>
  );
}

export function RetentionPage() {
  return <AnalyticsTimeRangeScope><RetentionContent /></AnalyticsTimeRangeScope>;
}
