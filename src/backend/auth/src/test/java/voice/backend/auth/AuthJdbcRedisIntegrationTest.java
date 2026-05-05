package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.hamcrest.Matchers.is;
import static org.hamcrest.Matchers.not;
import static org.hamcrest.Matchers.blankOrNullString;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Map;
import java.util.UUID;
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
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.security.JwtService;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
class AuthJdbcRedisIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @Container
  static final GenericContainer<?> redis =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  @Container
  static final PostgreSQLContainer<?> userPostgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("user_db")
          .withUsername("voice")
          .withPassword("voice")
          .withInitScript("integration-user-schema.sql");

  @DynamicPropertySource
  static void registerProps(DynamicPropertyRegistry registry) {
    registry.add("spring.datasource.url", postgres::getJdbcUrl);
    registry.add("spring.datasource.username", postgres::getUsername);
    registry.add("spring.datasource.password", postgres::getPassword);
    registry.add("auth.user-db.jdbc-url", userPostgres::getJdbcUrl);
    registry.add("auth.user-db.username", userPostgres::getUsername);
    registry.add("auth.user-db.password", userPostgres::getPassword);
    registry.add("spring.data.redis.host", redis::getHost);
    registry.add("spring.data.redis.port", () -> String.valueOf(redis.getMappedPort(6379)));
  }

  @Autowired MockMvc mockMvc;
  @Autowired ObjectMapper objectMapper;
  @Autowired JwtService jwtService;
  @Autowired @Qualifier("userJdbc") NamedParameterJdbcTemplate userJdbc;

  @Test
  void registerLoginRefreshValidateLogoutAndJwksWorkWithPostgresRedisAndStableJwks() throws Exception {
    JsonNode registered = postJson("/api/v1/auth/register",
        "{\"email\":\"jdbc@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}");

    String access = registered.get("access_token").asText();
    String refresh = registered.get("refresh_token").asText();
    assertThat(access).contains(".");
    assertThat(refresh).isNotBlank().doesNotContain(".");

    String accountIdStr = registered.get("account_id").asText();
    UUID accountId = UUID.fromString(accountIdStr);
    String profileId = registered.get("profile_id").asText();
    assertThat(profileId).matches(
        "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}");

    String dbProfileId =
        userJdbc.queryForObject(
            "SELECT id::text FROM profiles WHERE account_id = :accountId AND is_primary = true LIMIT 1",
            Map.of("accountId", accountId),
            String.class);
    assertThat(dbProfileId).isEqualTo(profileId);

    var accessClaims = jwtService.validate(access);
    assertThat(accessClaims.userId()).isEqualTo(accountIdStr);
    assertThat(accessClaims.profileId()).isEqualTo(profileId);

    mockMvc.perform(post("/api/v1/auth/validate")
            .header("Authorization", "Bearer " + access))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.user_id", is(accountIdStr)))
        .andExpect(jsonPath("$.profile_id", is(profileId)))
        .andExpect(jsonPath("$.jti", not(blankOrNullString())));

    JsonNode login =
        postJson(
            "/api/v1/auth/login",
            "{\"email\":\"jdbc@example.com\",\"password\":\"Correct horse battery staple\",\"device_info_json\":\"{}\"}");
    assertThat(login.get("profile_id").asText()).isEqualTo(profileId);
    assertThat(jwtService.validate(login.get("access_token").asText()).profileId()).isEqualTo(profileId);

    JsonNode rotated = postJson("/api/v1/auth/refresh",
        "{\"refresh_token\":\"" + refresh + "\",\"device_info_json\":\"{}\"}");
    assertThat(rotated.get("profile_id").asText()).isEqualTo(profileId);
    assertThat(jwtService.validate(rotated.get("access_token").asText()).profileId()).isEqualTo(profileId);

    mockMvc.perform(post("/api/v1/auth/logout")
            .header("Authorization", "Bearer " + rotated.get("access_token").asText())
            .contentType("application/json")
            .content("{\"refresh_token\":\"" + rotated.get("refresh_token").asText() + "\"}"))
        .andExpect(status().isNoContent());

    mockMvc.perform(post("/api/v1/auth/validate")
            .header("Authorization", "Bearer " + rotated.get("access_token").asText()))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("token_revoked")));

    mockMvc.perform(get("/api/v1/auth/.well-known/jwks.json"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.keys[0].kid", is("test-key")));
  }

  private JsonNode postJson(String path, String body) throws Exception {
    String response = mockMvc.perform(post(path).contentType("application/json").content(body))
        .andExpect(status().isOk())
        .andReturn()
        .getResponse()
        .getContentAsString();
    return objectMapper.readTree(response);
  }
}
