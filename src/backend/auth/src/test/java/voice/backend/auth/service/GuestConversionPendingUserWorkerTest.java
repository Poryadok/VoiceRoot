package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

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
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

class GuestConversionPendingUserWorkerTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Duration LEASE = Duration.ofMinutes(1);

  @Test
  void userFailureKeepsPendingUserAndRecordsFailureWithTheExactLease() {
    RecordingOperations operations = new RecordingOperations(operation());
    RecordingUser user = new RecordingUser();
    user.failure = new IllegalStateException("User unavailable");
    RecordingLocalPromotion local = new RecordingLocalPromotion();

    new GuestConversionPendingUserWorker(operations, user, local, Clock.fixed(NOW, ZoneOffset.UTC))
        .processDue(1, LEASE);

    assertThat(user.accountIds).containsExactly(operations.operation.accountId());
    assertThat(local.calls).isEmpty();
    assertThat(operations.leaseCalls).containsExactly(new LeaseCall(1, NOW, NOW.plus(LEASE)));
    assertThat(operations.failures).hasSize(1);
    FailureCall failure = operations.failures.getFirst();
    assertThat(failure.operationId()).isEqualTo(operations.operation.operationId());
    assertThat(failure.leaseUntil()).isEqualTo(operations.operation.lockedUntil());
    assertThat(failure.errorCode()).isNotBlank();
    assertThat(failure.nextAttemptAt()).isAfter(NOW);
  }

  @Test
  void localFailureKeepsPendingUserAndARecoveryRunRepeatsTheIdempotentUserRpc() {
    RecordingOperations operations = new RecordingOperations(operation());
    RecordingUser user = new RecordingUser();
    RecordingLocalPromotion local = new RecordingLocalPromotion();
    local.failure = new IllegalStateException("Auth persistence unavailable");
    GuestConversionPendingUserWorker worker =
        new GuestConversionPendingUserWorker(operations, user, local, Clock.fixed(NOW, ZoneOffset.UTC));

    worker.processDue(1, LEASE);
    FailureCall localFailure = operations.failures.getFirst();
    local.failure = null;
    operations.requeueWithLease(NOW.plus(LEASE));
    worker.processDue(1, LEASE);

    assertThat(user.accountIds).containsExactly(operations.operation.accountId(), operations.operation.accountId());
    assertThat(operations.failures).hasSize(1);
    assertThat(localFailure.operationId()).isEqualTo(operations.operation.operationId());
    assertThat(localFailure.leaseUntil()).isEqualTo(NOW.plus(LEASE));
    assertThat(localFailure.errorCode()).isNotBlank();
    assertThat(localFailure.nextAttemptAt()).isAfter(NOW);
    assertThat(local.calls).hasSize(2);
  }

  @Test
  void pendingEventLeaseIsIgnoredByThePendingUserWorker() {
    RecordingOperations operations = new RecordingOperations(operation(GuestConversionState.PENDING_EVENT));
    RecordingUser user = new RecordingUser();
    RecordingLocalPromotion local = new RecordingLocalPromotion();

    new GuestConversionPendingUserWorker(operations, user, local, Clock.fixed(NOW, ZoneOffset.UTC))
        .processDue(1, LEASE);

    assertThat(user.accountIds).isEmpty();
    assertThat(local.calls).isEmpty();
    assertThat(operations.failures).isEmpty();
  }

  private static GuestConversionOperation operation() {
    return operation(GuestConversionState.PENDING_USER);
  }

  private static GuestConversionOperation operation(GuestConversionState state) {
    return new GuestConversionOperation(
        UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), state, 0, NOW,
        NOW.plus(LEASE), null, null, null, null, NOW, NOW);
  }

  private static final class RecordingOperations implements GuestConversionOperationRepository {
    private GuestConversionOperation operation;
    private final List<LeaseCall> leaseCalls = new ArrayList<>();
    private final List<FailureCall> failures = new ArrayList<>();
    private RecordingOperations(GuestConversionOperation operation) { this.operation = operation; }
    void requeueWithLease(Instant lease) {
      operation = new GuestConversionOperation(operation.operationId(), operation.accountId(), operation.otpCodeId(),
          operation.state(), operation.attemptCount(), NOW, lease, operation.lastErrorCode(), operation.userMarkedAt(),
          operation.authPromotedAt(), operation.eventPublishedAt(), operation.createdAt(), NOW);
    }
    @Override public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) { throw new UnsupportedOperationException(); }
    @Override public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant leaseUntil) {
      leaseCalls.add(new LeaseCall(batchSize, now, leaseUntil));
      return List.of(operation);
    }
    @Override public GuestConversionAdvanceResult advance(UUID id, GuestConversionState state, Instant lease, Instant now) { throw new UnsupportedOperationException(); }
    @Override public Optional<GuestConversionOperation> recordFailure(UUID id, Instant lease, String error, Instant retry, Instant now) {
      failures.add(new FailureCall(id, lease, error, retry, now));
      return Optional.of(operation);
    }
  }

  private static final class RecordingUser implements PrimaryProfileProvisioner {
    private final List<UUID> accountIds = new ArrayList<>();
    private RuntimeException failure;
    @Override public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) { throw new UnsupportedOperationException(); }
    @Override public void clearGuestAccountFlag(UUID accountId) { accountIds.add(accountId); if (failure != null) throw failure; }
  }

  private static final class RecordingLocalPromotion implements GuestConversionLocalPromotion {
    private final List<GuestConversionOperation> calls = new ArrayList<>();
    private RuntimeException failure;
    @Override public GuestConversionAdvanceResult promoteAndAdvance(GuestConversionOperation operation, Instant now) {
      calls.add(operation); if (failure != null) throw failure; return GuestConversionAdvanceResult.APPLIED;
    }
  }

  private record FailureCall(UUID operationId, Instant leaseUntil, String errorCode, Instant nextAttemptAt, Instant now) {}
  private record LeaseCall(int batchSize, Instant now, Instant leaseUntil) {}
}
