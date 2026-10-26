package voice.backend.auth.linkoauth;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.net.URI;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.*;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.*;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.*;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.TokenClaims;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.repository.JdbcLinkedIdentityRepository;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

@Testcontainers(disabledWithoutDocker = true)
class LinkedOAuthJdbcTest {
  @Container static final PostgreSQLContainer<?> postgres = new PostgreSQLContainer<>("postgres:16-alpine")
      .withLabel("voice.task", "AUTH-SETTINGS-OAUTH-RETURN-TRANSPORT").withReuse(false);
  static final Instant NOW = Instant.parse("2026-10-26T01:00:00Z");
  static final String VERIFIER = "v".repeat(43);
  final UUID account = UUID.randomUUID(), profile = UUID.randomUUID();
  final AtomicReference<Instant> time = new AtomicReference<>(NOW);
  final Clock clock = new Clock() {
    public ZoneId getZone() { return ZoneOffset.UTC; }
    public Clock withZone(ZoneId zone) { return this; }
    public Instant instant() { return time.get(); }
  };
  final TokenBlacklist blacklist = mock(TokenBlacklist.class);
  final ProfileSwitchValidator profiles = mock(ProfileSwitchValidator.class);
  final AtomicInteger exchanges = new AtomicInteger();
  LinkedOAuthSettings settings;
  NamedParameterJdbcTemplate jdbc;
  TransactionTemplate tx;
  LinkedOAuthService service;
  TokenClaims claims;

