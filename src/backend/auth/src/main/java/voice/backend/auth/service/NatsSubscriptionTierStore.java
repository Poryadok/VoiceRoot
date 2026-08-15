package voice.backend.auth.service;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.JetStream;
import io.nats.client.JetStreamApiException;
import io.nats.client.JetStreamManagement;
import io.nats.client.JetStreamSubscription;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.nats.client.PushSubscribeOptions;
import io.nats.client.api.RetentionPolicy;
import io.nats.client.api.StorageType;
import io.nats.client.api.StreamConfiguration;
import java.io.IOException;
import java.time.Duration;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * NATS-backed subscription tier cache for Auth JWT claims.
 *
 * <p>Consumes {@code subscription.events} and updates an in-memory delegate used by {@link
 * SubscriptionTierResolver}.
 */
public final class NatsSubscriptionTierStore implements SubscriptionTierResolver, AutoCloseable {
  private static final Logger log = LoggerFactory.getLogger(NatsSubscriptionTierStore.class);
  private static final String STREAM = "subscription_events";
  private static final String SUBJECT = "subscription.>";
  private static final String DURABLE = "auth_subscription_tier";

  private final InMemorySubscriptionTierStore delegate = new InMemorySubscriptionTierStore();
  private final Connection connection;
  private final JetStreamSubscription subscription;

  public NatsSubscriptionTierStore(String natsUrl) {
    try {
      this.connection =
          Nats.connect(
              new Options.Builder()
                  .server(natsUrl)
                  .connectionName("voice-auth-subscription-tier")
                  .maxReconnects(-1)
                  .reconnectWait(Duration.ofSeconds(1))
                  .build());
      ensureStream(connection);
      JetStream js = connection.jetStream();
      Dispatcher dispatcher = connection.createDispatcher();
      PushSubscribeOptions opts =
          PushSubscribeOptions.builder().stream(STREAM).durable(DURABLE).build();
      this.subscription = js.subscribe(SUBJECT, dispatcher, this::onMessage, false, opts);
      log.info("subscription.events tier consumer started on {}", SUBJECT);
    } catch (Exception ex) {
      throw new IllegalStateException("connect subscription.events consumer", ex);
    }
  }

  private static void ensureStream(Connection connection) throws IOException, JetStreamApiException {
    JetStreamManagement jsm = connection.jetStreamManagement();
    try {
      jsm.getStreamInfo(STREAM);
    } catch (JetStreamApiException ex) {
      if (!isStreamNotFound(ex)) {
        throw ex;
      }
      jsm.addStream(
          StreamConfiguration.builder()
              .name(STREAM)
              .subjects(streamSubjects())
              .retentionPolicy(RetentionPolicy.Limits)
              .maxAge(Duration.ofDays(7))
              .storageType(StorageType.File)
              .build());
    }
  }

  private static boolean isStreamNotFound(JetStreamApiException ex) {
    int code = ex.getApiErrorCode();
    return code == 404 || code == 10059;
  }

  /** Matches subscription service JetStream subjects for cross-service compatibility. */
  private static String[] streamSubjects() {
    return new String[] {
      "subscription.plan_started",
      "subscription.plan_cancelled",
      "subscription.plan_expired",
      "subscription.downgrade",
      "subscription.payment_success",
      "subscription.payment_failed",
      "subscription.space_pro_started",
      "subscription.space_pro_expired",
    };
  }

  void onMessage(io.nats.client.Message msg) {
    SubscriptionEventParser.parseTierUpdate(msg.getData())
        .ifPresent(
            update -> {
              delegate.setTier(update.accountId(), update.tier());
              log.debug(
                  "subscription tier updated account={} tier={}",
                  update.accountId(),
                  update.tier());
            });
  }

  @Override
  public String resolveTier(UUID accountId) {
    return delegate.resolveTier(accountId);
  }

  /** Test hook: seed tier without NATS. */
  void setTier(UUID accountId, String tier) {
    delegate.setTier(accountId, tier);
  }

  @Override
  public void close() {
    try {
      if (subscription != null) {
        subscription.unsubscribe();
      }
    } catch (Exception ex) {
      log.warn("unsubscribe subscription tier consumer: {}", ex.getMessage());
    }
    try {
      if (connection != null) {
        connection.close();
      }
    } catch (Exception ex) {
      log.warn("close nats connection: {}", ex.getMessage());
    }
  }
}
