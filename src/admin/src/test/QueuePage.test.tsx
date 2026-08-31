import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueuePage } from "../pages/QueuePage";
import { listReports, resolveReport } from "../api/moderation";

vi.mock("../api/moderation", () => ({
  listReports: vi.fn(),
  resolveReport: vi.fn(),
  applySanction: vi.fn(),
}));

vi.mock("../api/users", () => ({
  resolveAccountIdForProfile: vi.fn().mockResolvedValue("acct-target"),
}));

const openReport = {
  id: "report-42",
  reporter_profile_id: "reporter-1",
  target_type: "user",
  target_id: "profile-target",
  category: "harassment",
  evidence_json: "{}",
  status: "reviewing",
  resolution_json: "",
  created_at: "2026-06-14T12:00:00Z",
};

describe("QueuePage report close workflow", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.mocked(listReports).mockClear();
    vi.mocked(resolveReport).mockClear();
    vi.mocked(listReports).mockResolvedValue({
      report_list: { reports: [openReport] },
    });
    vi.mocked(resolveReport).mockResolvedValue({
      report: { ...openReport, status: "resolved", resolution_json: '{"note":"ok"}' },
    });
  });

  it("resolves selected report and removes it from pending queue", async () => {
    const user = userEvent.setup();
    vi.mocked(listReports)
      .mockResolvedValueOnce({
        report_list: { reports: [openReport] },
      })
      .mockResolvedValue({
        report_list: { reports: [] },
      });

    render(<QueuePage />);

    expect(await screen.findByTestId("report-row-report-42")).toBeInTheDocument();
    await user.click(screen.getByTestId("report-row-report-42"));
    await user.type(screen.getByTestId("resolution-note"), "ok");
    await user.click(screen.getByTestId("resolve-report"));

    await waitFor(() => {
      expect(resolveReport).toHaveBeenCalledWith("report-42", {
        new_status: "resolved",
        resolution_json: JSON.stringify({ note: "ok" }),
      });
    });

    await waitFor(() => {
      expect(
        screen.queryByTestId("report-row-report-42"),
      ).not.toBeInTheDocument();
    });
    expect(screen.getByText("Select a report to view details.")).toBeInTheDocument();
  });

  it("dismisses selected report", async () => {
    const user = userEvent.setup();
    vi.mocked(resolveReport).mockResolvedValue({
      report: { ...openReport, status: "dismissed" },
    });

    render(<QueuePage />);

    expect(await screen.findByTestId("report-row-report-42")).toBeInTheDocument();
    await user.click(screen.getByTestId("report-row-report-42"));
    await user.click(screen.getByTestId("dismiss-report"));

    await waitFor(() => {
      expect(resolveReport).toHaveBeenCalledWith("report-42", {
        new_status: "dismissed",
        resolution_json: "{}",
      });
    });
  });

  it("assign-to-me uses OAuth session profile id", async () => {
    const user = userEvent.setup();
    const payload = btoa(JSON.stringify({ profile_id: "mod-profile" }));
    sessionStorage.setItem("voice_admin_access_token", `hdr.${payload}.sig`);

    render(<QueuePage />);

    expect(await screen.findByTestId("report-row-report-42")).toBeInTheDocument();
    await user.click(screen.getByTestId("report-row-report-42"));
    await user.click(screen.getByTestId("assign-to-me"));

    await waitFor(() => {
      expect(resolveReport).toHaveBeenCalledWith("report-42", {
        new_status: "reviewing",
        assigned_to_profile_id: "mod-profile",
        resolution_json: "{}",
      });
    });
  });

  it("shows OAuth-aware error when staff profile id is missing", async () => {
    const user = userEvent.setup();
    render(<QueuePage />);

    expect(await screen.findByTestId("report-row-report-42")).toBeInTheDocument();
    await user.click(screen.getByTestId("report-row-report-42"));
    await user.click(screen.getByTestId("assign-to-me"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "profile_id claim",
    );
    expect(resolveReport).not.toHaveBeenCalled();
  });
});
