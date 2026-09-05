package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRecord;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.sessionepoch.InMemorySessionEpochFloorStore;
import voice.backend.auth.userdb.InMemoryPhoneHashResolver;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;

class AccountRestoreExpiryTest {
  private static final Instant NOW = Instant.parse("2026-05-01T10:00:00Z");
  private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

  @Test
  void restoreOlderThanThirtyDaysIsInactiveAndConsumesTokenWithoutRestoring() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("expired-restore@example.com", null, "hash", "regular");
    Instant deletedAt = NOW.minus(Duration.ofDays(31));
    accounts.markDeleted(account.id(), deletedAt);
    OneTimeRestoreTokenStore restoreTokens = new OneTimeRestoreTokenStore(account.id());
    RecordingRefreshTokenRepository refreshTokens = new RecordingRefreshTokenRepository();
    AuthEventPublisher events = mock();
    AuthService service = service(accounts, refreshTokens, restoreTokens, events);

    assertThatThrownBy(() -> service.restoreAccount("restore-token"))
        .isInstanceOf(AuthException.class)
        .hasMessage("account_inactive");

    Account unchanged = accounts.findById(account.id().toString()).orElseThrow();
    assertThat(unchanged.status()).isEqualTo("deleted");
    assertThat(unchanged.deletedAt()).isEqualTo(deletedAt);
    assertThat(refreshTokens.createCount).isZero();
    verifyNoInteractions(events);
    assertThat(restoreTokens.consumeCount).isEqualTo(1);

    assertThatThrownBy(() -> service.restoreAccount("restore-token"))
        .isInstanceOf(AuthException.class)
        .hasMessage("invalid_token");
    assertThat(restoreTokens.consumeCount).isEqualTo(2);
    assertThat(accounts.findById(account.id().toString()).orElseThrow().status())
        .isEqualTo("deleted");
    verifyNoInteractions(events);
  }

  @Test
  void restoreAtExactlyThirtyDaysUsesInclusiveBoundaryAndIssuesSession() {
    InMemoryAccountRepository accounts = new InMemoryAccountRepository();
    Account account = accounts.create("boundary-restore@example.com", null, "hash", "regular");
    accounts.markDeleted(account.id(), NOW.minus(AuthService.ACCOUNT_RESTORE_GRACE));
    OneTimeRestoreTokenStore restoreTokens = new OneTimeRestoreTokenStore(account.id());
    RecordingRefreshTokenRepository refreshTokens = new RecordingRefreshTokenRepository();
    AuthEventPublisher events = mock();
    AuthSession session = service(accounts, refreshTokens, restoreTokens, events)
        .restoreAccount("restore-token");

    assertThat(session.accountId()).isEqualTo(account.id().toString());
    assertThat(session.accessToken()).isNotBlank();
    assertThat(session.refreshToken()).isNotBlank();
    Account restored = accounts.findById(account.id().toString()).orElseThrow();
    assertThat(restored.status()).isEqualTo("active");
    assertThat(restored.deletedAt()).isNull();
    assertThat(refreshTokens.createCount).isEqualTo(1);
    verify(events).publishAccountRestored(account.id());
    assertThat(restoreTokens.consumeCount).isEqualTo(1);
  }

  private AuthService service(
      InMemoryAccountRepository accounts,
      RefreshTokenRepository refreshTokens,
      AccountRestoreTokenStore restoreTokens,
      AuthEventPublisher events) {
    InMemoryPrimaryProfileProvisioner profiles = new InMemoryPrimaryProfileProvisioner();
    return new AuthService(
        accounts,
        refreshTokens,
        new RefreshTokenCodec(),
        new BCryptPasswordHasher(),
        JwtService.forTests("voice-auth", "voice-client", "restore-expiry-key", Duration.ofMinutes(15), CLOCK),
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

  private static final class OneTimeRestoreTokenStore implements AccountRestoreTokenStore {
    private final UUID accountId;
    private boolean available = true;
    private int consumeCount;

    private OneTimeRestoreTokenStore(UUID accountId) {
      this.accountId = accountId;
    }

    @Override
    public void store(String token, UUID accountId, Duration ttl) {}

    @Override
    public synchronized Optional<UUID> consume(String token) {
      consumeCount++;
      if (!available) {
        return Optional.empty();
      }
      available = false;
      return Optional.of(accountId);
    }
  }

  private static final class RecordingRefreshTokenRepository implements RefreshTokenRepository {
    private final InMemoryRefreshTokenRepository delegate = new InMemoryRefreshTokenRepository();
    private int createCount;

    @Override
    public RefreshTokenRecord create(
        UUID accountId,
        String tokenHash,
        String deviceInfoJson,
        String accessJti,
        Instant expiresAt,
        Instant now) {
      createCount++;
      return delegate.create(accountId, tokenHash, deviceInfoJson, accessJti, expiresAt, now);
    }

    @Override
    public Optional<RefreshTokenRecord> findByHash(String tokenHash) {
      return delegate.findByHash(tokenHash);
    }

    @Override
    public Optional<RefreshTokenRecord> findById(UUID id) {
      return delegate.findById(id);
    }

    @Override
    public List<RefreshTokenRecord> listActiveByAccount(UUID accountId) {
      return delegate.listActiveByAccount(accountId);
    }

    @Override
    public RefreshTokenRecord revoke(String tokenHash, Instant now) {
      return delegate.revoke(tokenHash, now);
    }

    @Override
    public RefreshTokenRecord revokeById(UUID id, Instant now) {
      return delegate.revokeById(id, now);
    }

    @Override
    public void revokeAllForAccount(UUID accountId, Instant now) {
      delegate.revokeAllForAccount(accountId, now);
    }
  }
}
