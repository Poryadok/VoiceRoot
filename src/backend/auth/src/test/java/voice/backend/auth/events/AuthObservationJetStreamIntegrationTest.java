package voice.backend.auth.events;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import io.nats.client.Nats;
import io.nats.client.api.StreamConfiguration;
import java.time.Clock;
import java.time.Duration;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import org.testcontainers.images.builder.Transferable;
import voice.events.v1.JetstreamEvents.UserStreamEvent;

@Testcontainers(disabledWithoutDocker = true)
class AuthObservationJetStreamIntegrationTest {
  @Container
  static final GenericContainer<?> NATS = new GenericContainer<>(DockerImageName.parse("nats:2.11-alpine"))
      .withCopyToContainer(Transferable.of(serverConfig()), "/etc/nats/auth-observations.conf")
      .withCommand("-js", "-c", "/etc/nats/auth-observations.conf").withExposedPorts(4222);

  private static final List<String> SUBJECTS = List.of(
      "user.registered", "user.logged_in", "user.logged_out", "user.2fa_enabled");

  @SuppressWarnings("unchecked")
  private static Map<String, Object> yaml(String relative) throws Exception {
    Path root = Path.of("").toAbsolutePath().resolve("../../..").normalize();
    try (var input = Files.newInputStream(root.resolve(relative))) {
      return new org.yaml.snakeyaml.Yaml().load(input);
    }
  }

  @SuppressWarnings("unchecked")
  private static byte[] serverConfig() {
    try {
      var policy = yaml("deploy/nats/acl-intent.yaml");
      var auth = (Map<String, Object>) ((Map<String, Object>) policy.get("services")).get("auth");
      // A disposable admin provisions only this test's isolated server. The publisher receives
      // the actual canonical Auth allowlists, never the admin identity or wildcard event grant.
      var users = List.of(
          Map.of("user", "fixture-admin", "password", "fixture-admin"),
          Map.of("user", "auth", "password", "fixture-auth", "permissions", Map.of(
              "publish", Map.of("allow", auth.get("publish")),
              "subscribe", Map.of("allow", auth.get("subscribe")))));
      return new com.fasterxml.jackson.databind.ObjectMapper().writeValueAsBytes(
          Map.of("authorization", Map.of("users", users)));
    } catch (Exception failure) { throw new IllegalStateException("canonical Auth fixture configuration", failure); }
  }

  @SuppressWarnings("unchecked")
  private static String[] streamSubjects() throws Exception {
    var manifest = yaml("deploy/nats/jetstream-publisher-streams.yaml");
    var streams = (List<Map<String, Object>>) manifest.get("streams");
    var subjects = (List<String>) streams.stream().filter(row -> "user_events".equals(row.get("name")))
        .findFirst().orElseThrow().get("subjects");
    return subjects.toArray(String[]::new);
  }

  @Test
  void productionTransportReceivesCorrectPubAckAndDeliversBinaryObservation() throws Exception {
    String url = "nats://" + NATS.getHost() + ":" + NATS.getMappedPort(4222);
    try (var consumer = Nats.connect(new io.nats.client.Options.Builder().server(url)
            .userInfo("fixture-admin", "fixture-admin").build());
        var publisher = new NatsAuthEventPublisher(Nats.connect(new io.nats.client.Options.Builder().server(url)
            .userInfo("auth", "fixture-auth").inboxPrefix("_INBOX.voice.auth.requests").build()))) {
      consumer.jetStreamManagement().addStream(StreamConfiguration.builder()
          .name("user_events").subjects(streamSubjects()).build());
      var subscription = consumer.jetStream().subscribe("user.>");
      var meters = new SimpleMeterRegistry();
      try (var dispatcher = new AuthObservationDispatcher(publisher::publishObservation, Clock.systemUTC(), meters)) {
        UUID id = UUID.randomUUID();
        dispatcher.registered(id, "guest", "email");
        dispatcher.loggedIn(id, "{\"ip\":\"spoofed\"}");
        dispatcher.loggedOut(id, "{}");
        dispatcher.twoFaEnabled(id);
        var received = new java.util.HashSet<String>();
        for (int i = 0; i < 4; i++) {
          var message = subscription.nextMessage(Duration.ofSeconds(5));
          assertThat(message).isNotNull();
          var event = UserStreamEvent.parseFrom(message.getData());
          received.add(message.getSubject());
          if (event.hasUserLoggedIn()) {
            assertThat(event.getUserLoggedIn().getAccountId()).isEqualTo(id.toString());
            assertThat(event.getUserLoggedIn().getIp()).isEmpty();
          }
          assertThat(message.getHeaders().getFirst("Nats-Msg-Id")).isEqualTo(event.getEventId());
          message.ack();
        }
        assertThat(received).containsExactlyInAnyOrderElementsOf(SUBJECTS);
        long deadline = System.nanoTime() + Duration.ofSeconds(2).toNanos();
        while (dispatcher.pending() != 0 && System.nanoTime() < deadline) Thread.sleep(5);
        assertThat(dispatcher.pending()).isZero();
        assertThat(meters.counter("auth.observations.published").count()).isEqualTo(4);
      }
      org.assertj.core.api.Assertions.assertThatThrownBy(() -> publisher.publishObservation(
          "user.unreviewed", new byte[] {1}, UUID.randomUUID().toString())).isInstanceOf(Exception.class);
    }
  }
}
