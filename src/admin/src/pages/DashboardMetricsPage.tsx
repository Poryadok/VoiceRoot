import { useEffect, useState } from "react";
import { fetchDashboard, type MetricPoint } from "../api/analytics";
import { AnalyticsSubnav } from "../components/AnalyticsSubnav";
import { AnalyticsTimeRangeFilter, AnalyticsTimeRangeScope, useAnalyticsTimeRange } from "../components/AnalyticsTimeRange";
import { DashboardMetricsTable } from "../components/DashboardMetricsTable";

const TITLES: Record<string, string> = {
  product: "Product analytics",
  engagement: "Engagement analytics",
  revenue: "Revenue analytics",
  health: "Health analytics",
  moderation: "Moderation analytics",
};

interface DashboardMetricsPageProps {
  dashboardType: keyof typeof TITLES;
}

function DashboardMetricsContent({ dashboardType }: DashboardMetricsPageProps) {
  const [metrics, setMetrics] = useState<MetricPoint[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const { range } = useAnalyticsTimeRange();

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    const request = range ? fetchDashboard(dashboardType, range) : fetchDashboard(dashboardType);
    void request
      .then((r) => { if (active) setMetrics(r.metrics ?? []); })
      .catch((e: Error) => { if (active) setError(e.message); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [dashboardType, range]);

  return (
    <>
      <AnalyticsSubnav />
      <AnalyticsTimeRangeFilter />
      <DashboardMetricsTable
        title={TITLES[dashboardType]}
        metrics={metrics}
        error={error}
        loading={loading}
      />
    </>
  );
}

export function DashboardMetricsPage(props: DashboardMetricsPageProps) {
  return <AnalyticsTimeRangeScope><DashboardMetricsContent {...props} /></AnalyticsTimeRangeScope>;
}
