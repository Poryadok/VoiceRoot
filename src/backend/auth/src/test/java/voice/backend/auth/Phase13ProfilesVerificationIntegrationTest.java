package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
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
import voice.backend.auth.service.LinkedAccountsService;

/**
 * Phase 13 red tests: active profile switch, OAuth link with mocked Twitch/YouTube, unlink clears verification.
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
class Phase13ProfilesVerificationIntegrationTest {
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
  @Autowired LinkedAccountsService linkedAccountsService;

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

    mockMvc
        .perform(
            post("/api/v1/auth/switch-profile")
                .header("Authorization", "Bearer " + access)
                .contentType("application/json")
                .content("{\"profile_id\":\"" + foreignProfileId + "\"}"))
        .andExpect(status().isForbidden());

    mockMvc
        .perform(
            post("/api/v1/auth/switch-profile")
                .header("Authorization", "Bearer " + access)
                .contentType("application/json")
                .content("{\"profile_id\":\"" + frozenProfileId + "\"}"))
        .andExpect(status().isPreconditionFailed());
  }

  @Test
  void oauthTwitchLinkFlowUsesMockPartnerCheck() throws Exception {
    AtomicReference<String> twitchUsersPath = new AtomicReference<>();
    HttpServer mockTwitch = HttpServer.create(new InetSocketAddress(0), 0);
    mockTwitch.createContext(
        "/helix/users",
        exchange -> {
          twitchUsersPath.set(exchange.getRequestURI().getPath());
          byte[] body =
              "{\"data\":[{\"login\":\"streamer\",\"broadcaster_type\":\"partner\"}]}"
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    mockTwitch.start();
    int port = mockTwitch.getAddress().getPort();
    linkedAccountsService.setTwitchApiBaseUrlForTests("http://127.0.0.1:" + port);
    try {
      JsonNode registered = registerSession("twitch-oauth@example.com");
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
          .andExpect(jsonPath("$.verification_type").value("personal"));

      assertThat(twitchUsersPath.get()).isEqualTo("/helix/users");
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void unlinkClearsVerificationViaUserService() throws Exception {
    JsonNode registered = registerSession("unlink@example.com");
    String profileId = registered.get("profile_id").asText();
    String access = registered.get("access_token").asText();

    userJdbc.update(
        """
        UPDATE profiles SET verification_type = 'personal', verification_badge = 'twitch'
        WHERE id = :profileId::uuid
        """,
        Map.of("profileId", profileId));

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
