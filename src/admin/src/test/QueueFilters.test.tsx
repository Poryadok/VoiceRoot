import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { QueuePage } from "../pages/QueuePage";

const sampleReports = {
  report_list: {
    reports: [
      {
        id: "report-1",
        reporter_profile_id: "reporter-1",
        target_type: "user",
        target_id: "target-1",
        category: "spam",
        evidence_json: "{}",
        status: "pending",
        resolution_json: "",
        created_at: "2026-06-14T10:00:00Z",
      },
      {
        id: "report-2",
        reporter_profile_id: "reporter-2",
        target_type: "space",
        target_id: "space-1",
        category: "harassment",
        evidence_json: "{}",
        status: "pending",
        resolution_json: "",
        created_at: "2026-06-14T11:00:00Z",
      },
    ],
  },
};

function renderQueue() {
  return render(
    <MemoryRouter>
      <QueuePage />
    </MemoryRouter>,
  );
}

describe("QueuePage filters", () => {
  beforeEach(() => {
    vi.stubEnv("VITE_VOICE_API_BASE", "http://gateway.test");
    vi.stubEnv("VITE_STAFF_TOKEN", "token");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => sampleReports,
      }),
    );
  });

  it("loads content queue with default pending status filter", async () => {
    renderQueue();

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(
        "http://gateway.test/api/v1/admin/moderation/reports?status=pending&queue=content",
        expect.objectContaining({
          headers: expect.any(Headers),
        }),
      );
    });
  });

  it("switches queue tab to spaces", async () => {
    const user = userEvent.setup();
    renderQueue();
    await screen.findByTestId("report-row-report-1");

    await user.click(screen.getByTestId("queue-tab-spaces"));

    await waitFor(() => {
      expect(fetch).toHaveBeenLastCalledWith(
        "http://gateway.test/api/v1/admin/moderation/reports?status=pending&queue=spaces",
        expect.any(Object),
      );
    });
  });

  it("updates status filter in API request", async () => {
    const user = userEvent.setup();
    renderQueue();
    await screen.findByTestId("report-row-report-1");

    await user.selectOptions(screen.getByTestId("filter-status"), "reviewing");

    await waitFor(() => {
      expect(fetch).toHaveBeenLastCalledWith(
        "http://gateway.test/api/v1/admin/moderation/reports?status=reviewing&queue=content",
        expect.any(Object),
      );
    });
  });

  it("filters reports by category client-side", async () => {
    const user = userEvent.setup();
    renderQueue();
    await screen.findByTestId("report-row-report-1");
    expect(screen.getByTestId("report-row-report-2")).toBeInTheDocument();

    await user.selectOptions(screen.getByTestId("filter-category"), "spam");

    expect(screen.getByTestId("report-row-report-1")).toBeInTheDocument();
    expect(screen.queryByTestId("report-row-report-2")).not.toBeInTheDocument();
  });
});
