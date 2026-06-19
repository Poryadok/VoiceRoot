package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import com.nimbusds.jwt.SignedJWT;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;

class SubscriptionTierAuthServiceTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-06-13T10:00:00Z"), ZoneOffset.UTC);

  @Test
  void loginIssuesJwtWithPremiumTierAfterSubscriptionActivated() throws Exception {
    InMemorySubscriptionTierStore tierStore = new InMemorySubscriptionTierStore();
    AuthService service = service(CLOCK, tierStore);
    AuthSession registered =
        service.register(
            new RegisterCommand("premium@example.com", null, "Correct horse battery staple", false, "{}"));
    tierStore.setTier(UUID.fromString(registered.accountId()), "premium");

    // Phase 12: simulate subscription.plan_started for this account before login.
    AuthSession login =
        service.login(
            new LoginCommand("premium@example.com", null, "Correct horse battery staple", null, "{}"));

    assertThat(SignedJWT.parse(login.accessToken()).getJWTClaimsSet().getStringClaim("subscription_tier"))
        .isEqualTo("premium");
  }

  private static AuthService service(Clock clock, SubscriptionTierResolver tierResolver) {
    JwtService jwt =
        JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), clock);
    var accounts = new InMemoryAccountRepository();
    var profiles = new voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner();
    return new AuthService(
        accounts,
        new InMemoryRefreshTokenRepository(),
        new RefreshTokenCodec(),
        new BCryptPasswordHasher(),
        jwt,
        new InMemoryTokenBlacklist(clock),
        new TotpService(new voice.backend.auth.config.AuthProperties()),
        new BackupCodeService(new InMemoryBackupCodeRepository()),
        clock,
        Duration.ofDays(30),
        profiles,
        new voice.backend.auth.userdb.InMemoryPhoneHashResolver(accounts, profiles),
        tierResolver,
        new voice.backend.auth.userdb.NoOpProfileSwitchValidator(),
        new voice.backend.auth.repository.InMemoryE2EKeyBackupRepository(),
        new NoopAuthEventPublisher());
  }
}
