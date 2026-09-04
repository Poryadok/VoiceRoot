package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;

/**
 * RED contract for the durable, pre-event portion of verified guest conversion.
 *
 * <p>There is deliberately no HTTP assertion or retry-delay policy here. The existing JDBC
 * repository contract owns durable fencing/retry metadata; this unit suite drives the missing
 * request acceptance and PENDING_USER orchestration seams with deterministic fakes.
 */
class GuestConversionPendingUserOrchestratorTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Duration USER_RPC_DEADLINE = Duration.ofSeconds(2);
  private static final Duration LEASE = Duration.ofMinutes(1);

  @Test
  void verifiedEmailOtpAcceptance_createsOrResumesTheDurableOperationBeforeAnyUserWork() {
    RecordingOperations operations = new RecordingOperations();
    UUID accountId = UUID.randomUUID();
    UUID otpId = UUID.randomUUID();

    new GuestConversionOtpAcceptance(operations).acceptVerifiedEmailOtp(accountId, otpId, NOW);

    assertThat(operations.createCalls).containsExactly(new CreateCall(accountId, otpId, NOW));
    assertThat(operations.leased).isEmpty();
  }

  @Test
  void verifiedEmailOtpAcceptance_retriesReuseTheOriginalOperationAndOtpIdentity() {
    RecordingOperations operations = new RecordingOperations();
    UUID accountId = UUID.randomUUID();
    UUID firstOtpId = UUID.randomUUID();
    UUID retryOtpId = UUID.randomUUID();
    GuestConversionOtpAcceptance acceptance = new GuestConversionOtpAcceptance(operations);

    acceptance.acceptVerifiedEmailOtp(accountId, firstOtpId, NOW);
    acceptance.acceptVerifiedEmailOtp(accountId, retryOtpId, NOW.plusSeconds(1));

    assertThat(operations.operations()).hasSize(1);
    GuestConversionOperation operation = operations.operations().getFirst();
    assertThat(operation.accountId()).isEqualTo(accountId);
    assertThat(operation.otpCodeId()).isEqualTo(firstOtpId);
    assertThat(operations.createCalls)
        .containsExactly(
            new CreateCall(accountId, firstOtpId, NOW),
            new CreateCall(accountId, retryOtpId, NOW.plusSeconds(1)));
  }

  @Test
  void otpRequestPath_delegatesVerifiedGuestAcceptanceInsteadOfCallingUserOrEventContinuation() throws IOException {
    String otpService =
        Files.readString(
            Path.of("src/main/java/voice/backend/auth/service/OtpService.java").toAbsolutePath());

    assertThat(otpService)
        .contains("GuestConversionOtpAcceptance")
        .doesNotContain("completeVerifiedGuestConversion")
        .doesNotContain("publishGuestConverted");
  }

  @Test
  void pendingUserWorker_marksUserWithBoundedInternalOperationTrace_thenPromotesAndAdvancesExactLease() {
    RecordingOperations operations = new RecordingOperations();
    GuestConversionOperation operation = operations.createOrResume(UUID.randomUUID(), UUID.randomUUID(), NOW);
    operations.lease(operation, NOW.plus(LEASE));
    RecordingUserMarker user = new RecordingUserMarker();
    RecordingAuthPromoter auth = new RecordingAuthPromoter();
    GuestConversionPendingUserWorker worker =
        new GuestConversionPendingUserWorker(operations, user, auth, Clock.fixed(NOW, ZoneOffset.UTC));

    worker.processDue(1, LEASE, USER_RPC_DEADLINE);

    assertThat(user.calls)
        .containsExactly(
            new GuestConversionUserMarker.Call(
                operation.accountId(), operation.operationId(), USER_RPC_DEADLINE));
    assertThat(auth.promotedAccountIds).containsExactly(operation.accountId());
    assertThat(operations.advances)
        .containsExactly(
            new AdvanceCall(
                operation.operationId(), GuestConversionState.PENDING_USER, NOW.plus(LEASE), NOW));
    assertThat(operations.failures).isEmpty();
  }

  @Test
  void pendingUserWorker_recordsRemoteOrLocalFailureWithoutLeavingPendingUser_andRepeatsIdempotentUserCallAfterCrash() {
    RecordingOperations operations = new RecordingOperations();
    GuestConversionOperation operation = operations.createOrResume(UUID.randomUUID(), UUID.randomUUID(), NOW);
    operations.lease(operation, NOW.plus(LEASE));
    RecordingUserMarker user = new RecordingUserMarker();
    RecordingAuthPromoter auth = new RecordingAuthPromoter();
    auth.failure = new IllegalStateException("auth DB interrupted after User success");
    GuestConversionPendingUserWorker worker =
        new GuestConversionPendingUserWorker(operations, user, auth, Clock.fixed(NOW, ZoneOffset.UTC));

    worker.processDue(1, LEASE, USER_RPC_DEADLINE);

    assertThat(user.calls).hasSize(1);
    assertThat(auth.promotedAccountIds).containsExactly(operation.accountId());
    assertThat(operations.current(operation.operationId()).state()).isEqualTo(GuestConversionState.PENDING_USER);
    assertThat(operations.failures).hasSize(1);
    assertThat(operations.advances).isEmpty();

    auth.failure = null;
    operations.lease(operations.current(operation.operationId()), NOW.plus(LEASE));
    worker.processDue(1, LEASE, USER_RPC_DEADLINE);

    assertThat(user.calls)
        .extracting(GuestConversionUserMarker.Call::accountId)
        .containsExactly(operation.accountId(), operation.accountId());
    assertThat(auth.promotedAccountIds).containsExactly(operation.accountId(), operation.accountId());
    assertThat(operations.advances).hasSize(1);
  }

  @Test
  void pendingUserWorker_recordsUserFailureWithoutPromotingOrAdvancing() {
    RecordingOperations operations = new RecordingOperations();
    GuestConversionOperation operation = operations.createOrResume(UUID.randomUUID(), UUID.randomUUID(), NOW);
    operations.lease(operation, NOW.plus(LEASE));
    RecordingUserMarker user = new RecordingUserMarker();
    user.failure = new IllegalStateException("User unavailable");
    RecordingAuthPromoter auth = new RecordingAuthPromoter();
    GuestConversionPendingUserWorker worker =
        new GuestConversionPendingUserWorker(operations, user, auth, Clock.fixed(NOW, ZoneOffset.UTC));

    worker.processDue(1, LEASE, USER_RPC_DEADLINE);

    assertThat(auth.promotedAccountIds).isEmpty();
    assertThat(operations.advances).isEmpty();
    assertThat(operations.current(operation.operationId()).state()).isEqualTo(GuestConversionState.PENDING_USER);
    assertThat(operations.failures).hasSize(1);
  }

  @Test
  void pendingUserWorker_doesNotPromoteOrAdvanceWhenItsLeaseExpiresAfterUserReturns() {
    RecordingOperations operations = new RecordingOperations();
    GuestConversionOperation operation = operations.createOrResume(UUID.randomUUID(), UUID.randomUUID(), NOW);
    Instant leaseUntil = NOW.plus(LEASE);
    operations.lease(operation, leaseUntil);
    RecordingUserMarker user = new RecordingUserMarker();
    user.afterCall = () -> operations.clock = leaseUntil;
    RecordingAuthPromoter auth = new RecordingAuthPromoter();
    GuestConversionPendingUserWorker worker =
        new GuestConversionPendingUserWorker(operations, user, auth, operations);

    worker.processDue(1, LEASE, USER_RPC_DEADLINE);

    assertThat(user.calls).hasSize(1);
    assertThat(auth.promotedAccountIds).isEmpty();
    assertThat(operations.advances).isEmpty();
    assertThat(operations.current(operation.operationId()).state()).isEqualTo(GuestConversionState.PENDING_USER);
  }

  @Test
  void pendingUserWorker_treatsAlreadyRegularLocalAuthAsRecoverySuccess() {
    RecordingOperations operations = new RecordingOperations();
    GuestConversionOperation operation = operations.createOrResume(UUID.randomUUID(), UUID.randomUUID(), NOW);
    operations.lease(operation, NOW.plus(LEASE));
    RecordingUserMarker user = new RecordingUserMarker();
    RecordingAuthPromoter auth = new RecordingAuthPromoter();
    auth.alreadyRegular = true;
    GuestConversionPendingUserWorker worker =
        new GuestConversionPendingUserWorker(operations, user, auth, Clock.fixed(NOW, ZoneOffset.UTC));

    worker.processDue(1, LEASE, USER_RPC_DEADLINE);

    assertThat(user.calls).hasSize(1);
    assertThat(auth.promotedAccountIds).containsExactly(operation.accountId());
    assertThat(operations.advances).hasSize(1);
    assertThat(operations.failures).isEmpty();
  }

  /** Minimal fake for the existing durable repository; no database semantics are duplicated here. */
  private static final class RecordingOperations extends Clock
      implements GuestConversionOperationRepository {
    private final List<GuestConversionOperation> rows = new ArrayList<>();
    private final List<CreateCall> createCalls = new ArrayList<>();
    private final List<AdvanceCall> advances = new ArrayList<>();
    private final List<FailureCall> failures = new ArrayList<>();
    private final List<GuestConversionOperation> leased = new ArrayList<>();
    private Instant clock = NOW;

    @Override
    public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) {
      createCalls.add(new CreateCall(accountId, otpCodeId, now));
      return rows.stream()
          .filter(row -> row.accountId().equals(accountId))
          .findFirst()
          .orElseGet(
              () -> {
                GuestConversionOperation row = row(UUID.randomUUID(), accountId, otpCodeId, now, null);
                rows.add(row);
                return row;
              });
    }

    void lease(GuestConversionOperation operation, Instant lockedUntil) {
      replace(withLease(operation, lockedUntil));
    }

    GuestConversionOperation current(UUID operationId) {
      return rows.stream().filter(row -> row.operationId().equals(operationId)).findFirst().orElseThrow();
    }

    List<GuestConversionOperation> operations() {
      return List.copyOf(rows);
    }

    @Override
    public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil) {
      return List.copyOf(leased);
    }

    @Override
    public GuestConversionAdvanceResult advance(
        UUID operationId, GuestConversionState expectedState, Instant expectedLockedUntil, Instant now) {
      advances.add(new AdvanceCall(operationId, expectedState, expectedLockedUntil, now));
      GuestConversionOperation current = current(operationId);
      if (!expectedLockedUntil.equals(current.lockedUntil()) || !expectedLockedUntil.isAfter(now)) {
        return GuestConversionAdvanceResult.LEASE_LOST;
      }
      replace(
          new GuestConversionOperation(
              current.operationId(), current.accountId(), current.otpCodeId(),
              GuestConversionState.PENDING_EVENT, current.attemptCount(), current.nextAttemptAt(), null,
              current.lastErrorCode(), now, now, null, current.createdAt(), now));
      return GuestConversionAdvanceResult.APPLIED;
    }

    @Override
    public Optional<GuestConversionOperation> recordFailure(
        UUID operationId, Instant expectedLockedUntil, String errorCode, Instant nextAttemptAt, Instant now) {
      failures.add(new FailureCall(operationId, expectedLockedUntil, errorCode, nextAttemptAt, now));
      GuestConversionOperation current = current(operationId);
      if (!expectedLockedUntil.equals(current.lockedUntil()) || !expectedLockedUntil.isAfter(now)) {
        return Optional.empty();
      }
      GuestConversionOperation failed =
          new GuestConversionOperation(
              current.operationId(), current.accountId(), current.otpCodeId(), current.state(),
              current.attemptCount() + 1, nextAttemptAt, null, errorCode, current.userMarkedAt(),
              current.authPromotedAt(), current.eventPublishedAt(), current.createdAt(), now);
      replace(failed);
      return Optional.of(failed);
    }

    @Override public ZoneOffset getZone() { return ZoneOffset.UTC; }
    @Override public Clock withZone(java.time.ZoneId zone) { return this; }
    @Override public Instant instant() { return clock; }

    private void replace(GuestConversionOperation replacement) {
      rows.replaceAll(row -> row.operationId().equals(replacement.operationId()) ? replacement : row);
      leased.clear();
      if (replacement.lockedUntil() != null) leased.add(replacement);
    }
  }

  private static final class RecordingUserMarker implements GuestConversionUserMarker {
    private final List<GuestConversionUserMarker.Call> calls = new ArrayList<>();
    private RuntimeException failure;
    private Runnable afterCall = () -> {};

    @Override
    public void markAccountRegular(GuestConversionUserMarker.Call call) {
      calls.add(call);
      afterCall.run();
      if (failure != null) throw failure;
    }
  }

  private static final class RecordingAuthPromoter implements GuestConversionAuthPromoter {
    private final List<UUID> promotedAccountIds = new ArrayList<>();
    private RuntimeException failure;
    private boolean alreadyRegular;

    @Override
    public void promoteGuestAccount(UUID accountId) {
      promotedAccountIds.add(accountId);
      if (failure != null) throw failure;
      if (alreadyRegular) throw new GuestConversionAuthPromoter.AlreadyRegular(accountId);
    }
  }

  private static GuestConversionOperation row(
      UUID operationId, UUID accountId, UUID otpId, Instant now, Instant lockedUntil) {
    return new GuestConversionOperation(
        operationId, accountId, otpId, GuestConversionState.PENDING_USER, 0, now, lockedUntil, null,
        null, null, null, now, now);
  }

  private static GuestConversionOperation withLease(GuestConversionOperation operation, Instant lockedUntil) {
    return new GuestConversionOperation(
        operation.operationId(), operation.accountId(), operation.otpCodeId(), operation.state(),
        operation.attemptCount(), operation.nextAttemptAt(), lockedUntil, operation.lastErrorCode(),
        operation.userMarkedAt(), operation.authPromotedAt(), operation.eventPublishedAt(),
        operation.createdAt(), operation.updatedAt());
  }

  private record CreateCall(UUID accountId, UUID otpCodeId, Instant now) {}
  private record AdvanceCall(UUID operationId, GuestConversionState state, Instant leaseUntil, Instant now) {}
  private record FailureCall(UUID operationId, Instant leaseUntil, String error, Instant retryAt, Instant now) {}
}
