package voice.backend.auth.repository;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

/** Deterministic in-memory implementation for the memory/test Auth profile. */
public final class InMemoryGuestConversionOperationRepository
    implements GuestConversionOperationRepository {
  private final Map<UUID, GuestConversionOperation> byOperationId = new HashMap<>();
  private final Map<UUID, UUID> operationIdByAccountId = new HashMap<>();
  private final Map<UUID, UUID> operationIdByOtpId = new HashMap<>();

  @Override
  public synchronized GuestConversionOperation createOrResume(
      UUID accountId, UUID otpCodeId, Instant now) {
    requireNonNull(accountId, "accountId");
    requireNonNull(otpCodeId, "otpCodeId");
    requireNonNull(now, "now");
    UUID existingId = operationIdByAccountId.get(accountId);
    if (existingId != null) {
      return byOperationId.get(existingId);
    }
    if (operationIdByOtpId.containsKey(otpCodeId)) {
      throw new IllegalArgumentException("OTP code is already bound to another account");
    }
    GuestConversionOperation operation =
        new GuestConversionOperation(
            UUID.randomUUID(),
            accountId,
            otpCodeId,
            GuestConversionState.PENDING_USER,
            0,
            now,
            null,
            null,
            null,
            null,
            null,
            now,
            now);
    byOperationId.put(operation.operationId(), operation);
    operationIdByAccountId.put(accountId, operation.operationId());
    operationIdByOtpId.put(otpCodeId, operation.operationId());
    return operation;
  }

  @Override
  public synchronized List<GuestConversionOperation> leaseDue(
      int batchSize, Instant now, Instant leaseUntil) {
    if (batchSize <= 0) {
      throw new IllegalArgumentException("batchSize must be positive");
    }
    requireNonNull(now, "now");
    requireNonNull(leaseUntil, "leaseUntil");
    if (!leaseUntil.isAfter(now)) {
      throw new IllegalArgumentException("leaseUntil must be after now");
    }
    List<GuestConversionOperation> due =
        byOperationId.values().stream()
            .filter(operation -> operation.state() != GuestConversionState.COMPLETED)
            .filter(operation -> !operation.nextAttemptAt().isAfter(now))
            .filter(
                operation ->
                    operation.lockedUntil() == null || !operation.lockedUntil().isAfter(now))
            .sorted(
                Comparator.comparing(GuestConversionOperation::nextAttemptAt)
                    .thenComparing(GuestConversionOperation::createdAt)
                    .thenComparing(GuestConversionOperation::operationId))
            .limit(batchSize)
            .toList();
    List<GuestConversionOperation> leased = new ArrayList<>(due.size());
    for (GuestConversionOperation operation : due) {
      GuestConversionOperation updated = withLease(operation, leaseUntil, now);
      byOperationId.put(updated.operationId(), updated);
      leased.add(updated);
    }
    return List.copyOf(leased);
  }

  @Override
  public synchronized GuestConversionAdvanceResult advance(
      UUID operationId, GuestConversionState expectedState, Instant expectedLockedUntil, Instant now) {
    requireNonNull(operationId, "operationId");
    requireNonNull(expectedState, "expectedState");
    requireNonNull(expectedLockedUntil, "expectedLockedUntil");
    requireNonNull(now, "now");
    GuestConversionOperation current = byOperationId.get(operationId);
    if (current == null) {
      return GuestConversionAdvanceResult.NOT_FOUND;
    }
    if (isTargetOrLater(current, expectedState)) {
      return GuestConversionAdvanceResult.ALREADY_APPLIED;
    }
    if (current.state() != expectedState
        || !expectedLockedUntil.equals(current.lockedUntil())
        || !expectedLockedUntil.isAfter(now)) {
      return GuestConversionAdvanceResult.LEASE_LOST;
    }
    GuestConversionOperation advanced =
        expectedState == GuestConversionState.PENDING_USER
            ? new GuestConversionOperation(
                current.operationId(),
                current.accountId(),
                current.otpCodeId(),
                GuestConversionState.PENDING_EVENT,
                current.attemptCount(),
                current.nextAttemptAt(),
                null,
                current.lastErrorCode(),
                now,
                now,
                null,
                current.createdAt(),
                now)
            : new GuestConversionOperation(
                current.operationId(),
                current.accountId(),
                current.otpCodeId(),
                GuestConversionState.COMPLETED,
                current.attemptCount(),
                current.nextAttemptAt(),
                null,
                current.lastErrorCode(),
                current.userMarkedAt(),
                current.authPromotedAt(),
                now,
                current.createdAt(),
                now);
    byOperationId.put(operationId, advanced);
    return GuestConversionAdvanceResult.APPLIED;
  }

  @Override
  public synchronized Optional<GuestConversionOperation> recordFailure(
      UUID operationId,
      Instant expectedLockedUntil,
      String errorCode,
      Instant nextAttemptAt,
      Instant now) {
    requireNonNull(operationId, "operationId");
    requireNonNull(expectedLockedUntil, "expectedLockedUntil");
    requireNonNull(errorCode, "errorCode");
    requireNonNull(nextAttemptAt, "nextAttemptAt");
    requireNonNull(now, "now");
    GuestConversionOperation current = byOperationId.get(operationId);
    if (current == null
        || current.state() == GuestConversionState.COMPLETED
        || !expectedLockedUntil.equals(current.lockedUntil())) {
      return Optional.empty();
    }
    GuestConversionOperation updated =
        new GuestConversionOperation(
            current.operationId(),
            current.accountId(),
            current.otpCodeId(),
            current.state(),
            current.attemptCount() + 1,
            nextAttemptAt,
            null,
            errorCode,
            current.userMarkedAt(),
            current.authPromotedAt(),
            current.eventPublishedAt(),
            current.createdAt(),
            now);
    byOperationId.put(operationId, updated);
    return Optional.of(updated);
  }

  private static GuestConversionOperation withLease(
      GuestConversionOperation operation, Instant leaseUntil, Instant now) {
    return new GuestConversionOperation(
        operation.operationId(),
        operation.accountId(),
        operation.otpCodeId(),
        operation.state(),
        operation.attemptCount(),
        operation.nextAttemptAt(),
        leaseUntil,
        operation.lastErrorCode(),
        operation.userMarkedAt(),
        operation.authPromotedAt(),
        operation.eventPublishedAt(),
        operation.createdAt(),
        now);
  }

  private static boolean isTargetOrLater(
      GuestConversionOperation operation, GuestConversionState expectedState) {
    return switch (expectedState) {
      case PENDING_USER ->
          operation.state() == GuestConversionState.PENDING_EVENT
              || operation.state() == GuestConversionState.COMPLETED;
      case PENDING_EVENT -> operation.state() == GuestConversionState.COMPLETED;
      case COMPLETED -> false;
    };
  }

  private static void requireNonNull(Object value, String name) {
    if (value == null) {
      throw new NullPointerException(name);
    }
  }
}
