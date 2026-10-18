package voice.backend.auth.events;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import io.nats.client.Nats;
import io.nats.client.api.StreamConfiguration;
import java.time.Clock;
import java.time.Duration;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

@Testcontainers(disabledWithoutDocker = true)
class AuthObservationJetStreamIntegrationTest {
  @Container
  static final GenericContainer<?> NATS = new GenericContainer<>(DockerImageName.parse("nats:2.11-alpine"))
      .withCommand("-js").withExposedPorts(4222);

  @Test
  void productionTransportReceivesCorrectPubAckAndDeliversBinaryObservation() throws Exception {
    String url = "nats://" + NATS.getHost() + ":" + NATS.getMappedPort(4222);
    try (var consumer = Nats.connect(url);
        var publisher = new NatsAuthEventPublisher(url, "")) {
      consumer.jetStreamManagement().addStream(StreamConfiguration.builder()
          .name("user_events").subjects("user.*").build());
      var subscription = consumer.jetStream().subscribe("user.logged_in");
      var meters = new SimpleMeterRegistry();
      try (var dispatcher = new AuthObservationDispatcher(publisher::publishObservation, Clock.systemUTC(), meters)) {
        UUID id = UUID.randomUUID();
        dispatcher.loggedIn(id, "{\"ip\":\"spoofed\"}");
        var message = subscription.nextMessage(Duration.ofSeconds(5));
        assertThat(message).isNotNull();
        var event = UserStreamEvent.parseFrom(message.getData());
        assertThat(event.getUserLoggedIn().getAccountId()).isEqualTo(id.toString());
        assertThat(event.getUserLoggedIn().getIp()).isEmpty();
        assertThat(message.getHeaders().getFirst("Nats-Msg-Id")).isEqualTo(event.getEventId());
        message.ack();
        long deadline = System.nanoTime() + Duration.ofSeconds(2).toNanos();
        while (dispatcher.pending() != 0 && System.nanoTime() < deadline) Thread.sleep(5);
        assertThat(dispatcher.pending()).isZero();
        assertThat(meters.counter("auth.observations.published").count()).isEqualTo(1);
      }
    }
  }
}
