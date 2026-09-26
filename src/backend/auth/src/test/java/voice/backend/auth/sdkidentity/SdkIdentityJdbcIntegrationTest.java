package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.JWSObject;
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
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.Date;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
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
    jdbc.getJdbcTemplate().execute("TRUNCATE sdk_sessions,sdk_devices,sdk_challenges,sdk_identities");
    applications = new HashMap<>();
    admit(app, env, client);
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
    assertThat(session.applicationId()).isEqualTo(app);
    assertThat(session.environmentId()).isEqualTo(env);
    assertThat(session.gameSubject()).isEqualTo("external-game-player");
    assertThat(session.accountType()).isEqualTo("sdk-account");
    assertThat(session.accessToken()).matches("[A-Za-z0-9_-]{43}");
    assertThat(session.expiresAt()).isEqualTo(NOW.plusSeconds(300));
    assertThat(count("accounts")).isZero();
    assertThat(count("sdk_identities")).isEqualTo(1);
    assertThat(count("sdk_devices")).isEqualTo(1);
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
        applications, clock);
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

  private String gameTicket(SdkIdentityService.Challenge challenge, UUID application,
      UUID environment, String subjectHash) throws Exception {
    return signed(new JWTClaimsSet.Builder().issuer("game:" + application + ":" + environment)
        .subject("external-game-player").audience("voice:sdk-enroll")
        .claim("nonce", challenge.nonce()).claim("independent_subject_hash", subjectHash)
        .issueTime(Date.from(NOW)).expirationTime(Date.from(NOW.plusSeconds(300))).build(), gameKey);
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

  private String subjectHash() throws Exception {
    return sha256(ISSUER + "\n" + SUBJECT);
  }

  private String sha256(String value) throws Exception {
    return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest(value.getBytes(StandardCharsets.UTF_8)));
  }
}
