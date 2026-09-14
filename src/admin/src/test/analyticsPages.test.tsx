import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { DashboardMetricsPage } from "../pages/DashboardMetricsPage";
import { FunnelsPage } from "../pages/FunnelsPage";
import { RetentionPage } from "../pages/RetentionPage";
import { AnalyticsTimeRangeProvider } from "../components/AnalyticsTimeRange";
import * as analytics from "../api/analytics";

vi.mock("../api/analytics", () => ({
  fetchDashboard: vi.fn(),
  fetchFunnel: vi.fn(),
  fetchRetention: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(analytics.fetchDashboard).mockResolvedValue({
    dashboard_type: "engagement",
    metrics: [{ name: "messages_sent", value: 42 }],
  });
  vi.mocked(analytics.fetchFunnel).mockResolvedValue({
    funnel_name: "registration",
    steps: [{ step: "started", count: 42 }],
  });
  vi.mocked(analytics.fetchRetention).mockResolvedValue({ cohorts: [] });
});

describe("DashboardMetricsPage", () => {
  it("loads engagement dashboard metrics", async () => {
    render(
      <MemoryRouter>
        <DashboardMetricsPage dashboardType="engagement" />
      </MemoryRouter>,
    );

    expect(await screen.findByText("messages_sent")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(analytics.fetchDashboard).toHaveBeenCalledTimes(1);
    expect(analytics.fetchDashboard).toHaveBeenNthCalledWith(1, "engagement");
  });

  it("passes the selected range when dashboard metrics are refetched", async () => {
    render(
      <MemoryRouter>
        <DashboardMetricsPage dashboardType="engagement" />
      </MemoryRouter>,
    );

    await screen.findByText("messages_sent");
    expect(analytics.fetchDashboard).toHaveBeenNthCalledWith(1, "engagement");

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-01-01T00:00" },
    });
    await waitFor(() => {
      expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement", {
        from: "2026-01-01T00:00:00Z",
      });
    });

    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-01-31T00:00" },
    });

    await waitFor(() => {
      expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement", {
        from: "2026-01-01T00:00:00Z",
        to: "2026-01-31T00:00:00Z",
      });
    });

    fireEvent.change(screen.getByLabelText("To"), { target: { value: "" } });
    await waitFor(() => {
      expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement", {
        from: "2026-01-01T00:00:00Z",
      });
    });

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "" } });
    await waitFor(() => {
      expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement");
    });
  });

  it("does not fetch an inverse UTC range and resumes after the range is corrected", async () => {
    render(
      <MemoryRouter>
        <DashboardMetricsPage dashboardType="engagement" />
      </MemoryRouter>,
    );

    await screen.findByText("messages_sent");

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-01-31T00:00" },
    });
    await waitFor(() => {
      expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement", {
        from: "2026-01-31T00:00:00Z",
      });
    });

    const callsBeforeInverseRange = vi.mocked(analytics.fetchDashboard).mock.calls.length;
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-01-01T00:00" },
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("From must be before or equal to To.");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(analytics.fetchDashboard).toHaveBeenCalledTimes(callsBeforeInverseRange);

    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-01-31T00:00" },
    });

    await waitFor(() => {
      expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement", {
        from: "2026-01-31T00:00:00Z",
        to: "2026-01-31T00:00:00Z",
      });
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("FunnelsPage", () => {
  it("passes the selected range when the registration funnel is refetched", async () => {
    render(
      <MemoryRouter>
        <FunnelsPage />
      </MemoryRouter>,
    );

    await screen.findByText("started");
    expect(analytics.fetchFunnel).toHaveBeenCalledTimes(1);
    expect(analytics.fetchFunnel).toHaveBeenNthCalledWith(1, "registration");

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-01-01T00:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-01-31T00:00" },
    });

    await waitFor(() => {
      expect(analytics.fetchFunnel).toHaveBeenLastCalledWith("registration", {
        from: "2026-01-01T00:00:00Z",
        to: "2026-01-31T00:00:00Z",
      });
    });
  });
});

describe("RetentionPage", () => {
  it("passes the selected range when retention is refetched", async () => {
    render(
      <MemoryRouter>
        <RetentionPage />
      </MemoryRouter>,
    );

    await screen.findByText("No cohort data for the selected range.");
    expect(analytics.fetchRetention).toHaveBeenCalledTimes(1);
    expect(analytics.fetchRetention).toHaveBeenNthCalledWith(1);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-01-01T00:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-01-31T00:00" },
    });

    await waitFor(() => {
      expect(analytics.fetchRetention).toHaveBeenLastCalledWith({
        from: "2026-01-01T00:00:00Z",
        to: "2026-01-31T00:00:00Z",
      });
    });
  });
});

it("shares the selected range between analytics pages", async () => {
  const { rerender } = render(
    <MemoryRouter>
      <AnalyticsTimeRangeProvider>
        <DashboardMetricsPage dashboardType="engagement" />
      </AnalyticsTimeRangeProvider>
    </MemoryRouter>,
  );

  await screen.findByText("messages_sent");
  fireEvent.change(screen.getByLabelText("From"), {
    target: { value: "2026-01-01T00:00" },
  });
  fireEvent.change(screen.getByLabelText("To"), {
    target: { value: "2026-01-31T00:00" },
  });

  await waitFor(() => {
    expect(analytics.fetchDashboard).toHaveBeenLastCalledWith("engagement", {
      from: "2026-01-01T00:00:00Z",
      to: "2026-01-31T00:00:00Z",
    });
  });

  rerender(
    <MemoryRouter>
      <AnalyticsTimeRangeProvider>
        <RetentionPage />
      </AnalyticsTimeRangeProvider>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(analytics.fetchRetention).toHaveBeenLastCalledWith({
      from: "2026-01-01T00:00:00Z",
      to: "2026-01-31T00:00:00Z",
    });
  });
});
