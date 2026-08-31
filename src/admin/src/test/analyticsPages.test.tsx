import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { DashboardMetricsPage } from "../pages/DashboardMetricsPage";
import * as analytics from "../api/analytics";

vi.mock("../api/analytics", () => ({
  fetchDashboard: vi.fn(),
}));

describe("DashboardMetricsPage", () => {
  beforeEach(() => {
    vi.mocked(analytics.fetchDashboard).mockResolvedValue({
      dashboard_type: "engagement",
      metrics: [{ name: "messages_sent", value: 42 }],
    });
  });

  it("loads engagement dashboard metrics", async () => {
    render(
      <MemoryRouter>
        <DashboardMetricsPage dashboardType="engagement" />
      </MemoryRouter>,
    );

    expect(await screen.findByText("messages_sent")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(analytics.fetchDashboard).toHaveBeenCalledWith("engagement");
  });
});
