import { useCallback, useEffect, useState } from "react";
import { listAppeals, reviewAppeal } from "../api/moderation";
import type { Appeal, AppealStatus } from "../api/types";
import { APPEAL_STATUSES } from "../api/types";
import { AppealTable } from "../components/AppealTable";

export function AppealsPage() {
  const [statusFilter, setStatusFilter] = useState<AppealStatus | "">("pending");
  const [appeals, setAppeals] = useState<Appeal[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [selected, setSelected] = useState<Appeal | null>(null);
  const [moderatorNote, setModeratorNote] = useState("");
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [reviewBusy, setReviewBusy] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [reviewError, setReviewError] = useState<string | undefined>();

  const loadAppeals = useCallback(
    async (cursor?: string) => {
      const append = Boolean(cursor);
      if (append) {
        setLoadingMore(true);
      } else {
        setLoading(true);
      }
      setError(undefined);
      try {
        const response = await listAppeals({
          status: statusFilter || undefined,
          cursor,
        });
        const pageAppeals = response.appeal_list?.appeals ?? [];
        setAppeals((current) =>
          append ? [...current, ...pageAppeals] : pageAppeals,
        );
        setNextCursor(response.appeal_list?.next_cursor || undefined);
      } catch (err) {
        if (!append) {
          setAppeals([]);
        }
        setError(err instanceof Error ? err.message : "Failed to load appeals");
      } finally {
        if (append) {
          setLoadingMore(false);
        } else {
          setLoading(false);
        }
      }
    },
    [statusFilter],
  );

  useEffect(() => {
    void loadAppeals();
  }, [loadAppeals]);

  useEffect(() => {
    setSelected(null);
    setModeratorNote("");
    setReviewError(undefined);
  }, [statusFilter]);

  async function handleReview(status: "approved" | "denied") {
    if (!selected) {
      return;
    }
    setReviewBusy(true);
    setReviewError(undefined);
    try {
      await reviewAppeal(selected.id, {
        status,
        moderator_note: moderatorNote.trim() || undefined,
      });
      setSelected(null);
      setModeratorNote("");
      await loadAppeals();
    } catch (err) {
      setReviewError(
        err instanceof Error ? err.message : `Failed to ${status} appeal`,
      );
    } finally {
      setReviewBusy(false);
    }
  }

  return (
    <div>
      <h2>Appeals</h2>

      <div className="filters" data-testid="appeals-filters">
        <label>
          Status
          <select
            data-testid="appeals-filter-status"
            value={statusFilter}
            onChange={(event) =>
              setStatusFilter(event.target.value as AppealStatus | "")
            }
          >
            <option value="">All statuses</option>
            {APPEAL_STATUSES.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </label>
      </div>

      {loading ? <p className="status-message">Loading appeals…</p> : null}
      {error ? (
        <p className="status-message error" role="alert">
          {error}
        </p>
      ) : null}

      <div className="queue-layout">
        <section className="panel" aria-label="Appeals queue">
          <AppealTable
            appeals={appeals}
            selectedId={selected?.id}
            onSelect={setSelected}
          />
          {nextCursor ? (
            <div className="btn-row">
              <button
                type="button"
                className="btn"
                onClick={() => void loadAppeals(nextCursor)}
                disabled={loadingMore}
                data-testid="load-more-appeals"
              >
                {loadingMore ? "Loading…" : "Load more"}
              </button>
            </div>
          ) : null}
        </section>

        <section className="panel" aria-label="Appeal detail">
          {!selected ? (
            <p className="status-message">Select an appeal to review.</p>
          ) : (
            <>
              <h3>Appeal {selected.id.slice(0, 8)}…</h3>
              <dl className="detail-list">
                <div>
                  <dt>Status</dt>
                  <dd>{selected.status}</dd>
                </div>
                <div>
                  <dt>Appellant account</dt>
                  <dd>{selected.appellant_account_id}</dd>
                </div>
                <div>
                  <dt>Sanction</dt>
                  <dd>{selected.sanction_id}</dd>
                </div>
                <div>
                  <dt>Reason</dt>
                  <dd>{selected.reason}</dd>
                </div>
                <div>
                  <dt>Created</dt>
                  <dd>{selected.created_at}</dd>
                </div>
              </dl>
              {selected.status === "pending" ? (
                <>
                  <label>
                    Moderator note
                    <textarea
                      data-testid="appeal-moderator-note"
                      value={moderatorNote}
                      onChange={(event) => setModeratorNote(event.target.value)}
                      rows={3}
                    />
                  </label>
                  <div className="btn-row">
                    <button
                      type="button"
                      className="btn"
                      disabled={reviewBusy}
                      data-testid="approve-appeal"
                      onClick={() => void handleReview("approved")}
                    >
                      Approve
                    </button>
                    <button
                      type="button"
                      className="btn"
                      disabled={reviewBusy}
                      data-testid="deny-appeal"
                      onClick={() => void handleReview("denied")}
                    >
                      Deny
                    </button>
                  </div>
                </>
              ) : null}
              {reviewError ? (
                <p className="status-message error" role="alert">
                  {reviewError}
                </p>
              ) : null}
            </>
          )}
        </section>
      </div>
    </div>
  );
}
