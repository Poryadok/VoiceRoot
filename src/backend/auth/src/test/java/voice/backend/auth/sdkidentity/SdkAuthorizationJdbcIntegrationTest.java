package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.catchThrowable;
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
import java.sql.Timestamp;
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
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.nio.file.Files;
import java.nio.file.Path;
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
  private final AtomicReference<RuntimeException> profileFailure = new AtomicReference<>();
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
    policy.set(new SdkAuthorizationPolicy.Policy(app, env, 3, "Example Game", Set.of(REDIRECT), SCOPES,
        Set.of("google")));
    profile.set(new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 11, false, false));
    profileFailure.set(null);
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
  void bindingExchangeConsumesTheT14CodeOnceAndReplaysOnlyWithTheRegisteredDeviceProof() throws Exception {
    Set<String> executionScopes = new java.util.HashSet<>(SCOPES);
    executionScopes.add("game.chat.send");
    policy.set(new SdkAuthorizationPolicy.Policy(app, env, 3, "Example Game", Set.of(REDIRECT), executionScopes,
        Set.of("google")));
    UUID challengeId = UUID.randomUUID();
    UUID authorizationKey = UUID.randomUUID();
    UUID operationId = authorizationKey;
    AtomicReference<SdkBindingChallengeAuthority.Challenge> savedChallenge = new AtomicReference<>();
    var challengeAuthority = new SdkBindingChallengeAuthority() {
      @Override public Challenge resolveBindingChallenge(UUID ignored) {
        var result = savedChallenge.get();
        if (result == null || !result.challengeId().equals(ignored)) throw new SdkIdentityDeniedException();
        return result;
      }
      @Override public Challenge createBindingChallenge(CreateRequest input) {
        var result = new Challenge(challengeId, "n".repeat(43), input.applicationId(), input.environmentId(), input.provider(),
            input.redirectUriSha256(), input.pkceChallenge(), input.deviceKeyId(), input.deviceKeyThumbprint(),
            input.operationId(), input.expiresAt(), "pending", input.sourceAccountId(), input.sourceActorId(),
            input.sourceDeviceId(), input.sourceGeneration(), input.targetAccountId(), input.targetProfileId(),
            input.profileRevision(), input.consentRevision(), input.policyRevision(), input.scopes());
        savedChallenge.set(result);
        return result;
      }
    };
    var executionClockNow = new AtomicReference<>(NOW);
    var executionClock = new MutableClock(executionClockNow, ZoneOffset.UTC);
    var issuer = principalIssuer(executionClock);
    var service = authorizationForBinding(CLOCK, challengeAuthority, issuer);
    String codeChallenge = pkce(VERIFIER);
    String requestHash = hash("voice-sdk-authorization-request-v1\n" + authorizationKey + "\n" + REDIRECT
        + "\n" + codeChallenge + "\n" + STATE + "\n" + String.join(",", executionScopes.stream().sorted().toList())
        + "\n" + authorizationKey);
    var request = service.start(source.accessToken(), proof(device,
        "voice-sdk-authorize-v1\n" + hash(source.accessToken()) + "\n" + requestHash), authorizationKey, REDIRECT,
        codeChallenge, STATE, executionScopes, true);
    var approval = service.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    assertThat(approval.gameBindingChallengeId()).isEqualTo(challengeId);
    assertThat(approval.gameBindingNonce()).isEqualTo("n".repeat(43));
    var approvalRetry = service.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    assertThat(approvalRetry).isEqualTo(approval).as("lost HTTP response retry returns the exact code/challenge receipt");
    assertThatThrownBy(() -> service.approve(request.requestId(), VOICE_BEARER, primaryProfile, 3))
        .isInstanceOf(SdkAuthorizationConflictException.class);
    String deviceProof = proof(device, "voice-sdk-code-v1\n" + request.requestId() + "\n"
        + hash(approval.code()) + "\n" + hash(VERIFIER));

    String handoff = service.exchangeGameBinding(challengeId, operationId, approval.code(), VERIFIER, deviceProof);
    assertThatThrownBy(() -> service.approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3))
        .isInstanceOf(SdkAuthorizationConflictException.class).as("consumed approval receipt cannot reauthorize");
    var verified = issuer.verifyGameBindingHandoff(handoff);
    assertThat(verified.challengeId()).isEqualTo(challengeId);
    assertThat(verified.operationId()).isEqualTo(operationId);
    assertThat(verified.targetProfileId()).isEqualTo(secondaryProfile);
    assertThat(verified.scopes()).containsExactlyElementsOf(executionScopes.stream().sorted().toList());
    assertThat(verified.providerSubjectDigest()).matches("hmac-sha256-v1:digest-2026:[0-9a-f]{64}");
    assertThat(service.exchangeGameBinding(challengeId, operationId, approval.code(), VERIFIER, deviceProof))
        .isEqualTo(handoff);
    var authClaims = new AuthGameBindingHandoffService(jdbc,
        new TransactionTemplate(new DataSourceTransactionManager(database)), issuer,
        (application, environment) -> app.equals(application) && env.equals(environment) ? policy.get() : null,
        (account, selected) -> targetAccount.equals(account) && secondaryProfile.equals(selected) ? profile.get() : null,
        identity(CLOCK), blacklist, CLOCK);
    String mutationHash = hash("exact canonical GIS binding operation");
    var claim = authClaims.claim(handoff, deviceProof, operationId, mutationHash);
    assertThat(claim.operationId()).isEqualTo(operationId);
    assertThat(authClaims.claim(handoff, deviceProof, operationId, mutationHash)).isEqualTo(claim);
    assertThatThrownBy(() -> authClaims.claim(handoff, "changed-proof", operationId, mutationHash))
        .isInstanceOf(SdkAuthorizationConflictException.class);
    assertThatThrownBy(() -> service.exchange(request.requestId(), approval.code(), REDIRECT, VERIFIER, deviceProof))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_game_binding_handoff_issuances WHERE operation_id=:id",
        Map.of("id", operationId), Long.class)).isEqualTo(1L);

    UUID bindingId = UUID.randomUUID();
    authClaims.complete(claim.claimId(), operationId, "succeeded", bindingId);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_game_message_grants WHERE binding_id=:binding",
        Map.of("binding", bindingId), Long.class)).isEqualTo(1L);
    assertThat(jdbc.queryForObject("SELECT status FROM sdk_game_message_grants WHERE binding_id=:binding",
        Map.of("binding", bindingId), String.class)).isEqualTo("active");
    long deviceAuthorityRevision = jdbc.queryForObject("SELECT authority_revision FROM sdk_devices WHERE device_id=:id",
        Map.of("id", source.deviceId()), Long.class);
    Timestamp keyExpiry = jdbc.queryForObject("SELECT not_after FROM sdk_device_keys WHERE key_id=:id",
        Map.of("id", source.keyId()), Timestamp.class);
    var deviceClaims = new java.util.TreeMap<String, Object>();
    deviceClaims.put("version", 1L); deviceClaims.put("iss", "auth"); deviceClaims.put("aud", "voice.game-message");
    deviceClaims.put("jti", UUID.randomUUID().toString()); deviceClaims.put("application_id", app.toString());
    deviceClaims.put("environment_id", env.toString()); deviceClaims.put("account_id", source.accountId().toString());
    deviceClaims.put("actor_id", source.actorId().toString()); deviceClaims.put("binding_id", bindingId.toString());
    deviceClaims.put("device_id", source.deviceId().toString()); deviceClaims.put("key_id", source.keyId().toString());
    deviceClaims.put("public_jwk", device.toPublicJWK().toJSONObject());
    deviceClaims.put("key_thumbprint", device.computeThumbprint().toString());
    deviceClaims.put("device_generation", source.deviceGeneration());
    deviceClaims.put("authority_revision", deviceAuthorityRevision); deviceClaims.put("status", "active");
    deviceClaims.put("not_after", keyExpiry.toInstant().toEpochMilli()); deviceClaims.put("iat", NOW.toEpochMilli());
    deviceClaims.put("exp", NOW.plusSeconds(4).toEpochMilli());
    String deviceAssertion = issuer.issueDeviceStatus(deviceClaims);
    var verifiedDevice = issuer.verifyDeviceStatusAssertion(deviceAssertion);
    jdbc.update("""
        INSERT INTO sdk_device_authority_issues(request_id,request_bytes,session_token_hash,assertion,device_id,key_id,issued_at,expires_at)
        VALUES (:id,:bytes,:session,:assertion,:device,:key,:issued,:expires)
        """, new org.springframework.jdbc.core.namedparam.MapSqlParameterSource()
            .addValue("id", UUID.randomUUID()).addValue("bytes", "permit-authority-request".getBytes(StandardCharsets.US_ASCII))
            .addValue("session", hash(source.accessToken())).addValue("assertion", deviceAssertion)
            .addValue("device", source.deviceId()).addValue("key", source.keyId()).addValue("issued", Timestamp.from(NOW))
            .addValue("expires", Timestamp.from(NOW.plusSeconds(4))));
    var currentDeviceAuthority = jdbc.queryForMap("""
        SELECT s.token_hash IS NOT NULL AS session_found, i.status AS identity_status,
               x.device_id=s.device_id AS issue_device_matches, x.key_id=k.key_id AS issue_key_matches,
               d.revoked_at IS NULL AS device_live, k.revoked_at IS NULL AS key_not_revoked,
               i.actor_id, i.application_id, i.environment_id,
               i.ownership_generation, d.authority_revision, k.generation, k.key_thumbprint,
               k.status, k.not_before<=:now AS key_started, k.not_after>:now AS key_live,
               x.expires_at>:now AS assertion_live
        FROM sdk_device_authority_issues x
        LEFT JOIN sdk_sessions s ON s.token_hash=x.session_token_hash
        LEFT JOIN sdk_identities i ON i.account_id=s.account_id
        LEFT JOIN sdk_devices d ON d.device_id=s.device_id
        LEFT JOIN sdk_device_keys k ON k.device_id=d.device_id AND k.key_id=x.key_id
        WHERE x.assertion=:assertion
        """, Map.of("now", Timestamp.from(NOW), "assertion", deviceAssertion));
    assertThat(currentDeviceAuthority.get("session_found")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("identity_status")).isEqualTo("active");
    assertThat(currentDeviceAuthority.get("issue_device_matches")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("issue_key_matches")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("device_live")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("key_not_revoked")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("actor_id")).isEqualTo(verifiedDevice.actorId());
    assertThat(currentDeviceAuthority.get("application_id")).isEqualTo(app);
    assertThat(currentDeviceAuthority.get("environment_id")).isEqualTo(env);
    assertThat(((Number) currentDeviceAuthority.get("ownership_generation")).longValue()).isEqualTo(verifiedDevice.deviceGeneration());
    assertThat(((Number) currentDeviceAuthority.get("authority_revision")).longValue()).isEqualTo(verifiedDevice.authorityRevision());
    assertThat(((Number) currentDeviceAuthority.get("generation")).longValue()).isEqualTo(verifiedDevice.deviceGeneration());
    assertThat(currentDeviceAuthority.get("key_thumbprint")).isEqualTo(verifiedDevice.keyThumbprint());
    assertThat(currentDeviceAuthority.get("status")).isIn("active", "overlap");
    assertThat(currentDeviceAuthority.get("key_started")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("key_live")).isEqualTo(true);
    assertThat(currentDeviceAuthority.get("assertion_live")).isEqualTo(true);
    UUID messageOperation = UUID.randomUUID();
    String messageHash = hash("canonical message payload");
    AtomicReference<Instant> gisPermitExpiry = new AtomicReference<>(NOW.plusMillis(3000));
    CountDownLatch gisIssueEntered = new CountDownLatch(1);
    CountDownLatch allowGisIssueToFinish = new CountDownLatch(1);
    CountDownLatch gisCompletionEntered = new CountDownLatch(1);
    CountDownLatch allowGisCompletionToFinish = new CountDownLatch(1);
    var executionProfileCalls = new AtomicInteger();
    var advanceClockDuringFinalProfileCheck = new AtomicBoolean(false);
    SdkGameIntegrationExecutionPermitAuthority gisAuthority = new SdkGameIntegrationExecutionPermitAuthority() {
      @Override public Permit issue(UUID binding, UUID operation, String assertion) {
        if (operation.equals(messageOperation)) {
          gisIssueEntered.countDown();
          try {
            if (!allowGisIssueToFinish.await(10, TimeUnit.SECONDS)) throw new IllegalStateException("GIS issue test gate timed out");
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("GIS issue test gate interrupted", interrupted);
          }
        }
        return new Permit(UUID.fromString("eb4ac69b-8ebd-45f7-9b5b-3947912a85b1"), binding, app, env, 9,
            verifiedDevice.jti(), operation, gisPermitExpiry.get());
      }
      @Override public Completion complete(UUID permit, UUID operation, String outcome) {
        if (operation.equals(messageOperation)) {
          gisCompletionEntered.countDown();
          try {
            if (!allowGisCompletionToFinish.await(10, TimeUnit.SECONDS)) {
              throw new IllegalStateException("GIS completion test gate timed out");
            }
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("GIS completion test gate interrupted", interrupted);
          }
        }
        return new Completion(permit, operation, outcome, "completed");
      }
    };
    var executionPermits = new AuthGameMessageExecutionPermitService(jdbc,
        new TransactionTemplate(new DataSourceTransactionManager(database)), issuer,
        (application, environment) -> policy.get(), (account, selected) -> {
          var eligibleProfile = profile.get();
          if (advanceClockDuringFinalProfileCheck.get() && executionProfileCalls.incrementAndGet() == 2) {
            executionClockNow.set(NOW.plusMillis(500));
          }
          return eligibleProfile;
        }, gisAuthority, executionClock);
    for (Instant invalidExpiry : List.of(NOW.plusMillis(500), NOW.plusMillis(3751))) {
      gisPermitExpiry.set(invalidExpiry);
      UUID rejectedOperation = UUID.randomUUID();
      assertThatThrownBy(() -> executionPermits.issue(deviceAssertion, rejectedOperation, messageHash))
          .isInstanceOf(SdkIdentityDeniedException.class)
          .as("Auth must reject GIS permits outside the 500 ms to 3750 ms accepted window");
      assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=:operation",
          Map.of("operation", rejectedOperation), Long.class)).isZero();
    }
    UUID finalCheckBoundaryOperation = UUID.randomUUID();
    gisPermitExpiry.set(NOW.plusMillis(1000));
    executionProfileCalls.set(0);
    advanceClockDuringFinalProfileCheck.set(true);
    assertThatThrownBy(() -> executionPermits.issue(deviceAssertion, finalCheckBoundaryOperation, messageHash))
        .isInstanceOf(SdkIdentityDeniedException.class)
        .as("a final profile check that leaves exactly 500 ms cannot issue a permit");
    advanceClockDuringFinalProfileCheck.set(false);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_game_message_execution_permits WHERE operation_id=:operation",
        Map.of("operation", finalCheckBoundaryOperation), Long.class)).isZero();

    executionClockNow.set(NOW);
    gisPermitExpiry.set(NOW.plusMillis(3000));
    executionProfileCalls.set(0);
    advanceClockDuringFinalProfileCheck.set(true);
    var raceWorkers = Executors.newFixedThreadPool(2);
    AuthGameMessageExecutionPermitService.Permit issuedPermit;
    AuthGameBindingHandoffService.RevocationReceipt concurrentRevoke;
    CountDownLatch revokeStarted = new CountDownLatch(1);
    try {
      var permitFuture = raceWorkers.submit(() -> executionPermits.issue(deviceAssertion, messageOperation, messageHash));
      assertThat(gisIssueEntered.await(5, TimeUnit.SECONDS)).isTrue();
      var revokeFuture = raceWorkers.submit(() -> {
        revokeStarted.countDown();
        return authClaims.revoke(operationId);
      });
      assertThat(revokeStarted.await(5, TimeUnit.SECONDS)).isTrue();
      allowGisIssueToFinish.countDown();
      issuedPermit = permitFuture.get(10, TimeUnit.SECONDS);
      advanceClockDuringFinalProfileCheck.set(false);
      concurrentRevoke = revokeFuture.get(10, TimeUnit.SECONDS);
    } finally {
      allowGisIssueToFinish.countDown();
      raceWorkers.shutdownNow();
    }
    assertThat(concurrentRevoke.status()).isEqualTo("revoking")
        .as("revoke waits for the permit transaction, then drains its admitted operation");
    var signedPermit = SignedJWT.parse(issuedPermit.permitJws());
    UUID permitJti = UUID.fromString(signedPermit.getJWTClaimsSet().getJWTID());
    assertThat(signedPermit.getJWTClaimsSet().getClaim("iat_ms")).isEqualTo(NOW.plusMillis(500).toEpochMilli());
    assertThat(signedPermit.getJWTClaimsSet().getClaim("expires_at_ms")).isEqualTo(NOW.plusMillis(3000).toEpochMilli());
    assertThat(signedPermit.getJWTClaimsSet().getClaim("scope")).isEqualTo("game.chat.send");
    assertThat(signedPermit.getJWTClaimsSet().getClaim("profile_id")).isEqualTo(secondaryProfile.toString());
    assertThat(executionPermits.issue(deviceAssertion, messageOperation, messageHash)).isEqualTo(issuedPermit);
    assertThatThrownBy(() -> executionPermits.issue(deviceAssertion, messageOperation, hash("changed payload")))
        .isInstanceOf(SdkAuthorizationConflictException.class);
    var revoked = concurrentRevoke;
    assertThat(revoked.status()).isEqualTo("revoking");
    assertThat(executionPermits.issue(deviceAssertion, messageOperation, messageHash)).isEqualTo(issuedPermit)
        .as("exact operation replay returns the immutable permit while revoke fences new issue");
    var completionWorkers = Executors.newFixedThreadPool(2);
    try {
      var completionFuture = completionWorkers.submit(
          () -> executionPermits.complete(permitJti, messageOperation, "committed"));
      assertThat(gisCompletionEntered.await(5, TimeUnit.SECONDS)).isTrue();
      var retryIssueFuture = completionWorkers.submit(
          () -> executionPermits.issue(deviceAssertion, messageOperation, messageHash));
      DatabaseLockWait lockWait = awaitDatabaseLockWait("%sdk_game_message_grants%for update%");
      assertThat(lockWait.waitingPid()).isPositive();
      assertThat(jdbc.queryForObject("SELECT state FROM pg_stat_activity WHERE pid=:pid",
          Map.of("pid", lockWait.blockingPid()), String.class)).isEqualTo("idle in transaction");
      allowGisCompletionToFinish.countDown();
      assertThat(completionFuture.get(10, TimeUnit.SECONDS).status()).isEqualTo("completed");
      assertThat(retryIssueFuture.get(10, TimeUnit.SECONDS)).isEqualTo(issuedPermit);
    } finally {
      allowGisCompletionToFinish.countDown();
      completionWorkers.shutdownNow();
      assertThat(completionWorkers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
    assertThat(executionPermits.complete(permitJti, messageOperation, "committed").status()).isEqualTo("completed");
    revoked = authClaims.revoke(operationId);
    assertThat(revoked.status()).isEqualTo("revoked");
    assertThat(executionPermits.issue(deviceAssertion, messageOperation, messageHash)).isEqualTo(issuedPermit)
        .as("exact permit replay remains stable after revoke");
    assertThat(authClaims.claim(handoff, deviceProof, operationId, mutationHash))
        .as("exact accepted-operation replay remains idempotent after revoke")
        .isEqualTo(claim);
    assertThat(jdbc.queryForObject("SELECT status FROM sdk_game_message_grants WHERE binding_id=:binding",
        Map.of("binding", bindingId), String.class)).isEqualTo("revoked");
    assertThat(jdbc.queryForObject("SELECT game_binding_status FROM sdk_authorizations WHERE request_id=:id",
        Map.of("id", request.requestId()), String.class)).isEqualTo("revoked");
    assertThatThrownBy(() -> authClaims.revoke(UUID.randomUUID())).isInstanceOf(SdkIdentityDeniedException.class);
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
          "Wrong app", Set.of(REDIRECT), SCOPES, Set.of("google"));
      case "wrong-environment" -> new SdkAuthorizationPolicy.Policy(app, UUID.randomUUID(), 3,
          "Wrong environment", Set.of(REDIRECT), SCOPES, Set.of("google"));
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
  @ValueSource(strings = {"wrong-owner", "wrong-profile", "missing", "zero-revision", "deleted", "frozen", "unavailable"})
  void selectedProfileMustHaveExactOwnerAndCurrentEligibility(String invalid) throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    if (invalid.equals("missing")) profile.set(null);
    if (invalid.equals("unavailable")) profileFailure.set(new IllegalStateException("User unavailable"));
    if (!invalid.equals("missing") && !invalid.equals("unavailable")) {
      profile.set(new SdkProfileEligibility.Profile(
          invalid.equals("wrong-owner") ? UUID.randomUUID() : targetAccount,
          invalid.equals("wrong-profile") ? primaryProfile : secondaryProfile, 11,
          invalid.equals("deleted"), invalid.equals("frozen")));
      if (invalid.equals("zero-revision")) {
        profile.set(new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 0, false, false));
      }
    }
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
  @ValueSource(strings = {"source-revoked", "source-generation", "profile-frozen", "profile-revision",
      "profile-deleted", "profile-missing", "profile-wrong-owner", "profile-wrong-id", "profile-zero-revision",
      "profile-unavailable", "policy-revision",
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
  void unknownAndRevokedLinkedCredentialsHaveTheSameCoarseDenial() throws Exception {
    var request = start(authorization(), UUID.randomUUID(), REDIRECT, STATE, SCOPES);
    var approval = authorization().approve(request.requestId(), VOICE_BEARER, secondaryProfile, 3);
    var linked = exchange(authorization(), request.requestId(), approval.code(), REDIRECT, VERIFIER, device);
    String linkedProof = proof(device, "voice-sdk-linked-v1\n" + hash(linked.accessToken()));
    changeAuthority("policy-revision");

    var revoked = catchThrowable(() -> authorization().linkedSession(linked.accessToken(), linkedProof));
    var unknown = catchThrowable(() -> authorization().linkedSession("u".repeat(43), linkedProof));

    assertThat(revoked).isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
    assertThat(unknown).isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
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
      case "profile-deleted" -> profile.set(
          new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 11, true, false));
      case "profile-revision" -> profile.set(
          new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 12, false, false));
      case "profile-missing" -> profile.set(null);
      case "profile-wrong-owner" -> profile.set(
          new SdkProfileEligibility.Profile(UUID.randomUUID(), secondaryProfile, 11, false, false));
      case "profile-wrong-id" -> profile.set(
          new SdkProfileEligibility.Profile(targetAccount, primaryProfile, 11, false, false));
      case "profile-zero-revision" -> profile.set(
          new SdkProfileEligibility.Profile(targetAccount, secondaryProfile, 0, false, false));
      case "profile-unavailable" -> profileFailure.set(new IllegalStateException("User unavailable"));
      case "policy-revision" -> policy.set(
          new SdkAuthorizationPolicy.Policy(app, env, 4, "Example Game", Set.of(REDIRECT), SCOPES,
              Set.of("google")));
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
        Map.of(app + "/" + env, new SdkApplication(app, env, client, gameKey.toPublicJWK())),
        (requestedApp, requestedEnv) -> app.equals(requestedApp) && env.equals(requestedEnv)
            ? new SdkAuthorizationPolicy.Policy(app, env, 3, "Example Game", Set.of(REDIRECT), SCOPES,
                Set.of("google")) : null,
        clock);
  }

  private SdkAuthorizationService authorization() {
    return authorization(CLOCK);
  }

  private SdkAuthorizationService authorization(Clock clock) {
    var dataSource = dataSource();
    return new SdkAuthorizationService(new NamedParameterJdbcTemplate(dataSource),
        new TransactionTemplate(new DataSourceTransactionManager(dataSource)), identity(clock), auth,
        (application, environment) -> app.equals(application) && env.equals(environment) ? policy.get() : null,
        (account, selected) -> {
          if (profileFailure.get() != null) throw profileFailure.get();
          return targetAccount.equals(account) && secondaryProfile.equals(selected) ? profile.get() : null;
        },
        blacklist, clock);
  }

  private SdkAuthorizationService authorizationForBinding(Clock clock,
      SdkBindingChallengeAuthority challenges, AuthUserPrincipalIssuer issuer) {
    var dataSource = dataSource();
    return new SdkAuthorizationService(new NamedParameterJdbcTemplate(dataSource),
        new TransactionTemplate(new DataSourceTransactionManager(dataSource)), identity(clock), auth,
        (application, environment) -> app.equals(application) && env.equals(environment) ? policy.get() : null,
        (account, selected) -> {
          if (profileFailure.get() != null) throw profileFailure.get();
          return targetAccount.equals(account) && secondaryProfile.equals(selected) ? profile.get() : null;
        }, blacklist, clock, challenges, issuer,
        new AuthGameBindingSubjectDigest("digest-2026",
            "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII)),
        new AuthGameBindingApprovalCodeVault("abcdef0123456789abcdef0123456789".getBytes(StandardCharsets.US_ASCII)));
  }

  private AuthUserPrincipalIssuer principalIssuer(Clock clock) throws Exception {
    Path directory = Files.createTempDirectory("voice-auth-game-binding-principal-");
    RSAKey current = new RSAKeyGenerator(2048).keyID("current").generate();
    RSAKey next = new RSAKeyGenerator(2048).keyID("next").generate();
    Files.writeString(directory.resolve("current.pem"), privateKeyPem(current));
    Files.writeString(directory.resolve("next.pem"), privateKeyPem(next));
    return AuthUserPrincipalIssuer.load(directory, current.getKeyID(), clock);
  }

  private static String privateKeyPem(RSAKey key) throws Exception {
    return "-----BEGIN PRIVATE KEY-----\n"
        + Base64.getMimeEncoder(64, new byte[] {'\n'}).encodeToString(key.toPrivateKey().getEncoded())
        + "\n-----END PRIVATE KEY-----\n";
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

  private DatabaseLockWait awaitDatabaseLockWait(String waitingQueryPattern) {
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
    while (System.nanoTime() < deadline) {
      List<Map<String, Object>> waits = jdbc.queryForList("""
          SELECT waiting.pid AS waiting_pid, blocking.pid AS blocking_pid
          FROM pg_stat_activity waiting
          CROSS JOIN LATERAL unnest(pg_blocking_pids(waiting.pid)) AS blockers(pid)
          JOIN pg_stat_activity blocking ON blocking.pid=blockers.pid
          WHERE waiting.datname=current_database() AND waiting.wait_event_type='Lock'
            AND lower(waiting.query) LIKE :waitingQueryPattern
            AND blocking.state='idle in transaction'
          """, Map.of("waitingQueryPattern", waitingQueryPattern));
      for (Map<String, Object> wait : waits) {
        int blocker = ((Number) wait.get("blocking_pid")).intValue();
        return new DatabaseLockWait(((Number) wait.get("waiting_pid")).intValue(), blocker);
      }
      try { Thread.sleep(20); }
      catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
        throw new AssertionError("interrupted while observing PostgreSQL lock wait", interrupted);
      }
    }
    throw new AssertionError("duplicate permit issue did not reach a PostgreSQL lock wait behind completion");
  }

  private record DatabaseLockWait(int waitingPid, int blockingPid) {}

  private String pkce(String value) throws Exception {
    return Base64.getUrlEncoder().withoutPadding().encodeToString(sha256(value));
  }

  private byte[] sha256(String value) throws Exception {
    return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
  }
}
