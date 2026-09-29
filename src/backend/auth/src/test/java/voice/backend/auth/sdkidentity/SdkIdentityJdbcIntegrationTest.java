package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.JWSObject;
import com.nimbusds.jose.JOSEObjectType;
import com.nimbusds.jose.Payload;
import com.nimbusds.jose.crypto.ECDSASigner;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.Curve;
import com.nimbusds.jose.jwk.ECKey;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.gen.ECKeyGenerator;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Date;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

/** GAME-AUTH-01: real crypto and PostgreSQL enforce independent identity and device fences. */
@Testcontainers(disabledWithoutDocker = true)
class SdkIdentityJdbcIntegrationTest {
  private static final Instant NOW = Instant.parse("2026-09-26T12:00:00Z");
  private static final String ISSUER = "https://accounts.google.com";
  private static final String SUBJECT = "player";
  private static RSAKey googleKey;
  private static RSAKey gameKey;

  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice")
          .withLabel("voice.task", "GAME-AUTH-01").withReuse(false);

  private final UUID app = UUID.randomUUID();
  private final UUID env = UUID.randomUUID();
  private final String client = "voice-owned-client.apps.googleusercontent.com";
  private NamedParameterJdbcTemplate jdbc;
  private Map<String, SdkApplication> applications;
  private SdkAuthorizationPolicy policies;
  private ECKey device;

  @BeforeAll
  static void migrateAndGenerateKeys() throws Exception {
    Flyway.configure().dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations("classpath:db/migration").load().migrate();
    googleKey = new RSAKeyGenerator(2048).keyID("trusted-google").generate();
    gameKey = new RSAKeyGenerator(2048).keyID("operator-game").generate();
  }

  @BeforeEach
  void initialize() throws Exception {
    jdbc = new NamedParameterJdbcTemplate(source());
    jdbc.getJdbcTemplate().execute("TRUNCATE sdk_game_message_execution_permits,sdk_game_message_grants,"
        + "sdk_device_authority_issues,sdk_game_binding_handoff_claims,"
        + "sdk_game_binding_handoff_issuances,"
        + "sdk_registration_intents,sdk_conversion_operations,"
        + "sdk_linked_sessions,sdk_authorizations,sdk_device_key_operations,sdk_sessions,"
        + "sdk_device_keys,sdk_devices,sdk_challenges,sdk_identities");
    applications = new HashMap<>();
    admit(app, env, client);
    policies = (application, environment) -> activePolicy(application, environment);
    device = new ECKeyGenerator(Curve.P_256).generate();
  }

  @Test
  void exchangeCreatesSeparateSdkPrincipalAndStoresOnlyTokenHash() throws Exception {
    var service = service(NOW);
    var challenge = challenge(service, app, env, device);
    assertThat(challenge.clientId()).isEqualTo(client);
    assertThat(challenge.nonce()).isNotBlank();
    assertThat(challenge.expiresAt()).isEqualTo(NOW.plusSeconds(300));
    String provider = provider(challenge, client);
    String ticket = gameTicket(challenge, app, env, subjectHash());
    var session = service.exchange(challenge.challengeId(), provider, ticket, enroll(device, challenge));

    assertThat(session.accountId()).isNotNull();
    assertThat(session.actorId()).isNotNull().isNotEqualTo(session.accountId());
    assertThat(session.deviceId()).isNotNull();
    assertThat(session.keyId()).isNotNull();
    assertThat(session.deviceGeneration()).isEqualTo(1L);
    assertThat(session.applicationId()).isEqualTo(app);
    assertThat(session.environmentId()).isEqualTo(env);
    assertThat(session.gameSubject()).isEqualTo("external-game-player");
    assertThat(session.accountType()).isEqualTo("sdk-account");
    assertThat(session.accessToken()).matches("[A-Za-z0-9_-]{43}");
    assertThat(session.expiresAt()).isEqualTo(NOW.plusSeconds(300));
    assertThat(count("accounts")).isZero();
    assertThat(count("sdk_identities")).isEqualTo(1);
    assertThat(count("sdk_devices")).isEqualTo(1);
    var enrolledKey = jdbc.queryForMap("SELECT key_id,generation,application_id,environment_id,public_jwk,"
        + "key_thumbprint,status,not_before,not_after,revoked_at FROM sdk_device_keys", Map.of());
    assertThat(enrolledKey.get("key_id")).isNotNull();
    assertThat(enrolledKey.get("key_id")).isEqualTo(session.keyId());
    assertThat(enrolledKey.get("generation")).isEqualTo(1L);
    assertThat(enrolledKey.get("application_id")).isEqualTo(app);
    assertThat(enrolledKey.get("environment_id")).isEqualTo(env);
    assertThat(enrolledKey.get("public_jwk")).isEqualTo(device.toPublicJWK().toJSONString());
    assertThat(enrolledKey.get("key_thumbprint")).isEqualTo(device.computeThumbprint().toString());
    assertThat(enrolledKey.get("status")).isEqualTo("active");
    assertThat(enrolledKey.get("not_before")).isEqualTo(java.sql.Timestamp.from(NOW));
    assertThat(enrolledKey.get("not_after")).isEqualTo(
        java.sql.Timestamp.from(NOW.plus(java.time.Duration.ofDays(90))));
    assertThat(enrolledKey.get("revoked_at")).isNull();
    String persisted = jdbc.queryForObject(
        "SELECT row_to_json(s)::text FROM sdk_sessions s", Map.of(), String.class);
    assertThat(persisted).doesNotContain(session.accessToken(), provider, ticket);
    for (String table : List.of("sdk_identities", "sdk_devices", "sdk_challenges", "sdk_sessions")) {
      for (String row : jdbc.queryForList("SELECT row_to_json(s)::text FROM " + table + " s",
          Map.of(), String.class)) {
        assertThat(row).doesNotContain(session.accessToken(), provider, ticket);
      }
    }
    assertThat(jdbc.queryForObject("SELECT ownership_generation FROM sdk_identities",
        Map.of(), Long.class)).isEqualTo(1L);

    var verified = service.session(session.accessToken(), sessionProof(device, session));
    assertThat(verified.accountId()).isEqualTo(session.accountId());
    assertThat(verified.actorId()).isEqualTo(session.actorId());
    assertThat(verified.applicationId()).isEqualTo(app);
    assertThat(verified.environmentId()).isEqualTo(env);
    assertThat(verified.accessToken()).isNull();
    assertThat(service(NOW).session(session.accessToken(), sessionProof(device, session)))
        .isEqualTo(verified);
  }

