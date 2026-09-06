package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.lang.reflect.Constructor;
import java.lang.reflect.InvocationHandler;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Arrays;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;
import voice.backend.auth.userdb.InMemoryPhoneHashResolver;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;

class AccountRestoreTransitionExpiryTest {
  private static final Instant DELETED_AT = Instant.parse("2026-04-01T10:00:00Z");
  private static final Instant CUTOFF = DELETED_AT.plus(AuthService.ACCOUNT_RESTORE_GRACE);
  private static final Instant PRECHECK = CUTOFF.minusNanos(1);
  private static final Instant AFTER_CUTOFF = CUTOFF.plusNanos(1);

  @Test
  void restoreRejectsExpiryCrossedBetweenPrecheckAndRepositoryClock() throws Exception {
    AccountRepository accounts =
        TransitionAwareAccountRepository.create(Clock.fixed(AFTER_CUTOFF, ZoneOffset.UTC));
    Account account = accounts.create("transition-expiry@example.com", null, "hash", "regular");
    accounts.markDeleted(account.id(), DELETED_AT);
    AuthEventPublisher events = mock();
    RefreshTokenRepository refreshTokens = mock();
    AuthService service =
        service(
            accounts,
            refreshTokens,
            new FixedRestoreTokenStore(account.id()),
            events,
            Clock.fixed(PRECHECK, ZoneOffset.UTC));

    assertThatThrownBy(() -> service.restoreAccount("restore-token"))
        .isInstanceOf(AuthException.class)
        .hasMessage("validation_failed");

    Account unchanged = accounts.findById(account.id().toString()).orElseThrow();
    assertThat(unchanged.status()).isEqualTo("deleted");
    assertThat(unchanged.deletedAt()).isEqualTo(DELETED_AT);
    verifyNoInteractions(refreshTokens);
    verifyNoInteractions(events);
  }

  @Test
  void restoreAtExactTransitionCutoffRemainsAllowed() throws Exception {
    AccountRepository accounts =
        TransitionAwareAccountRepository.create(Clock.fixed(CUTOFF, ZoneOffset.UTC));
    Account account = accounts.create("transition-boundary@example.com", null, "hash", "regular");
    accounts.markDeleted(account.id(), DELETED_AT);
    AuthEventPublisher events = mock();
    RefreshTokenRepository refreshTokens = mock();
    AuthSession session =
        service(
                accounts,
                refreshTokens,
                new FixedRestoreTokenStore(account.id()),
                events,
                Clock.fixed(CUTOFF, ZoneOffset.UTC))
            .restoreAccount("restore-token");

    assertThat(session.accountId()).isEqualTo(account.id().toString());
    assertThat(accounts.findById(account.id().toString()).orElseThrow())
        .extracting(Account::status, Account::deletedAt)
        .containsExactly("active", null);
    verify(refreshTokens).create(
        org.mockito.ArgumentMatchers.eq(account.id()),
        org.mockito.ArgumentMatchers.anyString(),
        org.mockito.ArgumentMatchers.eq("{}"),
        org.mockito.ArgumentMatchers.anyString(),
        org.mockito.ArgumentMatchers.any(),
        org.mockito.ArgumentMatchers.eq(CUTOFF));
    verify(events).publishAccountRestored(account.id());
  }

  private AuthService service(
      AccountRepository accounts,
      RefreshTokenRepository refreshTokens,
      AccountRestoreTokenStore restoreTokens,
      AuthEventPublisher events,
      Clock clock) {
    InMemoryPrimaryProfileProvisioner profiles = new InMemoryPrimaryProfileProvisioner();
    return new AuthService(
        accounts,
        refreshTokens,
        new RefreshTokenCodec(),
        new BCryptPasswordHasher(),
        JwtService.forTests("voice-auth", "voice-client", "restore-transition-key", Duration.ofMinutes(15), clock),
        new InMemoryTokenBlacklist(clock),
        new TotpService(memoryTotpProperties()),
        new BackupCodeService(new InMemoryBackupCodeRepository()),
        clock,
        Duration.ofDays(30),
        profiles,
        new InMemoryPhoneHashResolver(accounts, profiles),
        new InMemorySubscriptionTierStore(),
        new NoOpProfileSwitchValidator(),
        new InMemoryE2EKeyBackupRepository(),
        events,
        new SimpleMeterRegistry(),
        restoreTokens,
        new NoopMailSender(),
        new InMemorySessionEpochFloorStore());
  }

  private static AuthProperties memoryTotpProperties() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.MEMORY);
    return properties;
  }

  private static final class FixedRestoreTokenStore implements AccountRestoreTokenStore {
    private final UUID accountId;

    private FixedRestoreTokenStore(UUID accountId) {
      this.accountId = accountId;
    }

    @Override
    public void store(String token, UUID accountId, Duration ttl) {}

    @Override
    public Optional<UUID> peek(String token) {
      return Optional.of(accountId);
    }

    @Override
    public Optional<UUID> consume(String token) {
      return Optional.of(accountId);
    }
  }

  private static final class TransitionAwareAccountRepository implements InvocationHandler {
    private final InMemoryAccountRepository delegate;

    private TransitionAwareAccountRepository(Clock repositoryClock) throws Exception {
      this.delegate = inMemoryRepositoryWithClock(repositoryClock);
    }

    private static AccountRepository create(Clock repositoryClock) throws Exception {
      TransitionAwareAccountRepository handler =
          new TransitionAwareAccountRepository(repositoryClock);
      return (AccountRepository)
          Proxy.newProxyInstance(
              AccountRepository.class.getClassLoader(),
              new Class<?>[] {AccountRepository.class},
              handler);
    }

    @Override
    public Object invoke(Object proxy, Method method, Object[] args) throws Throwable {
      if (method.getName().equals("restoreDeleted")) {
        UUID accountId = (UUID) args[0];
        assertThat(args).hasSize(1);
        // The repository samples its own transition-time clock; no caller-supplied
        // pre-check instant is available to become a stale correctness source.
        return delegate.restoreDeleted(accountId);
      }
      try {
        return method.invoke(delegate, args == null ? new Object[0] : args);
      } catch (InvocationTargetException ex) {
        throw ex.getCause();
      }
    }

    private static InMemoryAccountRepository inMemoryRepositoryWithClock(Clock clock)
        throws Exception {
      Optional<Constructor<?>> constructor =
          Arrays.stream(InMemoryAccountRepository.class.getDeclaredConstructors())
              .filter(
                  candidate ->
                      Arrays.equals(candidate.getParameterTypes(), new Class<?>[] {Clock.class}))
              .findFirst();
      assertThat(constructor)
          .as("in-memory repository needs a Clock seam for transition-time expiry")
          .isPresent();
      constructor.orElseThrow().setAccessible(true);
      return (InMemoryAccountRepository) constructor.orElseThrow().newInstance(clock);
    }
  }
}
