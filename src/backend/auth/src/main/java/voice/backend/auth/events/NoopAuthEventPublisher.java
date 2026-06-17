package voice.backend.auth.events;

import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** No-op publisher when NATS is not configured. */
public class NoopAuthEventPublisher implements AuthEventPublisher {
  private static final Logger log = LoggerFactory.getLogger(NoopAuthEventPublisher.class);

  @Override
  public void publishGuestConverted(UUID accountId) {
    log.debug("guest converted event skipped (no NATS): account_id={}", accountId);
  }
}
