package voice.backend.auth.service;

import com.google.protobuf.Timestamp;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountDeletionState;
import voice.events.v1.JetstreamEvents.UserAccountDeleted;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** Re-publishes one stable JetStream deletion identity until a fenced completion is persisted. */
public final class AccountDeletionPendingEventWorker {
  private final AccountDeletionOperationRepository operations;
  private final AccountDeletionEventPublisher publisher;
  private final Clock clock;

  public AccountDeletionPendingEventWorker(
      AccountDeletionOperationRepository operations, AccountDeletionEventPublisher publisher, Clock clock) {
    this.operations = operations;
    this.publisher = publisher;
    this.clock = clock;
  }

  public void recover(int batchSize, Duration leaseDuration) {
    Instant now = Instant.now(clock);
    for (AccountDeletionOperation operation :
        operations.leaseDue(AccountDeletionState.PENDING_EVENT, batchSize, now, now.plus(leaseDuration))) {
      process(operation);
    }
  }

  /** Lets the request path drive only its own operation; scheduled recovery uses {@link #recover}. */
  public void recoverOperation(java.util.UUID operationId, Duration leaseDuration) {
    Instant now = Instant.now(clock);
    operations
        .lease(operationId, AccountDeletionState.PENDING_EVENT, now, now.plus(leaseDuration))
        .ifPresent(this::process);
  }

  private void process(AccountDeletionOperation operation) {
    try {
      String eventId = operation.operationId().toString();
      publisher.publishAccountDeleted(
          AuthEventPublisher.SUBJECT_ACCOUNT_DELETED,
          UserStreamEvent.newBuilder()
              .setEventId(eventId)
              .setOccurredAt(timestamp(operation.createdAt()))
              .setUserAccountDeleted(
                  UserAccountDeleted.newBuilder().setAccountId(operation.accountId().toString()))
              .build(),
          eventId);
      operations.markEventPublished(operation.operationId(), operation.lockedUntil(), Instant.now(clock));
    } catch (RuntimeException failure) {
      Instant failedAt = Instant.now(clock);
      operations.recordFailure(
          operation.operationId(), operation.lockedUntil(), "publish_account_deleted",
          failedAt, failedAt);
    }
  }

  private static Timestamp timestamp(Instant instant) {
    return Timestamp.newBuilder().setSeconds(instant.getEpochSecond()).setNanos(instant.getNano()).build();
  }
}
