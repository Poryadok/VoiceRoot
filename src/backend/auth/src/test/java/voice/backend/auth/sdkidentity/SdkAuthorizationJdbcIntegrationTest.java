package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

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
import java.util.Base64;
import java.util.Date;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Set;
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
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.TokenClaims;
import voice.backend.auth.sessionepoch.PreparedSessionEpoch;

/** Real SDK source proofs and PostgreSQL; explicit policy/User fixtures own consent eligibility. */
@Testcontainers(disabledWithoutDocker = true)
class SdkAuthorizationJdbcIntegrationTest {
  private static final Instant NOW = Instant.parse("2026-09-26T12:00:00Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);
  private static final String ISSUER = "https://accounts.google.com";
  private static final String REDIRECT = "https://game.example.test/voice/callback";
  private static final String VERIFIER = "correct-pkce-verifier-" + "a".repeat(43);
  private static final String STATE = "s".repeat(43);
  private static final String VOICE_BEARER = "Bearer current-voice-access-token";
  private static final String APPROVAL_JTI = "approval-jti";
  private static final Set<String> SCOPES = Set.of("game.voice.join", "game.chat.read");
  private static RSAKey googleKey;
  private static RSAKey gameKey;

  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice")
          .withLabel("voice.task", "GAME-AUTH-02").withReuse(false);

  private final UUID app = UUID.randomUUID();
  private final UUID env = UUID.randomUUID();
  private final UUID targetAccount = UUID.randomUUID();
  private final UUID primaryProfile = UUID.randomUUID();
  private final UUID secondaryProfile = UUID.randomUUID();
  private final String client = "operator-owned-google-client";
  private final AuthService auth = mock(AuthService.class);
  private final TokenBlacklist blacklist = mock(TokenBlacklist.class);
  private final AtomicReference<SdkAuthorizationPolicy.Policy> policy = new AtomicReference<>();
  private final AtomicReference<SdkProfileEligibility.Profile> profile = new AtomicReference<>();
  private NamedParameterJdbcTemplate jdbc;
  private DriverManagerDataSource database;
  private SdkIdentityService identity;
  private SdkIdentityService.Session source;
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
    database = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    jdbc = new NamedParameterJdbcTemplate(database);
    // Each test owns unique app/env and account IDs; retained rows do not cross those fences.
    jdbc.update("INSERT INTO accounts(id,password_hash,type,status,session_epoch) "
        + "VALUES (:id,'test-password-hash','regular','active',7)", Map.of("id", targetAccount));
    when(auth.validate(VOICE_BEARER)).thenReturn(new TokenClaims(targetAccount.toString(),
        primaryProfile.toString(), List.of("user"), "free", NOW.plusSeconds(600),
        APPROVAL_JTI, "regular", 7));
    when(auth.prepareOAuthAccessToken(targetAccount.toString()))
        .thenReturn(new PreparedSessionEpoch(targetAccount, 7));
    policy.set(new SdkAuthorizationPolicy.Policy(app, env, 3, "Example Game", Set.of(REDIRECT), SCOPES));
    profile.set(new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 11, false, false));
    device = new ECKeyGenerator(Curve.P_256).generate();
    identity = identity();
    var challenge = identity.challenge(app, env, device.toPublicJWK().toJSONString());
    String provider = signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject("player").audience(client)
        .claim("nonce", challenge.nonce()).issueTime(Date.from(NOW))
        .expirationTime(Date.from(NOW.plusSeconds(300))).build(), googleKey);
    String ticket = signed(new JWTClaimsSet.Builder().issuer("game:" + app + ":" + env)
        .subject("game-player").audience("voice:sdk-enroll").claim("nonce", challenge.nonce())
        .claim("independent_subject_hash", hash(ISSUER + "\nplayer")).issueTime(Date.from(NOW))
        .expirationTime(Date.from(NOW.plusSeconds(300))).build(), gameKey);
    source = identity.exchange(challenge.challengeId(), provider, ticket,
        proof(device, "voice-sdk-enroll-v1\n" + challenge.challengeId() + "\n" + challenge.nonce()));
  }

  @Test
  void browserConsentPreservesExplicitSecondaryProfileAndIssuesOnlyLinkedBootstrap() throws Exception {
    var authorization = authorization();
    var request = start(authorization, UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    assertThat(request.requestId()).isNotNull();
    assertThat(request.policyRevision()).isEqualTo(3);
    assertThat(request.displayName()).isEqualTo("Example Game");
    assertThat(request.scopes()).containsExactlyInAnyOrderElementsOf(SCOPES);
    assertThat(request.expiresAt()).isEqualTo(source.expiresAt());
    var approval = authorization.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    assertThat(approval.code()).matches("[A-Za-z0-9_-]{43}");
    assertThat(approval.expiresAt()).isEqualTo(NOW.plusSeconds(60));
    assertThat(approval.redirectUri().getScheme()).isEqualTo("https");
    assertThat(approval.redirectUri().getHost()).isEqualTo("game.example.test");
    assertThat(approval.redirectUri().getPath()).isEqualTo("/voice/callback");
    assertThat(approval.redirectUri().getRawQuery()).contains("state=" + STATE, "code=" + approval.code());

    var linked = exchange(authorization, request.requestId(), approval.code(), REDIRECT, VERIFIER, device);
    assertThat(linked.sourceAccountId()).isEqualTo(source.accountId());
    assertThat(linked.accountId()).isEqualTo(targetAccount);
    assertThat(linked.profileId()).isEqualTo(secondaryProfile).isNotEqualTo(primaryProfile);
    assertThat(linked.deviceId()).isEqualTo(source.deviceId());
    assertThat(linked.applicationId()).isEqualTo(app);
    assertThat(linked.environmentId()).isEqualTo(env);
    assertThat(linked.scopes()).containsExactlyInAnyOrderElementsOf(SCOPES);
    assertThat(linked.consentRevision()).isPositive();
    assertThat(linked.policyRevision()).isEqualTo(3);
    assertThat(linked.accessToken()).matches("[A-Za-z0-9_-]{43}").isNotEqualTo(source.accessToken());
    assertThat(linked.expiresAt()).isEqualTo(NOW.plusSeconds(300));
    var checked = authorization().linkedSession(linked.accessToken(),
        proof(device, "voice-sdk-linked-v1\n" + hash(linked.accessToken())));
    assertThat(checked.profileId()).isEqualTo(secondaryProfile);
    assertThat(checked.accountId()).isEqualTo(targetAccount);
    assertThat(checked.sourceAccountId()).isEqualTo(source.accountId());
    assertThat(checked.accessToken()).isNull();
    verify(auth, never()).issueOAuthAccessToken(anyString(), anyString());
    verify(auth, never()).issueOAuthAccessToken(anyString(), anyString(), any(PreparedSessionEpoch.class));
  }

  @Test
  void wrongPkceDoesNotConsumeCodeAndCorrectVerifierWorksOnce() throws Exception {
    var authorization = authorization();
    var request = start(authorization, UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    assertThatThrownBy(() -> exchange(authorization, request.requestId(), approval.code(), REDIRECT,
        "wrong-verifier-" + "b".repeat(43), device)).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(exchange(authorization, request.requestId(), approval.code(), REDIRECT, VERIFIER, device)
        .profileId()).isEqualTo(secondaryProfile);
    assertThatThrownBy(() -> exchange(authorization, request.requestId(), approval.code(), REDIRECT,
        VERIFIER, device)).isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void sameIdempotencyKeyAndBodyReturnOriginalRequestButChangedBodyConflicts() throws Exception {
    UUID key = UUID.randomUUID();
    var first = start(authorization(), key, REDIRECT, STATE, SCOPES);
    var retry = start(authorization(), key, REDIRECT, STATE, SCOPES);
    assertThat(retry).isEqualTo(first);
    assertThatThrownBy(() -> start(authorization(), key, REDIRECT, "z".repeat(43), SCOPES))
        .isInstanceOf(SdkAuthorizationConflictException.class);
  }

  @Test
  void authenticatedVoiceUiReadsAuthoritativeConsentContextWithoutIssuingCode() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var view = authorization().inspect(request.requestId(), VOICE_BEARER);
    assertThat(view.requestId()).isEqualTo(request.requestId());
    assertThat(view.applicationId()).isEqualTo(app);
    assertThat(view.environmentId()).isEqualTo(env);
    assertThat(view.displayName()).isEqualTo("Example Game");
    assertThat(view.scopes()).containsExactlyInAnyOrderElementsOf(SCOPES);
    assertThat(view.gameSubject()).isEqualTo("game-player");
    assertThat(view.policyRevision()).isEqualTo(3);
    assertThat(view.expiresAt()).isEqualTo(request.expiresAt());
    assertThat(authorization().inspect(request.requestId(), VOICE_BEARER)).isEqualTo(view);
    // Inspect is read-only: the first explicit consent can still issue its code.
    assertThat(authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3).code())
        .matches("[A-Za-z0-9_-]{43}");
    verify(auth, never()).issueOAuthAccessToken(anyString(), anyString());
    verify(auth, never()).issueOAuthAccessToken(anyString(), anyString(), any(PreparedSessionEpoch.class));
  }

  @Test
  void consentInspectionChecksCurrentRegularAccountInsteadOfStaleRegularClaims() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    jdbc.update("UPDATE accounts SET type='guest' WHERE id=:id", Map.of("id", targetAccount));
    assertThatThrownBy(() -> authorization().inspect(request.requestId(), VOICE_BEARER))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void concurrentCodeExchangeAcrossServiceInstancesHasExactlyOneWinner() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    var start = new CyclicBarrier(6);
    var workers = Executors.newFixedThreadPool(6);
    try {
      List<Callable<Boolean>> tasks = new ArrayList<>();
      for (int i = 0; i < 6; i++) {
        tasks.add(() -> {
          start.await(10, TimeUnit.SECONDS);
          try {
            exchange(authorization(), request.requestId(), approval.code(), REDIRECT, VERIFIER, device);
            return true;
          } catch (SdkIdentityDeniedException denied) {
            return false;
          }
        });
      }
      int winners = 0;
      for (var result : workers.invokeAll(tasks, 30, TimeUnit.SECONDS)) {
        assertThat(result.isCancelled()).isFalse();
        if (result.get()) winners++;
      }
      assertThat(winners).isEqualTo(1);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void redirectMustMatchExactlyAndUnknownScopesCannotBeRequested() throws Exception {
    assertThatThrownBy(() -> start(authorization(), UUID.randomUUID(), REDIRECT + "/", STATE, SCOPES))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> start(authorization(), UUID.randomUUID(), REDIRECT, STATE,
        Set.of("game.chat.read", "account.admin"))).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES).requestId()).isNotNull();
  }

  @ParameterizedTest
  @ValueSource(strings = {"missing", "wrong-app", "wrong-environment"})
  void unavailableOrMismatchedRegistryPolicyFailsClosed(String invalid) throws Exception {
    policy.set(switch (invalid) {
      case "wrong-app" -> new SdkAuthorizationPolicy.Policy(UUID.randomUUID(), env, 3,
          "Wrong app", Set.of(REDIRECT), SCOPES);
      case "wrong-environment" -> new SdkAuthorizationPolicy.Policy(app, UUID.randomUUID(), 3,
          "Wrong environment", Set.of(REDIRECT), SCOPES);
      default -> null;
    });
    assertThatThrownBy(() -> start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void authorizeProofBindsSourceTokenDeviceAndCompleteRequestBody() throws Exception {
    UUID key = UUID.randomUUID();
    String challenge = pkce(VERIFIER);
    String digest = hash("voice-sdk-authorization-request-v1\n" + key + "\n" + REDIRECT + "\n"
        + challenge + "\n" + STATE + "\n" + String.join(",", SCOPES.stream().sorted().toList()));
    String payload = "voice-sdk-authorize-v1\n" + hash(source.accessToken()) + "\n" + digest;
    var attacker = new ECKeyGenerator(Curve.P_256).generate();
    assertThatThrownBy(() -> authorization().start(source.accessToken(), proof(attacker, payload),
        key, REDIRECT, challenge, STATE, SCOPES)).isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> authorization().start(source.accessToken(), proof(device, payload),
        key, REDIRECT, challenge, "z".repeat(43), SCOPES)).isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> authorization().start(source.accessToken(), proof(device,
        "voice-sdk-authorize-v1\n" + hash("another-source-token") + "\n" + digest),
        key, REDIRECT, challenge, STATE, SCOPES)).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(start(authorization(), key, REDIRECT, STATE, SCOPES).requestId()).isNotNull();
  }

  @ParameterizedTest
  @ValueSource(strings = {"type='guest'", "status='suspended'", "status='deleted'",
      "regular_email_verification_pending=TRUE", "session_epoch=8"})
  void currentTargetDatabaseStateOverridesRegularClaimsAtApproval(String mutation) throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    jdbc.update("UPDATE accounts SET " + mutation + " WHERE id=:id", Map.of("id", targetAccount));
    assertThatThrownBy(() -> authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @ParameterizedTest
  @ValueSource(strings = {"wrong-owner", "wrong-profile", "deleted", "frozen"})
  void selectedProfileMustHaveExactOwnerAndCurrentEligibility(String invalid) throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    profile.set(new SdkProfileEligibility.Profile(
        invalid.equals("wrong-owner") ? UUID.randomUUID() : targetAccount,
        invalid.equals("wrong-profile") ? primaryProfile : secondaryProfile, 11,
        invalid.equals("deleted"), invalid.equals("frozen")));
    assertThatThrownBy(() -> authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void approvalRequiresExactPolicyRevisionAndCannotReissueLostCode() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    assertThatThrownBy(() -> authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 2))
        .isInstanceOf(SdkIdentityDeniedException.class);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    assertThatThrownBy(() -> authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(exchange(authorization(), request.requestId(), approval.code(), REDIRECT, VERIFIER, device)
        .profileId()).isEqualTo(secondaryProfile);
  }

  @ParameterizedTest
  @ValueSource(strings = {"source-revoked", "source-generation", "profile-frozen", "profile-revision", "policy-revision",
      "target-epoch", "logout", "target-guest", "target-suspended", "target-email-pending"})
  void exchangeRechecksCurrentAuthoritiesAfterBrowserApproval(String mutation) throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    changeAuthority(mutation);
    assertThatThrownBy(() -> exchange(authorization(), request.requestId(), approval.code(), REDIRECT,
        VERIFIER, device)).isInstanceOf(SdkIdentityDeniedException.class);
  }

  @ParameterizedTest
  @ValueSource(strings = {"source-revoked", "source-generation", "profile-frozen", "profile-revision",
      "policy-revision", "target-epoch", "logout"})
  void issuedLinkedCredentialImmediatelyObservesAuthorityRevocation(String mutation) throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    var linked = exchange(authorization(), request.requestId(), approval.code(), REDIRECT, VERIFIER, device);
    String linkedProof = proof(device, "voice-sdk-linked-v1\n" + hash(linked.accessToken()));
    assertThat(authorization().linkedSession(linked.accessToken(), linkedProof).profileId())
        .isEqualTo(secondaryProfile);
    changeAuthority(mutation);
    assertThatThrownBy(() -> authorization().linkedSession(linked.accessToken(), linkedProof))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void sourceBootstrapExpiryAloneDoesNotExpireAnOtherwiseCurrentLinkedCredential() throws Exception {
    var currentTime = new AtomicReference<>(NOW);
    var authorization = authorization(new MutableClock(currentTime, ZoneOffset.UTC));
    var request = start(authorization, UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    currentTime.set(NOW.plusSeconds(30));
    var linked = exchange(authorization, request.requestId(), approval.code(), REDIRECT, VERIFIER, device);
    assertThat(linked.expiresAt()).isEqualTo(NOW.plusSeconds(330));
    currentTime.set(NOW.plusSeconds(301));
    assertThat(source.expiresAt()).isBefore(currentTime.get());
    var checked = authorization.linkedSession(linked.accessToken(),
        proof(device, "voice-sdk-linked-v1\n" + hash(linked.accessToken())));
    assertThat(checked.profileId()).isEqualTo(secondaryProfile);
    assertThat(checked.expiresAt()).isEqualTo(NOW.plusSeconds(330));
    assertThat(checked.accessToken()).isNull();
  }

  @Test
  void codeExchangeStillRequiresExactRedirectAndEnrolledDevice() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    assertThatThrownBy(() -> exchange(authorization(), request.requestId(), approval.code(), REDIRECT + "/",
        VERIFIER, device)).isInstanceOf(SdkIdentityDeniedException.class);
    var attacker = new ECKeyGenerator(Curve.P_256).generate();
    assertThatThrownBy(() -> exchange(authorization(), request.requestId(), approval.code(), REDIRECT,
        VERIFIER, attacker)).isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(exchange(authorization(), request.requestId(), approval.code(), REDIRECT, VERIFIER, device)
        .profileId()).isEqualTo(secondaryProfile);
  }

  @Test
  void authorizationCodeAndBootstrapCredentialsPersistOnlyAsSha256Hashes() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    Map<String, Object> stored = jdbc.queryForMap(
        "SELECT code_hash,source_session_hash FROM sdk_authorizations WHERE request_id=:id",
        Map.of("id", request.requestId()));
    assertThat(stored.get("code_hash")).isEqualTo(hash(approval.code()));
    assertThat(stored.get("source_session_hash")).isEqualTo(hash(source.accessToken()));
    var linked = exchange(authorization(), request.requestId(), approval.code(), REDIRECT, VERIFIER, device);
    assertThat(jdbc.queryForObject("SELECT token_hash FROM sdk_linked_sessions WHERE request_id=:id",
        Map.of("id", request.requestId()), String.class)).isEqualTo(hash(linked.accessToken()));
    for (String table : List.of("sdk_authorizations", "sdk_linked_sessions")) {
      String row = jdbc.queryForObject("SELECT row_to_json(s)::text FROM " + table + " s WHERE request_id=:id",
          Map.of("id", request.requestId()), String.class);
      assertThat(row).doesNotContain(approval.code(), source.accessToken(), linked.accessToken(), VOICE_BEARER, VERIFIER);
    }
  }

  @Test
  void codeExpiryIsRecheckedAfterWaitingForTheAuthorizationRowLock() throws Exception {
    var currentTime = new AtomicReference<>(NOW);
    var authorization = authorization(new MutableClock(currentTime, ZoneOffset.UTC));
    var request = start(authorization, UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    var worker = Executors.newSingleThreadExecutor();
    try (var blocker = database.getConnection()) {
      blocker.setAutoCommit(false);
      int blockerPid;
      try (var statement = blocker.createStatement();
           var result = statement.executeQuery("SELECT pg_backend_pid()")) {
        assertThat(result.next()).isTrue();
        blockerPid = result.getInt(1);
      }
      try (var lock = blocker.prepareStatement("SELECT request_id FROM sdk_authorizations WHERE request_id=? FOR UPDATE")) {
        lock.setObject(1, request.requestId());
        lock.execute();
      }
      var exchange = worker.submit(() -> exchange(authorization, request.requestId(), approval.code(),
          REDIRECT, VERIFIER, device));
      try {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
        boolean waiting = false;
        while (System.nanoTime() < deadline && !exchange.isDone()) {
          waiting = Boolean.TRUE.equals(jdbc.queryForObject("""
              SELECT EXISTS(SELECT 1 FROM pg_stat_activity a
                WHERE a.datname=current_database() AND a.wait_event_type='Lock'
                  AND :blocker=ANY(pg_blocking_pids(a.pid)))
              """, Map.of("blocker", blockerPid), Boolean.class));
          if (waiting) break;
          Thread.sleep(20);
        }
        assertThat(waiting).as("exchange is blocked on the authorization while code is valid").isTrue();
        // Only code expires; source/request remain valid until +300 and Voice approval until +600.
        currentTime.set(NOW.plusSeconds(61));
      } finally {
        blocker.rollback();
      }
      assertThatThrownBy(() -> exchange.get(10, TimeUnit.SECONDS))
          .isInstanceOf(ExecutionException.class).hasCauseInstanceOf(SdkIdentityDeniedException.class);
      assertThat(jdbc.queryForObject("SELECT consumed_at IS NULL FROM sdk_authorizations WHERE request_id=:id",
          Map.of("id", request.requestId()), Boolean.class)).isTrue();
      assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_linked_sessions WHERE request_id=:id",
          Map.of("id", request.requestId()), Long.class)).isZero();
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

  private void changeAuthority(String mutation) throws Exception {
    switch (mutation) {
      case "source-revoked" -> identity.revoke(source.accessToken(),
          proof(device, "voice-sdk-revoke-v1\n" + hash(source.accessToken())));
      case "source-generation" -> jdbc.update(
          "UPDATE sdk_identities SET ownership_generation=ownership_generation+1 WHERE account_id=:id",
          Map.of("id", source.accountId()));
      case "profile-frozen" -> profile.set(
          new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 11, false, true));
      case "profile-revision" -> profile.set(
          new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 12, false, false));
      case "policy-revision" -> policy.set(
          new SdkAuthorizationPolicy.Policy(app, env, 4, "Example Game", Set.of(REDIRECT), SCOPES));
      case "target-epoch" -> {
        jdbc.update("UPDATE accounts SET session_epoch=8 WHERE id=:id", Map.of("id", targetAccount));
        when(auth.prepareOAuthAccessToken(targetAccount.toString()))
            .thenReturn(new PreparedSessionEpoch(targetAccount, 8));
      }
      case "logout" -> when(blacklist.isRevoked(APPROVAL_JTI)).thenReturn(true);
      case "target-guest" -> jdbc.update("UPDATE accounts SET type='guest' WHERE id=:id", Map.of("id", targetAccount));
      case "target-suspended" -> jdbc.update("UPDATE accounts SET status='suspended' WHERE id=:id", Map.of("id", targetAccount));
      case "target-email-pending" -> jdbc.update(
          "UPDATE accounts SET regular_email_verification_pending=TRUE WHERE id=:id", Map.of("id", targetAccount));
      default -> throw new AssertionError("Unknown authority mutation: " + mutation);
    }
  }

  private DriverManagerDataSource dataSource() {
    return database;
  }

  private SdkIdentityService identity() {
    return identity(CLOCK);
  }

  private SdkIdentityService identity(Clock clock) {
    var dataSource = dataSource();
    return new SdkIdentityService(new NamedParameterJdbcTemplate(dataSource),
        new TransactionTemplate(new DataSourceTransactionManager(dataSource)),
        new GoogleOidcProofVerifier(clock,
            kid -> googleKey.getKeyID().equals(kid) ? googleKey.toPublicJWK() : null),
        Map.of(app + "/" + env, new SdkApplication(app, env, client, gameKey.toPublicJWK())), clock);
  }

  private SdkAuthorizationService authorization() {
    return authorization(CLOCK);
  }

  private SdkAuthorizationService authorization(Clock clock) {
    var dataSource = dataSource();
    return new SdkAuthorizationService(new NamedParameterJdbcTemplate(dataSource),
        new TransactionTemplate(new DataSourceTransactionManager(dataSource)), identity(clock), auth,
        (application, environment) -> app.equals(application) && env.equals(environment) ? policy.get() : null,
        (account, selected) -> targetAccount.equals(account) && secondaryProfile.equals(selected) ? profile.get() : null,
        blacklist, clock);
  }

  private SdkAuthorizationService.AuthorizationRequest start(SdkAuthorizationService service, UUID key,
      String redirect, String state, Set<String> scopes) throws Exception {
    String challenge = pkce(VERIFIER);
    String digest = hash("voice-sdk-authorization-request-v1\n" + key + "\n" + redirect + "\n"
        + challenge + "\n" + state + "\n" + String.join(",", scopes.stream().sorted().toList()));
    return service.start(source.accessToken(),
        proof(device, "voice-sdk-authorize-v1\n" + hash(source.accessToken()) + "\n" + digest),
        key, redirect, challenge, state, scopes);
  }

  private SdkAuthorizationService.LinkedSession exchange(SdkAuthorizationService service, UUID request,
      String code, String redirect, String verifier, ECKey key) throws Exception {
    return service.exchange(request, code, redirect, verifier,
        proof(key, "voice-sdk-code-v1\n" + request + "\n" + hash(code) + "\n" + hash(verifier)));
  }

  private String signed(JWTClaimsSet claims, RSAKey key) throws Exception {
    var jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(key.getKeyID()).build(), claims);
    jwt.sign(new RSASSASigner(key));
    return jwt.serialize();
  }

  private String proof(ECKey key, String payload) throws Exception {
    var jws = new JWSObject(new JWSHeader(JWSAlgorithm.ES256), new Payload(payload));
    jws.sign(new ECDSASigner(key));
    return jws.serialize();
  }

  private String hash(String value) throws Exception {
    return HexFormat.of().formatHex(sha256(value));
  }

  private String pkce(String value) throws Exception {
    return Base64.getUrlEncoder().withoutPadding().encodeToString(sha256(value));
  }

  private byte[] sha256(String value) throws Exception {
    return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
  }
}
