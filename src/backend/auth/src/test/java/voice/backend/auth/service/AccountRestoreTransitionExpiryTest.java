package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.lang.reflect.InvocationHandler;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneId;
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
  void restoreRepositoryContractCarriesTransitionInstantForAtomicExpiryFence() {
    Optional<Method> conditionalRestore =
        Arrays.stream(AccountRepository.class.getMethods())
            .filter(method -> method.getName().equals("restoreDeleted"))
            .filter(method -> Arrays.equals(method.getParameterTypes(), new Class<?>[] {UUID.class, Instant.class}))
            .findFirst();

    assertThat(conditionalRestore)
        .as("restoreDeleted must fence the 30-day cutoff at the atomic transition")
        .isPresent();
    assertThat(conditionalRestore.orElseThrow().getReturnType()).isEqualTo(boolean.class);
  }

  @Test
  void restoreRejectsExpiryCrossedBetweenPrecheckAndAtomicTransition() {
    AccountRepository accounts =
        TransitionAwareAccountRepository.create(AFTER_CUTOFF, true);
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
            new SequencedClock(PRECHECK, AFTER_CUTOFF));

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
  void restoreAtExactTransitionCutoffRemainsAllowed() {
    AccountRepository accounts =
        TransitionAwareAccountRepository.create(CUTOFF, false);
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
                new SequencedClock(PRECHECK, CUTOFF))
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
    public Optional<UUID> consume(String token) {
      return Optional.of(accountId);
    }
  }

  private static final class SequencedClock extends Clock {
    private final Instant[] instants;
    private int index;

    private SequencedClock(Instant... instants) {
      this.instants = instants;
    }

    @Override
    public ZoneId getZone() {
      return ZoneId.of("UTC");
    }

    @Override
    public Clock withZone(ZoneId zone) {
      return this;
    }

    @Override
    public synchronized Instant instant() {
      return instants[Math.min(index++, instants.length - 1)];
    }
  }

  private static final class TransitionAwareAccountRepository implements InvocationHandler {
    private final InMemoryAccountRepository delegate = new InMemoryAccountRepository();
    private final Instant transitionInstant;
    private final boolean expiredAtTransition;

    private TransitionAwareAccountRepository(Instant transitionInstant, boolean expiredAtTransition) {
      this.transitionInstant = transitionInstant;
      this.expiredAtTransition = expiredAtTransition;
    }

    private static AccountRepository create(Instant transitionInstant, boolean expiredAtTransition) {
      TransitionAwareAccountRepository handler =
          new TransitionAwareAccountRepository(transitionInstant, expiredAtTransition);
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
        if (args.length == 2) {
          assertThat(args[1]).isEqualTo(transitionInstant);
          if (expiredAtTransition) {
            return false;
          }
        }
        // The legacy UUID-only method models the current TOCTOU bug: it restores
        // without receiving or checking the transition instant.
        return delegate.restoreDeleted(accountId);
      }
      try {
        return method.invoke(delegate, args == null ? new Object[0] : args);
      } catch (InvocationTargetException ex) {
        throw ex.getCause();
      }
    }
  }
}
