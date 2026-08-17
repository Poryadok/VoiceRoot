package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import com.sun.net.httpserver.HttpServer;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicBoolean;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.repository.InMemoryLinkedIdentityRepository;
import voice.backend.auth.userdb.NoOpUserVerificationSync;

/** Compose stub IdP path: configured client + non-mock code hits token URL then Helix/YPP. */
class LinkedAccountsServiceStubIdpTest {

  @Test
  void twitchComposeCodeExchangesTokenThenGrantsPartnerBadge() throws Exception {
    AtomicBoolean tokenHit = new AtomicBoolean();
    HttpServer stub = HttpServer.create(new InetSocketAddress(0), 0);
    stub.createContext(
        "/oauth2/token",
        exchange -> {
          tokenHit.set(true);
          byte[] body = "{\"access_token\":\"stub-access\"}".getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    stub.createContext(
        "/helix/users",
        exchange -> {
          byte[] body =
              "{\"data\":[{\"id\":\"tw-compose\",\"login\":\"voicepartner\",\"broadcaster_type\":\"partner\"}]}"
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    stub.start();
    try {
      String base = "http://127.0.0.1:" + stub.getAddress().getPort();
      AuthProperties.OAuth oauth = new AuthProperties.OAuth();
      oauth.getTwitch().setClientId("voice-twitch-compose");
      oauth.getTwitch().setClientSecret("voice-twitch-compose-secret");
      LinkedAccountsService svc =
          new LinkedAccountsService(
              new NoOpUserVerificationSync(), new InMemoryLinkedIdentityRepository(), oauth);
      svc.setTwitchEndpointsForTests(base, base + "/oauth2/token");

      LinkedAccountsService.VerificationResult result =
          svc.completeTwitchCallback(
              UUID.randomUUID(), UUID.randomUUID(), "compose-code", "https://app.voice.test/oauth/twitch");

      assertThat(tokenHit.get()).isTrue();
      assertThat(result.verificationType()).isEqualTo("personal");
      assertThat(result.badge()).isEqualTo("twitch");
    } finally {
      stub.stop(0);
    }
  }

  @Test
  void youtubeComposeCodeExchangesTokenThenGrantsYppBadge() throws Exception {
    AtomicBoolean tokenHit = new AtomicBoolean();
    HttpServer stub = HttpServer.create(new InetSocketAddress(0), 0);
    stub.createContext(
        "/oauth2/token",
        exchange -> {
          tokenHit.set(true);
          byte[] body = "{\"access_token\":\"stub-access\"}".getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    stub.createContext(
        "/youtube/v3/channels",
        exchange -> {
          byte[] body =
              "{\"items\":[{\"id\":\"yt-compose\",\"snippet\":{\"title\":\"Voice YPP\"},\"status\":{\"longUploadsStatus\":\"allowed\"}}]}"
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    stub.start();
    try {
      String base = "http://127.0.0.1:" + stub.getAddress().getPort();
      AuthProperties.OAuth oauth = new AuthProperties.OAuth();
      oauth.getYoutube().setClientId("voice-youtube-compose");
      oauth.getYoutube().setClientSecret("voice-youtube-compose-secret");
      LinkedAccountsService svc =
          new LinkedAccountsService(
              new NoOpUserVerificationSync(), new InMemoryLinkedIdentityRepository(), oauth);
      svc.setYoutubeEndpointsForTests(base, base + "/oauth2/token");

      LinkedAccountsService.VerificationResult result =
          svc.completeYoutubeCallback(
              UUID.randomUUID(),
              UUID.randomUUID(),
              "compose-code",
              "https://app.voice.test/oauth/youtube");

      assertThat(tokenHit.get()).isTrue();
      assertThat(result.verificationType()).isEqualTo("personal");
      assertThat(result.badge()).isEqualTo("youtube");
    } finally {
      stub.stop(0);
    }
  }
}
