package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.nimbusds.jwt.SignedJWT;
import com.sun.net.httpserver.HttpServer;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.lifecycle.VerificationStatusRefresh;
import voice.backend.auth.service.LinkedAccountsService;

/**
 * multi-profile/verification (docs/features/verification.md): Twitch/YouTube OAuth, linked_identities,
 * cron refresh.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
class ProfilesVerificationIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @Container
  static final PostgreSQLContainer<?> userPostgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("user_db")
          .withUsername("voice")
          .withPassword("voice")
          .withInitScript("integration-user-schema.sql");

  @Container
  static final GenericContainer<?> redis =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  @DynamicPropertySource
  static void registerProps(DynamicPropertyRegistry registry) {
    registry.add("voice.auth.jdbc.url", postgres::getJdbcUrl);
    registry.add("spring.datasource.username", postgres::getUsername);
    registry.add("spring.datasource.password", postgres::getPassword);
    registry.add("spring.flyway.user", postgres::getUsername);
    registry.add("spring.flyway.password", postgres::getPassword);
    registry.add("auth.user-db.jdbc-url", userPostgres::getJdbcUrl);
    registry.add("auth.user-db.username", userPostgres::getUsername);
    registry.add("auth.user-db.password", userPostgres::getPassword);
    registry.add("spring.data.redis.host", redis::getHost);
    registry.add("spring.data.redis.port", () -> String.valueOf(redis.getMappedPort(6379)));
  }

  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc;
  @Autowired NamedParameterJdbcTemplate jdbc;
  @Autowired LinkedAccountsService linkedAccountsService;
  @Autowired VerificationStatusRefresh verificationStatusRefresh;

  @Test
  void switchActiveProfileIssuesJwtWithNewProfileIdAndRejectsForeignOrFrozen() throws Exception {
    JsonNode registered = registerSession("switch-profile@example.com");
    String accountId = registered.get("account_id").asText();
    String access = registered.get("access_token").asText();

    UUID altProfileId = UUID.randomUUID();
    userJdbc.update(
        """
        INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
        VALUES (:id, :accountId, 'altwork', '0042', 'Work Alt', false)
        """,
        Map.of("id", altProfileId, "accountId", UUID.fromString(accountId)));

    UUID foreignProfileId = UUID.randomUUID();
    userJdbc.update(
        """
        INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
        VALUES (:id, :accountId, 'foreign', '0099', 'Foreign', true)
        """,
        Map.of("id", foreignProfileId, "accountId", UUID.randomUUID()));

    UUID frozenProfileId = UUID.randomUUID();
    userJdbc.update(
        """
        INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, frozen_at)
        VALUES (:id, :accountId, 'frozen', '0098', 'Frozen Alt', false, now())
        """,
        Map.of("id", frozenProfileId, "accountId", UUID.fromString(accountId)));

    MvcResult switched =
        mockMvc
            .perform(
                post("/api/v1/auth/switch-profile")
                    .header("Authorization", "Bearer " + access)
                    .contentType("application/json")
                    .content("{\"profile_id\":\"" + altProfileId + "\"}"))
            .andExpect(status().isOk())
            .andReturn();
    JsonNode body = objectMapper.readTree(switched.getResponse().getContentAsString());
    assertThat(body.get("profile_id").asText()).isEqualTo(altProfileId.toString());
    var jwt = SignedJWT.parse(body.get("access_token").asText()).getJWTClaimsSet();
    assertThat(jwt.getStringClaim("profile_id")).isEqualTo(altProfileId.toString());
    String switchedAccess = body.get("access_token").asText();

    mockMvc
        .perform(
            post("/api/v1/auth/switch-profile")
                .header("Authorization", "Bearer " + switchedAccess)
                .contentType("application/json")
                .content("{\"profile_id\":\"" + foreignProfileId + "\"}"))
        .andExpect(status().isForbidden());

    mockMvc
        .perform(
            post("/api/v1/auth/switch-profile")
                .header("Authorization", "Bearer " + switchedAccess)
                .contentType("application/json")
                .content("{\"profile_id\":\"" + frozenProfileId + "\"}"))
        .andExpect(status().isPreconditionFailed());
  }

  @Test
  void oauthTwitchPartnerPersistsLinkedIdentityAndListsIt() throws Exception {
    AtomicReference<String> twitchUsersPath = new AtomicReference<>();
    HttpServer mockTwitch = HttpServer.create(new InetSocketAddress(0), 0);
    mockTwitch.createContext(
        "/helix/users",
        exchange -> {
          twitchUsersPath.set(exchange.getRequestURI().getPath());
          byte[] body =
              "{\"data\":[{\"id\":\"tw123\",\"login\":\"streamer\",\"broadcaster_type\":\"partner\"}]}"
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    mockTwitch.start();
    int port = mockTwitch.getAddress().getPort();
    linkedAccountsService.setTwitchEndpointsForTests(
        "http://127.0.0.1:" + port, "http://127.0.0.1:" + port + "/oauth2/token");
    try {
      JsonNode registered = registerSession("twitch-oauth@example.com");
      String accountId = registered.get("account_id").asText();
      String profileId = registered.get("profile_id").asText();
      String access = registered.get("access_token").asText();

      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + access)
                  .contentType("application/json")
                  .content(
                      "{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1:"
                          + port
                          + "/callback\"}"))
          .andExpect(status().isOk())
          .andExpect(jsonPath("$.verification_type").value("personal"))
          .andExpect(jsonPath("$.badge").value("twitch"));

      assertThat(twitchUsersPath.get()).isEqualTo("/helix/users");

      Integer linked =
          jdbc.queryForObject(
              """
              SELECT COUNT(*) FROM linked_identities
              WHERE account_id = :accountId::uuid AND platform = 'twitch' AND status = 'active'
                AND external_id = 'tw123' AND profile_id = :profileId::uuid
              """,
              Map.of("accountId", accountId, "profileId", profileId),
              Integer.class);
      assertThat(linked).isEqualTo(1);

      String verificationType =
          userJdbc.queryForObject(
              "SELECT verification_type FROM profiles WHERE id = :profileId::uuid",
              Map.of("profileId", profileId),
              String.class);
      assertThat(verificationType).isEqualTo("personal");

      mockMvc
          .perform(get("/api/v1/auth/linked-accounts").header("Authorization", "Bearer " + access))
          .andExpect(status().isOk())
          .andExpect(jsonPath("$.linked_accounts[0].platform").value("twitch"))
          .andExpect(jsonPath("$.linked_accounts[0].external_id").value("tw123"));
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void oauthTwitchAffiliateIsDenied() throws Exception {
    HttpServer mockTwitch = HttpServer.create(new InetSocketAddress(0), 0);
    mockTwitch.createContext(
        "/helix/users",
        exchange -> {
          byte[] body =
              "{\"data\":[{\"id\":\"tw999\",\"login\":\"affiliate\",\"broadcaster_type\":\"affiliate\"}]}"
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    mockTwitch.start();
    linkedAccountsService.setTwitchEndpointsForTests(
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort(),
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort() + "/oauth2/token");
    try {
      JsonNode registered = registerSession("twitch-affiliate@example.com");
      String access = registered.get("access_token").asText();
      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + access)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isForbidden())
          .andExpect(jsonPath("$.error").value("verification_denied"));
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void oauthYoutubeYppPersistsLinkedIdentity() throws Exception {
    HttpServer mockYt = HttpServer.create(new InetSocketAddress(0), 0);
    mockYt.createContext(
        "/youtube/v3/channels",
        exchange -> {
          byte[] body =
              "{\"items\":[{\"id\":\"yt42\",\"snippet\":{\"title\":\"Pro Channel\"},\"status\":{\"longUploadsStatus\":\"allowed\"}}]}"
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    mockYt.start();
    int port = mockYt.getAddress().getPort();
    linkedAccountsService.setYoutubeEndpointsForTests(
        "http://127.0.0.1:" + port, "http://127.0.0.1:" + port + "/token");
    try {
      JsonNode registered = registerSession("youtube-oauth@example.com");
      String accountId = registered.get("account_id").asText();
      String access = registered.get("access_token").asText();

      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/youtube/callback")
                  .header("Authorization", "Bearer " + access)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isOk())
          .andExpect(jsonPath("$.badge").value("youtube"));

      Integer linked =
          jdbc.queryForObject(
              """
              SELECT COUNT(*) FROM linked_identities
              WHERE account_id = :accountId::uuid AND platform = 'youtube' AND status = 'active'
                AND external_id = 'yt42'
              """,
              Map.of("accountId", accountId),
              Integer.class);
      assertThat(linked).isEqualTo(1);
    } finally {
      mockYt.stop(0);
    }
  }

  @Test
  void verificationStatusRefreshClearsBadgeWhenPartnerLost() throws Exception {
    AtomicInteger helixCalls = new AtomicInteger();
    HttpServer mockTwitch = HttpServer.create(new InetSocketAddress(0), 0);
    mockTwitch.createContext(
        "/helix/users",
        exchange -> {
          helixCalls.incrementAndGet();
          // First call (link): partner; subsequent (cron): empty broadcaster_type
          String type = helixCalls.get() == 1 ? "partner" : "";
          byte[] body =
              ("{\"data\":[{\"id\":\"tw-refresh\",\"login\":\"was-partner\",\"broadcaster_type\":\""
                      + type
                      + "\"}]}")
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    mockTwitch.start();
    linkedAccountsService.setTwitchEndpointsForTests(
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort(),
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort() + "/oauth2/token");
    try {
      JsonNode registered = registerSession("twitch-refresh@example.com");
      String profileId = registered.get("profile_id").asText();
      String access = registered.get("access_token").asText();

      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + access)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isOk());

      verificationStatusRefresh.refresh();

      String verificationType =
          userJdbc.queryForObject(
              "SELECT verification_type FROM profiles WHERE id = :profileId::uuid",
              Map.of("profileId", profileId),
              String.class);
      assertThat(verificationType).isEqualTo("none");
      assertThat(helixCalls.get()).isGreaterThanOrEqualTo(2);
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void unlinkClearsVerificationViaUserService() throws Exception {
    JsonNode registered = registerSession("unlink@example.com");
    String profileId = registered.get("profile_id").asText();
    String accountId = registered.get("account_id").asText();
    String access = registered.get("access_token").asText();

    userJdbc.update(
        """
        UPDATE profiles SET verification_type = 'personal', verification_badge = 'twitch'
        WHERE id = :profileId::uuid
        """,
        Map.of("profileId", profileId));
    jdbc.update(
        """
        INSERT INTO linked_identities (account_id, profile_id, platform, external_id, status)
        VALUES (:accountId::uuid, :profileId::uuid, 'twitch', 'tw-unlink', 'active')
        """,
        Map.of("accountId", accountId, "profileId", profileId));

    mockMvc
        .perform(
            post("/api/v1/auth/linked-accounts/twitch/unlink")
                .header("Authorization", "Bearer " + access))
        .andExpect(status().isNoContent());

    String verificationType =
        userJdbc.queryForObject(
            "SELECT verification_type FROM profiles WHERE id = :profileId::uuid",
            Map.of("profileId", profileId),
            String.class);
    assertThat(verificationType).isEqualTo("none");
    String status =
        jdbc.queryForObject(
            """
            SELECT status FROM linked_identities
            WHERE account_id = :accountId::uuid AND platform = 'twitch'
            """,
            Map.of("accountId", accountId),
            String.class);
    assertThat(status).isEqualTo("revoked");
  }

  private JsonNode registerSession(String email) throws Exception {
    MvcResult result =
        mockMvc
            .perform(
                post("/api/v1/auth/register")
                    .contentType("application/json")
                    .content(
                        "{\"email\":\""
                            + email
                            + "\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}"))
            .andExpect(status().isOk())
            .andReturn();
    JsonNode root = objectMapper.readTree(result.getResponse().getContentAsString());
    return root.has("session") ? root.get("session") : root;
  }
}
