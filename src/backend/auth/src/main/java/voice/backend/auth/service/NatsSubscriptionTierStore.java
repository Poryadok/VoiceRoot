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
import io.nats.client.impl.Headers;
import io.nats.client.impl.NatsJetStreamMetaData;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Duration;
import java.util.Base64;
import java.util.HexFormat;
import java.util.UUID;
import java.util.function.Function;
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
  private static final String QUARANTINE_SUBJECT = "subscription.auth_quarantined";
  private static final long MAX_DELIVER = 5;
  private static final java.util.List<Duration> RETRY_BACKOFF =
      java.util.List.of(
          Duration.ofSeconds(1),
          Duration.ofSeconds(5),
          Duration.ofSeconds(30),
          Duration.ofMinutes(2),
          Duration.ofMinutes(5));
  private static final Duration NAK_DELAY = Duration.ofSeconds(5);

  private final InMemorySubscriptionTierStore delegate = new InMemorySubscriptionTierStore();
  private final Connection connection;
  private final JetStream jetStream;
  private final JetStreamSubscription subscription;
  private final Consumer<SubscriptionEventParser.TierUpdate> tierUpdater;
  private final Function<io.nats.client.Message, DeliveryMetadata> deliveryMetadataProvider;

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
    this(connection, null, NatsSubscriptionTierStore::readDeliveryMetadata);
  }

  NatsSubscriptionTierStore(
      Connection connection, Consumer<SubscriptionEventParser.TierUpdate> tierUpdater) {
    this(connection, tierUpdater, NatsSubscriptionTierStore::readDeliveryMetadata);
  }

  NatsSubscriptionTierStore(
      Connection connection,
      Consumer<SubscriptionEventParser.TierUpdate> tierUpdater,
      Function<io.nats.client.Message, DeliveryMetadata> deliveryMetadataProvider) {
    this.connection = connection;
    this.deliveryMetadataProvider = deliveryMetadataProvider;
    this.tierUpdater =
        tierUpdater != null
            ? tierUpdater
            : update -> delegate.setTier(update.accountId(), update.tier());
    try {
      JetStreamManagement jsm = connection.jetStreamManagement();
      validateConsumer(jsm.getConsumerInfo(STREAM, DURABLE));
      this.jetStream = connection.jetStream();
      Dispatcher dispatcher = connection.createDispatcher();
      PushSubscribeOptions opts =
          PushSubscribeOptions.builder()
              .stream(STREAM)
              .durable(DURABLE)
              .deliverSubject(DELIVER)
              .bind(true)
              .build();
      this.subscription = jetStream.subscribe(SUBJECT, dispatcher, this::onMessage, false, opts);
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
    if (config.getMaxDeliver() != MAX_DELIVER || !RETRY_BACKOFF.equals(config.getBackoff())) {
      throw new IllegalStateException("subscription tier consumer retry policy mismatch");
    }
  }

  void onMessage(io.nats.client.Message msg) {
    try {
      java.util.Optional<SubscriptionEventParser.TierUpdate> update =
          SubscriptionEventParser.parseTierUpdate(msg.getData());
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
      handleFailedDelivery(msg, ex);
    }
  }

  private void handleFailedDelivery(io.nats.client.Message msg, Exception failure) {
    DeliveryMetadata metadata = deliveryMetadataProvider.apply(msg);
    long deliveryCount = metadata.deliveryCount();
    String reason =
        failure instanceof IllegalArgumentException
            ? "unsupported_subscription_payload"
            : "subscription_tier_update_failed";
    if (deliveryCount < MAX_DELIVER) {
      log.warn("retry subscription.events reason={} delivery={}", reason, deliveryCount);
      nakWithDelay(msg);
      return;
    }

    try {
      publishQuarantine(msg, metadata, deliveryCount, reason);
      msg.ack();
      log.error(
          "subscription.events quarantined after bounded retries stream={} sequence={} reason={}",
          metadata.stream(),
          metadata.streamSequence(),
          reason);
    } catch (Exception quarantineFailure) {
      log.error(
          "subscription.events quarantine publish failed; source retained for operator recovery stream={} sequence={}",
          metadata.stream(),
          metadata.streamSequence(),
          quarantineFailure);
      nakWithDelay(msg);
    }
  }

  private void publishQuarantine(
      io.nats.client.Message msg,
      DeliveryMetadata metadata,
      long deliveryCount,
      String reason)
      throws Exception {
    long sequence = metadata.streamSequence();
    if (sequence <= 0) {
      throw new IllegalStateException("subscription event has no source sequence");
    }
    byte[] payload = msg.getData();
    String digest = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(payload));
    String envelope =
        "{\"source_stream\":\"subscription_events\",\"source_sequence\":"
            + sequence
            + ",\"delivery_count\":"
            + deliveryCount
            + ",\"reason\":\""
            + reason
            + "\",\"sha256\":\""
            + digest
            + "\",\"payload_base64\":\""
            + Base64.getEncoder().encodeToString(payload)
            + "\"}";
    Headers headers =
        new Headers().add("Nats-Msg-Id", "auth-subscription-tier-quarantine-" + sequence);
    jetStream.publish(
        QUARANTINE_SUBJECT, headers, envelope.getBytes(StandardCharsets.UTF_8));
  }

  private static DeliveryMetadata readDeliveryMetadata(io.nats.client.Message msg) {
    NatsJetStreamMetaData metadata = msg.metaData();
    return metadata == null
        ? new DeliveryMetadata(0, 0, "unknown")
        : new DeliveryMetadata(
            metadata.deliveredCount(), metadata.streamSequence(), metadata.getStream());
  }

  record DeliveryMetadata(long deliveryCount, long streamSequence, String stream) {}

  private static void nakWithDelay(io.nats.client.Message msg) {
    try {
      msg.nakWithDelay(NAK_DELAY);
    } catch (Exception nakEx) {
      log.error("nak subscription.events failed; source retained for operator recovery", nakEx);
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
