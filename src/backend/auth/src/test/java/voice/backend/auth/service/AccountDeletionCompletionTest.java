package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.time.Clock;
import java.time.Instant;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionState;

class AccountDeletionCompletionTest {
  private final UUID operationId = UUID.randomUUID();
  private final UUID accountId = UUID.randomUUID();

  @Test
  void requestObservesCompetingEventWorkersDurableCompletion() {
    var pending = operation(AccountDeletionState.PENDING_EVENT, true);
    var completed = operation(AccountDeletionState.COMPLETED, false);
    assertThat(AccountDeletionCompletion.awaitConcurrentWorker(pending, () -> completed, Duration.ofSeconds(1), Clock.systemUTC()))
        .isEqualTo(completed);
  }

  @Test
  void floorWaitStopsAtNextStageWithoutClaimingItsWork() {
    var pending = operation(AccountDeletionState.PENDING_FLOOR, true);
    var next = operation(AccountDeletionState.PENDING_EVENT, false);
    assertThat(AccountDeletionCompletion.awaitConcurrentWorker(pending, () -> next, Duration.ofSeconds(1), Clock.systemUTC()))
        .isEqualTo(next);
  }

  @Test
  void timeoutAndReleasedFailureNeverImplyCompletion() {
    var pending = operation(AccountDeletionState.PENDING_EVENT, true);
    AtomicInteger reads = new AtomicInteger();
    assertThat(AccountDeletionCompletion.awaitConcurrentWorker(pending, () -> {
      reads.incrementAndGet();
      return operation(AccountDeletionState.COMPLETED, false);
    }, Duration.ZERO, Clock.systemUTC())).isEqualTo(pending);
    assertThat(reads).hasValue(0);
    var failed = operation(AccountDeletionState.PENDING_EVENT, false);
    assertThat(AccountDeletionCompletion.awaitConcurrentWorker(failed, () -> {
      reads.incrementAndGet();
      return pending;
    }, Duration.ofSeconds(1), Clock.systemUTC())).isEqualTo(failed);
    assertThat(reads).hasValue(0);
  }

  @Test
  void interruptionPreservesCancellationAndPendingState() {
    var pending = operation(AccountDeletionState.PENDING_EVENT, true);
    Thread.currentThread().interrupt();
    try {
      assertThat(AccountDeletionCompletion.awaitConcurrentWorker(pending, () -> {
        throw new AssertionError("cancelled request must not poll");
      }, Duration.ofSeconds(1), Clock.systemUTC())).isEqualTo(pending);
      assertThat(Thread.currentThread().isInterrupted()).isTrue();
    } finally {
      Thread.interrupted();
    }
  }

  @Test
  void expiredLeaseDoesNotDelayTheRequest() {
    var pending = operation(AccountDeletionState.PENDING_EVENT, true);
    assertThat(AccountDeletionCompletion.awaitConcurrentWorker(pending, () -> {
      throw new AssertionError("expired lease must not poll");
    }, Duration.ofSeconds(1), Clock.fixed(pending.lockedUntil(), java.time.ZoneOffset.UTC)))
        .isEqualTo(pending);
  }

  private AccountDeletionOperation operation(AccountDeletionState state, boolean leased) {
    Instant now = Instant.now();
    return new AccountDeletionOperation(operationId, accountId, 2, "hash", state, 0, now,
        leased ? now.plusSeconds(30) : null, null, now,
        state == AccountDeletionState.COMPLETED ? now : null, now, now);
  }
}
