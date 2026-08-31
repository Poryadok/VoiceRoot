import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ReportDetail } from "../components/ReportDetail";

const baseReport = {
  id: "report-1",
  reporter_profile_id: "reporter-1",
  target_type: "user",
  target_id: "user-1",
  category: "spam",
  evidence_json: "{}",
  status: "reviewing",
  resolution_json: "",
  created_at: "2026-06-14T12:00:00Z",
};

describe("ReportDetail resolve workflow", () => {
  it("shows resolve and dismiss actions for open reports", () => {
    render(
      <ReportDetail
        report={baseReport}
        onAssignToMe={vi.fn()}
        assignBusy={false}
        onResolve={vi.fn()}
        onDismiss={vi.fn()}
        closeBusy={false}
      />,
    );

    expect(screen.getByTestId("resolve-report")).toBeInTheDocument();
    expect(screen.getByTestId("dismiss-report")).toBeInTheDocument();
    expect(screen.getByTestId("resolution-note")).toBeInTheDocument();
  });

  it("hides close actions for resolved reports", () => {
    render(
      <ReportDetail
        report={{ ...baseReport, status: "resolved" }}
        onAssignToMe={vi.fn()}
        assignBusy={false}
        onResolve={vi.fn()}
        onDismiss={vi.fn()}
        closeBusy={false}
      />,
    );

    expect(screen.queryByTestId("resolve-report")).not.toBeInTheDocument();
    expect(screen.queryByTestId("dismiss-report")).not.toBeInTheDocument();
  });

  it("invokes resolve with optional resolution note", async () => {
    const user = userEvent.setup();
    const onResolve = vi.fn();
    render(
      <ReportDetail
        report={baseReport}
        onAssignToMe={vi.fn()}
        assignBusy={false}
        onResolve={onResolve}
        onDismiss={vi.fn()}
        closeBusy={false}
      />,
    );

    await user.type(screen.getByTestId("resolution-note"), "Spam confirmed");
    await user.click(screen.getByTestId("resolve-report"));

    expect(onResolve).toHaveBeenCalledWith("Spam confirmed");
  });

  it("invokes dismiss without requiring a note", async () => {
    const user = userEvent.setup();
    const onDismiss = vi.fn();
    render(
      <ReportDetail
        report={baseReport}
        onAssignToMe={vi.fn()}
        assignBusy={false}
        onResolve={vi.fn()}
        onDismiss={onDismiss}
        closeBusy={false}
      />,
    );

    await user.click(screen.getByTestId("dismiss-report"));

    expect(onDismiss).toHaveBeenCalledWith("");
  });

  it("shows close error from parent", () => {
    render(
      <ReportDetail
        report={baseReport}
        onAssignToMe={vi.fn()}
        assignBusy={false}
        onResolve={vi.fn()}
        onDismiss={vi.fn()}
        closeBusy={false}
        closeError="Failed to resolve report"
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Failed to resolve report",
    );
  });
});
