package voice.backend.auth.service;

import java.time.Duration;
import java.time.Clock;
import java.time.Instant;
import java.util.function.Supplier;
import java.util.concurrent.locks.LockSupport;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionState;

/** Observes a competing worker's durable result; never claims or completes its lease. */
final class AccountDeletionCompletion {
  private AccountDeletionCompletion() {}

  static AccountDeletionOperation awaitConcurrentWorker(
      AccountDeletionOperation observed,
      Supplier<AccountDeletionOperation> reload,
      Duration budget,
      Clock clock) {
    long started = System.nanoTime();
    long remaining = budget.toNanos();
    var operationId = observed.operationId();
    var pending = observed.state();
    while ((pending == AccountDeletionState.PENDING_FLOOR || pending == AccountDeletionState.PENDING_EVENT)
        && observed.state() == pending && observed.lockedUntil() != null
        && observed.lockedUntil().isAfter(Instant.now(clock))
        && remaining > 0 && !Thread.currentThread().isInterrupted()) {
      LockSupport.parkNanos(Math.min(remaining, Duration.ofMillis(10).toNanos()));
      if (Thread.currentThread().isInterrupted()) {
        break;
      }
      observed = reload.get();
      if (!operationId.equals(observed.operationId())) {
        throw new IllegalStateException("account deletion operation changed while awaiting completion");
      }
      remaining = budget.toNanos() - (System.nanoTime() - started);
    }
    return observed;
  }
}
