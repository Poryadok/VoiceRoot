package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;
import voice.backend.auth.userdb.InMemoryPhoneHashResolver;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;

class AuthServiceRestoreRaceTest {
  private static final Instant NOW = Instant.now();
  private static final Clock CLOCK =
      Clock.fixed(NOW.plus(Duration.ofDays(1)), ZoneOffset.UTC);

  @Test
  void repositoryDoesNotReactivateAnAccountOutsideDeletedState() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("suspended@example.com", null, "hash", "regular");
    accounts.setStatus(account.id(), "suspended");

    accounts.restoreDeleted(account.id());

    Account afterRestoreAttempt = accounts.findById(account.id().toString()).orElseThrow();
    assertThat(afterRestoreAttempt.status()).isEqualTo("suspended");
    assertThat(afterRestoreAttempt.deletedAt()).isNull();
  }

  @Test
  void duplicateRestoreDeliveriesIssueOnlyOneSessionAndRestoredEvent() throws Exception {
    GatedDeletedReadAccountRepository accounts = new GatedDeletedReadAccountRepository();
    Account account = accounts.create("restore-race@example.com", null, "hash", "regular");
    accounts.markDeleted(account.id(), NOW);
    InMemoryRefreshTokenRepository refreshTokens = new InMemoryRefreshTokenRepository();
    AuthEventPublisher events = mock();
    AccountRestoreTokenStore duplicateDelivery = new DuplicateDeliveryTokenStore(account.id());
    AuthService first = service(accounts, refreshTokens, duplicateDelivery, events);
    AuthService second = service(accounts, refreshTokens, duplicateDelivery, events);

    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      List<Future<AuthSession>> results =
          List.of(workers.submit(() -> first.restoreAccount("same-token")), workers.submit(() -> second.restoreAccount("same-token")));
      List<Throwable> failures = new java.util.ArrayList<>();
      int successfulSessions = 0;
      for (Future<AuthSession> result : results) {
        try {
          result.get();
          successfulSessions++;
        } catch (java.util.concurrent.ExecutionException ex) {
          failures.add(ex.getCause());
        }
      }

      assertThat(successfulSessions).isEqualTo(1);
      assertThat(failures).hasSize(1);
      assertThat(failures.getFirst()).isInstanceOf(AuthException.class);
      assertThat(failures.getFirst().getMessage()).isEqualTo("validation_failed");
    } finally {
      workers.shutdownNow();
    }

    assertThat(refreshTokens.listActiveByAccount(account.id())).hasSize(1);
    verify(events, times(1)).publishAccountRestored(account.id());
  }

  private AuthService service(
      InMemoryAccountRepository accounts,
      InMemoryRefreshTokenRepository refreshTokens,
      AccountRestoreTokenStore restoreTokens,
      AuthEventPublisher events) {
    InMemoryPrimaryProfileProvisioner profiles = new InMemoryPrimaryProfileProvisioner();
    return new AuthService(
        accounts,
        refreshTokens,
        new RefreshTokenCodec(),
        new BCryptPasswordHasher(),
        JwtService.forTests("voice-auth", "voice-client", "restore-race-key", Duration.ofMinutes(15), CLOCK),
        new InMemoryTokenBlacklist(CLOCK),
        new TotpService(memoryTotpProperties()),
        new BackupCodeService(new InMemoryBackupCodeRepository()),
        CLOCK,
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

  private static final class DuplicateDeliveryTokenStore implements AccountRestoreTokenStore {
    private final UUID accountId;

    private DuplicateDeliveryTokenStore(UUID accountId) {
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

  private static final class GatedDeletedReadAccountRepository extends InMemoryAccountRepository {
    private final CyclicBarrier deletedReads = new CyclicBarrier(2);

    @Override
    public Optional<Account> findById(String id) {
      Optional<Account> account = super.findById(id);
      if (account.filter(found -> "deleted".equals(found.status())).isPresent()) {
        try {
          deletedReads.await();
        } catch (Exception ex) {
          throw new AssertionError(ex);
        }
      }
      return account;
    }
  }
}
