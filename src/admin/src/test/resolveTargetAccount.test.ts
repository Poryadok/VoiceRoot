import { beforeEach, describe, expect, it, vi } from "vitest";
import { resolveTargetAccountId } from "../lib/resolveTargetAccount";
import { apiJson } from "../api/client";
import { resolveAccountIdForProfile } from "../api/users";

vi.mock("../api/client", () => ({
  apiJson: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock("../api/users", () => ({
  resolveAccountIdForProfile: vi.fn(),
}));

const baseReport = {
  id: "report-1",
  reporter_profile_id: "reporter-1",
  category: "spam",
  evidence_json: "{}",
  status: "pending",
  resolution_json: "",
  created_at: "2026-06-14T12:00:00Z",
};

describe("resolveTargetAccountId", () => {
  beforeEach(() => {
    vi.mocked(apiJson).mockReset();
    vi.mocked(resolveAccountIdForProfile).mockReset();
    vi.mocked(resolveAccountIdForProfile).mockResolvedValue("acct-resolved");
  });

  it("resolves user targets via profile lookup", async () => {
    const accountId = await resolveTargetAccountId({
      ...baseReport,
      target_type: "user",
      target_id: "profile-user",
    });
    expect(accountId).toBe("acct-resolved");
    expect(resolveAccountIdForProfile).toHaveBeenCalledWith("profile-user");
  });

  it("resolves space targets via owner profile", async () => {
    vi.mocked(apiJson).mockResolvedValue({
      space: { owner_profile_id: "owner-profile" },
    });
    const accountId = await resolveTargetAccountId({
      ...baseReport,
      target_type: "space",
      target_id: "space-1",
    });
    expect(accountId).toBe("acct-resolved");
    expect(apiJson).toHaveBeenCalledWith("/api/v1/spaces/space-1");
    expect(resolveAccountIdForProfile).toHaveBeenCalledWith("owner-profile");
  });

  it("resolves message targets via chat evidence and sender profile", async () => {
    vi.mocked(apiJson).mockResolvedValue({
      message_list: {
        messages: [{ id: "msg-1", sender_profile_id: "sender-profile" }],
      },
    });
    const accountId = await resolveTargetAccountId({
      ...baseReport,
      target_type: "message",
      target_id: "msg-1",
      evidence_json: JSON.stringify({ chat_id: "chat-1", message_id: "msg-1" }),
    });
    expect(accountId).toBe("acct-resolved");
    expect(apiJson).toHaveBeenCalledWith(
      "/api/v1/messages?chat_id=chat-1&page_size=100",
    );
    expect(resolveAccountIdForProfile).toHaveBeenCalledWith("sender-profile");
  });
});
