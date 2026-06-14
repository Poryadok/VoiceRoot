package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jwt.SignedJWT;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import org.junit.jupiter.api.Test;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;

class AuthServiceTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-05-01T10:00:00Z"), ZoneOffset.UTC);

  @Test
  void refreshRotatesTokenAndRejectsReuse() throws Exception {
    AuthService service = service(CLOCK);
    AuthSession login = service.register(new RegisterCommand("first@example.com", null, "Correct horse battery staple", false, "{}"));

    AuthSession refreshed = service.refresh(new RefreshCommand(login.refreshToken(), "{\"platform\":\"web\"}"));

    assertThat(SignedJWT.parse(login.accessToken()).getJWTClaimsSet().getStringClaim("profile_id"))
        .isEqualTo(login.profileId());
    assertThat(SignedJWT.parse(login.accessToken()).getJWTClaimsSet().getStringClaim("user_id"))
        .isEqualTo(login.accountId());
    assertThat(SignedJWT.parse(refreshed.accessToken()).getJWTClaimsSet().getStringClaim("profile_id"))
        .isEqualTo(login.profileId());
    assertThat(SignedJWT.parse(refreshed.accessToken()).getJWTClaimsSet().getStringClaim("user_id"))
        .isEqualTo(login.accountId());

    assertThat(refreshed.accessToken()).isNotEqualTo(login.accessToken());
    assertThat(refreshed.refreshToken()).isNotEqualTo(login.refreshToken());
    assertThat(refreshed.expiresInSeconds()).isEqualTo(900);
    assertThatThrownBy(() -> service.refresh(new RefreshCommand(login.refreshToken(), "{}")))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_revoked");
  }

  @Test
  void refreshRejectsExpiredRevokedUnknownMalformedAndEmptyTokens() {
    AuthService service = service(CLOCK);
    AuthSession session = service.register(new RegisterCommand("second@example.com", null, "Correct horse battery staple", false, "{}"));
    service.logout(new LogoutCommand(session.accessToken(), session.refreshToken()));

    assertThatThrownBy(() -> service.refresh(new RefreshCommand(session.refreshToken(), "{}")))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_revoked");
    assertThatThrownBy(() -> service.refresh(new RefreshCommand("not-a-real-token", "{}")))
        .isInstanceOf(AuthException.class)
        .hasMessage("invalid_token");
    assertThatThrownBy(() -> service.refresh(new RefreshCommand("", "{}")))
        .isInstanceOf(AuthException.class)
        .hasMessage("invalid_token");

    AuthService afterThirtyDays = service(CLOCK);
    AuthSession expiring = afterThirtyDays.register(new RegisterCommand("third@example.com", null, "Correct horse battery staple", false, "{}"));
    AuthService later = afterThirtyDays.withClock(Clock.fixed(Instant.parse("2026-06-01T10:00:01Z"), ZoneOffset.UTC));
    assertThatThrownBy(() -> later.refresh(new RefreshCommand(expiring.refreshToken(), "{}")))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_expired");
  }

  @Test
  void logoutRevokesOneRefreshTokenAndBlacklistsAccessTokenJti() {
    InMemoryTokenBlacklist blacklist = new InMemoryTokenBlacklist(CLOCK);
    AuthService service = service(CLOCK, blacklist);
    AuthSession session = service.register(new RegisterCommand("logout@example.com", null, "Correct horse battery staple", false, "{}"));
    TokenClaims claims = service.validate(session.accessToken());

    service.logout(new LogoutCommand(session.accessToken(), session.refreshToken()));

    assertThat(blacklist.isRevoked(claims.jti())).isTrue();
    assertThat(blacklist.ttl(claims.jti())).isEqualTo(Duration.ofMinutes(15));
    assertThatThrownBy(() -> service.validate(session.accessToken()))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_revoked");
    assertThatThrownBy(() -> service.refresh(new RefreshCommand(session.refreshToken(), "{}")))
        .isInstanceOf(AuthException.class)
        .hasMessage("token_revoked");
  }

  @Test
  void concurrentRefreshAllowsOnlyOneRotation() throws Exception {
    AuthService service = service(CLOCK);
    AuthSession session = service.register(new RegisterCommand("race@example.com", null, "Correct horse battery staple", false, "{}"));

    var pool = java.util.concurrent.Executors.newFixedThreadPool(2);
    try {
      var first = pool.submit(() -> service.refresh(new RefreshCommand(session.refreshToken(), "{}")));
      var second = pool.submit(() -> service.refresh(new RefreshCommand(session.refreshToken(), "{}")));
      int success = 0;
      int revoked = 0;
      for (var future : java.util.List.of(first, second)) {
        try {
          future.get();
          success++;
        } catch (java.util.concurrent.ExecutionException ex) {
          if (ex.getCause() instanceof AuthException auth && "token_revoked".equals(auth.getMessage())) {
            revoked++;
          } else {
            throw ex;
          }
        }
      }
      assertThat(success).isEqualTo(1);
      assertThat(revoked).isEqualTo(1);
    } finally {
      pool.shutdownNow();
    }
  }

  private static AuthService service(Clock clock) {
    return service(clock, new InMemoryTokenBlacklist(clock));
  }

  private static AuthService service(Clock clock, InMemoryTokenBlacklist blacklist) {
    return service(clock, blacklist, new InMemorySubscriptionTierStore());
  }

  private static AuthService service(Clock clock, InMemoryTokenBlacklist blacklist, SubscriptionTierResolver tierResolver) {
    JwtService jwt = JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), clock);
    return new AuthService(
        new InMemoryAccountRepository(),
        new InMemoryRefreshTokenRepository(),
        new RefreshTokenCodec(),
        new BCryptPasswordHasher(),
        jwt,
        blacklist,
        new TotpService(new voice.backend.auth.config.AuthProperties()),
        new BackupCodeService(new InMemoryBackupCodeRepository()),
        clock,
        Duration.ofDays(30),
        new voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner(),
        tierResolver,
        new voice.backend.auth.userdb.NoOpProfileSwitchValidator());
  }
}
