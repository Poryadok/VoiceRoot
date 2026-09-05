package voice.backend.auth.service;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountDeletionState;
import voice.backend.auth.sessionepoch.SessionEpochFloorMissingException;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

/** Recovers durable deletion epoch floors without requiring the original JWT. */
public final class AccountDeletionPendingFloorWorker {
  private final AccountDeletionOperationRepository operations;
  private final SessionEpochFloorStore floors;
  private final Clock clock;

  public AccountDeletionPendingFloorWorker(
      AccountDeletionOperationRepository operations, SessionEpochFloorStore floors, Clock clock) {
    this.operations = operations;
    this.floors = floors;
    this.clock = clock;
  }

  public void recover(int batchSize, Duration leaseDuration) {
    Instant now = Instant.now(clock);
    for (AccountDeletionOperation operation :
        operations.leaseDue(AccountDeletionState.PENDING_FLOOR, batchSize, now, now.plus(leaseDuration))) {
      process(operation);
    }
  }

  /** Lets the request path drive only its own operation; scheduled recovery uses {@link #recover}. */
  public void recoverOperation(java.util.UUID operationId, Duration leaseDuration) {
    Instant now = Instant.now(clock);
    operations
        .lease(operationId, AccountDeletionState.PENDING_FLOOR, now, now.plus(leaseDuration))
        .ifPresent(this::process);
  }

  private void process(AccountDeletionOperation operation) {
    try {
      long floor = currentFloor(operation);
      if (floor < operation.sessionEpoch()) {
        floor = floors.recordAtLeast(operation.accountId(), operation.sessionEpoch());
      }
      if (floor < operation.sessionEpoch()) {
        throw new IllegalStateException("session epoch floor did not reach durable epoch");
      }
      operations.markFloorRecorded(operation.operationId(), operation.lockedUntil(), Instant.now(clock));
    } catch (RuntimeException failure) {
      Instant failedAt = Instant.now(clock);
      operations.recordFailure(
          operation.operationId(), operation.lockedUntil(), "epoch_floor", failedAt, failedAt);
    }
  }

  private long currentFloor(AccountDeletionOperation operation) {
    try {
      return floors.requireFloor(operation.accountId());
    } catch (SessionEpochFloorMissingException ignored) {
      return 0L;
    }
  }
}
