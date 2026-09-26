package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.ProfileSwitchValidator;

class AuthServiceRegistrationIntentTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-09-26T12:00:00Z"), ZoneOffset.UTC);
  private static final String PASSWORD = "Correct horse battery staple";

  private static final class Harness {
    final InMemoryAccountRepository accounts = spy(new InMemoryAccountRepository(CLOCK));
    final InMemoryRefreshTokenRepository refresh = spy(new InMemoryRefreshTokenRepository());
    final BCryptPasswordHasher passwords = mock(BCryptPasswordHasher.class);
    final AuthService service;

    Harness() {
      when(passwords.hash(anyString())).thenReturn("test-password-hash");
      service = new AuthService(accounts, refresh, new RefreshTokenCodec(), passwords,
          JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK),
          new InMemoryTokenBlacklist(CLOCK), mock(TotpService.class), mock(BackupCodeService.class), CLOCK,
          Duration.ofDays(30), new InMemoryPrimaryProfileProvisioner(), mock(PhoneHashResolver.class),
          new InMemorySubscriptionTierStore(), mock(ProfileSwitchValidator.class),
          new InMemoryE2EKeyBackupRepository(), mock(AuthEventPublisher.class), new SimpleMeterRegistry(),
          new InMemoryAccountRestoreTokenStore(), mock(MailSender.class), new InMemorySessionEpochFloorStore());
    }

    void noAccountOrRefreshCreated() {
      verify(accounts, never()).create(any(), any(), any(), any());
      verify(accounts, never()).createRegularEmailPending(any(), any());
      verifyNoInteractions(refresh);
    }
  }

  @Test
  void legacyFiveArgumentCommandKeepsOrdinaryPendingEmailRegistrationWorking() {
    var harness = new Harness();
    var command = new RegisterCommand("ordinary@example.test", null, PASSWORD, false, "{}");
    assertThat(command.registrationIntentId()).isNull();
    var session = harness.service.register(command);
    assertThat(session.accountType()).isEqualTo("guest");
    assertThat(session.accessToken()).isNotBlank();
    assertThat(session.refreshToken()).isNotBlank();
    assertThat(harness.accounts.isRegularEmailVerificationPending(UUID.fromString(session.accountId()))).isTrue();
    assertThat(harness.service.validate(session.accessToken()).userId()).isEqualTo(session.accountId());
  }

  @Test
  void guestWithIntentIsRejectedBeforeAccountCreationEvenWithPreparerConfigured() {
    var harness = new Harness();
    var preparer = mock(RegistrationSessionEpochPreparer.class);
    harness.service.configureRegistrationSessionEpochPreparer(preparer);
    assertThatThrownBy(() -> harness.service.register(new RegisterCommand(
        null, null, PASSWORD, true, "{}", UUID.randomUUID())))
        .isInstanceOf(AuthException.class).hasMessage("validation_failed");
    harness.noAccountOrRefreshCreated();
    verifyNoInteractions(preparer);
  }

  @Test
  void intentCannotSilentlyFallBackToMemoryRegistrationWithoutPreparer() {
    var harness = new Harness();
    assertThatThrownBy(() -> harness.service.register(new RegisterCommand(
        "new-target@example.test", null, PASSWORD, false, "{}", UUID.randomUUID())))
        .isInstanceOf(AuthException.class).hasMessage("auth_unavailable");
    harness.noAccountOrRefreshCreated();
  }

  @Test
  void sixthCommandFieldReachesTransactionalPreparerWithNormalPendingEmailSemantics() {
    var harness = new Harness();
    var preparer = mock(RegistrationSessionEpochPreparer.class);
    UUID intent = UUID.randomUUID();
    when(preparer.prepare("new-target@example.test", null, "test-password-hash", "guest", true, intent))
        .thenThrow(new AuthException("validation_failed"));
    harness.service.configureRegistrationSessionEpochPreparer(preparer);
    assertThatThrownBy(() -> harness.service.register(new RegisterCommand(
        " New-Target@Example.Test ", null, PASSWORD, false, "{}", intent)))
        .isInstanceOf(AuthException.class).hasMessage("validation_failed");
    verify(preparer).prepare("new-target@example.test", null, "test-password-hash", "guest", true, intent);
    harness.noAccountOrRefreshCreated();
  }
}