  @BeforeAll static void migrate() {
    Flyway.configure().dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations("classpath:db/migration").load().migrate();
  }
  @BeforeEach void setup() {
    var ds = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    jdbc = new NamedParameterJdbcTemplate(ds);
    tx = new TransactionTemplate(new DataSourceTransactionManager(ds));
    jdbc.update("INSERT INTO accounts(id,password_hash,type,status,session_epoch) VALUES (:id,'fixture','regular','active',7)", Map.of("id", account));
    claims = new TokenClaims(account.toString(), profile.toString(), List.of("user"), "free", NOW.plusSeconds(600), UUID.randomUUID().toString(), "regular", 7);
    settings = settings();
    service = service((provider, code, redirect, verifier) -> {
      exchanges.incrementAndGet();
      assertThat(redirect).isEqualTo("https://api.example.test/api/v1/auth/linked-accounts/twitch/oauth-callback");
      assertThat(verifier).matches("[A-Za-z0-9_-]{43}");
      return new LinkedOAuthProvider.Result("success", "partner-" + account, "partner", "secret-access", "secret-refresh");
    });
  }
  LinkedOAuthSettings settings() {
    var p = new LinkedOAuthSettings.Provider(true, true, "https://id.twitch.tv/oauth2/authorize", "https://id.twitch.tv/oauth2/token",
        "https://api.twitch.tv/helix/users", "fixture-client", "fixture-secret",
        "https://api.example.test/api/v1/auth/linked-accounts/twitch/oauth-callback", Map.of("web", "https://app.example.test/settings/oauth-return"));
    return new LinkedOAuthSettings("fixture-v1", Duration.ofSeconds(300), Duration.ofSeconds(60), "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII), Map.of("twitch", p));
  }
  LinkedOAuthService service(LinkedOAuthProvider exchange) {
    return new LinkedOAuthService(jdbc, tx, settings, blacklist, profiles, clock, exchange);
  }
  LinkedOAuthService.Start start() { return service.start(claims, "twitch", "web", LinkedOAuthService.s256(VERIFIER), "S256"); }
  LinkedOAuthService.Return callback(LinkedOAuthService.Start start) { return service.callback("twitch", state(start), "provider-code", null); }
  String consume(LinkedOAuthService.Start start, LinkedOAuthService.Return result) {
    return service.consume(claims, "twitch", start.attemptId(), result.resultHandle(), VERIFIER);
  }
  static String state(LinkedOAuthService.Start start) {
    return Arrays.stream(URI.create(start.authorizationUrl()).getRawQuery().split("&"))
        .filter(p -> p.startsWith("state=")).map(p -> URLDecoder.decode(p.substring(6), StandardCharsets.UTF_8)).findFirst().orElseThrow();
  }
  long sources() { return jdbc.queryForObject("SELECT count(*) FROM linked_identities WHERE account_id=:a AND status='active'", Map.of("a", account), Long.class); }
  void error(Runnable action, String error) { assertThatThrownBy(action::run).isInstanceOf(LinkedOAuthException.class).hasMessage(error); }

  @Test void s256StartStagesEncryptedResultAndConsumesAtomicallyOnce() {
    var start = start();
    assertThat(start.expiresAt()).isEqualTo(NOW.plusSeconds(300));
    assertThat(start.authorizationUrl()).contains("code_challenge_method=S256", "redirect_uri=").doesNotContain(VERIFIER);
    var result = callback(start);
    assertThat(sources()).isZero();
    String row = jdbc.queryForObject("SELECT row_to_json(a)::text FROM linked_oauth_attempts a WHERE id=:id", Map.of("id", start.attemptId()), String.class);
    assertThat(row).doesNotContain(state(start), result.resultHandle(), "secret-access", "secret-refresh", VERIFIER, "provider-code");
    assertThat(consume(start, result)).isEqualTo("success");
    assertThat(sources()).isEqualTo(1);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM verification_source_sync_targets WHERE account_id=:a AND verified", Map.of("a", account), Long.class)).isEqualTo(1);
    error(() -> consume(start, result), "result_consumed");
    error(() -> callback(start), "invalid_link_attempt");
    assertThat(exchanges).hasValue(1);
  }
  @Test void wrongStateProviderProofAndOriginalCredentialCannotConsumeValidResult() {
    var start = start();
    error(() -> service.callback("twitch", "x".repeat(43), "code", null), "invalid_link_attempt");
    error(() -> service.callback("youtube", state(start), "code", null), "invalid_link_attempt");
    assertThat(exchanges).hasValue(0);
    var result = callback(start);
    for (var wrong : List.of(
        new TokenClaims(UUID.randomUUID().toString(), profile.toString(), List.of(), "free", claims.expiresAt(), claims.jti(), "regular", 7),
        new TokenClaims(account.toString(), UUID.randomUUID().toString(), List.of(), "free", claims.expiresAt(), claims.jti(), "regular", 7),
        new TokenClaims(account.toString(), profile.toString(), List.of(), "free", claims.expiresAt(), "refreshed-jti", "regular", 7),
        new TokenClaims(account.toString(), profile.toString(), List.of(), "free", claims.expiresAt(), claims.jti(), "regular", 8))) {
      error(() -> service.consume(wrong, "twitch", start.attemptId(), result.resultHandle(), VERIFIER), "invalid_link_attempt");
    }
    error(() -> service.consume(claims, "twitch", start.attemptId(), result.resultHandle(), "w".repeat(43)), "invalid_link_attempt");
    assertThat(sources()).isZero();
    assertThat(consume(start, result)).isEqualTo("success");
  }
  @Test void expiryEqualityUsesBothOriginalJwtAndSixtySecondResultCap() {
    claims = new TokenClaims(account.toString(), profile.toString(), List.of(), "free", NOW.plusSeconds(40), claims.jti(), "regular", 7);
    var start = start(); var result = callback(start);
    assertThat(start.expiresAt()).isEqualTo(claims.expiresAt());
    time.set(NOW.plusSeconds(40)); error(() -> consume(start, result), "result_expired");
    time.set(NOW); claims = new TokenClaims(account.toString(), profile.toString(), List.of(), "free", NOW.plusSeconds(600), claims.jti(), "regular", 7);
    var second = start(); var secondResult = callback(second);
    time.set(NOW.plusSeconds(60)); error(() -> consume(second, secondResult), "result_expired");
    service.cleanup();
    assertThat(jdbc.queryForObject("SELECT count(*) FROM linked_oauth_attempts WHERE account_id=:a AND (provider_secret IS NOT NULL OR result_secret IS NOT NULL)", Map.of("a", account), Long.class)).isZero();
    assertThat(sources()).isZero();
  }
  @Test void revokedOriginalJtiAndChangedEpochAreCheckedBeforeExchangeAndConsume() {
    var start = start(); when(blacklist.isRevoked(claims.jti())).thenReturn(true);
    error(() -> callback(start), "invalid_link_attempt"); assertThat(exchanges).hasValue(0);
    when(blacklist.isRevoked(claims.jti())).thenReturn(false);
    var next = start(); var result = callback(next);
    jdbc.update("UPDATE accounts SET session_epoch=8 WHERE id=:a", Map.of("a", account));
    error(() -> consume(next, result), "invalid_link_attempt"); assertThat(sources()).isZero();
  }
  @Test void cancelSupersessionAndProviderErrorsNeverChangeSources() {
    var old = start(); var current = start();
    error(() -> callback(old), "invalid_link_attempt");
    service.cancel(claims, "twitch", current.attemptId());
    error(() -> callback(current), "invalid_link_attempt");
    var denied = start();
    var result = service.callback("twitch", state(denied), null, "access_denied");
    assertThat(consume(denied, result)).isEqualTo("cancelled");
    var failed = start(); var failure = service.callback("twitch", state(failed), null, "unsafe-provider-description");
    assertThat(consume(failed, failure)).isEqualTo("provider_error");
    assertThat(sources()).isZero(); assertThat(exchanges).hasValue(0);
  }
  @Test void callbackExchangeIsClaimedOnceAndCancelWinsDuringNetworkCall() throws Exception {
    var entered = new CountDownLatch(1); var release = new CountDownLatch(1);
    service = service((p,c,r,v) -> {
      exchanges.incrementAndGet(); entered.countDown();
      try { assertThat(release.await(10, TimeUnit.SECONDS)).isTrue(); } catch (InterruptedException e) { throw new AssertionError(e); }
      return new LinkedOAuthProvider.Result("success", "id", "login", "secret-access", null);
    });
    var start = start(); var workers = Executors.newSingleThreadExecutor();
    try {
      var pending = workers.submit(() -> callback(start));
      assertThat(entered.await(10, TimeUnit.SECONDS)).isTrue();
      error(() -> callback(start), "invalid_link_attempt");
      service.cancel(claims, "twitch", start.attemptId()); release.countDown();
      assertThatThrownBy(() -> pending.get(10, TimeUnit.SECONDS)).hasCauseInstanceOf(LinkedOAuthException.class);
      assertThat(exchanges).hasValue(1); assertThat(sources()).isZero();
    } finally { release.countDown(); workers.shutdownNow(); assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue(); }
  }
  @Test void concurrentConsumeCommitsOneSource() throws Exception {
    var start = start(); var result = callback(start);
    var pool = Executors.newFixedThreadPool(2);
    try {
      var first = pool.submit(() -> { try { return consume(start,result); } catch (LinkedOAuthException e) { return e.getMessage(); } });
      var second = pool.submit(() -> { try { return consume(start,result); } catch (LinkedOAuthException e) { return e.getMessage(); } });
      assertThat(List.of(first.get(10,TimeUnit.SECONDS),second.get(10,TimeUnit.SECONDS))).containsExactlyInAnyOrder("success", "result_consumed");
      assertThat(sources()).isEqualTo(1);
    } finally { pool.shutdownNow(); assertThat(pool.awaitTermination(10, TimeUnit.SECONDS)).isTrue(); }
  }
  @Test void unavailableConfigurationAndPlainProofCannotStart() {
    error(() -> service.start(claims, "youtube", "web", LinkedOAuthService.s256(VERIFIER), "S256"), "oauth_unavailable");
    error(() -> service.start(claims, "twitch", "web", VERIFIER, "plain"), "invalid_link_attempt");
    error(() -> service.start(claims, "twitch", "https://attacker.test", LinkedOAuthService.s256(VERIFIER), "S256"), "oauth_unavailable");
  }

  @Test void sourceConflictAndFailureAfterSourceWriteRollBackResultAndSource() {
    var repository=new JdbcLinkedIdentityRepository(jdbc);
    var existing=repository.linkActive(account,UUID.randomUUID(),"twitch","legacy-"+account,"legacy",null,null);
    var start=start(); var result=callback(start);
    error(() -> consume(start,result),"linked_account_profile_conflict");
    assertThat(repository.listActiveByAccount(account).getFirst().version()).isEqualTo(existing.version());
    assertThat(jdbc.queryForObject("SELECT status FROM linked_oauth_attempts WHERE id=:id",Map.of("id",start.attemptId()),String.class)).isEqualTo("ready");
    repository.revokeIfUnchanged(existing);
    // Fail the second authority check after linkActive has written both source and sync target.
    doNothing().doThrow(new IllegalStateException("User unavailable")).when(profiles)
        .validateOwnedSwitchable(account,profile,profile,"free");
    error(() -> consume(start,result),"invalid_link_attempt");
    assertThat(sources()).isZero();
    assertThat(jdbc.queryForObject("SELECT status FROM linked_oauth_attempts WHERE id=:id",Map.of("id",start.attemptId()),String.class)).isEqualTo("ready");
    reset(profiles);
    assertThat(consume(start,result)).isEqualTo("success");
  }
  @ParameterizedTest @ValueSource(strings={"revoked","epoch","profile","expired","superseded"})
  void delayedExchangeRechecksAuthoritiesBeforePublishingResult(String mutation) throws Exception {
    var entered=new CountDownLatch(1); var release=new CountDownLatch(1);
    service=service((p,c,r,v)->{
      entered.countDown(); try { assertThat(release.await(10,TimeUnit.SECONDS)).isTrue(); } catch(InterruptedException e){throw new AssertionError(e);}
      return new LinkedOAuthProvider.Result("success","delayed-"+account,"partner","secret",null);
    });
    var start=start(); var pool=Executors.newSingleThreadExecutor();
    try {
      var future=pool.submit(()->callback(start)); assertThat(entered.await(10,TimeUnit.SECONDS)).isTrue();
      switch(mutation) {
        case "revoked" -> when(blacklist.isRevoked(claims.jti())).thenReturn(true);
        case "epoch" -> jdbc.update("UPDATE accounts SET session_epoch=8 WHERE id=:a",Map.of("a",account));
        case "profile" -> doThrow(new IllegalStateException("profile deleted or unavailable")).when(profiles).validateOwnedSwitchable(account,profile,profile,"free");
        case "expired" -> time.set(NOW.plusSeconds(300));
        case "superseded" -> start();
      }
      release.countDown(); assertThatThrownBy(()->future.get(10,TimeUnit.SECONDS)).hasCauseInstanceOf(LinkedOAuthException.class);
      assertThat(sources()).isZero();
      assertThat(jdbc.queryForObject("SELECT result_secret IS NULL AND result_hash IS NULL FROM linked_oauth_attempts WHERE id=:id",Map.of("id",start.attemptId()),Boolean.class)).isTrue();
    } finally {release.countDown();pool.shutdownNow();assertThat(pool.awaitTermination(10,TimeUnit.SECONDS)).isTrue();}
  }
  @Test void attemptExpiryEqualityPreventsExchangeAndProfileOutageFailsClosed() {
    var start=start();time.set(NOW.plusSeconds(300));error(()->callback(start),"result_expired");assertThat(exchanges).hasValue(0);
    time.set(NOW);doThrow(new IllegalStateException("User unavailable")).when(profiles).validateOwnedSwitchable(account,profile,profile,"free");
    error(this::start,"invalid_link_attempt");
  }
  @Test void wrongHandleProviderAndChangedRegistrationPreserveValidResult() {
    var start=start();var result=callback(start);
    error(()->service.consume(claims,"twitch",start.attemptId(),"x".repeat(43),VERIFIER),"invalid_link_attempt");
    error(()->service.consume(claims,"youtube",start.attemptId(),result.resultHandle(),VERIFIER),"invalid_link_attempt");
    var original=service;
    settings=new LinkedOAuthSettings("changed",settings.attemptTtl(),settings.resultTtl(),settings.encryptionKey(),settings.providers());
    service=service((p,c,r,v)->null);error(()->consume(start,result),"invalid_link_attempt");
    service=original;assertThat(consume(start,result)).isEqualTo("success");
  }
  @Test void authenticatedCiphertextCorruptionFailsWithoutConsumingAndCancelledReadyClearsSecrets() {
    var start=start();var result=callback(start);
    byte[] cipher=jdbc.queryForObject("SELECT result_secret FROM linked_oauth_attempts WHERE id=:id",Map.of("id",start.attemptId()),byte[].class);
    byte[] corrupt=cipher.clone();corrupt[corrupt.length-1]^=1;
    jdbc.update("UPDATE linked_oauth_attempts SET result_secret=:secret WHERE id=:id",Map.of("secret",corrupt,"id",start.attemptId()));
    error(()->consume(start,result),"invalid_link_attempt");assertThat(sources()).isZero();
    jdbc.update("UPDATE linked_oauth_attempts SET result_secret=:secret WHERE id=:id",Map.of("secret",cipher,"id",start.attemptId()));
    service.cancel(claims,"twitch",start.attemptId());error(()->consume(start,result),"invalid_link_attempt");
    assertThat(jdbc.queryForObject("SELECT result_secret IS NULL AND provider_secret IS NULL FROM linked_oauth_attempts WHERE id=:id",Map.of("id",start.attemptId()),Boolean.class)).isTrue();
  }
  @Test void providerDenialErrorAndCancelPreserveExistingIdentityAndRevision() {
    var repo=new JdbcLinkedIdentityRepository(jdbc);
    var source=repo.linkActive(account,profile,"twitch","retained-"+account,"retained",null,null);
    for(String error:List.of("access_denied","provider-error")) {
      var start=start();var result=service.callback("twitch",state(start),null,error);consume(start,result);
      assertThat(repo.listActiveByAccount(account).getFirst()).isEqualTo(source);
    }
    var next=start();service.cancel(claims,"twitch",next.attemptId());assertThat(repo.listActiveByAccount(account).getFirst()).isEqualTo(source);
  }
  @Test void youtubeQualificationUnknownPreservesHistoricalSourceWithoutProxyCall() throws Exception {
    var requests=new AtomicInteger();var server=com.sun.net.httpserver.HttpServer.create(new java.net.InetSocketAddress("127.0.0.1",0),0);
    server.createContext("/youtube/v3/channels",x->{
      requests.incrementAndGet();byte[] data="{\"items\":[{\"id\":\"old\",\"status\":{\"longUploadsStatus\":\"disallowed\"}}]}".getBytes(StandardCharsets.UTF_8);
      x.sendResponseHeaders(200,data.length);try(var out=x.getResponseBody()){out.write(data);}
    });server.start();
    try {
      var repo=new JdbcLinkedIdentityRepository(jdbc);
      var original=repo.linkActive(account,profile,"youtube","historical-"+account,"historical","old-token".getBytes(StandardCharsets.US_ASCII),null);
      var legacy=new voice.backend.auth.service.LinkedAccountsService(new voice.backend.auth.userdb.NoOpUserVerificationSync(),repo,new voice.backend.auth.config.AuthProperties.OAuth());
      legacy.setYoutubeEndpointsForTests("http://127.0.0.1:"+server.getAddress().getPort(),"http://127.0.0.1:"+server.getAddress().getPort()+"/token");
      legacy.refreshVerificationStatuses();
      assertThat(repo.listActiveByAccount(account)).usingRecursiveComparison().isEqualTo(List.of(original));
      assertThat(requests).hasValue(0);
    } finally {server.stop(0);}
  }
}
