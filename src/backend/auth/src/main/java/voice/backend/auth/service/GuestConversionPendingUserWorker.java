package voice.backend.auth.service;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Objects;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

/** Executes only the User and Auth-local stages of a durable guest conversion. */
public final class GuestConversionPendingUserWorker {
  private static final Duration DEFAULT_RETRY_DELAY = Duration.ofMinutes(1);

  private final GuestConversionOperationRepository operations;
  private final PrimaryProfileProvisioner primaryProfiles;
  private final GuestConversionLocalPromotion localPromotion;
  private final Clock clock;
  private final GuestConversionRetrySchedule retrySchedule;

  public GuestConversionPendingUserWorker(
      GuestConversionOperationRepository operations,
      PrimaryProfileProvisioner primaryProfiles,
      GuestConversionLocalPromotion localPromotion,
      Clock clock) {
    this(
        operations,
        primaryProfiles,
        localPromotion,
        clock,
        (operation, failure, now) -> now.plus(DEFAULT_RETRY_DELAY));
  }

  public GuestConversionPendingUserWorker(
      GuestConversionOperationRepository operations,
      PrimaryProfileProvisioner primaryProfiles,
      GuestConversionLocalPromotion localPromotion,
      Clock clock,
      GuestConversionRetrySchedule retrySchedule) {
    this.operations = Objects.requireNonNull(operations, "operations");
    this.primaryProfiles = Objects.requireNonNull(primaryProfiles, "primaryProfiles");
    this.localPromotion = Objects.requireNonNull(localPromotion, "localPromotion");
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
    Instant leaseUntil = now.plus(leaseDuration);
    List<GuestConversionOperation> leased = operations.leaseDue(batchSize, now, leaseUntil);
    for (GuestConversionOperation operation : leased) {
      if (operation.state() != GuestConversionState.PENDING_USER) {
        continue;
      }
      process(operation, now);
    }
  }

  private void process(GuestConversionOperation operation, Instant now) {
    try {
      primaryProfiles.clearGuestAccountFlag(operation.accountId());
      GuestConversionAdvanceResult result = localPromotion.promoteAndAdvance(operation, now);
      if (result == GuestConversionAdvanceResult.APPLIED
          || result == GuestConversionAdvanceResult.ALREADY_APPLIED
          || result == GuestConversionAdvanceResult.LEASE_LOST
          || result == GuestConversionAdvanceResult.NOT_FOUND) {
        return;
      }
      throw new IllegalStateException("unsupported guest conversion advance result");
    } catch (RuntimeException failure) {
      Instant nextAttemptAt = retrySchedule.nextAttemptAt(operation, failure, now);
      if (nextAttemptAt == null || nextAttemptAt.isBefore(now)) {
        throw new IllegalStateException("guest conversion retry schedule returned an invalid time", failure);
      }
      operations.recordFailure(
          operation.operationId(),
          operation.lockedUntil(),
          failureCode(failure),
          nextAttemptAt,
          now);
    }
  }

  private static String failureCode(RuntimeException failure) {
    String simpleName = failure.getClass().getSimpleName();
    return simpleName.isBlank() ? "guest_conversion_failure" : simpleName;
  }
}
