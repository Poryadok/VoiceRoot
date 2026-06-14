import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
