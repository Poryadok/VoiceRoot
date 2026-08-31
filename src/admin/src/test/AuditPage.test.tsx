import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuditPage } from "../pages/AuditPage";
import * as moderation from "../api/moderation";

vi.mock("../api/moderation", () => ({
  fetchAuditExport: vi.fn(),
  downloadAuditExport: vi.fn(),
}));

describe("AuditPage", () => {
  beforeEach(() => {
    vi.mocked(moderation.fetchAuditExport).mockResolvedValue({
      entries: [
        {
          id: "audit-1",
          actor_profile_id: "staff-1",
          action: "resolve_report",
          target_type: "report",
          target_id: "report-1",
          details: "{}",
          created_at: "2026-06-14T12:00:00Z",
        },
      ],
    });
    vi.mocked(moderation.downloadAuditExport).mockResolvedValue(undefined);
  });

  it("loads audit entries on mount", async () => {
    render(<AuditPage />);

    expect(await screen.findByText("resolve_report")).toBeInTheDocument();
    expect(moderation.fetchAuditExport).toHaveBeenCalled();
  });

  it("triggers JSON export download", async () => {
    const user = userEvent.setup();
    render(<AuditPage />);

    await screen.findByText("resolve_report");
    await user.click(screen.getByTestId("audit-export"));

    expect(moderation.downloadAuditExport).toHaveBeenCalled();
  });
});
