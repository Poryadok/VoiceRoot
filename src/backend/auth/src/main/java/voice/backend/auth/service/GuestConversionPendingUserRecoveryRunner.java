package voice.backend.auth.service;

import java.util.Objects;
import java.util.concurrent.atomic.AtomicBoolean;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import voice.backend.auth.config.GuestConversionPendingUserRecoveryProperties;

/** Serializes scheduled PENDING_USER recovery ticks in this Auth process. */
public final class GuestConversionPendingUserRecoveryRunner {
  private static final Logger log = LoggerFactory.getLogger(GuestConversionPendingUserRecoveryRunner.class);

  private final GuestConversionPendingUserWorker worker;
  private final GuestConversionPendingUserRecoveryProperties properties;
  private final AtomicBoolean running = new AtomicBoolean();

  public GuestConversionPendingUserRecoveryRunner(
      GuestConversionPendingUserWorker worker, GuestConversionPendingUserRecoveryProperties properties) {
    this.worker = Objects.requireNonNull(worker, "worker");
    this.properties = Objects.requireNonNull(properties, "properties");
  }

  @Scheduled(
      fixedDelayString = "${auth.guest-conversion.pending-user.interval:PT30S}",
      initialDelayString = "${auth.guest-conversion.pending-user.interval:PT30S}")
  public void tick() {
    if (!running.compareAndSet(false, true)) {
      log.warn("guest_conversion_recovery_overlap stage=PENDING_USER");
      return;
    }
    try {
      worker.processDue(properties.getBatchSize(), properties.getLeaseDuration());
    } catch (RuntimeException failure) {
      log.warn("guest_conversion_recovery_failed stage=PENDING_USER error={}", failure.getClass().getSimpleName(), failure);
      throw failure;
    } finally {
      running.set(false);
    }
  }
}
