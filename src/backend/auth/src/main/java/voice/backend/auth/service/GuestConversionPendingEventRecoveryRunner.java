package voice.backend.auth.service;

import java.util.Objects;
import java.util.concurrent.atomic.AtomicBoolean;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import voice.backend.auth.config.GuestConversionPendingEventRecoveryProperties;

/** Serializes scheduled PENDING_EVENT recovery ticks in this Auth process. */
public final class GuestConversionPendingEventRecoveryRunner {
  private static final Logger log = LoggerFactory.getLogger(GuestConversionPendingEventRecoveryRunner.class);

  private final GuestConversionPendingEventWorker worker;
  private final GuestConversionPendingEventRecoveryProperties properties;
  private final AtomicBoolean running = new AtomicBoolean();

  public GuestConversionPendingEventRecoveryRunner(
      GuestConversionPendingEventWorker worker, GuestConversionPendingEventRecoveryProperties properties) {
    this.worker = Objects.requireNonNull(worker, "worker");
    this.properties = Objects.requireNonNull(properties, "properties");
  }

  @Scheduled(
      fixedDelayString = "${auth.guest-conversion.pending-event.interval:PT30S}",
      initialDelayString = "${auth.guest-conversion.pending-event.interval:PT30S}")
  public void tick() {
    if (!running.compareAndSet(false, true)) {
      log.warn("guest_conversion_recovery_overlap stage=PENDING_EVENT");
      return;
    }
    try {
      worker.processDue(properties.getBatchSize(), properties.getLeaseDuration());
    } catch (RuntimeException failure) {
      log.warn("guest_conversion_recovery_failed stage=PENDING_EVENT error={}", failure.getClass().getSimpleName(), failure);
      throw failure;
    } finally {
      running.set(false);
    }
  }
}
