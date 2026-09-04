package voice.backend.auth.service;

import com.google.protobuf.Timestamp;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Objects;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.events.v1.JetstreamEvents.UserGuestConverted;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** Publishes only durable PENDING_EVENT operations after a JetStream PubAck. */
public final class GuestConversionPendingEventWorker {
  private static final String SUBJECT = "user.guest_converted";
  private static final Duration DEFAULT_RETRY_DELAY = Duration.ofMinutes(1);

  private final GuestConversionOperationRepository operations;
  private final GuestConversionEventPublisher publisher;
  private final Clock clock;
  private final GuestConversionRetrySchedule retrySchedule;

  public GuestConversionPendingEventWorker(
      GuestConversionOperationRepository operations,
      GuestConversionEventPublisher publisher,
      Clock clock) {
    this(operations, publisher, clock, (operation, failure, now) -> now.plus(DEFAULT_RETRY_DELAY));
  }

  public GuestConversionPendingEventWorker(
      GuestConversionOperationRepository operations,
      GuestConversionEventPublisher publisher,
      Clock clock,
      GuestConversionRetrySchedule retrySchedule) {
    this.operations = Objects.requireNonNull(operations, "operations");
    this.publisher = Objects.requireNonNull(publisher, "publisher");
    this.clock = Objects.requireNonNull(clock, "clock");
    this.retrySchedule = Objects.requireNonNull(retrySchedule, "retrySchedule");
  }

  public void processDue(int batchSize, Duration leaseDuration) {
    Objects.requireNonNull(leaseDuration, "leaseDuration");
    if (batchSize <= 0) {
      throw new IllegalArgumentException("batchSize must be positive");
    }
    if (leaseDuration.isZero() || leaseDuration.isNegative()) {
      throw new IllegalArgumentException("leaseDuration must be positive");
    }
    Instant now = Instant.now(clock);
    List<GuestConversionOperation> leased = operations.leaseDue(
        GuestConversionState.PENDING_EVENT, batchSize, now, now.plus(leaseDuration));
    for (GuestConversionOperation operation : leased) {
      process(operation, Instant.now(clock));
    }
  }

  private void process(GuestConversionOperation operation, Instant now) {
    try {
      String operationId = operation.operationId().toString();
      GuestConversionPublishAck acknowledgement = publisher.publishGuestConverted(
          SUBJECT, envelope(operation, now), operationId);
      if (acknowledgement == null) {
        throw new IllegalStateException("JetStream did not return a PubAck");
      }
      GuestConversionAdvanceResult result =
          operations.advance(
              operation.operationId(),
              GuestConversionState.PENDING_EVENT,
              operation.lockedUntil(),
              Instant.now(clock));
      if (result != GuestConversionAdvanceResult.APPLIED
          && result != GuestConversionAdvanceResult.ALREADY_APPLIED
          && result != GuestConversionAdvanceResult.LEASE_LOST
          && result != GuestConversionAdvanceResult.NOT_FOUND) {
        throw new IllegalStateException("unsupported guest conversion advance result");
      }
    } catch (RuntimeException failure) {
      Instant failureNow = Instant.now(clock);
      Instant nextAttemptAt = retrySchedule.nextAttemptAt(operation, failure, failureNow);
      if (nextAttemptAt == null || nextAttemptAt.isBefore(failureNow)) {
        throw new IllegalStateException("guest conversion retry schedule returned an invalid time", failure);
      }
      operations.recordFailure(
          operation.operationId(),
          operation.lockedUntil(),
          failureCode(failure),
          nextAttemptAt,
          failureNow);
    }
  }

  private static UserStreamEvent envelope(GuestConversionOperation operation, Instant now) {
    return UserStreamEvent.newBuilder()
        .setEventId(operation.operationId().toString())
        .setOccurredAt(Timestamp.newBuilder().setSeconds(now.getEpochSecond()).setNanos(now.getNano()))
        .setUserGuestConverted(
            UserGuestConverted.newBuilder().setAccountId(operation.accountId().toString()))
        .build();
  }

  private static String failureCode(RuntimeException failure) {
    String simpleName = failure.getClass().getSimpleName();
    return simpleName.isBlank() ? "guest_conversion_failure" : simpleName;
  }
}
