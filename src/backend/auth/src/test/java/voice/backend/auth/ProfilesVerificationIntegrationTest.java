package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import app.voice.user.v1.ClearVerificationRequest;
import app.voice.user.v1.ClearVerificationResponse;
import app.voice.user.v1.ApplyVerificationSourceStateRequest;
import app.voice.user.v1.ApplyVerificationSourceStateResponse;
import app.voice.user.v1.EnsurePrimaryProfileRequest;
import app.voice.user.v1.EnsurePrimaryProfileResponse;
import app.voice.user.v1.Profile;
import app.voice.user.v1.SetVerificationRequest;
import app.voice.user.v1.SetVerificationResponse;
import app.voice.user.v1.SwitchProfileRequest;
import app.voice.user.v1.SwitchProfileResponse;
import app.voice.user.v1.UserServiceGrpc;
import app.voice.user.v1.VerificationStatus;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.nimbusds.jwt.SignedJWT;
import com.sun.net.httpserver.HttpServer;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
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
  static final TestUserGrpcService userGrpc = new TestUserGrpcService();
  static final Server userGrpcServer = startUserGrpcServer();

  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

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
    registry.add("auth.user-grpc.addr", () -> "localhost:" + userGrpcServer.getPort());
    registry.add("spring.data.redis.host", redis::getHost);
    registry.add("spring.data.redis.port", () -> String.valueOf(redis.getMappedPort(6379)));
  }

  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired NamedParameterJdbcTemplate jdbc;
  @Autowired LinkedAccountsService linkedAccountsService;
  @Autowired VerificationStatusRefresh verificationStatusRefresh;

  @AfterAll
  static void stopUserGrpcServer() {
    userGrpcServer.shutdownNow();
  }

  @Test
  void switchActiveProfileIssuesJwtWithNewProfileIdAndRejectsForeignOrFrozen() throws Exception {
    JsonNode registered = registerSession("switch-profile@example.com");
    String accountId = registered.get("account_id").asText();
    String access = registered.get("access_token").asText();

    UUID altProfileId = UUID.randomUUID();
    userGrpc.addSwitchableProfile(altProfileId, UUID.fromString(accountId));

    UUID foreignProfileId = UUID.randomUUID();
    userGrpc.rejectSwitch(foreignProfileId, Status.PERMISSION_DENIED);

    UUID frozenProfileId = UUID.randomUUID();
    userGrpc.rejectSwitch(frozenProfileId, Status.FAILED_PRECONDITION);

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

      assertThat(userGrpc.verificationType(profileId)).isEqualTo("personal");
      assertThat(userGrpc.setVerificationRequests())
          .anySatisfy(
              request -> {
                assertThat(request.getProfileId()).isEqualTo(profileId);
                assertThat(request.getVerificationType()).isEqualTo("personal");
                assertThat(request.getBadge()).isEqualTo("twitch");
              });

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

      assertThat(userGrpc.verificationType(profileId)).isEqualTo("none");
      assertThat(userGrpc.clearVerificationRequests())
          .anySatisfy(request -> assertThat(request.getProfileId()).isEqualTo(profileId));
      assertThat(helixCalls.get()).isGreaterThanOrEqualTo(2);
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void verificationStatusRefreshKeepsBadgeWhenProviderIsUnavailable() throws Exception {
    AtomicInteger helixCalls = new AtomicInteger();
    HttpServer mockTwitch = HttpServer.create(new InetSocketAddress(0), 0);
    mockTwitch.createContext(
        "/helix/users",
        exchange -> {
          int call = helixCalls.incrementAndGet();
          if (call == 2) {
            exchange.sendResponseHeaders(503, -1);
            exchange.close();
            return;
          }
          byte[] body =
              "{\"data\":[{\"id\":\"tw-unavailable\",\"login\":\"partner\",\"broadcaster_type\":\"partner\"}]}"
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
      JsonNode registered = registerSession("twitch-unavailable@example.com");
      String accountId = registered.get("account_id").asText();
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
      verificationStatusRefresh.refresh();

      assertThat(userGrpc.verificationType(profileId)).isEqualTo("personal");
      assertThat(helixCalls.get()).isGreaterThanOrEqualTo(3);
      Integer active =
          jdbc.queryForObject(
              """
              SELECT COUNT(*) FROM linked_identities
              WHERE account_id = :accountId::uuid AND platform = 'twitch' AND status = 'active'
              """,
              Map.of("accountId", accountId),
              Integer.class);
      assertThat(active).isEqualTo(1);
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void staleNegativeRefreshCannotRevokeARelinkedIdentity() throws Exception {
    AtomicInteger helixCalls = new AtomicInteger();
    AtomicReference<UUID> accountId = new AtomicReference<>();
    AtomicReference<UUID> profileId = new AtomicReference<>();
    HttpServer mockTwitch = HttpServer.create(new InetSocketAddress(0), 0);
    mockTwitch.setExecutor(Executors.newCachedThreadPool());
    mockTwitch.createContext(
        "/helix/users",
        exchange -> {
          int call = helixCalls.incrementAndGet();
          if (call == 2) {
            // The refresh has already read its snapshot. Re-link before returning
            // an authoritative denial for that stale snapshot.
            linkedAccountsService.completeTwitchCallback(
                accountId.get(), profileId.get(), "mock-code", "http://127.0.0.1/cb");
            byte[] denied = "{\"data\":[]}".getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().add("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, denied.length);
            try (OutputStream os = exchange.getResponseBody()) {
              os.write(denied);
            }
            return;
          }
          byte[] partner =
              ("{\"data\":[{\"id\":\"tw-relinked-"
                      + call
                      + "\",\"login\":\"partner\",\"broadcaster_type\":\"partner\"}]}")
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, partner.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(partner);
          }
        });
    mockTwitch.start();
    linkedAccountsService.setTwitchEndpointsForTests(
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort(),
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort() + "/oauth2/token");
    try {
      JsonNode registered = registerSession("twitch-stale-refresh@example.com");
      accountId.set(UUID.fromString(registered.get("account_id").asText()));
      profileId.set(UUID.fromString(registered.get("profile_id").asText()));
      String access = registered.get("access_token").asText();
      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + access)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isOk());

      assertThat(linkedAccountsService.refreshVerificationStatuses()).isZero();

      Integer active =
          jdbc.queryForObject(
              """
              SELECT COUNT(*) FROM linked_identities
              WHERE account_id = :accountId::uuid AND platform = 'twitch' AND status = 'active'
              """,
              Map.of("accountId", accountId.get().toString()),
              Integer.class);
      assertThat(active).isEqualTo(1);
      assertThat(userGrpc.verificationType(profileId.get().toString())).isEqualTo("personal");
      assertThat(helixCalls.get()).isGreaterThanOrEqualTo(3);
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void unlinkRetainsAnotherVerifyingProviderOnTheSameProfile() throws Exception {
    JsonNode registered = registerSession("unlink-two-providers@example.com");
    String profileId = registered.get("profile_id").asText();
    String accountId = registered.get("account_id").asText();
    String access = registered.get("access_token").asText();
    userGrpc.seedVerification(profileId, "personal", "twitch");
    userGrpc.seedSource(profileId, "youtube", 1, true);
    jdbc.update(
        """
        INSERT INTO linked_identities (account_id, profile_id, platform, external_id, status)
        VALUES (:accountId::uuid, :profileId::uuid, 'twitch', 'tw-keep', 'active'),
               (:accountId::uuid, :profileId::uuid, 'youtube', 'yt-keep', 'active')
        """,
        Map.of("accountId", accountId, "profileId", profileId));

    mockMvc
        .perform(
            post("/api/v1/auth/linked-accounts/twitch/unlink")
                .header("Authorization", "Bearer " + access))
        .andExpect(status().isNoContent());

    assertThat(userGrpc.verificationType(profileId)).isEqualTo("personal");
    assertThat(userGrpc.applyVerificationSourceStateRequests())
        .anySatisfy(
            request -> {
              assertThat(request.getProfileId()).isEqualTo(profileId);
              assertThat(request.getSource()).isEqualTo("twitch");
              assertThat(request.getVerified()).isFalse();
            });
  }

  @Test
  void unlinkAfterProfileSwitchReconcilesTheProfileThatOwnedTheRevokedLink() throws Exception {
    JsonNode registered = registerSession("unlink-profile-switch@example.com");
    String linkedProfileId = registered.get("profile_id").asText();
    String accountId = registered.get("account_id").asText();
    String originalAccess = registered.get("access_token").asText();
    UUID currentProfileId = UUID.randomUUID();
    userGrpc.addSwitchableProfile(currentProfileId, UUID.fromString(accountId));
    MvcResult switched =
        mockMvc
            .perform(
                post("/api/v1/auth/switch-profile")
                    .header("Authorization", "Bearer " + originalAccess)
                    .contentType("application/json")
                    .content("{\"profile_id\":\"" + currentProfileId + "\"}"))
            .andExpect(status().isOk())
            .andReturn();
    String currentAccess =
        objectMapper.readTree(switched.getResponse().getContentAsString()).get("access_token").asText();
    userGrpc.seedVerification(linkedProfileId, "personal", "twitch");
    userGrpc.seedVerification(currentProfileId.toString(), "personal", "youtube");
    jdbc.update(
        """
        INSERT INTO linked_identities (account_id, profile_id, platform, external_id, status)
        VALUES (:accountId::uuid, :linkedProfileId::uuid, 'twitch', 'tw-profile-a', 'active')
        """,
        Map.of("accountId", accountId, "linkedProfileId", linkedProfileId));

    mockMvc
        .perform(
            post("/api/v1/auth/linked-accounts/twitch/unlink")
                .header("Authorization", "Bearer " + currentAccess))
        .andExpect(status().isNoContent());

    assertThat(userGrpc.verificationType(linkedProfileId)).isEqualTo("none");
    assertThat(userGrpc.verificationType(currentProfileId.toString())).isEqualTo("personal");
  }

  @Test
  void activeProviderCannotMoveProfilesAndUnlinkThenRelinkConvergesBothProfiles() throws Exception {
    HttpServer mockTwitch = partnerTwitchServer("tw-profile-boundary");
    mockTwitch.start();
    linkedAccountsService.setTwitchEndpointsForTests(
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort(),
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort() + "/oauth2/token");
    try {
      JsonNode registered = registerSession("provider-profile-boundary@example.com");
      UUID accountId = UUID.fromString(registered.get("account_id").asText());
      String profileA = registered.get("profile_id").asText();
      String accessA = registered.get("access_token").asText();

      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + accessA)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isOk());

      UUID profileB = UUID.randomUUID();
      userGrpc.addSwitchableProfile(profileB, accountId);
      MvcResult switched =
          mockMvc
              .perform(
                  post("/api/v1/auth/switch-profile")
                      .header("Authorization", "Bearer " + accessA)
                      .contentType("application/json")
                      .content("{\"profile_id\":\"" + profileB + "\"}"))
              .andExpect(status().isOk())
              .andReturn();
      String accessB =
          objectMapper.readTree(switched.getResponse().getContentAsString()).get("access_token").asText();

      mockMvc
          .perform(get("/api/v1/auth/linked-accounts").header("Authorization", "Bearer " + accessB))
          .andExpect(status().isOk())
          .andExpect(jsonPath("$.linked_accounts[0].profile_id").value(profileA));

      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + accessB)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isConflict())
          .andExpect(jsonPath("$.error").value("linked_account_profile_conflict"));

      userGrpc.failNextClearVerification();
      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/unlink")
                  .header("Authorization", "Bearer " + accessB))
          .andExpect(status().isNoContent());
      assertThat(userGrpc.verificationType(profileA)).isEqualTo("personal");

      mockMvc
          .perform(
              post("/api/v1/auth/linked-accounts/twitch/callback")
                  .header("Authorization", "Bearer " + accessB)
                  .contentType("application/json")
                  .content("{\"code\":\"mock-code\",\"redirect_uri\":\"http://127.0.0.1/cb\"}"))
          .andExpect(status().isOk());

      assertThat(userGrpc.verificationType(profileA)).isEqualTo("none");
      assertThat(userGrpc.verificationType(profileB.toString())).isEqualTo("personal");
      mockMvc
          .perform(get("/api/v1/auth/linked-accounts").header("Authorization", "Bearer " + accessB))
          .andExpect(status().isOk())
          .andExpect(jsonPath("$.linked_accounts[0].profile_id").value(profileB.toString()));
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void linkSyncFailureIsRetriedFromDurableLinkedIdentity() throws Exception {
    HttpServer mockTwitch = partnerTwitchServer("tw-link-retry");
    mockTwitch.start();
    linkedAccountsService.setTwitchEndpointsForTests(
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort(),
        "http://127.0.0.1:" + mockTwitch.getAddress().getPort() + "/oauth2/token");
    try {
      JsonNode registered = registerSession("link-sync-retry@example.com");
      UUID accountId = UUID.fromString(registered.get("account_id").asText());
      UUID profileId = UUID.fromString(registered.get("profile_id").asText());
      userGrpc.failNextSetVerification();

      linkedAccountsService.completeTwitchCallback(
          accountId, profileId, "mock-code", "http://127.0.0.1/cb");

      assertThat(userGrpc.verificationType(profileId.toString())).isNull();
      verificationStatusRefresh.refresh();
      assertThat(userGrpc.verificationType(profileId.toString())).isEqualTo("personal");
    } finally {
      mockTwitch.stop(0);
    }
  }

  @Test
  void revokeSyncFailureIsRetriedFromDurableRevokedIdentity() throws Exception {
    JsonNode registered = registerSession("revoke-sync-retry@example.com");
    UUID accountId = UUID.fromString(registered.get("account_id").asText());
    UUID profileId = UUID.fromString(registered.get("profile_id").asText());
    userGrpc.seedVerification(profileId.toString(), "personal", "twitch");
    jdbc.update(
        """
        INSERT INTO linked_identities (account_id, profile_id, platform, external_id, status)
        VALUES (:accountId, :profileId, 'twitch', 'tw-revoke-retry', 'active')
        """,
        Map.of("accountId", accountId, "profileId", profileId));
    userGrpc.failNextClearVerification();

    linkedAccountsService.unlinkTwitch(accountId, profileId);

    assertThat(userGrpc.verificationType(profileId.toString())).isEqualTo("personal");
    verificationStatusRefresh.refresh();
    assertThat(userGrpc.verificationType(profileId.toString())).isEqualTo("none");
  }

  private static HttpServer partnerTwitchServer(String externalId) throws Exception {
    HttpServer server = HttpServer.create(new InetSocketAddress(0), 0);
    server.createContext(
        "/helix/users",
        exchange -> {
          byte[] body =
              ("{\"data\":[{\"id\":\""
                      + externalId
                      + "\",\"login\":\"partner\",\"broadcaster_type\":\"partner\"}]}")
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().add("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream os = exchange.getResponseBody()) {
            os.write(body);
          }
        });
    return server;
  }

  @Test
  void unlinkClearsVerificationViaUserService() throws Exception {
    JsonNode registered = registerSession("unlink@example.com");
    String profileId = registered.get("profile_id").asText();
    String accountId = registered.get("account_id").asText();
    String access = registered.get("access_token").asText();

    userGrpc.seedVerification(profileId, "personal", "twitch");
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

    assertThat(userGrpc.verificationType(profileId)).isEqualTo("none");
    assertThat(userGrpc.clearVerificationRequests())
        .anySatisfy(request -> assertThat(request.getProfileId()).isEqualTo(profileId));
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

  private static Server startUserGrpcServer() {
    try {
      return ServerBuilder.forPort(0).directExecutor().addService(userGrpc).build().start();
    } catch (Exception ex) {
      throw new ExceptionInInitializerError(ex);
    }
  }

  /** Stateful test double for the User-owned profile data exercised over production gRPC ports. */
  private static final class TestUserGrpcService extends UserServiceGrpc.UserServiceImplBase {
    private final Map<String, String> primaryProfiles = new ConcurrentHashMap<>();
    private final Map<String, Profile> switchableProfiles = new ConcurrentHashMap<>();
    private final Map<String, Status> switchFailures = new ConcurrentHashMap<>();
    private final Map<String, VerificationStatus> verificationStatuses = new ConcurrentHashMap<>();
    private final Map<String, SourceState> sourceStates = new ConcurrentHashMap<>();
    private final List<SetVerificationRequest> setVerificationRequests =
        java.util.Collections.synchronizedList(new ArrayList<>());
    private final List<ClearVerificationRequest> clearVerificationRequests =
        java.util.Collections.synchronizedList(new ArrayList<>());
    private final List<ApplyVerificationSourceStateRequest> applyVerificationSourceStateRequests =
        java.util.Collections.synchronizedList(new ArrayList<>());
    private final AtomicInteger failNextSetVerification = new AtomicInteger();
    private final AtomicInteger failNextClearVerification = new AtomicInteger();

    void failNextSetVerification() {
      failNextSetVerification.incrementAndGet();
    }

    void failNextClearVerification() {
      failNextClearVerification.incrementAndGet();
    }

    void addSwitchableProfile(UUID profileId, UUID accountId) {
      switchableProfiles.put(
          profileId.toString(),
          Profile.newBuilder()
              .setId(profileId.toString())
              .setAccountId(accountId.toString())
              .build());
    }

    void rejectSwitch(UUID profileId, Status status) {
      switchFailures.put(profileId.toString(), status);
    }

    void seedVerification(String profileId, String verificationType, String badge) {
      verificationStatuses.put(
          profileId,
          VerificationStatus.newBuilder()
              .setProfileId(profileId)
              .setVerificationType(verificationType)
              .setBadge(badge)
              .build());
      if ("personal".equals(verificationType)
          && ("twitch".equals(badge) || "youtube".equals(badge))) {
        sourceStates.put(profileId + "|" + badge, new SourceState(1, true, badge));
      }
    }

    void seedSource(String profileId, String source, long revision, boolean verified) {
      sourceStates.put(profileId + "|" + source, new SourceState(revision, verified, source));
    }

    String verificationType(String profileId) {
      VerificationStatus status = verificationStatuses.get(profileId);
      return status == null ? null : status.getVerificationType();
    }

    List<SetVerificationRequest> setVerificationRequests() {
      return List.copyOf(setVerificationRequests);
    }

    List<ClearVerificationRequest> clearVerificationRequests() {
      return List.copyOf(clearVerificationRequests);
    }

    List<ApplyVerificationSourceStateRequest> applyVerificationSourceStateRequests() {
      return List.copyOf(applyVerificationSourceStateRequests);
    }

    @Override
    public void ensurePrimaryProfile(
        EnsurePrimaryProfileRequest request,
        StreamObserver<EnsurePrimaryProfileResponse> observer) {
      String profileId =
          primaryProfiles.computeIfAbsent(
              request.getAccountId(), ignored -> UUID.randomUUID().toString());
      observer.onNext(
          EnsurePrimaryProfileResponse.newBuilder()
              .setProfile(
                  Profile.newBuilder()
                      .setId(profileId)
                      .setAccountId(request.getAccountId())
                      .setIsPrimary(true))
              .build());
      observer.onCompleted();
    }

    @Override
    public void switchProfile(
        SwitchProfileRequest request, StreamObserver<SwitchProfileResponse> observer) {
      Status failure = switchFailures.get(request.getProfileId());
      if (failure != null) {
        observer.onError(failure.asRuntimeException());
        return;
      }
      Profile profile = switchableProfiles.get(request.getProfileId());
      if (profile == null) {
        observer.onError(Status.NOT_FOUND.asRuntimeException());
        return;
      }
      observer.onNext(SwitchProfileResponse.newBuilder().setProfile(profile).build());
      observer.onCompleted();
    }

    @Override
    public void applyVerificationSourceState(
        ApplyVerificationSourceStateRequest request,
        StreamObserver<ApplyVerificationSourceStateResponse> observer) {
      applyVerificationSourceStateRequests.add(request);
      AtomicInteger failure = request.getVerified() ? failNextSetVerification : failNextClearVerification;
      if (failure.getAndUpdate(value -> Math.max(0, value - 1)) > 0) {
        observer.onError(Status.UNAVAILABLE.asRuntimeException());
        return;
      }
      if (request.getVerified()) {
        setVerificationRequests.add(
            SetVerificationRequest.newBuilder()
                .setProfileId(request.getProfileId())
                .setVerificationType("personal")
                .setBadge(request.getBadge())
                .build());
      } else {
        clearVerificationRequests.add(
            ClearVerificationRequest.newBuilder().setProfileId(request.getProfileId()).build());
      }
      String key = request.getProfileId() + "|" + request.getSource();
      SourceState current = sourceStates.get(key);
      boolean applied = current == null || request.getRevision() > current.revision();
      if (applied) {
        sourceStates.put(
            key,
            new SourceState(request.getRevision(), request.getVerified(), request.getBadge()));
      }
      SourceState twitch = sourceStates.get(request.getProfileId() + "|twitch");
      SourceState youtube = sourceStates.get(request.getProfileId() + "|youtube");
      String badge = twitch != null && twitch.verified()
          ? twitch.badge()
          : youtube != null && youtube.verified() ? youtube.badge() : "";
      VerificationStatus.Builder status =
          VerificationStatus.newBuilder()
              .setProfileId(request.getProfileId())
              .setVerificationType(badge.isEmpty() ? "none" : "personal");
      if (!badge.isEmpty()) {
        status.setBadge(badge);
      }
      VerificationStatus built = status.build();
      verificationStatuses.put(request.getProfileId(), built);
      observer.onNext(
          ApplyVerificationSourceStateResponse.newBuilder()
              .setVerificationStatus(built)
              .setApplied(applied)
              .build());
      observer.onCompleted();
    }

    @Override
    public void setVerification(
        SetVerificationRequest request, StreamObserver<SetVerificationResponse> observer) {
      setVerificationRequests.add(request);
      if (failNextSetVerification.getAndUpdate(value -> Math.max(0, value - 1)) > 0) {
        observer.onError(Status.UNAVAILABLE.asRuntimeException());
        return;
      }
      VerificationStatus status =
          VerificationStatus.newBuilder()
              .setProfileId(request.getProfileId())
              .setVerificationType(request.getVerificationType())
              .setBadge(request.getBadge())
              .build();
      verificationStatuses.put(request.getProfileId(), status);
      observer.onNext(SetVerificationResponse.newBuilder().setVerificationStatus(status).build());
      observer.onCompleted();
    }

    @Override
    public void clearVerification(
        ClearVerificationRequest request, StreamObserver<ClearVerificationResponse> observer) {
      clearVerificationRequests.add(request);
      if (failNextClearVerification.getAndUpdate(value -> Math.max(0, value - 1)) > 0) {
        observer.onError(Status.UNAVAILABLE.asRuntimeException());
        return;
      }
      VerificationStatus status =
          VerificationStatus.newBuilder()
              .setProfileId(request.getProfileId())
              .setVerificationType("none")
              .build();
      verificationStatuses.put(request.getProfileId(), status);
      observer.onNext(ClearVerificationResponse.newBuilder().setVerificationStatus(status).build());
      observer.onCompleted();
    }

    private record SourceState(long revision, boolean verified, String badge) {}
  }
}
