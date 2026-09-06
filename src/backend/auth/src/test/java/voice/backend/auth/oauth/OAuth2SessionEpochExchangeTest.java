package voice.backend.auth.oauth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.AccountRestoreTokenStore;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.BackupCodeService;
import voice.backend.auth.service.InMemoryAccountRestoreTokenStore;
import voice.backend.auth.service.InMemorySubscriptionTierStore;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.service.TotpService;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.userdb.InMemoryPhoneHashResolver;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;

class OAuth2SessionEpochExchangeTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-05-01T10:00:00Z"), ZoneOffset.UTC);
  private static final String CLIENT = "epoch-client";
  private static final String REDIRECT = "http://localhost:9082/callback";
  private static final String VERIFIER = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~";

  @Test
  void floorFailureLeavesCodeForHealthyRedisAheadRetryAndDirectIssuerFailsClosed() throws Exception {
    Harness harness = new Harness(new RecordingCodeStore());
    var session = harness.auth.register(new RegisterCommand("oauth-epoch@example.com", null, "Correct horse battery staple", false, "{}"));
    harness.floor.calls = 0;
    harness.accounts.advanceCalls = 0;
    OAuthAuthorizationCode record = harness.record("code-1", session.accountId(), session.profileId());
    harness.codes.save(record, Duration.ofMinutes(1));
    harness.floor.failure = new IllegalStateException("redis down");

    assertThatThrownBy(() -> harness.oauth.exchangeAuthorizationCode(harness.request("code-1")))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(harness.codes.peek("code-1")).contains(record);
    assertThat(harness.codes.consumeCalls).isZero();
    assertThat(harness.floor.calls).isEqualTo(1);

    harness.floor.failure = null;
    harness.floor.result = 7L;
    harness.floor.calls = 0;
    OAuthTokenResponse retry = harness.oauth.exchangeAuthorizationCode(harness.request("code-1"));
    var claims = com.nimbusds.jwt.SignedJWT.parse(retry.accessToken()).getJWTClaimsSet();
    assertThat(claims.getLongClaim("session_epoch")).isEqualTo(7L);
    assertThat(claims.getStringClaim("user_id")).isEqualTo(session.accountId());
    assertThat(claims.getStringClaim("profile_id")).isEqualTo(session.profileId());
    assertThat(claims.getStringClaim("account_type")).isEqualTo(session.accountType());
    var validatedRetry = harness.auth.validate(retry.accessToken());
    assertThat(validatedRetry.userId()).isEqualTo(session.accountId());
    assertThat(validatedRetry.profileId()).isEqualTo(session.profileId());
    assertThat(validatedRetry.normalizedAccountType()).isEqualTo(session.accountType());
    assertThat(harness.floor.calls).isEqualTo(1);
    assertThat(harness.accounts.findById(session.accountId()).orElseThrow().sessionEpoch()).isEqualTo(7L);
    assertThat(harness.accounts.advanceCalls).isEqualTo(1);

    harness.codes.save(harness.record("code-2", session.accountId(), session.profileId()), Duration.ofMinutes(1));
    harness.floor.calls = 0;
    harness.floor.failure = new IllegalStateException("redis down");
    assertThatThrownBy(() -> harness.auth.issueOAuthAccessToken(session.accountId(), session.profileId()))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);
    assertThat(harness.floor.calls).isEqualTo(1);

    harness.floor.failure = null;
    harness.floor.result = 9L;
    harness.floor.calls = 0;
    harness.accounts.advanceCalls = 0;
    String direct = harness.auth.issueOAuthAccessToken(session.accountId(), session.profileId());
    var directClaims = com.nimbusds.jwt.SignedJWT.parse(direct).getJWTClaimsSet();
    assertThat(directClaims.getLongClaim("session_epoch")).isEqualTo(9L);
    assertThat(directClaims.getStringClaim("user_id")).isEqualTo(session.accountId());
    assertThat(directClaims.getStringClaim("profile_id")).isEqualTo(session.profileId());
    assertThat(directClaims.getStringClaim("account_type")).isEqualTo(session.accountType());
    var validatedDirect = harness.auth.validate(direct);
    assertThat(validatedDirect.userId()).isEqualTo(session.accountId());
    assertThat(validatedDirect.profileId()).isEqualTo(session.profileId());
    assertThat(validatedDirect.normalizedAccountType()).isEqualTo(session.accountType());
    assertThat(harness.floor.calls).isEqualTo(1);
    assertThat(harness.accounts.advanceCalls).isEqualTo(1);
  }

  @Test
  void changedConsumedRecordIsRejectedAfterValidPeek() {
    Harness harness = new Harness(new SwappingCodeStore());
    var first = harness.auth.register(new RegisterCommand("oauth-first@example.com", null, "Correct horse battery staple", false, "{}"));
    var second = harness.auth.register(new RegisterCommand("oauth-second@example.com", null, "Correct horse battery staple", false, "{}"));
    OAuthAuthorizationCode peeked = harness.record("code-swap", first.accountId(), first.profileId());
    OAuthAuthorizationCode consumed = harness.record("code-swap", second.accountId(), second.profileId());
    ((SwappingCodeStore) harness.codes).peeked = peeked;
    ((SwappingCodeStore) harness.codes).consumed = consumed;

    assertThatThrownBy(() -> harness.oauth.exchangeAuthorizationCode(harness.request("code-swap")))
        .isInstanceOf(OAuthException.class)
        .hasMessage("invalid_grant");
  }

  @Test
  void concurrentExchangesKeepOneAtomicConsumeWinner() throws Exception {
    Harness harness = new Harness(new RecordingCodeStore());
    var session = harness.auth.register(new RegisterCommand("oauth-race@example.com", null, "Correct horse battery staple", false, "{}"));
    harness.codes.save(harness.record("code-race", session.accountId(), session.profileId()), Duration.ofMinutes(1));
    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      var first = workers.submit(() -> harness.oauth.exchangeAuthorizationCode(harness.request("code-race")));
      var second = workers.submit(() -> harness.oauth.exchangeAuthorizationCode(harness.request("code-race")));
      int success = 0;
      OAuthException loser = null;
      for (var result : List.of(first, second)) try {
        OAuthTokenResponse response = result.get(5, TimeUnit.SECONDS);
        assertThat(com.nimbusds.jwt.SignedJWT.parse(response.accessToken()).getJWTClaimsSet().getStringClaim("user_id")).isEqualTo(session.accountId());
        success++;
      } catch (java.util.concurrent.ExecutionException ex) {
        assertThat(ex.getCause()).isInstanceOf(OAuthException.class);
        loser = (OAuthException) ex.getCause();
      }
      assertThat(success).isEqualTo(1);
      assertThat(loser).isNotNull();
      assertThat(loser.getMessage()).isEqualTo("invalid_grant");
    } finally { workers.shutdownNow(); }
  }

  @Test
  void expiredConsumedRecordIsRejectedAfterValidPeek() {
    Harness harness = new Harness(new SwappingCodeStore());
    var session = harness.auth.register(new RegisterCommand("oauth-expired-swap@example.com", null, "Correct horse battery staple", false, "{}"));
    OAuthAuthorizationCode good = harness.record("code-expired", session.accountId(), session.profileId());
    OAuthAuthorizationCode expired = new OAuthAuthorizationCode("code-expired", session.accountId(), session.profileId(), CLIENT, REDIRECT, PkceVerifier.s256Challenge(VERIFIER), "S256", CLOCK.instant());
    ((SwappingCodeStore) harness.codes).peeked = good;
    ((SwappingCodeStore) harness.codes).consumed = expired;
    assertThatThrownBy(() -> harness.oauth.exchangeAuthorizationCode(harness.request("code-expired"))).isInstanceOf(OAuthException.class).hasMessage("invalid_grant");
  }

  @Test
  void consumedRecordClientRedirectAndPkceBindingsMustMatchPeekedRecord() {
    assertConsumedBindingMismatch(
        good -> new OAuthAuthorizationCode("different-code", good.accountId(), good.profileId(), good.clientId(), good.redirectUri(), good.codeChallenge(), good.codeChallengeMethod(), good.expiresAt()));
    assertConsumedBindingMismatch(
        good -> new OAuthAuthorizationCode(good.code(), good.accountId(), good.profileId(), "other-client", good.redirectUri(), good.codeChallenge(), good.codeChallengeMethod(), good.expiresAt()));
    assertConsumedBindingMismatch(
        good -> new OAuthAuthorizationCode(good.code(), good.accountId(), good.profileId(), good.clientId(), "https://voice.app/other", good.codeChallenge(), good.codeChallengeMethod(), good.expiresAt()));
    assertConsumedBindingMismatch(
        good -> new OAuthAuthorizationCode(good.code(), good.accountId(), good.profileId(), good.clientId(), good.redirectUri(), "different-challenge", good.codeChallengeMethod(), good.expiresAt()));
  }

  private void assertConsumedBindingMismatch(java.util.function.UnaryOperator<OAuthAuthorizationCode> change) {
    Harness harness = new Harness(new SwappingCodeStore());
    var session = harness.auth.register(new RegisterCommand("oauth-binding-" + UUID.randomUUID() + "@example.com", null, "Correct horse battery staple", false, "{}"));
    OAuthAuthorizationCode good = harness.record("code-binding", session.accountId(), session.profileId());
    ((SwappingCodeStore) harness.codes).peeked = good;
    ((SwappingCodeStore) harness.codes).consumed = change.apply(good);
    assertThatThrownBy(() -> harness.oauth.exchangeAuthorizationCode(harness.request(good.code())))
        .isInstanceOf(OAuthException.class)
        .hasMessage("invalid_grant");
  }

  private static final class Harness {
    final RecordingAccounts accounts = new RecordingAccounts();
    final RecordingFloor floor = new RecordingFloor();
    final RecordingCodeStore codes;
    final AuthService auth;
    final OAuth2Service oauth;
    Harness(RecordingCodeStore codes) {
      this.codes = codes;
      AuthProperties properties = properties();
      InMemoryPrimaryProfileProvisioner profiles = new InMemoryPrimaryProfileProvisioner();
      auth = new AuthService(accounts, new InMemoryRefreshTokenRepository(), new RefreshTokenCodec(), new BCryptPasswordHasher(), JwtService.forTests("voice-auth", "voice-client", "key", Duration.ofMinutes(15), CLOCK), new InMemoryTokenBlacklist(CLOCK), new TotpService(memory()), new BackupCodeService(new InMemoryBackupCodeRepository()), CLOCK, Duration.ofDays(30), profiles, new InMemoryPhoneHashResolver(accounts, profiles), new InMemorySubscriptionTierStore(), new NoOpProfileSwitchValidator(), new InMemoryE2EKeyBackupRepository(), new NoopAuthEventPublisher(), new SimpleMeterRegistry(), new InMemoryAccountRestoreTokenStore(), new NoopMailSender(), floor);
      oauth = new OAuth2Service(properties, auth, codes, CLOCK);
    }
    OAuthAuthorizationCode record(String code, String account, String profile) { return new OAuthAuthorizationCode(code, account, profile, CLIENT, REDIRECT, PkceVerifier.s256Challenge(VERIFIER), "S256", CLOCK.instant().plusSeconds(60)); }
    OAuthTokenRequest request(String code) { return new OAuthTokenRequest("authorization_code", code, REDIRECT, CLIENT, VERIFIER, null); }
  }

  private static final class RecordingAccounts extends InMemoryAccountRepository {
    int advanceCalls;
    @Override public synchronized long advanceSessionEpochAtLeast(UUID id, long epoch) { advanceCalls++; return super.advanceSessionEpochAtLeast(id, epoch); }
  }
  private static AuthProperties properties() { AuthProperties p = memory(); p.getOauth().getDeveloperPortal().setEnabled(true); p.getOauth().getDeveloperPortal().setClientId(CLIENT); p.getOauth().getDeveloperPortal().setRedirectUris(List.of(REDIRECT)); return p; }
  private static AuthProperties memory() { AuthProperties p = new AuthProperties(); p.setPersistence(AuthProperties.PersistenceMode.MEMORY); return p; }
  private static class RecordingCodeStore extends InMemoryOAuthAuthorizationCodeStore { int consumeCalls; RecordingCodeStore(){super(CLOCK);} @Override public Optional<OAuthAuthorizationCode> consume(String code){consumeCalls++; return super.consume(code);} }
  private static final class SwappingCodeStore extends RecordingCodeStore { OAuthAuthorizationCode peeked; OAuthAuthorizationCode consumed; @Override public Optional<OAuthAuthorizationCode> peek(String code){return Optional.ofNullable(peeked);} @Override public Optional<OAuthAuthorizationCode> consume(String code){consumeCalls++; return Optional.ofNullable(consumed);} }
  private static final class RecordingFloor implements SessionEpochFloorStore { RuntimeException failure; long result=1; int calls; public long recordAtLeast(UUID id,long epoch){calls++; if(failure!=null)throw failure; return result;} public long requireFloor(UUID id){throw new AssertionError();} }
}
