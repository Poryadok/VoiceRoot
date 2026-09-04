package voice.backend.auth.repository;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatIllegalArgumentException;
import static org.assertj.core.api.Assertions.assertThatNullPointerException;

import java.time.Instant;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

class InMemoryGuestConversionOperationRepositoryContractTest {
  private static final Instant NOW = Instant.parse("2026-09-04T10:15:30Z");
  private static final Instant LEASE_UNTIL = NOW.plusSeconds(60);

  @Test
  void recordFailureValidatesTheSameRetryInputsAsJdbc() {
    InMemoryGuestConversionOperationRepository repository =
        new InMemoryGuestConversionOperationRepository();
    GuestConversionOperation operation = leased(repository);

    assertThatNullPointerException()
        .isThrownBy(
            () ->
                repository.recordFailure(
                    operation.operationId(), operation.lockedUntil(), null, NOW.plusSeconds(1), NOW));
    assertThatIllegalArgumentException()
        .isThrownBy(
            () ->
                repository.recordFailure(
                    operation.operationId(), operation.lockedUntil(), " ", NOW.plusSeconds(1), NOW));
    assertThatIllegalArgumentException()
        .isThrownBy(
            () ->
                repository.recordFailure(
                    operation.operationId(), operation.lockedUntil(), "retryable", NOW.minusSeconds(1), NOW));
  }

  @Test
  void expiredOrMismatchedFailureLeaseIsANoOp() {
    InMemoryGuestConversionOperationRepository repository =
        new InMemoryGuestConversionOperationRepository();
    GuestConversionOperation operation = leased(repository);

    assertThat(
            repository.recordFailure(
                operation.operationId(),
                operation.lockedUntil(),
                "retryable",
                LEASE_UNTIL.plusSeconds(1),
                LEASE_UNTIL))
        .isEmpty();
    assertThat(
            repository.recordFailure(
                operation.operationId(),
                operation.lockedUntil().plusSeconds(1),
                "retryable",
                NOW.plusSeconds(1),
                NOW))
        .isEmpty();
  }

  @Test
  void advanceRequiresTheExactUnexpiredLeaseAndPreservesPendingEventBoundaries() {
    InMemoryGuestConversionOperationRepository repository =
        new InMemoryGuestConversionOperationRepository();
    GuestConversionOperation operation = leased(repository);

    assertThatIllegalArgumentException()
        .isThrownBy(
            () ->
                repository.advance(
                    operation.operationId(),
                    GuestConversionState.COMPLETED,
                    operation.lockedUntil(),
                    NOW));
    assertThat(
            repository.advance(
                operation.operationId(),
                GuestConversionState.PENDING_USER,
                operation.lockedUntil(),
                LEASE_UNTIL))
        .isEqualTo(GuestConversionAdvanceResult.LEASE_LOST);
    assertThat(
            repository.advance(
                operation.operationId(),
                GuestConversionState.PENDING_USER,
                operation.lockedUntil().plusSeconds(1),
                NOW))
        .isEqualTo(GuestConversionAdvanceResult.LEASE_LOST);
    assertThat(
            repository.advance(
                operation.operationId(),
                GuestConversionState.PENDING_USER,
                operation.lockedUntil(),
                NOW.plusSeconds(1)))
        .isEqualTo(GuestConversionAdvanceResult.APPLIED);

    GuestConversionOperation pendingEvent =
        repository.leaseDue(1, NOW.plusSeconds(1), NOW.plusSeconds(61)).getFirst();
    assertThat(pendingEvent.state()).isEqualTo(GuestConversionState.PENDING_EVENT);
    assertThat(
            repository.advance(
                pendingEvent.operationId(),
                GuestConversionState.PENDING_EVENT,
                pendingEvent.lockedUntil(),
                NOW.plusSeconds(2)))
        .isEqualTo(GuestConversionAdvanceResult.APPLIED);
  }

  @Test
  void concurrentExactLeaseAdvanceHasOneAppliedWinnerAndOneIdempotentRecoveryResult()
      throws Exception {
    InMemoryGuestConversionOperationRepository repository =
        new InMemoryGuestConversionOperationRepository();
    GuestConversionOperation operation = leased(repository);
    CountDownLatch start = new CountDownLatch(1);
    ExecutorService executor = Executors.newFixedThreadPool(2);
    try {
      var first = executor.submit(() -> advance(repository, operation, start));
      var second = executor.submit(() -> advance(repository, operation, start));
      start.countDown();

      assertThat(List.of(first.get(1, TimeUnit.SECONDS), second.get(1, TimeUnit.SECONDS)))
          .containsExactlyInAnyOrder(
              GuestConversionAdvanceResult.APPLIED, GuestConversionAdvanceResult.ALREADY_APPLIED);
    } finally {
      executor.shutdownNow();
    }
  }

  @Test
  void stateAwareClaimsKeepPendingUserAndPendingEventWorkDisjoint() {
    InMemoryGuestConversionOperationRepository repository =
        new InMemoryGuestConversionOperationRepository();
    GuestConversionOperation pendingUser = leased(repository);
    assertThat(
            repository.advance(
                pendingUser.operationId(),
                GuestConversionState.PENDING_USER,
                pendingUser.lockedUntil(),
                NOW.plusSeconds(1)))
        .isEqualTo(GuestConversionAdvanceResult.APPLIED);

    assertThat(
            repository.leaseDue(
                GuestConversionState.PENDING_USER,
                1,
                NOW.plusSeconds(1),
                NOW.plusSeconds(61)))
        .isEmpty();
    assertThat(
            repository.leaseDue(
                GuestConversionState.PENDING_EVENT,
                1,
                NOW.plusSeconds(1),
                NOW.plusSeconds(61)))
        .singleElement()
        .satisfies(operation -> assertThat(operation.state()).isEqualTo(GuestConversionState.PENDING_EVENT));
  }

  private static GuestConversionAdvanceResult advance(
      InMemoryGuestConversionOperationRepository repository,
      GuestConversionOperation operation,
      CountDownLatch start) {
    try {
      start.await();
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw new IllegalStateException(interrupted);
    }
    return repository.advance(
        operation.operationId(),
        GuestConversionState.PENDING_USER,
        operation.lockedUntil(),
        NOW.plusSeconds(1));
  }

  private static GuestConversionOperation leased(InMemoryGuestConversionOperationRepository repository) {
    repository.createOrResume(UUID.randomUUID(), UUID.randomUUID(), NOW);
    return repository.leaseDue(1, NOW, LEASE_UNTIL).getFirst();
  }
}
