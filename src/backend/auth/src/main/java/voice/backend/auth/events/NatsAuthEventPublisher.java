package voice.backend.auth.events;

import io.nats.client.Connection;
import io.nats.client.JetStream;
import io.nats.client.Nats;
import io.nats.client.Options;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import voice.backend.auth.service.GuestConversionEventPublisher;
import voice.backend.auth.service.GuestConversionPublishAck;
import voice.backend.auth.service.JetStreamGuestConversionEventPublisher;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

/** Publishes Auth events to NATS JetStream subject user.guest_converted. */
public class NatsAuthEventPublisher implements AuthEventPublisher, GuestConversionEventPublisher {
  private static final Logger log = LoggerFactory.getLogger(NatsAuthEventPublisher.class);

  private final Connection connection;

  public NatsAuthEventPublisher(String natsUrl) {
    try {
      this.connection =
          Nats.connect(
              new Options.Builder()
                  .server(natsUrl)
                  .connectionName("voice-auth-events")
                  .maxReconnects(-1)
                  .reconnectWait(Duration.ofSeconds(1))
                  .build());
    } catch (Exception ex) {
      throw new IllegalStateException("connect nats", ex);
    }
  }

  @Override
  public void publishGuestConverted(UUID accountId) {
    if (accountId == null) {
      return;
    }
    publish(SUBJECT_GUEST_CONVERTED, accountId);
  }

  @Override
  public void publishAccountDeleted(UUID accountId) {
    if (accountId == null) {
      return;
    }
    publish(SUBJECT_ACCOUNT_DELETED, accountId);
  }

  @Override
  public void publishAccountRestored(UUID accountId) {
    if (accountId == null) {
      return;
    }
    publish(SUBJECT_ACCOUNT_RESTORED, accountId);
  }

  @Override
  public GuestConversionPublishAck publishGuestConverted(
      String subject, UserStreamEvent envelope, String natsMessageId) {
    try {
      JetStream jetStream = connection.jetStream();
      return new JetStreamGuestConversionEventPublisher(jetStream)
          .publishGuestConverted(subject, envelope, natsMessageId);
    } catch (Exception failure) {
      throw new IllegalStateException("create JetStream publisher", failure);
    }
  }

  private void publish(String subject, UUID accountId) {
    String payload = "{\"account_id\":\"" + accountId + "\"}";
    try {
      connection.publish(subject, payload.getBytes(StandardCharsets.UTF_8));
    } catch (Exception ex) {
      log.warn("publish {} failed: {}", subject, ex.getMessage());
    }
  }
}
