import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountSanctions } from "../components/AccountSanctions";
import {
  fetchAccountSanctions,
  revokeSanction,
} from "../api/moderation";

vi.mock("../api/moderation", () => ({
  fetchAccountSanctions: vi.fn(),
  revokeSanction: vi.fn(),
}));

describe("AccountSanctions revoke flow", () => {
  beforeEach(() => {
    vi.mocked(fetchAccountSanctions).mockReset();
    vi.mocked(revokeSanction).mockReset();
    vi.mocked(fetchAccountSanctions).mockResolvedValue({
      sanction_list: {
        sanctions: [
          {
            id: "sanction-9",
            target_account_id: "acct-1",
            type: "temp_ban",
            reason: "spam",
            issued_by_profile_id: "mod-1",
            created_at: "2026-06-14T12:00:00Z",
          },
        ],
      },
    });
    vi.mocked(revokeSanction).mockResolvedValue(undefined);
  });

  it("revokes an active sanction after confirmation", async () => {
    const user = userEvent.setup();
    render(<AccountSanctions accountId="acct-1" />);

    expect(await screen.findByTestId("sanction-row-sanction-9")).toBeInTheDocument();
    await user.click(screen.getByTestId("revoke-sanction-sanction-9"));
    expect(screen.getByTestId("confirm-dialog")).toBeInTheDocument();

    await user.click(screen.getByTestId("confirm-dialog-confirm"));

    await waitFor(() => {
      expect(revokeSanction).toHaveBeenCalledWith("sanction-9");
    });
  });
});
