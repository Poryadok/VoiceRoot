package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.hamcrest.Matchers.is;
import static org.hamcrest.Matchers.not;
import static org.hamcrest.Matchers.blankOrNullString;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.ValidateTokenRequest;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.NoSuchAlgorithmException;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.Date;
import java.util.List;
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
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.security.JwtService;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
class AuthJdbcRedisIntegrationTest {
  private static final String JWT_ISSUER = "voice-auth";
  private static final String JWT_AUDIENCE = "voice-client";
  private static final String JWT_KID = "test-key";

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
  @Autowired AuthGrpcService grpcService;
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

    var registeredJwt = SignedJWT.parse(access).getJWTClaimsSet();
    assertThat(registeredJwt.getStringClaim("user_id")).isEqualTo(accountIdStr);
    assertThat(registeredJwt.getStringClaim("profile_id")).isEqualTo(profileId);

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
    var loginJwt = SignedJWT.parse(login.get("access_token").asText()).getJWTClaimsSet();
    assertThat(loginJwt.getStringClaim("user_id")).isEqualTo(accountIdStr);
    assertThat(loginJwt.getStringClaim("profile_id")).isEqualTo(profileId);

    JsonNode rotated = postJson("/api/v1/auth/refresh",
        "{\"refresh_token\":\"" + refresh + "\",\"device_info_json\":\"{}\"}");
    assertThat(rotated.get("profile_id").asText()).isEqualTo(profileId);
    assertThat(jwtService.validate(rotated.get("access_token").asText()).profileId()).isEqualTo(profileId);
    var refreshJwt = SignedJWT.parse(rotated.get("access_token").asText()).getJWTClaimsSet();
    assertThat(refreshJwt.getStringClaim("user_id")).isEqualTo(accountIdStr);
    assertThat(refreshJwt.getStringClaim("profile_id")).isEqualTo(profileId);

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

  /**
   * Phase 1 invariant (primary-profile-bootstrap): access JWT must include {@code profile_id}; validation rejects
   * otherwise-well-signed tokens without it (same checks as {@link JwtService#validate}).
   */
  @Test
  void validateRestRejectsWellSignedAccessJwtMissingProfileIdClaim() throws Exception {
    String pem = readClasspathPem();
    String forged =
        signJwt(
            pem,
            new JWTClaimsSet.Builder()
                .issuer(JWT_ISSUER)
                .audience(JWT_AUDIENCE)
                .subject(UUID.randomUUID().toString())
                .claim("user_id", UUID.randomUUID().toString())
                .claim("roles", List.of("user"))
                .claim("subscription_tier", "free")
                .jwtID(UUID.randomUUID().toString())
                .issueTime(Date.from(Instant.now()))
                .expirationTime(Date.from(Instant.now().plus(Duration.ofMinutes(15))))
                .build());

    mockMvc.perform(post("/api/v1/auth/validate").header("Authorization", "Bearer " + forged))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("invalid_token")));
  }

  @Test
  void validateGrpcRejectsWellSignedAccessJwtMissingProfileIdClaim() throws Exception {
    String pem = readClasspathPem();
    String forged =
        signJwt(
            pem,
            new JWTClaimsSet.Builder()
                .issuer(JWT_ISSUER)
                .audience(JWT_AUDIENCE)
                .subject(UUID.randomUUID().toString())
                .claim("user_id", UUID.randomUUID().toString())
                .claim("roles", List.of("user"))
                .claim("subscription_tier", "free")
                .jwtID(UUID.randomUUID().toString())
                .issueTime(Date.from(Instant.now()))
                .expirationTime(Date.from(Instant.now().plus(Duration.ofMinutes(15))))
                .build());

    String serverName = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(serverName).directExecutor().addService(grpcService).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
    var client = AuthServiceGrpc.newBlockingStub(channel);
    try {
      assertThatThrownBy(
              () ->
                  client.validateToken(
                      ValidateTokenRequest.newBuilder().setAccessToken(forged).build()))
          .isInstanceOf(StatusRuntimeException.class)
          .hasMessageContaining("invalid_token");
    } finally {
      channel.shutdownNow();
      server.shutdownNow();
    }
  }

  /** Cryptographically valid JWT with {@code user_id}/{@code profile_id} but no matching account in {@code auth_db}. */
  @Test
  void validateRestRejectsWellSignedJwtForUnknownAccount() throws Exception {
    String pem = readClasspathPem();
    String forged =
        signJwt(
            pem,
            new JWTClaimsSet.Builder()
                .issuer(JWT_ISSUER)
                .audience(JWT_AUDIENCE)
                .subject(UUID.randomUUID().toString())
                .claim("user_id", UUID.randomUUID().toString())
                .claim("profile_id", UUID.randomUUID().toString())
                .claim("roles", List.of("user"))
                .claim("subscription_tier", "free")
                .jwtID(UUID.randomUUID().toString())
                .issueTime(Date.from(Instant.now()))
                .expirationTime(Date.from(Instant.now().plus(Duration.ofMinutes(15))))
                .build());

    mockMvc.perform(post("/api/v1/auth/validate").header("Authorization", "Bearer " + forged))
        .andExpect(status().isUnauthorized())
        .andExpect(jsonPath("$.error", is("invalid_token")));
  }

  private static String readClasspathPem() throws Exception {
    try (var in = AuthJdbcRedisIntegrationTest.class.getClassLoader().getResourceAsStream("jwt-test-private.pem")) {
      assertThat(in).isNotNull();
      return new String(in.readAllBytes(), StandardCharsets.UTF_8);
    }
  }

  private static String signJwt(String pkcs8Pem, JWTClaimsSet claims) throws Exception {
    RSAKey rsaJwk = rsaKeyFromPkcs8Pem(pkcs8Pem);
    SignedJWT signedJwt =
        new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(JWT_KID).build(), claims);
    signedJwt.sign(new RSASSASigner(rsaJwk.toPrivateKey()));
    return signedJwt.serialize();
  }

  private static RSAKey rsaKeyFromPkcs8Pem(String pem) throws InvalidKeySpecException, NoSuchAlgorithmException {
    RSAPrivateCrtKey privateKey = parsePkcs8RsaPrivateCrtKey(pem);
    KeyFactory keyFactory = KeyFactory.getInstance("RSA");
    RSAPublicKey publicKey =
        (RSAPublicKey) keyFactory.generatePublic(new RSAPublicKeySpec(privateKey.getModulus(), privateKey.getPublicExponent()));
    return new RSAKey.Builder(publicKey)
        .privateKey(privateKey)
        .keyUse(KeyUse.SIGNATURE)
        .keyID(JWT_KID)
        .algorithm(JWSAlgorithm.RS256)
        .build();
  }

  private static RSAPrivateCrtKey parsePkcs8RsaPrivateCrtKey(String pem) throws InvalidKeySpecException, NoSuchAlgorithmException {
    String normalized =
        pem.replace("-----BEGIN PRIVATE KEY-----", "")
            .replace("-----END PRIVATE KEY-----", "")
            .replaceAll("\\s", "");
    byte[] pkcs8 = Base64.getDecoder().decode(normalized);
    PKCS8EncodedKeySpec spec = new PKCS8EncodedKeySpec(pkcs8);
    KeyFactory keyFactory = KeyFactory.getInstance("RSA");
    var key = keyFactory.generatePrivate(spec);
    if (!(key instanceof RSAPrivateCrtKey rsaPrivateCrtKey)) {
      throw new InvalidKeySpecException("expected RSA private key with CRT parameters");
    }
    return rsaPrivateCrtKey;
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