  @Test
  void initialEnrollmentRejectsCallerSelectedKeyIdAndUnknownJwkMembers() throws Exception {
    var callerKid = new com.nimbusds.jose.jwk.ECKey.Builder(device.toPublicJWK())
        .keyID(UUID.randomUUID().toString()).build();
    assertThatThrownBy(() -> service(NOW).challenge(app, env, callerKid.toJSONString()))
        .isInstanceOf(SdkIdentityDeniedException.class);
    var unknownMember = new java.util.LinkedHashMap<>(device.toPublicJWK().toJSONObject());
    unknownMember.put("x-voice-untrusted", "ignored");
    assertThatThrownBy(() -> service(NOW).challenge(app, env,
        com.nimbusds.jose.util.JSONObjectUtils.toJSONString(unknownMember)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_challenges")).isZero();
  }

  @Test
  void newDeviceKeyExpiresAtNinetyDaysAndCannotAuthorizeAtTheBoundary() throws Exception {
    var current = new AtomicReference<>(NOW);
    var service = service(new MutableClock(current, ZoneOffset.UTC));
    var session = exchange(service, app, env, client, device);
    current.set(NOW.plus(java.time.Duration.ofDays(90)).minusMillis(1));
    var beforeExpiry = challenge(service, app, env, device);
    var accepted = service.exchange(beforeExpiry.challengeId(), providerAt(beforeExpiry, client, current.get()),
        gameTicketAt(beforeExpiry, app, env, subjectHash(), current.get()), enroll(device, beforeExpiry));
    assertThat(accepted.keyId()).isEqualTo(session.keyId());
    current.set(NOW.plus(java.time.Duration.ofDays(90)));
    var atExpiry = challenge(service, app, env, device);
    assertThatThrownBy(() -> service.exchange(atExpiry.challengeId(), providerAt(atExpiry, client, current.get()),
        gameTicketAt(atExpiry, app, env, subjectHash(), current.get()), enroll(device, atExpiry)))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void lifecycleChallengesArePurposeBoundAndCannotBeUsedForInitialEnrollment() throws Exception {
    var session = exchange(service(NOW), app, env, client, device);
    var replacement = new ECKeyGenerator(Curve.P_256).generate();
    var rotate = service(NOW).deviceKeyChallenge("rotate", app, env, session.accessToken(), null,
        replacement.toPublicJWK().toJSONString());
    assertThat(rotate.purpose()).isEqualTo("rotate");
    assertThat(rotate.deviceId()).isEqualTo(session.deviceId());
    assertThat(rotate.replacesDeviceId()).isNull();
    assertThat(rotate.expiresAt()).isEqualTo(NOW.plusSeconds(300));
    var recover = service(NOW).deviceKeyChallenge("recover", app, env, null, session.deviceId(),
        replacement.toPublicJWK().toJSONString());
    assertThat(recover.purpose()).isEqualTo("recover");
    assertThat(recover.deviceId()).isNull();
    assertThat(recover.replacesDeviceId()).isEqualTo(session.deviceId());
    assertThatThrownBy(() -> service(NOW).exchange(rotate.challengeId(), "bad", "bad", "bad"))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void rotationAtomicallyAssignsNextKeyAndStoresExactIdempotentResult() throws Exception {
    var service = service(NOW);
    var original = exchange(service, app, env, client, device);
    var replacement = new ECKeyGenerator(Curve.P_256).generate();
    var challenge = service.deviceKeyChallenge("rotate", app, env, original.accessToken(), null,
        replacement.toPublicJWK().toJSONString());
    UUID requestId = UUID.randomUUID();
    String thumbprint = replacement.computeThumbprint().toString();
    var currentProof = lifecycleProof(device, original.keyId(), lifecycleClaims("rotate_current", requestId,
        challenge, original.deviceId(), original.keyId(), device.computeThumbprint().toString(), thumbprint, NOW));
    var newProof = lifecycleProof(replacement, null, lifecycleClaims("rotate_new", requestId,
        challenge, original.deviceId(), null, thumbprint, thumbprint, NOW));
    byte[] requestBytes = (challenge.challengeId() + "|" + requestId + "|rotate").getBytes(StandardCharsets.UTF_8);

    var result = service.rotate(challenge.challengeId(), provider(challenge, client), currentProof,
        newProof, requestId, requestBytes);
    assertThat(result.deviceId()).isEqualTo(original.deviceId());
    assertThat(result.keyId()).isNotEqualTo(original.keyId());
    assertThat(result.generation()).isEqualTo(2L);
    assertThat(result.authorityRevision()).isEqualTo(2L);
    var oldKey = jdbc.queryForMap("SELECT status,not_after FROM sdk_device_keys WHERE key_id=:id",
        Map.of("id", original.keyId()));
    assertThat(oldKey.get("status")).isEqualTo("overlap");
    assertThat(oldKey.get("not_after")).isEqualTo(java.sql.Timestamp.from(NOW.plusSeconds(600)));
    String independentProof = provider(challenge, client);
    var start = new CyclicBarrier(8);
    var workers = Executors.newFixedThreadPool(8);
    try {
      List<Callable<SdkIdentityService.DeviceKeyResult>> tasks = new ArrayList<>();
      for (int i = 0; i < 8; i++) {
        tasks.add(() -> {
          start.await(10, TimeUnit.SECONDS);
          return service(NOW).rotate(challenge.challengeId(), independentProof, currentProof,
              newProof, requestId, requestBytes);
        });
      }
      for (var future : workers.invokeAll(tasks, 30, TimeUnit.SECONDS)) {
        assertThat(future.isCancelled()).isFalse();
        assertThat(future.get()).isEqualTo(result);
      }
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
    assertThatThrownBy(() -> service.rotate(challenge.challengeId(), "expired-proof", "expired-proof",
        "expired-proof", requestId, (challenge.challengeId() + "changed").getBytes(StandardCharsets.UTF_8)))
        .isInstanceOf(SdkDeviceKeyConflictException.class);
    assertThat(count("sdk_device_key_operations")).isEqualTo(1);
  }

  @Test
  void recoveryReplacesOnlyNamedLostDeviceAndConcurrentRetriesReturnOneReceipt() throws Exception {
    var service = service(NOW);
    var lost = exchange(service, app, env, client, device);
    var survivorKey = new ECKeyGenerator(Curve.P_256).generate();
    var survivor = exchange(service, app, env, client, survivorKey);
    var recoveredKey = new ECKeyGenerator(Curve.P_256).generate();
    var challenge = service.deviceKeyChallenge("recover", app, env, null, lost.deviceId(),
        recoveredKey.toPublicJWK().toJSONString());
    UUID requestId = UUID.randomUUID();
    String thumbprint = recoveredKey.computeThumbprint().toString();
    String newProof = lifecycleProof(recoveredKey, null, lifecycleClaims("recover_new", requestId,
        challenge, null, null, thumbprint, null, NOW));
    String providerToken = provider(challenge, client);
    byte[] requestBytes = (challenge.challengeId() + "|" + requestId + "|recover")
        .getBytes(StandardCharsets.UTF_8);
    var result = service.recover(challenge.challengeId(), providerToken, newProof, requestId, requestBytes);
    assertThat(result.deviceId()).isNotEqualTo(lost.deviceId()).isNotEqualTo(survivor.deviceId());
    assertThat(result.generation()).isEqualTo(1L);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_device_keys WHERE device_id=:device "
        + "AND status='revoked'", Map.of("device", lost.deviceId()), Long.class)).isEqualTo(1L);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_device_keys WHERE device_id=:device "
        + "AND status='active'", Map.of("device", survivor.deviceId()), Long.class)).isEqualTo(1L);
    assertThat(service.session(survivor.accessToken(), sessionProof(survivorKey, survivor)).deviceId())
        .isEqualTo(survivor.deviceId());
    assertThatThrownBy(() -> service.session(lost.accessToken(), sessionProof(device, lost)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service.deviceKeyChallenge("rotate", app, env, lost.accessToken(), null,
        recoveredKey.toPublicJWK().toJSONString())).isInstanceOf(SdkIdentityDeniedException.class);

    var start = new CyclicBarrier(8);
    var workers = Executors.newFixedThreadPool(8);
    try {
      List<Callable<SdkIdentityService.DeviceKeyResult>> tasks = new ArrayList<>();
      for (int i = 0; i < 8; i++) {
        tasks.add(() -> {
          start.await(10, TimeUnit.SECONDS);
          return service(NOW).recover(challenge.challengeId(), "expired-proof", "expired-proof",
              requestId, requestBytes);
        });
      }
      for (var future : workers.invokeAll(tasks, 30, TimeUnit.SECONDS)) {
        assertThat(future.isCancelled()).isFalse();
        assertThat(future.get()).isEqualTo(result);
      }
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
    assertThat(count("sdk_device_key_operations")).isEqualTo(1);
    assertThat(count("sdk_devices")).isEqualTo(3);
  }

  @Test
  void explicitRevokeUsesFreshIdentityAndCurrentKeyProofAndStopsAuthImmediately() throws Exception {
    var service = service(NOW);
    var enrollment = challenge(service, app, env, device);
    var session = service.exchange(enrollment.challengeId(), provider(enrollment, client),
        gameTicket(enrollment, app, env, subjectHash()), enroll(device, enrollment));
    UUID requestId = UUID.randomUUID();
    var claims = new java.util.TreeMap<String, Object>();
    claims.put("version", 1L);
    claims.put("purpose", "revoke");
    claims.put("audience", "voice.auth.device-key");
    claims.put("request_id", requestId.toString());
    claims.put("application_id", app.toString());
    claims.put("environment_id", env.toString());
    claims.put("device_id", session.deviceId().toString());
    claims.put("key_id", session.keyId().toString());
    claims.put("key_thumbprint", device.computeThumbprint().toString());
    claims.put("issued_at", NOW.toEpochMilli());
    String proof = lifecycleProof(device, session.keyId(), claims);
    byte[] requestBytes = (session.deviceId() + "|" + requestId + "|revoke")
        .getBytes(StandardCharsets.UTF_8);

    var result = service.revokeDevice(session.deviceId(), provider(enrollment, client), proof,
        requestId, requestBytes);
    assertThat(result.deviceId()).isEqualTo(session.deviceId());
    assertThat(result.keyId()).isEqualTo(session.keyId());
    assertThat(result.authorityRevision()).isEqualTo(2L);
    assertThat(jdbc.queryForObject("SELECT status FROM sdk_device_keys WHERE key_id=:id",
        Map.of("id", session.keyId()), String.class)).isEqualTo("revoked");
    assertThat(jdbc.queryForObject("SELECT not_after FROM sdk_device_keys WHERE key_id=:id",
        Map.of("id", session.keyId()), java.sql.Timestamp.class)).isEqualTo(java.sql.Timestamp.from(NOW));
    assertThatThrownBy(() -> service.session(session.accessToken(), sessionProof(device, session)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(service.revokeDevice(session.deviceId(), "expired", "expired", requestId, requestBytes))
        .isEqualTo(result);
    assertThatThrownBy(() -> service.revokeDevice(session.deviceId(), "expired", "expired", requestId,
        (session.deviceId() + "changed").getBytes(StandardCharsets.UTF_8)))
        .isInstanceOf(SdkDeviceKeyConflictException.class);
  }

  @Test
  void deviceAuthorityUsesCurrentKeyAndBindingAndCapsAssertionAtKeyExpiry() throws Exception {
    var enrollmentService = service(NOW);
    var enrollment = challenge(enrollmentService, app, env, device);
    var session = enrollmentService.exchange(enrollment.challengeId(), provider(enrollment, client),
        gameTicket(enrollment, app, env, subjectHash()), enroll(device, enrollment));
    jdbc.update("UPDATE sdk_device_keys SET not_after=:deadline WHERE key_id=:key",
        Map.of("deadline", java.sql.Timestamp.from(NOW.plusSeconds(2)), "key", session.keyId()));
    var requestId = UUID.randomUUID();
    byte[] request = deviceAuthorityProof(device, session, requestId, NOW).getBytes(StandardCharsets.US_ASCII);
    var signedClaims = new AtomicReference<Map<String, Object>>();
    SdkDeviceStatusIssuer signer = claims -> { signedClaims.set(Map.copyOf(claims)); return "saved-assertion"; };

    assertThatThrownBy(() -> service(Clock.fixed(NOW, ZoneOffset.UTC), signer, null)
        .deviceAuthority(session.accessToken(), new String(request, StandardCharsets.US_ASCII), request))
        .isInstanceOf(SdkIdentityDeniedException.class);

    var binding = new SdkBindingAuthority.Binding(session.actorId(), UUID.randomUUID());
    SdkBindingAuthority authority = (application, environment, account, deviceId) -> {
      assertThat(application).isEqualTo(app);
      assertThat(environment).isEqualTo(env);
      assertThat(account).isEqualTo(session.accountId());
      assertThat(deviceId).isEqualTo(session.deviceId());
      return java.util.Optional.of(binding);
    };
    var service = service(Clock.fixed(NOW, ZoneOffset.UTC), signer, authority);
    assertThat(service.deviceAuthority(session.accessToken(), new String(request, StandardCharsets.US_ASCII), request))
        .isEqualTo("saved-assertion");
    assertThatThrownBy(() -> service.deviceAuthority("A".repeat(43),
        new String(request, StandardCharsets.US_ASCII), request)).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(signedClaims.get()).containsEntry("binding_id", binding.bindingId().toString())
        .containsEntry("actor_id", binding.actorId().toString())
        .containsEntry("key_id", session.keyId().toString())
        .containsEntry("authority_revision", 1L)
        .containsEntry("exp", NOW.plusSeconds(2).toEpochMilli());
    assertThat(service.deviceAuthority(session.accessToken(), new String(request, StandardCharsets.US_ASCII), request))
        .isEqualTo("saved-assertion");
    assertThat(count("sdk_device_authority_issues")).isEqualTo(1);

    var revokeClaims = new java.util.TreeMap<String, Object>();
    revokeClaims.put("version", 1L);
    revokeClaims.put("purpose", "revoke");
    revokeClaims.put("audience", "voice.auth.device-key");
    UUID revokeId = UUID.randomUUID();
    revokeClaims.put("request_id", revokeId.toString());
    revokeClaims.put("application_id", app.toString());
    revokeClaims.put("environment_id", env.toString());
    revokeClaims.put("device_id", session.deviceId().toString());
    revokeClaims.put("key_id", session.keyId().toString());
    revokeClaims.put("key_thumbprint", device.computeThumbprint().toString());
    revokeClaims.put("issued_at", NOW.toEpochMilli());
    String revokeProof = lifecycleProof(device, session.keyId(), revokeClaims);
    enrollmentService.revokeDevice(session.deviceId(), provider(enrollment, client), revokeProof,
        revokeId, "revoke".getBytes(StandardCharsets.US_ASCII));
    assertThat(service.deviceAuthority(session.accessToken(), new String(request, StandardCharsets.US_ASCII), request))
        .isEqualTo("saved-assertion");
    UUID freshRequestId = UUID.randomUUID();
    byte[] freshRequest = deviceAuthorityProof(device, session, freshRequestId, NOW)
        .getBytes(StandardCharsets.US_ASCII);
    assertThatThrownBy(() -> service.deviceAuthority(session.accessToken(),
        new String(freshRequest, StandardCharsets.US_ASCII), freshRequest))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void concurrentRevokeWaitsForInFlightAssertionAndPreventsAnyLaterAssertion() throws Exception {
    var base = service(NOW);
    var enrollment = challenge(base, app, env, device);
    var session = base.exchange(enrollment.challengeId(), provider(enrollment, client),
        gameTicket(enrollment, app, env, subjectHash()), enroll(device, enrollment));
    UUID assertionId = UUID.randomUUID();
    byte[] request = deviceAuthorityProof(device, session, assertionId, NOW).getBytes(StandardCharsets.US_ASCII);
    CountDownLatch signerEntered = new CountDownLatch(1);
    CountDownLatch allowSignerCommit = new CountDownLatch(1);
    SdkDeviceStatusIssuer blockingSigner = claims -> {
      signerEntered.countDown();
      try {
        if (!allowSignerCommit.await(10, TimeUnit.SECONDS)) throw new IllegalStateException("signer gate timeout");
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        throw new IllegalStateException(interrupted);
      }
      return "assertion-issued-before-revoke";
    };
    SdkBindingAuthority binding = (application, environment, account, deviceId) -> java.util.Optional.of(
        new SdkBindingAuthority.Binding(session.actorId(), UUID.randomUUID()));
    var issuerService = service(Clock.fixed(NOW, ZoneOffset.UTC), blockingSigner, binding);
    var workers = Executors.newFixedThreadPool(2);
    try {
      var assertionFuture = workers.submit(() -> issuerService.deviceAuthority(session.accessToken(),
          new String(request, StandardCharsets.US_ASCII), request));
      assertThat(signerEntered.await(10, TimeUnit.SECONDS)).isTrue();
      var revokeId = UUID.randomUUID();
      var revokeClaims = new java.util.TreeMap<String, Object>();
      revokeClaims.put("version", 1L);
      revokeClaims.put("purpose", "revoke");
      revokeClaims.put("audience", "voice.auth.device-key");
      revokeClaims.put("request_id", revokeId.toString());
      revokeClaims.put("application_id", app.toString());
      revokeClaims.put("environment_id", env.toString());
      revokeClaims.put("device_id", session.deviceId().toString());
      revokeClaims.put("key_id", session.keyId().toString());
      revokeClaims.put("key_thumbprint", device.computeThumbprint().toString());
      revokeClaims.put("issued_at", NOW.toEpochMilli());
      String revokeProof = lifecycleProof(device, session.keyId(), revokeClaims);
      var revokeFuture = workers.submit(() -> base.revokeDevice(session.deviceId(), provider(enrollment, client),
          revokeProof, revokeId, "concurrent-revoke".getBytes(StandardCharsets.US_ASCII)));
      allowSignerCommit.countDown();
      assertThat(assertionFuture.get(10, TimeUnit.SECONDS)).isEqualTo("assertion-issued-before-revoke");
      assertThat(revokeFuture.get(10, TimeUnit.SECONDS).authorityRevision()).isEqualTo(2L);
      assertThat(issuerService.deviceAuthority(session.accessToken(), new String(request, StandardCharsets.US_ASCII),
          request)).isEqualTo("assertion-issued-before-revoke");
      UUID laterId = UUID.randomUUID();
      byte[] later = deviceAuthorityProof(device, session, laterId, NOW).getBytes(StandardCharsets.US_ASCII);
      assertThatThrownBy(() -> issuerService.deviceAuthority(session.accessToken(),
          new String(later, StandardCharsets.US_ASCII), later)).isInstanceOf(SdkIdentityDeniedException.class);
    } finally {
      allowSignerCommit.countDown();
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void gameBindingRevokeFirstDeniesAuthorityAndPersistsNoIssueRow() throws Exception {
    var session = exchange(service(NOW), app, env, client, device);
    var authorityFixture = installActiveBindingGrant(session);
    try (var gis = authorityServer(authorityFixture, false)) {
      var issuerService = productionAuthorityService(gis);
      var tx = transactionTemplate();
      var revokeService = handoffService(tx);
      CountDownLatch revokeUpdated = new CountDownLatch(1);
      CountDownLatch allowRevokeCommit = new CountDownLatch(1);
      java.util.concurrent.atomic.AtomicInteger revokePid = new java.util.concurrent.atomic.AtomicInteger();
      var workers = Executors.newFixedThreadPool(2);
      try {
        var revokeFuture = workers.submit(() -> tx.execute(status -> {
          revokePid.set(jdbc.getJdbcTemplate().queryForObject("SELECT pg_backend_pid()", Integer.class));
          var receipt = revokeService.revoke(authorityFixture.bindingOperationId());
          revokeUpdated.countDown();
          awaitGate(allowRevokeCommit, "revoke commit gate timed out");
          return receipt;
        }));
        assertThat(revokeUpdated.await(10, TimeUnit.SECONDS)).isTrue();
        var requestId = UUID.randomUUID();
        byte[] proof = deviceAuthorityProof(device, session, requestId, NOW).getBytes(StandardCharsets.US_ASCII);
        var issueFuture = workers.submit(() -> issuerService.deviceAuthority(
            session.accessToken(), new String(proof, StandardCharsets.US_ASCII), proof));
        DatabaseLockWait wait = awaitDatabaseLockWait(0, revokePid.get(), "%sdk_identities%", "%");
        assertThat(wait.waitingPid()).isPositive();
        assertThat(wait.blockingPid()).isEqualTo(revokePid.get());
        allowRevokeCommit.countDown();
        revokeFuture.get(10, TimeUnit.SECONDS);
        assertThatThrownBy(() -> issueFuture.get(10, TimeUnit.SECONDS))
            .hasCauseInstanceOf(SdkIdentityDeniedException.class);
        assertThat(count("sdk_device_authority_issues")).isZero();
        assertThat(gis.requests()).isZero();
      } finally {
        allowRevokeCommit.countDown();
        workers.shutdownNow();
        assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
      }
    }
  }

  @Test
  void gameBindingAuthorityIssueFirstCommitsBeforeConcurrentRevoke() throws Exception {
    var session = exchange(service(NOW), app, env, client, device);
    var authorityFixture = installActiveBindingGrant(session);
    try (var gis = authorityServer(authorityFixture, true)) {
      var issueBackendPid = new java.util.concurrent.atomic.AtomicInteger();
      var issueTransactionId = new java.util.concurrent.atomic.AtomicReference<Long>();
      var issuerService = productionAuthorityService(gis, issueBackendPid, issueTransactionId);
      var tx = transactionTemplate();
      var revokeService = handoffService(tx);
      CountDownLatch revokeStarted = new CountDownLatch(1);
      java.util.concurrent.atomic.AtomicInteger revokePid = new java.util.concurrent.atomic.AtomicInteger();
      var workers = Executors.newFixedThreadPool(2);
      try {
        var requestId = UUID.randomUUID();
        byte[] proof = deviceAuthorityProof(device, session, requestId, NOW).getBytes(StandardCharsets.US_ASCII);
        var issueFuture = workers.submit(() -> issuerService.deviceAuthority(
            session.accessToken(), new String(proof, StandardCharsets.US_ASCII), proof));
        assertThat(gis.requestSeen().await(10, TimeUnit.SECONDS)).isTrue();
        assertThat(issueBackendPid.get()).isPositive();
        assertThat(issueTransactionId.get()).isNotNull().isPositive();
        assertThat(jdbc.queryForObject("SELECT state FROM pg_stat_activity WHERE pid=:pid",
            Map.of("pid", issueBackendPid.get()), String.class)).isEqualTo("idle in transaction");
        var revokeFuture = workers.submit(() -> tx.execute(status -> {
          revokePid.set(jdbc.getJdbcTemplate().queryForObject("SELECT pg_backend_pid()", Integer.class));
          revokeStarted.countDown();
          return revokeService.revoke(authorityFixture.bindingOperationId());
        }));
        assertThat(revokeStarted.await(10, TimeUnit.SECONDS)).isTrue();
        DatabaseLockWait wait = awaitDatabaseLockWait(revokePid.get(), issueBackendPid.get(),
            "%sdk_identities%", "%sdk_game_message_grants%");
        assertThat(wait.waitingPid()).isEqualTo(revokePid.get());
        assertThat(wait.blockingPid()).isEqualTo(issueBackendPid.get());
        assertThat(gis.isHoldingResponse()).as("GIS remains held while the database lock edge is observed").isTrue();
        assertThat(jdbc.queryForObject("SELECT state FROM pg_stat_activity WHERE pid=:pid",
            Map.of("pid", wait.blockingPid()), String.class)).isEqualTo("idle in transaction");
        assertThat(count("sdk_device_authority_issues")).isZero();
        gis.releaseResponse();
        assertThat(issueFuture.get(10, TimeUnit.SECONDS)).isEqualTo("binding-authority-assertion");
        assertThat(count("sdk_device_authority_issues")).isEqualTo(1);
        assertThat(revokeFuture.get(10, TimeUnit.SECONDS).status()).isEqualTo("revoked");
        assertThat(jdbc.queryForObject("SELECT game_binding_status FROM sdk_authorizations WHERE game_binding_operation_id=:id",
            Map.of("id", authorityFixture.bindingOperationId()), String.class)).isEqualTo("revoked");
      } finally {
        gis.releaseResponse();
        workers.shutdownNow();
        assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
      }
    }
  }

  @Test
  void returningIndependentProofRegistersNewDeviceWithSameAccountAndActor() throws Exception {
    var first = exchange(service(NOW), app, env, client, device);
    var nextDevice = new ECKeyGenerator(Curve.P_256).generate();
    var second = exchange(service(NOW), app, env, client, nextDevice);
    assertThat(second.accountId()).isEqualTo(first.accountId());
    assertThat(second.actorId()).isEqualTo(first.actorId());
    assertThat(second.deviceId()).isNotEqualTo(first.deviceId());
    assertThat(second.accessToken()).isNotEqualTo(first.accessToken());
    assertThat(count("sdk_identities")).isEqualTo(1);
    assertThat(count("sdk_devices")).isEqualTo(2);
  }

  @Test
  void concurrentReplayAcrossServiceInstancesIssuesExactlyOneSession() throws Exception {
    var challenge = challenge(service(NOW), app, env, device);
    String provider = provider(challenge, client);
    String ticket = gameTicket(challenge, app, env, subjectHash());
    String proof = enroll(device, challenge);
    var start = new CyclicBarrier(8);
    var workers = Executors.newFixedThreadPool(8);
    try {
      List<Callable<Boolean>> tasks = new ArrayList<>();
      for (int i = 0; i < 8; i++) {
        tasks.add(() -> {
          start.await(10, TimeUnit.SECONDS);
          try {
            service(NOW).exchange(challenge.challengeId(), provider, ticket, proof);
            return true;
          } catch (SdkIdentityDeniedException denied) {
            return false;
          }
        });
      }
      int successes = 0;
      for (var result : workers.invokeAll(tasks, 30, TimeUnit.SECONDS)) {
        assertThat(result.isCancelled()).isFalse();
        if (result.get()) successes++;
      }
      assertThat(successes).isEqualTo(1);
      assertThat(count("sdk_sessions")).isEqualTo(1);
      assertThat(count("sdk_identities")).isEqualTo(1);
      assertThat(count("sdk_devices")).isEqualTo(1);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void sameProviderSubjectHasSeparateAccountAndActorForEachApplicationAndEnvironment() throws Exception {
    UUID otherApp = UUID.randomUUID();
    UUID otherEnv = UUID.randomUUID();
    admit(otherApp, env, "other-app-client");
    admit(app, otherEnv, "other-env-client");
    var first = exchange(service(NOW), app, env, client, device);
    var second = exchange(service(NOW), otherApp, env, "other-app-client", device);
    var third = exchange(service(NOW), app, otherEnv, "other-env-client", device);
    assertThat(List.of(first.accountId(), second.accountId(), third.accountId())).doesNotHaveDuplicates();
    assertThat(List.of(first.actorId(), second.actorId(), third.actorId())).doesNotHaveDuplicates();
    assertThat(count("sdk_identities")).isEqualTo(3);
    var wrongAudience = challenge(service(NOW), app, env, device);
    assertThatThrownBy(() -> service(NOW).exchange(wrongAudience.challengeId(),
        provider(wrongAudience, "other-app-client"), gameTicket(wrongAudience, app, env, subjectHash()),
        enroll(device, wrongAudience))).isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void providerEmailAndDisplayClaimsNeverMergeOrSplitSdkAccounts() throws Exception {
    var sameEmailFirst = exchangeWithProviderClaims("subject-a", "shared@example.test", "Shared Name");
    var sameEmailSecond = exchangeWithProviderClaims("subject-b", "shared@example.test", "Shared Name");
    var changedProfileClaims = exchangeWithProviderClaims("subject-a", "renamed@example.test", "Another Name");

    assertThat(sameEmailFirst.accountId()).isNotEqualTo(sameEmailSecond.accountId());
    assertThat(changedProfileClaims.accountId()).isEqualTo(sameEmailFirst.accountId());
    assertThat(changedProfileClaims.actorId()).isEqualTo(sameEmailFirst.actorId());
    assertThat(count("sdk_identities")).isEqualTo(2);
  }

  @Test
  void developerCredentialAndWrongDeviceKeyCannotIssueSession() throws Exception {
    var challenge = challenge(service(NOW), app, env, device);
    var attacker = new ECKeyGenerator(Curve.P_256).generate();
    assertThatThrownBy(() -> service(NOW).exchange(challenge.challengeId(),
        provider(challenge, client), gameTicket(challenge, app, env, subjectHash()),
        enroll(attacker, challenge))).isInstanceOf(SdkIdentityDeniedException.class);
    var developerChallenge = challenge(service(NOW), app, env, device);
    assertThatThrownBy(() -> service(NOW).exchange(developerChallenge.challengeId(),
        "developer-server-secret", gameTicket(developerChallenge, app, env, subjectHash()),
        enroll(device, developerChallenge))).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_identities")).isZero();
    assertThat(count("sdk_sessions")).isZero();
  }

  @Test
  void proofCannotBeMovedToAnotherChallengeOrPurpose() throws Exception {
    var first = challenge(service(NOW), app, env, device);
    var second = challenge(service(NOW), app, env, device);
    assertThatThrownBy(() -> service(NOW).exchange(second.challengeId(), provider(first, client),
        gameTicket(first, app, env, subjectHash()), enroll(device, first)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    var target = challenge(service(NOW), app, env, device);
    assertThatThrownBy(() -> service(NOW).exchange(target.challengeId(), provider(target, client),
        gameTicket(target, app, env, subjectHash()), enroll(device, first)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    var session = exchange(service(NOW), app, env, client, device);
    assertThatThrownBy(() -> service(NOW).session(session.accessToken(), enroll(device, first)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW).revoke(session.accessToken(), sessionProof(device, session)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(service(NOW).session(session.accessToken(), sessionProof(device, session)).accountId())
        .isEqualTo(session.accountId());
  }

  @Test
  void gameTicketRequiresIndependentSubjectLinkAndCorrectAppEnvironment() throws Exception {
    for (int invalid = 0; invalid < 4; invalid++) {
      var challenge = challenge(service(NOW), app, env, device);
      String ticket = switch (invalid) {
        case 0 -> gameTicket(challenge, app, env, sha256(ISSUER + "\nother-player"));
        case 1 -> gameTicket(challenge, UUID.randomUUID(), env, subjectHash());
        case 2 -> gameTicket(challenge, app, UUID.randomUUID(), subjectHash());
        default -> provider(challenge, client);
      };
      assertThatThrownBy(() -> service(NOW).exchange(challenge.challengeId(),
          provider(challenge, client), ticket, enroll(device, challenge)))
          .isInstanceOf(SdkIdentityDeniedException.class);
    }
    assertThat(count("sdk_sessions")).isZero();
    assertThat(count("sdk_identities")).isZero();
  }

  @ParameterizedTest
  @ValueSource(strings = {"signature", "nonce", "audience", "extra-audience", "subject",
      "expired", "expiry-boundary", "stale", "future", "missing-iat", "missing-exp", "missing-hash",
      "missing-ticket"})
  void invalidGameTicketCannotSubstituteForValidatedIndependentProof(String invalid) throws Exception {
    var challenge = challenge(service(NOW), app, env, device);
    var claims = new JWTClaimsSet.Builder().issuer("game:" + app + ":" + env)
        .subject("external-game-player").audience("voice:sdk-enroll").claim("nonce", challenge.nonce())
        .claim("independent_subject_hash", subjectHash()).issueTime(Date.from(NOW))
        .expirationTime(Date.from(NOW.plusSeconds(300)));
    switch (invalid) {
      case "nonce" -> claims.claim("nonce", "another-challenge");
      case "audience" -> claims.audience("developer-service");
      case "extra-audience" -> claims.audience(List.of("voice:sdk-enroll", "another-service"));
      case "subject" -> claims.subject(null);
      case "expired" -> claims.expirationTime(Date.from(NOW.minusSeconds(1)));
      case "expiry-boundary" -> claims.expirationTime(Date.from(NOW));
      case "stale" -> claims.issueTime(Date.from(NOW.minusSeconds(301)));
      case "future" -> claims.issueTime(Date.from(NOW.plusSeconds(31)));
      case "missing-iat" -> claims.issueTime(null);
      case "missing-exp" -> claims.expirationTime(null);
      case "missing-hash" -> claims.claim("independent_subject_hash", null);
      default -> { }
    }
    var gameJwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256)
        .keyID(gameKey.getKeyID()).build(), claims.build());
    gameJwt.sign(new RSASSASigner(invalid.equals("signature") ? googleKey : gameKey));
    String ticket = invalid.equals("missing-ticket") ? "" : gameJwt.serialize();
    assertThatThrownBy(() -> service(NOW).exchange(challenge.challengeId(), provider(challenge, client),
        ticket, enroll(device, challenge))).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_identities")).isZero();
    assertThat(count("sdk_sessions")).isZero();
  }

  @Test
  void revokeImmediatelyInvalidatesAllDeviceSessionsAndTombstonesItsKey() throws Exception {
    var first = exchange(service(NOW), app, env, client, device);
    var second = exchange(service(NOW), app, env, client, device);
    assertThat(second.deviceId()).isEqualTo(first.deviceId());
    var otherDevice = new ECKeyGenerator(Curve.P_256).generate();
    var other = exchange(service(NOW), app, env, client, otherDevice);
    service(NOW).revoke(first.accessToken(), proof(device,
        "voice-sdk-revoke-v1\n" + sha256(first.accessToken())));
    assertThat(jdbc.queryForObject("SELECT status FROM sdk_device_keys WHERE device_id=:device",
        Map.of("device", first.deviceId()), String.class)).isEqualTo("revoked");
    assertThat(jdbc.queryForObject("SELECT authority_revision FROM sdk_devices WHERE device_id=:device",
        Map.of("device", first.deviceId()), Long.class)).isEqualTo(2L);
    for (var revoked : List.of(first, second)) {
      assertThatThrownBy(() -> service(NOW).session(revoked.accessToken(), sessionProof(device, revoked)))
          .isInstanceOf(SdkIdentityDeniedException.class);
    }
    assertThatThrownBy(() -> service(NOW).revoke(first.accessToken(), proof(device,
        "voice-sdk-revoke-v1\n" + sha256(first.accessToken()))))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> exchange(service(NOW), app, env, client, device))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(service(NOW).session(other.accessToken(), sessionProof(otherDevice, other)).accountId())
        .isEqualTo(first.accountId());
    var replacement = exchange(service(NOW), app, env, client,
        new ECKeyGenerator(Curve.P_256).generate());
    assertThat(replacement.accountId()).isEqualTo(first.accountId());
    assertThat(replacement.actorId()).isEqualTo(first.actorId());
  }

  @ParameterizedTest
  @ValueSource(strings = {"suspended", "deleted", "retired"})
  void blockedIdentityCannotAuthenticateOrResurrectThroughIndependentLogin(String status) throws Exception {
    var session = exchange(service(NOW), app, env, client, device);
    jdbc.update("UPDATE sdk_identities SET status=:status WHERE account_id=:id",
        Map.of("status", status, "id", session.accountId()));
    assertThatThrownBy(() -> service(NOW).session(session.accessToken(), sessionProof(device, session)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> exchange(service(NOW), app, env, client,
        new ECKeyGenerator(Curve.P_256).generate())).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_identities")).isEqualTo(1);
    assertThat(count("sdk_devices")).isEqualTo(1);
  }

  @Test
  void challengeAndSessionExpireExactlyAtFiveMinutes() throws Exception {
    var challenge = challenge(service(NOW), app, env, device);
    var session = exchange(service(NOW), app, env, client, device);
    String freshProvider = signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT)
        .audience(client).claim("nonce", challenge.nonce()).issueTime(Date.from(NOW.plusSeconds(300)))
        .expirationTime(Date.from(NOW.plusSeconds(600))).build(), googleKey);
    String freshTicket = signed(new JWTClaimsSet.Builder().issuer("game:" + app + ":" + env)
        .subject("external-game-player").audience("voice:sdk-enroll").claim("nonce", challenge.nonce())
        .claim("independent_subject_hash", subjectHash()).issueTime(Date.from(NOW.plusSeconds(300)))
        .expirationTime(Date.from(NOW.plusSeconds(600))).build(), gameKey);
    assertThatThrownBy(() -> service(NOW.plusSeconds(300)).exchange(challenge.challengeId(),
        freshProvider, freshTicket, enroll(device, challenge))).isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW.plusSeconds(300)).session(
        session.accessToken(), sessionProof(device, session))).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_sessions")).isEqualTo(1);
  }

  @Test
  void removingOperatorAdmissionDeniesChallengeExchangeAndCurrentSession() throws Exception {
    var pending = challenge(service(NOW), app, env, device);
    var session = exchange(service(NOW), app, env, client, device);
    applications.clear();
    assertThatThrownBy(() -> challenge(service(NOW), app, env, device))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW).exchange(pending.challengeId(), provider(pending, client),
        gameTicket(pending, app, env, subjectHash()), enroll(device, pending)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW).session(session.accessToken(), sessionProof(device, session)))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void registryAppOrEnvironmentMismatchDeniesChallengeExchangeAndSession() throws Exception {
    var pending = challenge(service(NOW), app, env, device);
    var session = exchange(service(NOW), app, env, client, device);
    policies = (application, environment) -> activePolicy(UUID.randomUUID(), environment);

    assertThatThrownBy(() -> challenge(service(NOW), app, env, device))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW).exchange(pending.challengeId(), provider(pending, client),
        gameTicket(pending, app, env, subjectHash()), enroll(device, pending)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW).session(session.accessToken(), sessionProof(device, session)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_sessions")).isEqualTo(1);
    assertThat(count("sdk_identities")).isEqualTo(1);

    policies = (application, environment) -> activePolicy(application, UUID.randomUUID());
    assertThatThrownBy(() -> challenge(service(NOW), app, env, device))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void exchangeReResolvesRegistryPolicyAfterProofVerificationAndBeforeWrites() throws Exception {
    var pending = challenge(service(NOW), app, env, device);
    var resolutions = new java.util.concurrent.atomic.AtomicInteger();
    policies = (application, environment) -> {
      if (resolutions.incrementAndGet() > 1) throw new SdkIdentityDeniedException();
      return activePolicy(application, environment);
    };

    assertThatThrownBy(() -> service(NOW).exchange(pending.challengeId(), provider(pending, client),
        gameTicket(pending, app, env, subjectHash()), enroll(device, pending)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(resolutions).hasValue(2);
    assertThat(count("sdk_identities")).isZero();
    assertThat(count("sdk_devices")).isZero();
    assertThat(count("sdk_sessions")).isZero();
    assertThat(jdbc.queryForObject("SELECT consumed_at IS NULL FROM sdk_challenges WHERE challenge_id=:id",
        Map.of("id", pending.challengeId()), Boolean.class)).isTrue();
  }

  @Test
  void challengeExchangeAndSessionResolveTheCurrentPolicyRevisionEachTime() throws Exception {
    var revisions = new java.util.concurrent.CopyOnWriteArrayList<Long>();
    var googleEnabled = new java.util.concurrent.atomic.AtomicBoolean(true);
    policies = (application, environment) -> {
      long revision = revisions.size() + 1L;
      revisions.add(revision);
      return new SdkAuthorizationPolicy.Policy(application, environment, revision, "Example Game",
          java.util.Set.of("voicegame://auth/callback"), java.util.Set.of("game.identity.read"),
          googleEnabled.get() ? java.util.Set.of("google") : java.util.Set.of("apple"));
    };

    var service = service(NOW);
    var challenge = challenge(service, app, env, device);
    googleEnabled.set(false);
    assertThatThrownBy(() -> service.exchange(challenge.challengeId(), provider(challenge, client),
        gameTicket(challenge, app, env, subjectHash()), enroll(device, challenge)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_identities")).isZero();
    assertThat(count("sdk_sessions")).isZero();

    googleEnabled.set(true);
    var session = service.exchange(challenge.challengeId(), provider(challenge, client),
        gameTicket(challenge, app, env, subjectHash()), enroll(device, challenge));
    googleEnabled.set(false);
    assertThatThrownBy(() -> service.session(session.accessToken(), sessionProof(device, session)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(revisions).containsExactly(1L, 2L, 3L, 4L, 5L, 6L);
  }

  @Test
  void possessionOfAnotherDeviceKeyOrTokenCannotReadSession() throws Exception {
    var first = exchange(service(NOW), app, env, client, device);
    var attacker = new ECKeyGenerator(Curve.P_256).generate();
    var second = exchange(service(NOW), app, env, client, attacker);
    assertThatThrownBy(() -> service(NOW).session(first.accessToken(), sessionProof(attacker, first)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> service(NOW).session(second.accessToken(), sessionProof(device, first)))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void identityCapRejectsNewSubjectButKeepsExistingIdentityAndOtherEnvironmentUsable() throws Exception {
    var first = exchange(service(NOW), app, env, client, device);
    jdbc.update("""
        INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,
          issuer,provider_subject,ownership_generation,status,created_at)
        SELECT gen_random_uuid(),gen_random_uuid(),:app,:env,:issuer,
          'seed-player-' || n,1,'active',:now FROM generate_series(1,999) AS n
        """, Map.of("app", app, "env", env, "issuer", ISSUER, "now", java.sql.Timestamp.from(NOW)));
    var challenge = challenge(service(NOW), app, env, device);
    String newSubjectProof = signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject("new-player")
        .audience(client).claim("nonce", challenge.nonce()).issueTime(Date.from(NOW))
        .expirationTime(Date.from(NOW.plusSeconds(300))).build(), googleKey);
    assertThatThrownBy(() -> service(NOW).exchange(challenge.challengeId(), newSubjectProof,
        gameTicket(challenge, app, env, sha256(ISSUER + "\nnew-player")), enroll(device, challenge)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(count("sdk_identities")).isEqualTo(1000);
    assertThat(exchange(service(NOW), app, env, client, device).accountId()).isEqualTo(first.accountId());
    UUID independentEnv = UUID.randomUUID();
    admit(app, independentEnv, "independent-client");
    assertThat(exchange(service(NOW), app, independentEnv, "independent-client", device).accountId())
        .isNotEqualTo(first.accountId());
  }

  @Test
  void tenActiveDeviceCapAllowsExistingDeviceAndFreesSlotAfterRevocation() throws Exception {
    var first = exchange(service(NOW), app, env, client, device);
    for (int i = 1; i < 10; i++) {
      exchange(service(NOW), app, env, client, new ECKeyGenerator(Curve.P_256).generate());
    }
    assertThat(count("sdk_devices")).isEqualTo(10);
    var replacement = new ECKeyGenerator(Curve.P_256).generate();
    assertThatThrownBy(() -> exchange(service(NOW), app, env, client, replacement))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(exchange(service(NOW), app, env, client, device).deviceId()).isEqualTo(first.deviceId());
    service(NOW).revoke(first.accessToken(), proof(device,
        "voice-sdk-revoke-v1\n" + sha256(first.accessToken())));
    assertThat(exchange(service(NOW), app, env, client, replacement).accountId()).isEqualTo(first.accountId());
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_devices WHERE revoked_at IS NULL",
        Map.of(), Long.class)).isEqualTo(10L);
    assertThat(count("sdk_devices")).isEqualTo(11);
  }

  @ParameterizedTest
  @ValueSource(strings = {"challenge", "provider", "game"})
  void exchangeRechecksFreshnessAfterWaitingForApplicationAdmissionLock(String expiring) throws Exception {
    var currentTime = new AtomicReference<>(NOW);
    var service = service(new MutableClock(currentTime, ZoneOffset.UTC));
    var challenge = challenge(service, app, env, device);
    // Both proofs remain fresh for the challenge-expiry case: iat is within the
    // allowed 30-second future skew initially and less than 300 seconds old later.
    String provider = signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT)
        .audience(client).claim("nonce", challenge.nonce()).issueTime(Date.from(NOW.plusSeconds(30)))
        .expirationTime(Date.from(NOW.plusSeconds(expiring.equals("provider") ? 60 : 600)))
        .build(), googleKey);
    String ticket = signed(new JWTClaimsSet.Builder().issuer("game:" + app + ":" + env)
        .subject("external-game-player").audience("voice:sdk-enroll")
        .claim("nonce", challenge.nonce()).claim("independent_subject_hash", subjectHash())
        .issueTime(Date.from(NOW.plusSeconds(30)))
        .expirationTime(Date.from(NOW.plusSeconds(expiring.equals("game") ? 60 : 600)))
        .build(), gameKey);
    String proof = enroll(device, challenge);
    var worker = Executors.newSingleThreadExecutor();
    try (var blocker = source().getConnection()) {
      blocker.setAutoCommit(false);
      int blockerPid;
      try (var statement = blocker.createStatement();
           var result = statement.executeQuery("SELECT pg_backend_pid()")) {
        assertThat(result.next()).isTrue();
        blockerPid = result.getInt(1);
      }
      try (var lock = blocker.prepareStatement(
          "SELECT pg_advisory_xact_lock(hashtextextended(?,0))")) {
        lock.setString(1, "voice-sdk:" + app + "/" + env);
        lock.execute();
      }
      var exchange = worker.submit(() -> service.exchange(challenge.challengeId(), provider, ticket, proof));
      try {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
        boolean waiting = false;
        while (System.nanoTime() < deadline && !exchange.isDone()) {
          waiting = Boolean.TRUE.equals(jdbc.queryForObject("""
              SELECT EXISTS(SELECT 1 FROM pg_stat_activity a
                WHERE a.datname=current_database() AND a.wait_event_type='Lock'
                  AND a.wait_event='advisory' AND :blocker=ANY(pg_blocking_pids(a.pid)))
              """, Map.of("blocker", blockerPid), Boolean.class));
          if (waiting) break;
          Thread.sleep(20);
        }
        assertThat(waiting).as("exchange reached the advisory lock while its proofs were valid").isTrue();
        currentTime.set(NOW.plusSeconds(expiring.equals("challenge") ? 301 : 61));
      } finally {
        blocker.rollback();
      }
      assertThatThrownBy(() -> exchange.get(10, TimeUnit.SECONDS))
          .isInstanceOf(ExecutionException.class).hasCauseInstanceOf(SdkIdentityDeniedException.class);
      assertThat(count("sdk_identities")).isZero();
      assertThat(count("sdk_devices")).isZero();
      assertThat(count("sdk_sessions")).isZero();
      assertThat(jdbc.queryForObject("SELECT consumed_at IS NULL FROM sdk_challenges WHERE challenge_id=:id",
          Map.of("id", challenge.challengeId()), Boolean.class)).isTrue();
    } finally {
      worker.shutdownNow();
      assertThat(worker.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void exchangeRechecksGoogleAdmissionAfterWaitingForApplicationAdmissionLock() throws Exception {
    var googleEnabled = new AtomicBoolean(true);
    policies = (application, environment) -> new SdkAuthorizationPolicy.Policy(
        application, environment, 1, "Example Game", java.util.Set.of("voicegame://auth/callback"),
        java.util.Set.of("game.identity.read"),
        googleEnabled.get() ? java.util.Set.of("google") : java.util.Set.of("apple"));
    var service = service(NOW);
    var challenge = challenge(service, app, env, device);
    String provider = provider(challenge, client);
    String ticket = gameTicket(challenge, app, env, subjectHash());
    String proof = enroll(device, challenge);
    var worker = Executors.newSingleThreadExecutor();
    try (var blocker = source().getConnection()) {
      blocker.setAutoCommit(false);
      int blockerPid;
      try (var statement = blocker.createStatement();
           var result = statement.executeQuery("SELECT pg_backend_pid()")) {
        assertThat(result.next()).isTrue();
        blockerPid = result.getInt(1);
      }
      try (var lock = blocker.prepareStatement(
          "SELECT pg_advisory_xact_lock(hashtextextended(?,0))")) {
        lock.setString(1, "voice-sdk:" + app + "/" + env);
        lock.execute();
      }
      var exchange = worker.submit(() -> service.exchange(challenge.challengeId(), provider, ticket, proof));
      try {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
        boolean waiting = false;
        while (System.nanoTime() < deadline && !exchange.isDone()) {
          waiting = Boolean.TRUE.equals(jdbc.queryForObject("""
              SELECT EXISTS(SELECT 1 FROM pg_stat_activity a
                WHERE a.datname=current_database() AND a.wait_event_type='Lock'
                  AND a.wait_event='advisory' AND :blocker=ANY(pg_blocking_pids(a.pid)))
              """, Map.of("blocker", blockerPid), Boolean.class));
          if (waiting) break;
          Thread.sleep(20);
        }
        assertThat(waiting).as("exchange reached the app/env lock").isTrue();
        googleEnabled.set(false);
      } finally {
        blocker.rollback();
      }
      assertThatThrownBy(() -> exchange.get(10, TimeUnit.SECONDS))
          .isInstanceOf(ExecutionException.class).hasCauseInstanceOf(SdkIdentityDeniedException.class);
      assertThat(count("sdk_identities")).isZero();
      assertThat(count("sdk_devices")).isZero();
      assertThat(count("sdk_sessions")).isZero();
      assertThat(jdbc.queryForObject("SELECT consumed_at IS NULL FROM sdk_challenges WHERE challenge_id=:id",
          Map.of("id", challenge.challengeId()), Boolean.class)).isTrue();
    } finally {
      worker.shutdownNow();
      assertThat(worker.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  private static final class MutableClock extends Clock {
    private final AtomicReference<Instant> now;
    private final ZoneId zone;

    private MutableClock(AtomicReference<Instant> now, ZoneId zone) {
      this.now = now;
      this.zone = zone;
    }

    @Override public ZoneId getZone() { return zone; }
    @Override public Clock withZone(ZoneId nextZone) { return new MutableClock(now, nextZone); }
    @Override public Instant instant() { return now.get(); }
  }

  private DriverManagerDataSource source() {
    return new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
  }

  private SdkIdentityService service(Instant now) {
    return service(Clock.fixed(now, ZoneOffset.UTC));
  }

  private SdkIdentityService service(Clock clock) {
    var source = source();
    return new SdkIdentityService(new NamedParameterJdbcTemplate(source),
        new TransactionTemplate(new DataSourceTransactionManager(source)),
        new GoogleOidcProofVerifier(clock,
            kid -> googleKey.getKeyID().equals(kid) ? googleKey.toPublicJWK() : null),
        applications, policies, clock);
  }

  private SdkIdentityService service(Clock clock, SdkDeviceStatusIssuer signer, SdkBindingAuthority binding) {
    var dataSource = jdbc.getJdbcTemplate().getDataSource();
    return new SdkIdentityService(jdbc,
        new TransactionTemplate(new DataSourceTransactionManager(dataSource)),
        new GoogleOidcProofVerifier(clock,
            kid -> googleKey.getKeyID().equals(kid) ? googleKey.toPublicJWK() : null),
        applications, policies, clock, signer, binding);
  }

  private BindingGrantFixture installActiveBindingGrant(SdkIdentityService.Session session) throws Exception {
    UUID bindingId = UUID.randomUUID();
    UUID requestId = UUID.randomUUID();
    UUID challengeId = UUID.randomUUID();
    UUID bindingOperationId = UUID.randomUUID();
    UUID targetAccountId = UUID.randomUUID();
    UUID targetProfileId = UUID.randomUUID();
    Instant expiry = NOW.plusSeconds(3600);
    jdbc.getJdbcTemplate().update("INSERT INTO accounts(id,password_hash,type,status) VALUES (?,'synthetic','regular','active')",
        targetAccountId);
    jdbc.update("""
        INSERT INTO sdk_authorizations(request_id,source_account_id,device_id,source_session_hash,source_generation,
          application_id,environment_id,idempotency_key,request_hash,redirect_uri,code_challenge,client_state,scopes,
          policy_revision,display_name,expires_at,target_account_id,target_profile_id,target_epoch,profile_revision,
          consumed_at,game_binding_id,game_binding_status,game_binding_authority_revision,game_binding_challenge_id,
          game_binding_operation_id,game_binding_intent,game_binding_challenge_nonce)
        VALUES(:request,:source,:device,:sessionHash,1,:app,:env,:idempotency,repeat('a',64),
          'https://voice.test/callback',repeat('b',43),'test','game.chat.send',1,'test game',:expires,
          :targetAccount,:targetProfile,1,1,:now,:binding,'active',1,:challenge,:operation,true,repeat('c',43))
        """, Map.ofEntries(Map.entry("request", requestId), Map.entry("source", session.accountId()),
            Map.entry("device", session.deviceId()), Map.entry("sessionHash", sha256(session.accessToken())),
            Map.entry("app", app), Map.entry("env", env), Map.entry("idempotency", UUID.randomUUID()),
            Map.entry("expires", java.sql.Timestamp.from(expiry)), Map.entry("targetAccount", targetAccountId),
            Map.entry("targetProfile", targetProfileId), Map.entry("now", java.sql.Timestamp.from(NOW)),
            Map.entry("binding", bindingId), Map.entry("challenge", challengeId), Map.entry("operation", bindingOperationId)));
    String linkedHash = sha256("linked-session:" + requestId);
    jdbc.update("INSERT INTO sdk_linked_sessions(token_hash,request_id,expires_at) VALUES(:hash,:request,:expires)",
        Map.of("hash", linkedHash, "request", requestId, "expires", java.sql.Timestamp.from(expiry)));
    Long consentRevision = jdbc.queryForObject("SELECT consent_revision FROM sdk_linked_sessions WHERE request_id=:request",
        Map.of("request", requestId), Long.class);
    jdbc.update("UPDATE sdk_authorizations SET game_binding_consent_revision=:consent WHERE request_id=:request",
        Map.of("consent", consentRevision, "request", requestId));
    jdbc.update("""
        INSERT INTO sdk_game_message_grants(grant_id,authorization_request_id,application_id,environment_id,
          target_account_id,target_profile_id,target_epoch,binding_id,consent_revision,scopes,policy_revision,
          profile_revision,status,authority_revision,created_at,updated_at)
        VALUES(:grant,:request,:app,:env,:targetAccount,:targetProfile,1,:binding,:consent,'game.chat.send',1,1,
          'active',1,:now,:now)
        """, Map.of("grant", UUID.randomUUID(), "request", requestId, "app", app, "env", env,
            "targetAccount", targetAccountId, "targetProfile", targetProfileId, "binding", bindingId,
            "consent", consentRevision, "now", java.sql.Timestamp.from(NOW)));
    return new BindingGrantFixture(bindingId, bindingOperationId, session.accountId(), session.deviceId(),
        session.actorId(), app, env);
  }

  private SdkIdentityService productionAuthorityService(AuthorityHttpFixture gis) {
    return productionAuthorityService(gis, new java.util.concurrent.atomic.AtomicInteger(),
        new java.util.concurrent.atomic.AtomicReference<>());
  }

  private SdkIdentityService productionAuthorityService(AuthorityHttpFixture gis,
      java.util.concurrent.atomic.AtomicInteger issueBackendPid,
      java.util.concurrent.atomic.AtomicReference<Long> issueTransactionId) {
    String key = Base64.getEncoder().encodeToString(AuthorityHttpFixture.WORKLOAD_KEY);
    var client = new SdkGameIntegrationPolicyClient(gis.baseUrl(), key, true,
        Clock.fixed(NOW, ZoneOffset.UTC), HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build(),
        Duration.ofSeconds(30));
    return service(Clock.fixed(NOW, ZoneOffset.UTC), claims -> "binding-authority-assertion",
        (application, environment, account, deviceId) -> {
          issueBackendPid.set(jdbc.getJdbcTemplate().queryForObject("SELECT pg_backend_pid()", Integer.class));
          issueTransactionId.set(jdbc.getJdbcTemplate().queryForObject("SELECT txid_current()", Long.class));
          return new JdbcSdkBindingAuthority(jdbc, client)
              .currentBinding(application, environment, account, deviceId);
        });
  }

  private AuthGameBindingHandoffService handoffService(TransactionTemplate tx) {
    return new AuthGameBindingHandoffService(jdbc, tx, null, null, null, null, null,
        Clock.fixed(NOW, ZoneOffset.UTC));
  }

  private TransactionTemplate transactionTemplate() {
    return new TransactionTemplate(new DataSourceTransactionManager(jdbc.getJdbcTemplate().getDataSource()));
  }

  private AuthorityHttpFixture authorityServer(BindingGrantFixture binding, boolean blockResponse) throws IOException {
    return new AuthorityHttpFixture(binding, blockResponse);
  }

  private static void awaitGate(CountDownLatch gate, String message) {
    try {
      if (!gate.await(10, TimeUnit.SECONDS)) throw new IllegalStateException(message);
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw new IllegalStateException(message, interrupted);
    }
  }

  private DatabaseLockWait awaitDatabaseLockWait(int expectedWaitingPid, int expectedBlockingPid,
      String waitingQueryPattern, String blockingQueryPattern) {
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
    List<Map<String, Object>> observed = List.of();
    while (System.nanoTime() < deadline) {
      observed = jdbc.queryForList("""
          SELECT waiting.pid AS waiting_pid, blocking.pid AS blocking_pid,
                 waiting.wait_event_type, waiting.wait_event, waiting.query AS waiting_query,
                 blocking.state AS blocking_state, blocking.query AS blocking_query
          FROM pg_stat_activity waiting
          CROSS JOIN LATERAL unnest(pg_blocking_pids(waiting.pid)) AS blockers(pid)
          JOIN pg_stat_activity blocking ON blocking.pid=blockers.pid
          WHERE waiting.datname=current_database() AND waiting.wait_event_type='Lock'
          """, Map.of());
      List<Map<String, Object>> waits = jdbc.queryForList("""
          SELECT waiting.pid AS waiting_pid, blocking.pid AS blocking_pid
          FROM pg_stat_activity waiting
          CROSS JOIN LATERAL unnest(pg_blocking_pids(waiting.pid)) AS blockers(pid)
          JOIN pg_stat_activity blocking ON blocking.pid=blockers.pid
          WHERE waiting.datname=current_database() AND waiting.wait_event_type='Lock'
            AND lower(waiting.query) LIKE :waitingQueryPattern
            AND lower(blocking.query) LIKE :blockingQueryPattern
            AND (:waitingPid=0 OR waiting.pid=:waitingPid)
            AND (:blockerPid=0 OR blocking.pid=:blockerPid)
          """, Map.of("waitingQueryPattern", waitingQueryPattern,
              "blockingQueryPattern", blockingQueryPattern,
              "waitingPid", expectedWaitingPid, "blockerPid", expectedBlockingPid));
      for (Map<String, Object> wait : waits) {
        int blocker = ((Number) wait.get("blocking_pid")).intValue();
        if (expectedBlockingPid == 0 || expectedBlockingPid == blocker) {
          return new DatabaseLockWait(((Number) wait.get("waiting_pid")).intValue(), blocker);
        }
      }
      try { Thread.sleep(20); }
      catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        throw new AssertionError("interrupted while observing PostgreSQL lock wait", interrupted);
      }
    }
    throw new AssertionError("competing Auth transaction never reached the expected PostgreSQL lock wait; observed="
        + observed);
  }

  private record DatabaseLockWait(int waitingPid, int blockingPid) {}

  private record BindingGrantFixture(UUID bindingId, UUID bindingOperationId, UUID accountId, UUID deviceId,
      UUID actorId, UUID applicationId, UUID environmentId) {}

  private static final class AuthorityHttpFixture implements AutoCloseable {
    private static final byte[] WORKLOAD_KEY = "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII);
    private final HttpServer server;
    private final BindingGrantFixture binding;
    private final CountDownLatch requestSeen = new CountDownLatch(1);
    private final CountDownLatch releaseResponse = new CountDownLatch(1);
    private final java.util.concurrent.atomic.AtomicInteger requestCount = new java.util.concurrent.atomic.AtomicInteger();
    private final AtomicBoolean holdingResponse = new AtomicBoolean();

    private AuthorityHttpFixture(BindingGrantFixture binding, boolean blockResponse) throws IOException {
      this.binding = binding;
      this.server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
      this.server.createContext("/internal/v1/bindings/", this::handle);
      this.server.start();
      if (!blockResponse) releaseResponse.countDown();
    }

    String baseUrl() { return "http://127.0.0.1:" + server.getAddress().getPort(); }
    CountDownLatch requestSeen() { return requestSeen; }
    int requests() { return requestCount.get(); }
    void releaseResponse() { releaseResponse.countDown(); }
    boolean isHoldingResponse() { return holdingResponse.get(); }

    private void handle(HttpExchange exchange) throws IOException {
      requestCount.incrementAndGet();
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      String requestSignature = exchange.getRequestHeaders().getFirst("X-Voice-Signature");
      if (!"GET".equals(exchange.getRequestMethod())
          || !("auth".equals(exchange.getRequestHeaders().getFirst("X-Voice-Workload")))
          || !path.equals("/internal/v1/bindings/" + binding.bindingId() + "/authority")
          || timestamp == null || nonce == null || !constantTimeEquals(requestSignature,
              workloadRequestSignature(path, timestamp, nonce))) {
        exchange.sendResponseHeaders(401, -1);
        exchange.close();
        return;
      }
      holdingResponse.set(true);
      requestSeen.countDown();
      try {
        if (!releaseResponse.await(30, TimeUnit.SECONDS)) {
          exchange.sendResponseHeaders(503, -1);
          exchange.close();
          return;
        }
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        exchange.sendResponseHeaders(503, -1);
        exchange.close();
        return;
      } finally {
        holdingResponse.set(false);
      }
      byte[] body = ("{\"application_id\":\"" + binding.applicationId() + "\",\"environment_id\":\""
          + binding.environmentId() + "\",\"binding_id\":\"" + binding.bindingId()
          + "\",\"status\":\"active\",\"binding_revision\":1,\"character_context\":[]}\n")
          .getBytes(StandardCharsets.UTF_8);
      exchange.getResponseHeaders().set("Cache-Control", "no-store");
      exchange.getResponseHeaders().set("Content-Type", "application/json");
      exchange.getResponseHeaders().set("X-Voice-Response-Timestamp", timestamp);
      exchange.getResponseHeaders().set("X-Voice-Response-Nonce", nonce);
      exchange.getResponseHeaders().set("X-Voice-Response-Signature", responseSignature(path, timestamp, nonce, body));
      exchange.sendResponseHeaders(200, body.length);
      exchange.getResponseBody().write(body);
      exchange.close();
    }

    @Override public void close() { releaseResponse.countDown(); server.stop(0); }

    private static String workloadRequestSignature(String path, String timestamp, String nonce) {
      String emptyDigest = HexFormat.of().formatHex(digest(new byte[0]));
      return hmac("v1\nGET\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + emptyDigest);
    }

    private static String responseSignature(String path, String timestamp, String nonce, byte[] body) {
      String digest = HexFormat.of().formatHex(digest(body));
      return hmac("v1\n200\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + digest);
    }

    private static String hmac(String message) {
      try {
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(WORKLOAD_KEY, "HmacSHA256"));
        return Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(message.getBytes(StandardCharsets.UTF_8)));
      } catch (Exception failure) { throw new IllegalStateException(failure); }
    }

    private static byte[] digest(byte[] value) {
      try { return MessageDigest.getInstance("SHA-256").digest(value); }
      catch (Exception failure) { throw new IllegalStateException(failure); }
    }

    private static boolean constantTimeEquals(String left, String right) {
      return left != null && MessageDigest.isEqual(left.getBytes(StandardCharsets.US_ASCII), right.getBytes(StandardCharsets.US_ASCII));
    }
  }

  private static SdkAuthorizationPolicy.Policy activePolicy(UUID application, UUID environment) {
    return activePolicy(application, environment, 1);
  }

  private static SdkAuthorizationPolicy.Policy activePolicy(UUID application, UUID environment, long revision) {
    return new SdkAuthorizationPolicy.Policy(application, environment, revision, "Example Game",
        java.util.Set.of("voicegame://auth/callback"), java.util.Set.of("game.identity.read"),
        java.util.Set.of("google"));
  }

  private void admit(UUID application, UUID environment, String audience) {
    applications.put(application + "/" + environment,
        new SdkApplication(application, environment, audience, gameKey.toPublicJWK()));
  }

  private long count(String table) {
    return jdbc.getJdbcTemplate().queryForObject("SELECT count(*) FROM " + table, Long.class);
  }

  private SdkIdentityService.Challenge challenge(SdkIdentityService service, UUID application,
      UUID environment, ECKey key) {
    return service.challenge(application, environment, key.toPublicJWK().toJSONString());
  }

  private SdkIdentityService.Session exchange(SdkIdentityService service, UUID application,
      UUID environment, String audience, ECKey key) throws Exception {
    var challenge = challenge(service, application, environment, key);
    return service.exchange(challenge.challengeId(), provider(challenge, audience),
        gameTicket(challenge, application, environment, subjectHash()), enroll(key, challenge));
  }

  private String provider(SdkIdentityService.Challenge challenge, String audience) throws Exception {
    return signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT).audience(audience)
        .claim("nonce", challenge.nonce()).issueTime(Date.from(NOW))
        .expirationTime(Date.from(NOW.plusSeconds(300))).build(), googleKey);
  }

  private SdkIdentityService.Session exchangeWithProviderClaims(String subject, String email, String name)
      throws Exception {
    var service = service(NOW);
    var challenge = challenge(service, app, env, device);
    String provider = signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject(subject).audience(client)
        .claim("nonce", challenge.nonce()).claim("email", email).claim("email_verified", true)
        .claim("name", name).claim("picture", "https://images.example.test/avatar.png")
        .issueTime(Date.from(NOW)).expirationTime(Date.from(NOW.plusSeconds(300))).build(), googleKey);
    return service.exchange(challenge.challengeId(), provider,
        gameTicket(challenge, app, env, subjectHashFor(subject)), enroll(device, challenge));
  }

  private String providerAt(SdkIdentityService.Challenge challenge, String audience, Instant now) throws Exception {
    return signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject(SUBJECT).audience(audience)
        .claim("nonce", challenge.nonce()).issueTime(Date.from(now))
        .expirationTime(Date.from(now.plusSeconds(300))).build(), googleKey);
  }

  private String gameTicket(SdkIdentityService.Challenge challenge, UUID application,
      UUID environment, String subjectHash) throws Exception {
    return signed(new JWTClaimsSet.Builder().issuer("game:" + application + ":" + environment)
        .subject("external-game-player").audience("voice:sdk-enroll")
        .claim("nonce", challenge.nonce()).claim("independent_subject_hash", subjectHash)
        .issueTime(Date.from(NOW)).expirationTime(Date.from(NOW.plusSeconds(300))).build(), gameKey);
  }

  private String gameTicketAt(SdkIdentityService.Challenge challenge, UUID application,
      UUID environment, String subjectHash, Instant now) throws Exception {
    return signed(new JWTClaimsSet.Builder().issuer("game:" + application + ":" + environment)
        .subject("external-game-player").audience("voice:sdk-enroll")
        .claim("nonce", challenge.nonce()).claim("independent_subject_hash", subjectHash)
        .issueTime(Date.from(now)).expirationTime(Date.from(now.plusSeconds(300))).build(), gameKey);
  }

  private String signed(JWTClaimsSet claims, RSAKey key) throws Exception {
    var jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(key.getKeyID()).build(), claims);
    jwt.sign(new RSASSASigner(key));
    return jwt.serialize();
  }

  private String enroll(ECKey key, SdkIdentityService.Challenge challenge) throws Exception {
    return proof(key, "voice-sdk-enroll-v1\n" + challenge.challengeId() + "\n" + challenge.nonce());
  }

  private String sessionProof(ECKey key, SdkIdentityService.Session session) throws Exception {
    return proof(key, "voice-sdk-session-v1\n" + sha256(session.accessToken()));
  }

  private String proof(ECKey key, String payload) throws Exception {
    var jws = new JWSObject(new JWSHeader(JWSAlgorithm.ES256), new Payload(payload));
    jws.sign(new ECDSASigner(key));
    return jws.serialize();
  }

  private String lifecycleProof(ECKey key, UUID kid, Map<String, Object> claims) throws Exception {
    var builder = new JWSHeader.Builder(JWSAlgorithm.ES256)
        .type(new JOSEObjectType("voice.game-device-key-proof+jws"));
    if (kid != null) builder.keyID(kid.toString());
    var canonical = new java.util.TreeMap<>(claims);
    var jws = new JWSObject(builder.build(), new Payload(
        com.nimbusds.jose.util.JSONObjectUtils.toJSONString(canonical)));
    jws.sign(new ECDSASigner(key));
    return jws.serialize();
  }

  private String deviceAuthorityProof(ECKey key, SdkIdentityService.Session session, UUID requestId,
      Instant now) throws Exception {
    var claims = new java.util.TreeMap<String, Object>();
    claims.put("version", 1L);
    claims.put("audience", "voice.game-message");
    claims.put("request_id", requestId.toString());
    claims.put("application_id", session.applicationId().toString());
    claims.put("environment_id", session.environmentId().toString());
    claims.put("device_id", session.deviceId().toString());
    claims.put("issued_at", now.toEpochMilli());
    var jws = new JWSObject(new JWSHeader.Builder(JWSAlgorithm.ES256)
        .keyID(session.keyId().toString()).type(new JOSEObjectType("voice.game-device-authority-request+jws"))
        .build(), new Payload(com.nimbusds.jose.util.JSONObjectUtils.toJSONString(claims)));
    jws.sign(new ECDSASigner(key));
    return jws.serialize();
  }

  private Map<String, Object> lifecycleClaims(String purpose, UUID requestId,
      SdkIdentityService.Challenge challenge, UUID deviceId, UUID keyId, String keyThumbprint,
      String newKeyThumbprint, Instant now) {
    var claims = new java.util.TreeMap<String, Object>();
    claims.put("version", 1L);
    claims.put("purpose", purpose);
    claims.put("audience", "voice.auth.device-key");
    claims.put("request_id", requestId.toString());
    claims.put("challenge_id", challenge.challengeId().toString());
    claims.put("nonce", challenge.nonce());
    claims.put("application_id", app.toString());
    claims.put("environment_id", env.toString());
    if ("recover_new".equals(purpose)) claims.put("replaces_device_id", challenge.replacesDeviceId().toString());
    else claims.put("device_id", deviceId.toString());
    if (keyId != null) claims.put("key_id", keyId.toString());
    claims.put("key_thumbprint", keyThumbprint);
    if ("rotate_current".equals(purpose)) claims.put("new_key_thumbprint", newKeyThumbprint);
    claims.put("issued_at", now.toEpochMilli());
    return claims;
  }

  private String subjectHash() throws Exception {
    return subjectHashFor(SUBJECT);
  }

  private String subjectHashFor(String subject) throws Exception {
    return sha256(ISSUER + "\n" + subject);
  }

  private String sha256(String value) throws Exception {
    return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest(value.getBytes(StandardCharsets.UTF_8)));
  }
}
