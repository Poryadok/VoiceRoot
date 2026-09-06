package voice.backend.auth.service;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Objects;
import java.util.UUID;
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
    List<GuestConversionOperation> leased =
        operations.leaseDue(GuestConversionState.PENDING_USER, batchSize, now, leaseUntil);
    for (GuestConversionOperation operation : leased) {
      if (operation.state() != GuestConversionState.PENDING_USER) {
        continue;
      }
      process(operation, Instant.now(clock));
    }
  }

  /** Completes the User-policy and Auth-local stages for one just-accepted conversion. */
  public boolean processDueForAccount(UUID accountId, Duration leaseDuration) {
    Objects.requireNonNull(accountId, "accountId");
    Objects.requireNonNull(leaseDuration, "leaseDuration");
    if (leaseDuration.isZero() || leaseDuration.isNegative()) {
      throw new IllegalArgumentException("leaseDuration must be positive");
    }
    Instant now = Instant.now(clock);
    return operations
        .leaseDueForAccount(
            GuestConversionState.PENDING_USER, accountId, now, now.plus(leaseDuration))
        .map(operation -> process(operation, Instant.now(clock)))
        .orElse(false);
  }

  private boolean process(GuestConversionOperation operation, Instant now) {
    try {
      primaryProfiles.clearGuestAccountFlag(operation.accountId());
      GuestConversionAdvanceResult result =
          localPromotion.promoteAndAdvance(operation, Instant.now(clock));
      if (result == GuestConversionAdvanceResult.APPLIED
          || result == GuestConversionAdvanceResult.ALREADY_APPLIED) {
        return true;
      }
      if (result == GuestConversionAdvanceResult.LEASE_LOST
          || result == GuestConversionAdvanceResult.NOT_FOUND) {
        return false;
      }
      throw new IllegalStateException("unsupported guest conversion advance result");
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
      return false;
    }
  }

  private static String failureCode(RuntimeException failure) {
    String simpleName = failure.getClass().getSimpleName();
    return simpleName.isBlank() ? "guest_conversion_failure" : simpleName;
  }
}
