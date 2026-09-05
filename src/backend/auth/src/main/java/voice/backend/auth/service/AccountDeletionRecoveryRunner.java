package voice.backend.auth.service;

import java.time.Duration;
import org.springframework.scheduling.annotation.Scheduled;

/** Runs both fenced deletion stages after crashes or transient Redis/JetStream failures. */
public final class AccountDeletionRecoveryRunner {
  private static final int BATCH_SIZE = 25;
  private static final Duration LEASE_DURATION = Duration.ofSeconds(30);
  private final AccountDeletionPendingFloorWorker floorWorker;
  private final AccountDeletionPendingEventWorker eventWorker;

  public AccountDeletionRecoveryRunner(
      AccountDeletionPendingFloorWorker floorWorker, AccountDeletionPendingEventWorker eventWorker) {
    this.floorWorker = floorWorker;
    this.eventWorker = eventWorker;
  }

  @Scheduled(fixedDelayString = "${auth.account-deletion.recovery.interval:PT5S}")
  public void tick() {
    floorWorker.recover(BATCH_SIZE, LEASE_DURATION);
    eventWorker.recover(BATCH_SIZE, LEASE_DURATION);
  }
}
