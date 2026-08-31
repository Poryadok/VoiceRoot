import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppealsPage } from "../pages/AppealsPage";
import { listAppeals, reviewAppeal } from "../api/moderation";

vi.mock("../api/moderation", () => ({
  listAppeals: vi.fn(),
  reviewAppeal: vi.fn(),
  listReports: vi.fn(),
  resolveReport: vi.fn(),
  applySanction: vi.fn(),
  fetchAccountSanctions: vi.fn(),
  revokeSanction: vi.fn(),
}));

const pendingAppeal = {
  id: "appeal-42",
  sanction_id: "sanction-1",
  appellant_account_id: "acct-1",
  reason: "unfair ban",
  status: "pending",
  created_at: "2026-06-14T12:00:00Z",
};

describe("AppealsPage", () => {
  beforeEach(() => {
    vi.mocked(listAppeals).mockReset();
    vi.mocked(reviewAppeal).mockReset();
    vi.mocked(listAppeals).mockResolvedValue({
      appeal_list: { appeals: [pendingAppeal] },
    });
    vi.mocked(reviewAppeal).mockResolvedValue({
      appeal: { ...pendingAppeal, status: "approved" },
    });
  });

  it("loads pending appeals by default", async () => {
    render(<AppealsPage />);

    expect(await screen.findByTestId("appeal-row-appeal-42")).toBeInTheDocument();
    expect(listAppeals).toHaveBeenCalledWith({ status: "pending", cursor: undefined });
  });

  it("approves selected appeal and reloads queue", async () => {
    const user = userEvent.setup();
    vi.mocked(listAppeals)
      .mockResolvedValueOnce({
        appeal_list: { appeals: [pendingAppeal] },
      })
      .mockResolvedValue({
        appeal_list: { appeals: [] },
      });

    render(<AppealsPage />);
    await user.click(await screen.findByTestId("appeal-row-appeal-42"));
    await user.type(screen.getByTestId("appeal-moderator-note"), "looks valid");
    await user.click(screen.getByTestId("approve-appeal"));

    await waitFor(() => {
      expect(reviewAppeal).toHaveBeenCalledWith("appeal-42", {
        status: "approved",
        moderator_note: "looks valid",
      });
    });
    await waitFor(() => {
      expect(screen.queryByTestId("appeal-row-appeal-42")).not.toBeInTheDocument();
    });
  });

  it("denies selected appeal", async () => {
    const user = userEvent.setup();
    render(<AppealsPage />);
    await user.click(await screen.findByTestId("appeal-row-appeal-42"));
    await user.click(screen.getByTestId("deny-appeal"));

    await waitFor(() => {
      expect(reviewAppeal).toHaveBeenCalledWith("appeal-42", {
        status: "denied",
        moderator_note: undefined,
      });
    });
  });
});
