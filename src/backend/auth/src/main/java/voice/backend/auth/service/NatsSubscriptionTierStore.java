package voice.backend.auth.service;

import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.JetStream;
import io.nats.client.JetStreamManagement;
import io.nats.client.JetStreamSubscription;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.nats.client.PushSubscribeOptions;
import io.nats.client.api.AckPolicy;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.ConsumerInfo;
import io.nats.client.api.DeliverPolicy;
import java.time.Duration;
import java.util.UUID;
import java.util.function.Consumer;
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
  private static final String DELIVER = "_INBOX.voice.auth.subscription_tier";

  private final InMemorySubscriptionTierStore delegate = new InMemorySubscriptionTierStore();
  private final Connection connection;
  private final JetStreamSubscription subscription;
  private final Consumer<SubscriptionEventParser.TierUpdate> tierUpdater;

  public NatsSubscriptionTierStore(String natsUrl, String credentialsFile) {
    this(connect(natsUrl, credentialsFile));
  }

  private static Connection connect(String natsUrl, String credentialsFile) {
    try {
      Options.Builder options = new Options.Builder()
              .server(natsUrl)
              .connectionName("voice-auth-subscription-tier")
              .inboxPrefix("_INBOX.voice.auth.requests")
              .maxReconnects(-1)
              .reconnectWait(Duration.ofSeconds(1));
      if (credentialsFile != null && !credentialsFile.isBlank()) {
        options.credentialPath(credentialsFile);
      }
      return Nats.connect(options.build());
    } catch (Exception ex) {
      throw new IllegalStateException("connect subscription.events consumer", ex);
    }
  }

  NatsSubscriptionTierStore(Connection connection) {
    this(connection, null);
  }

  NatsSubscriptionTierStore(
      Connection connection, Consumer<SubscriptionEventParser.TierUpdate> tierUpdater) {
    this.connection = connection;
    this.tierUpdater =
        tierUpdater != null
            ? tierUpdater
            : update -> delegate.setTier(update.accountId(), update.tier());
    try {
      JetStreamManagement jsm = connection.jetStreamManagement();
      validateConsumer(jsm.getConsumerInfo(STREAM, DURABLE));
      JetStream js = connection.jetStream();
      Dispatcher dispatcher = connection.createDispatcher();
      PushSubscribeOptions opts =
          PushSubscribeOptions.builder()
              .stream(STREAM)
              .durable(DURABLE)
              .deliverSubject(DELIVER)
              .bind(true)
              .build();
      this.subscription = js.subscribe(SUBJECT, dispatcher, this::onMessage, false, opts);
      log.info("subscription.events tier consumer started on {}", SUBJECT);
    } catch (Exception ex) {
      try {
        connection.close();
      } catch (Exception closeEx) {
        ex.addSuppressed(closeEx);
      }
      throw new IllegalStateException("connect subscription.events consumer", ex);
    }
  }

  private static void validateConsumer(ConsumerInfo info) {
    if (info == null || !STREAM.equals(info.getStreamName())) {
      throw new IllegalStateException("subscription tier consumer stream mismatch");
    }
    if (!DURABLE.equals(info.getName())) {
      throw new IllegalStateException("subscription tier consumer durable mismatch");
    }
    ConsumerConfiguration config = info.getConsumerConfiguration();
    if (config == null || !DURABLE.equals(config.getDurable())) {
      throw new IllegalStateException("subscription tier consumer durable config mismatch");
    }
    if (!java.util.List.of(SUBJECT).equals(config.getFilterSubjects())) {
      throw new IllegalStateException("subscription tier consumer filter mismatch");
    }
    if (!DELIVER.equals(config.getDeliverSubject())) {
      throw new IllegalStateException("subscription tier consumer deliver subject mismatch");
    }
    if (config.getDeliverGroup() != null && !config.getDeliverGroup().isEmpty()) {
      throw new IllegalStateException("subscription tier consumer deliver group mismatch");
    }
    if (config.getAckPolicy() != AckPolicy.Explicit) {
      throw new IllegalStateException("subscription tier consumer ack policy mismatch");
    }
    if (config.getDeliverPolicy() != DeliverPolicy.New) {
      throw new IllegalStateException("subscription tier consumer deliver policy mismatch");
    }
  }

  void onMessage(io.nats.client.Message msg) {
    java.util.Optional<SubscriptionEventParser.TierUpdate> update;
    try {
      update = SubscriptionEventParser.parseTierUpdate(msg.getData());
    } catch (IllegalArgumentException ex) {
      try {
        msg.term();
      } catch (Exception termEx) {
        log.warn("term malformed subscription.events: {}", termEx.getMessage());
      }
      return;
    }
    try {
      update.ifPresent(
          tierUpdate -> {
            tierUpdater.accept(tierUpdate);
            log.debug(
                "subscription tier updated account={} tier={}",
                tierUpdate.accountId(),
                tierUpdate.tier());
          });
      msg.ack();
    } catch (Exception ex) {
      log.warn("process subscription.events: {}", ex.getMessage());
      try {
        msg.nak();
      } catch (Exception nakEx) {
        log.warn("nak subscription.events: {}", nakEx.getMessage());
      }
    }
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
