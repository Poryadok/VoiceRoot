package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
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
import java.util.Base64;
import java.util.Date;
import java.util.HashMap;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
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
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.RegistrationSessionEpochPreparer;
import voice.backend.auth.service.TokenClaims;
import voice.backend.auth.sessionepoch.PreparedSessionEpoch;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

/** GAME-AUTH-03 uses real SDK/linked proofs and PostgreSQL registration transactions. */
@Testcontainers(disabledWithoutDocker = true)
class SdkConversionJdbcIntegrationTest {
  private static final Instant NOW = Instant.parse("2026-09-26T12:00:00Z");
  private static final String ISSUER = "https://accounts.google.com";
  private static final String REDIRECT = "https://game.example.test/voice/callback";
  private static final String VERIFIER = "v".repeat(64);
  private static final String STATE = "s".repeat(43);
  private static final Set<String> SCOPES = Set.of("game.identity.read", "game.chat.read");
  private static RSAKey googleKey;
  private static RSAKey gameKey;

  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db").withUsername("voice").withPassword("voice")
          .withLabel("voice.task", "GAME-AUTH-03").withReuse(false);

  private final UUID app = UUID.randomUUID();
  private final UUID env = UUID.randomUUID();
  private final UUID existingAccount = UUID.randomUUID();
  private final UUID primaryProfile = UUID.randomUUID();
  private final UUID secondaryProfile = UUID.randomUUID();
  private final AuthService auth = mock(AuthService.class);
  private final TokenBlacklist blacklist = mock(TokenBlacklist.class);
  private final PrimaryProfileProvisioner primaries = mock(PrimaryProfileProvisioner.class);
  private final Map<String, SdkProfileEligibility.Profile> profiles = new HashMap<>();
  private final AtomicReference<Instant> currentTime = new AtomicReference<>(NOW);
  private final Clock clock = new MutableClock(currentTime, ZoneOffset.UTC);
  private DriverManagerDataSource database;
  private NamedParameterJdbcTemplate jdbc;
  private SdkIdentityService identity;
  private SdkAuthorizationService authorization;
  private SdkIdentityService.Session source;
  private ECKey device;
  private String existingBearer;

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
    jdbc.update("INSERT INTO accounts(id,password_hash,type,status,session_epoch) "
        + "VALUES (:id,'test-password-hash','regular','active',7)", Map.of("id", existingAccount));
    existingBearer = authenticate(existingAccount, primaryProfile, 7);
    when(auth.prepareOAuthAccessToken(anyString())).thenAnswer(call -> {
      UUID id = UUID.fromString(call.getArgument(0));
      long epoch = jdbc.queryForObject("SELECT session_epoch FROM accounts WHERE id=:id", Map.of("id", id), Long.class);
      return new PreparedSessionEpoch(id, epoch);
    });
    profiles.put(existingAccount + "/" + secondaryProfile,
        new SdkProfileEligibility.Profile(existingAccount, secondaryProfile, 11, false, false));
    device = new ECKeyGenerator(Curve.P_256).generate();
    identity = new SdkIdentityService(jdbc, transactions(), new GoogleOidcProofVerifier(clock,
        kid -> googleKey.getKeyID().equals(kid) ? googleKey.toPublicJWK() : null),
        Map.of(app + "/" + env, new SdkApplication(app, env, "operator-client", gameKey.toPublicJWK())), clock);
    var policy = new SdkAuthorizationPolicy.Policy(app, env, 3, "Example Game", Set.of(REDIRECT), SCOPES);
    authorization = new SdkAuthorizationService(jdbc, transactions(), identity, auth,
        (requestedApp, requestedEnv) -> app.equals(requestedApp) && env.equals(requestedEnv) ? policy : null,
        this::profile, blacklist, clock);
    var challenge = identity.challenge(app, env, device.toPublicJWK().toJSONString());
    String provider = signed(new JWTClaimsSet.Builder().issuer(ISSUER).subject("player").audience("operator-client")
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
  void newPreparationDurablyCreatesFifteenMinuteIntentBeforeRegistration() throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    assertThat(operation.operationId()).isNotNull();
    assertThat(operation.mode()).isEqualTo("new");
    assertThat(operation.state()).isEqualTo("prepared");
    assertThat(operation.revision()).isEqualTo(1);
    assertThat(operation.registrationIntentId()).isNotNull();
    assertThat(operation.registrationIntentExpiresAt()).isEqualTo(NOW.plusSeconds(900));
    assertThat(operation.registeredAccountId()).isNull();
    assertThat(operation.targetAccountId()).isNull();
    assertThat(operation.targetProfileId()).isNull();
    assertThat(status(operation.operationId())).isEqualTo(operation);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_registration_intents WHERE intent_id=:id",
        Map.of("id", operation.registrationIntentId()), Long.class)).isEqualTo(1L);
    verifyNoInteractions(primaries);
  }

  @Test
  void preparationRetryReturnsSameOperationAndIntentWithoutExtendingExpiry() throws Exception {
    UUID key = UUID.randomUUID();
    UUID binding = UUID.randomUUID();
    var original = prepareNew(key, binding);
    currentTime.set(NOW.plusSeconds(30));
    assertThat(prepareNew(key, binding)).isEqualTo(original);
    assertThatThrownBy(() -> prepareNew(key, UUID.randomUUID()))
        .isInstanceOf(SdkAuthorizationConflictException.class);
    assertThat(status(original.operationId())).isEqualTo(original);
  }

  @Test
  void existingPreparationPreservesSecondaryProfileAndSeparatesModeWithoutProvisioning() throws Exception {
    var linked = linked();
    UUID key = UUID.randomUUID();
    UUID binding = UUID.randomUUID();
    var fresh = prepareNew(key, binding);
    var existing = prepareExisting(linked, key, binding);
    assertThat(existing.operationId()).isNotEqualTo(fresh.operationId());
    assertThat(existing.mode()).isEqualTo("existing");
    assertThat(existing.state()).isEqualTo("prepared");
    assertThat(existing.targetAccountId()).isEqualTo(existingAccount);
    assertThat(existing.targetProfileId()).isEqualTo(secondaryProfile).isNotEqualTo(primaryProfile);
    assertThat(existing.registrationIntentId()).isNull();
    assertThat(prepareExisting(linked, key, binding)).isEqualTo(existing);
    verifyNoInteractions(primaries);
  }

  @Test
  void normalRegistrationAtomicallyBindsNewAccountAndConsumesIntent() throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    String email = "new-" + UUID.randomUUID() + "@example.test";
    var registered = registration().prepare(email, null, "password-hash", "regular", true,
        operation.registrationIntentId());
    var recovered = status(operation.operationId());
    assertThat(recovered.registeredAccountId()).isEqualTo(registered.account().id());
    assertThat(recovered.targetAccountId()).isNull();
    var intent = jdbc.queryForMap("SELECT account_id,consumed_at FROM sdk_registration_intents WHERE intent_id=:id",
        Map.of("id", operation.registrationIntentId()));
    assertThat(intent.get("account_id")).isEqualTo(registered.account().id());
    assertThat(intent.get("consumed_at")).isNotNull();
    assertThat(jdbc.queryForObject("SELECT count(*) FROM accounts WHERE email=:email", Map.of("email", email), Long.class))
        .isEqualTo(1L);
    assertThat(registered.preparedEpoch().sessionEpoch()).isPositive();
    verifyNoInteractions(primaries);
  }

  @ParameterizedTest
  @ValueSource(strings = {"unknown", "expired", "used", "source-revoked", "source-generation", "guest"})
  void invalidRegistrationIntentRollsBackNewAccountAndPreservesOperation(String invalid) throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    if (invalid.equals("used")) {
      registration().prepare("first-" + UUID.randomUUID() + "@example.test", null, "password-hash",
          "regular", true, operation.registrationIntentId());
    }
    if (invalid.equals("expired")) currentTime.set(NOW.plusSeconds(900));
    if (invalid.equals("source-revoked")) revokeSource();
    if (invalid.equals("source-generation")) jdbc.update(
        "UPDATE sdk_identities SET ownership_generation=ownership_generation+1 WHERE account_id=:id",
        Map.of("id", source.accountId()));
    var before = status(operation.operationId());
    String intentBefore = jdbc.queryForObject(
        "SELECT row_to_json(i)::text FROM sdk_registration_intents i WHERE intent_id=:id",
        Map.of("id", operation.registrationIntentId()), String.class);
    String email = "rejected-" + UUID.randomUUID() + "@example.test";
    UUID intent = invalid.equals("unknown") ? UUID.randomUUID() : operation.registrationIntentId();
    assertThatThrownBy(() -> registration().prepare(email, null, "password-hash",
        invalid.equals("guest") ? "guest" : "regular", !invalid.equals("guest"), intent))
        .isInstanceOf(AuthException.class).hasMessage("validation_failed");
    assertThat(jdbc.queryForObject("SELECT count(*) FROM accounts WHERE email=:email",
        Map.of("email", email), Long.class)).isZero();
    assertThat(status(operation.operationId())).isEqualTo(before);
    assertThat(jdbc.queryForObject("SELECT row_to_json(i)::text FROM sdk_registration_intents i WHERE intent_id=:id",
        Map.of("id", operation.registrationIntentId()), String.class)).isEqualTo(intentBefore);
    verifyNoInteractions(primaries);
  }

  @Test
  void registrationIntentOutlivesSourceBootstrapWhileDeviceAndGenerationRemainCurrent() throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    currentTime.set(NOW.plusSeconds(600));
    assertThat(source.expiresAt()).isBefore(clock.instant());
    var registered = registration().prepare("delayed-" + UUID.randomUUID() + "@example.test", null,
        "password-hash", "regular", true, operation.registrationIntentId());
    assertThat(status(operation.operationId()).registeredAccountId()).isEqualTo(registered.account().id());
  }

  @Test
  void attachAcceptsOnlyRecordedVerifiedNewAccountThenProvisionsAndChecksItsPrimary() throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    var registered = registration().prepare("pending-" + UUID.randomUUID() + "@example.test", null,
        "password-hash", "regular", true, operation.registrationIntentId());
    UUID account = registered.account().id();
    UUID newPrimary = UUID.randomUUID();
    String bearer = authenticate(account, newPrimary, registered.account().sessionEpoch());
    assertThatThrownBy(() -> conversion().attachNewTarget(operation.operationId(), existingBearer))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> conversion().attachNewTarget(operation.operationId(), bearer))
        .isInstanceOf(SdkIdentityDeniedException.class);
    verifyNoInteractions(primaries);

    jdbc.update("UPDATE accounts SET type='regular',regular_email_verification_pending=FALSE WHERE id=:id",
        Map.of("id", account));
    when(primaries.ensurePrimaryProfile(eq(account), anyString(), eq(false))).thenAnswer(call -> {
      // Eligibility appears only after the User provisioning side effect.
      profiles.put(account + "/" + newPrimary,
          new SdkProfileEligibility.Profile(account, newPrimary, 1, false, false));
      return newPrimary.toString();
    });
    var attached = conversion().attachNewTarget(operation.operationId(), bearer);
    assertThat(attached.registeredAccountId()).isEqualTo(account);
    assertThat(attached.targetAccountId()).isEqualTo(account);
    assertThat(attached.targetProfileId()).isEqualTo(newPrimary);
    assertThat(attached.state()).isEqualTo("prepared");
    assertThat(attached.revision()).isGreaterThan(operation.revision());
    verify(primaries).ensurePrimaryProfile(eq(account), anyString(), eq(false));
    assertThat(status(operation.operationId())).isEqualTo(attached);
  }

  @Test
  void attachCannotPersistProvisionedPrimaryWithoutCurrentEligibility() throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    var registered = registration().prepare("verified-" + UUID.randomUUID() + "@example.test", null,
        "password-hash", "regular", false, operation.registrationIntentId());
    UUID account = registered.account().id();
    UUID newPrimary = UUID.randomUUID();
    String bearer = authenticate(account, newPrimary, registered.account().sessionEpoch());
    when(primaries.ensurePrimaryProfile(eq(account), anyString(), eq(false))).thenReturn(newPrimary.toString());
    profiles.put(account + "/" + newPrimary,
        new SdkProfileEligibility.Profile(account, newPrimary, 1, false, true));
    assertThatThrownBy(() -> conversion().attachNewTarget(operation.operationId(), bearer))
        .isInstanceOf(SdkIdentityDeniedException.class);
    var stillPrepared = status(operation.operationId());
    assertThat(stillPrepared.targetAccountId()).isNull();
    assertThat(stillPrepared.targetProfileId()).isNull();
    assertThat(stillPrepared.registeredAccountId()).isEqualTo(account);
  }

  @Test
  void recoveryStatusRemainsReadableAfterSourceRevocationWithoutGrantingNewPreparation() throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    revokeSource();
    currentTime.set(NOW.plusSeconds(1000));
    assertThat(status(operation.operationId())).isEqualTo(operation);
    assertThatThrownBy(() -> prepareNew(UUID.randomUUID(), UUID.randomUUID()))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @ParameterizedTest
  @ValueSource(longs = {-61, 31})
  void statusRejectsProofOutsideFreshnessWindow(long offset) throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    long issuedAt = NOW.plusSeconds(offset).getEpochSecond();
    assertThatThrownBy(() -> conversion().status(operation.operationId(), issuedAt,
        proof(device, "voice-sdk-conversion-status-v1\n" + operation.operationId() + "\n" + issuedAt)))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @ParameterizedTest
  @ValueSource(longs = {-60, 30})
  void statusAcceptsExactFreshnessBoundaries(long offset) throws Exception {
    var operation = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    long issuedAt = NOW.plusSeconds(offset).getEpochSecond();
    assertThat(conversion().status(operation.operationId(), issuedAt,
        proof(device, "voice-sdk-conversion-status-v1\n" + operation.operationId() + "\n" + issuedAt)))
        .isEqualTo(operation);
  }

  @Test
  void recoveryProofBindsOperationTimestampAndOriginalDeviceKey() throws Exception {
    var first = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    var second = prepareNew(UUID.randomUUID(), UUID.randomUUID());
    long issuedAt = NOW.getEpochSecond();
    String payload = "voice-sdk-conversion-status-v1\n" + first.operationId() + "\n" + issuedAt;
    var wrongKey = new ECKeyGenerator(Curve.P_256).generate();
    assertThatThrownBy(() -> conversion().status(first.operationId(), issuedAt, proof(wrongKey, payload)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> conversion().status(second.operationId(), issuedAt, proof(device, payload)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> conversion().status(first.operationId(), issuedAt + 1, proof(device, payload)))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(status(first.operationId())).isEqualTo(first);
  }

  @Test
  void preparationProofBindsModeTokenIdempotencyAndBinding() throws Exception {
    UUID key = UUID.randomUUID();
    UUID binding = UUID.randomUUID();
    String payload = "voice-sdk-conversion-new-v1\n" + hash(source.accessToken()) + "\n" + key + "\n" + binding;
    var wrongKey = new ECKeyGenerator(Curve.P_256).generate();
    assertThatThrownBy(() -> conversion().prepareNew(source.accessToken(), proof(wrongKey, payload), key, binding))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> conversion().prepareNew(source.accessToken(), proof(device, payload), UUID.randomUUID(), binding))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThatThrownBy(() -> conversion().prepareNew(source.accessToken(), proof(device, payload), key, UUID.randomUUID()))
        .isInstanceOf(SdkIdentityDeniedException.class);
    var linked = linked();
    assertThatThrownBy(() -> conversion().prepareExisting(linked.accessToken(), proof(device, payload), key, binding))
        .isInstanceOf(SdkIdentityDeniedException.class);
    assertThat(prepareNew(key, binding).operationId()).isNotNull();
  }

  private void revokeSource() throws Exception {
    identity.revoke(source.accessToken(), proof(device, "voice-sdk-revoke-v1\n" + hash(source.accessToken())));
  }

  private TransactionTemplate transactions() {
    return new TransactionTemplate(new DataSourceTransactionManager(database));
  }

  private SdkConversionService conversion() {
    return new SdkConversionService(jdbc, transactions(), identity, authorization, auth,
        this::profile, primaries, blacklist, clock);
  }

  private RegistrationSessionEpochPreparer registration() {
    var accounts = new JdbcAccountRepository(jdbc);
    var floors = mock(SessionEpochFloorStore.class);
    when(floors.recordAtLeast(any(UUID.class), anyLong())).thenAnswer(call -> call.getArgument(1));
    return new RegistrationSessionEpochPreparer(transactions(), accounts,
        new SessionEpochIssuanceGate(accounts, floors), new JdbcSdkRegistrationIntentBinder(jdbc, clock));
  }

  private SdkProfileEligibility.Profile profile(UUID account, UUID selected) {
    return profiles.get(account + "/" + selected);
  }

  private String authenticate(UUID account, UUID selected, long epoch) {
    String bearer = "Bearer voice-session-" + account;
    when(auth.validate(bearer)).thenReturn(new TokenClaims(account.toString(), selected.toString(),
        List.of("user"), "free", NOW.plusSeconds(1800), "jti-" + account, "regular", epoch));
    return bearer;
  }

  private SdkConversionService.Operation prepareNew(UUID key, UUID binding) throws Exception {
    return conversion().prepareNew(source.accessToken(), proof(device, "voice-sdk-conversion-new-v1\n"
        + hash(source.accessToken()) + "\n" + key + "\n" + binding), key, binding);
  }

  private SdkConversionService.Operation prepareExisting(SdkAuthorizationService.LinkedSession linked,
      UUID key, UUID binding) throws Exception {
    return conversion().prepareExisting(linked.accessToken(), proof(device, "voice-sdk-conversion-existing-v1\n"
        + hash(linked.accessToken()) + "\n" + key + "\n" + binding), key, binding);
  }

  private SdkConversionService.Operation status(UUID operation) throws Exception {
    long issuedAt = clock.instant().getEpochSecond();
    return conversion().status(operation, issuedAt,
        proof(device, "voice-sdk-conversion-status-v1\n" + operation + "\n" + issuedAt));
  }

  private SdkAuthorizationService.LinkedSession linked() throws Exception {
    UUID key = UUID.randomUUID();
    String challenge = Base64.getUrlEncoder().withoutPadding().encodeToString(sha256(VERIFIER));
    String digest = hash("voice-sdk-authorization-request-v1\n" + key + "\n" + REDIRECT + "\n"
        + challenge + "\n" + STATE + "\n" + String.join(",", SCOPES.stream().sorted().toList()));
    var request = authorization.start(source.accessToken(), proof(device,
        "voice-sdk-authorize-v1\n" + hash(source.accessToken()) + "\n" + digest),
        key, REDIRECT, challenge, STATE, SCOPES);
    var approval = authorization.approve(request.requestId(), existingBearer, secondaryProfile, 3);
    return authorization.exchange(request.requestId(), approval.code(), REDIRECT, VERIFIER,
        proof(device, "voice-sdk-code-v1\n" + request.requestId() + "\n" + hash(approval.code()) + "\n" + hash(VERIFIER)));
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

  private byte[] sha256(String value) throws Exception {
    return MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8));
  }

  private static final class MutableClock extends Clock {
    private final AtomicReference<Instant> now;
    private final ZoneId zone;
    private MutableClock(AtomicReference<Instant> now, ZoneId zone) { this.now = now; this.zone = zone; }
    @Override public ZoneId getZone() { return zone; }
    @Override public Clock withZone(ZoneId nextZone) { return new MutableClock(now, nextZone); }
    @Override public Instant instant() { return now.get(); }
  }
}
