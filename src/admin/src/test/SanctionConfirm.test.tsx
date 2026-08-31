import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { applySanction } from "../api/moderation";
import { SanctionActions } from "../components/SanctionActions";

vi.mock("../api/moderation", () => ({
  applySanction: vi.fn().mockResolvedValue({ sanction: { id: "sanction-1" } }),
}));

const report = {
  id: "report-42",
  reporter_profile_id: "mod-profile",
  target_type: "user",
  target_id: "acct-target",
  category: "harassment",
  evidence_json: "{}",
  status: "reviewing",
  resolution_json: "",
  created_at: "2026-06-14T12:00:00Z",
};

describe("SanctionActions confirm flow", () => {
  beforeEach(() => {
    vi.mocked(applySanction).mockClear();
    vi.setSystemTime(new Date("2026-06-14T12:00:00.000Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("requires confirmation before destructive sanctions", async () => {
    const user = userEvent.setup();
    render(<SanctionActions report={report} targetAccountId="acct-target" />);

    await user.click(screen.getByTestId("sanction-perm_ban"));

    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();
    expect(applySanction).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("confirm-dialog-confirm"));

    await waitFor(() => {
      expect(applySanction).toHaveBeenCalledWith({
        target_account_id: "acct-target",
        type: "perm_ban",
        reason: "Sanction from report report-42",
        report_id: "report-42",
      });
    });
  });

  it("applies temp ban with expires_at", async () => {
    const user = userEvent.setup();
    render(<SanctionActions report={report} targetAccountId="acct-target" />);

    await user.clear(screen.getByTestId("temp-ban-days"));
    await user.type(screen.getByTestId("temp-ban-days"), "3");
    await user.click(screen.getByTestId("sanction-temp_ban"));
    await user.click(screen.getByTestId("confirm-dialog-confirm"));

    await waitFor(() => {
      expect(applySanction).toHaveBeenCalledWith(
        expect.objectContaining({
          target_account_id: "acct-target",
          type: "temp_ban",
          expires_at: "2026-06-17T12:00:00.000Z",
        }),
      );
    });
  });

  it("closes dialog without applying when cancelled", async () => {
    const user = userEvent.setup();
    render(<SanctionActions report={report} targetAccountId="acct-target" />);

    await user.click(screen.getByTestId("sanction-shadow_ban"));
    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument();
    expect(applySanction).not.toHaveBeenCalled();
  });

  it("applies warning without confirmation dialog", async () => {
    const user = userEvent.setup();
    render(<SanctionActions report={report} targetAccountId="acct-target" />);

    await user.click(screen.getByTestId("sanction-warning"));

    await waitFor(() => {
      expect(applySanction).toHaveBeenCalledWith({
        target_account_id: "acct-target",
        type: "warning",
        reason: "Sanction from report report-42",
        report_id: "report-42",
      });
    });
    expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument();
  });
});
