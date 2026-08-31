import { useEffect, useState } from "react";
import { fetchRetention, type RetentionCohort } from "../api/analytics";
import { AnalyticsSubnav } from "../components/AnalyticsSubnav";

export function RetentionPage() {
  const [cohorts, setCohorts] = useState<RetentionCohort[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchRetention()
      .then((r) => setCohorts(r.cohorts ?? []))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <>
      <AnalyticsSubnav />
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
