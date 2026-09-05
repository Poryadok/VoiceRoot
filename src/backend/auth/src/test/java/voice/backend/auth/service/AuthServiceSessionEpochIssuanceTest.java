package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.nimbusds.jwt.SignedJWT;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.events.NoopAuthEventPublisher;
import voice.backend.auth.mail.NoopMailSender;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryBackupCodeRepository;
import voice.backend.auth.repository.InMemoryE2EKeyBackupRepository;
import voice.backend.auth.repository.InMemoryRefreshTokenRepository;
import voice.backend.auth.repository.RefreshTokenRecord;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.InMemoryTokenBlacklist;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.userdb.InMemoryPhoneHashResolver;
import voice.backend.auth.userdb.InMemoryPrimaryProfileProvisioner;
import voice.backend.auth.userdb.NoOpProfileSwitchValidator;

class AuthServiceSessionEpochIssuanceTest {
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-05-01T10:00:00Z"), ZoneOffset.UTC);
  private static final String PASSWORD = "Correct horse battery staple";

  @Test
  void loginFloorFailurePreservesBackupCodeAndMintsSessionOnlyAfterHealthyRetry() {
    Harness harness = new Harness();
    AuthSession registered = harness.register("login-floor@example.com");
    UUID accountId = UUID.fromString(registered.accountId());
    harness.enableBackupCode(accountId);
    String backupCode = harness.backups.generateAndStore(accountId).getFirst();
    harness.resetSideEffects();
    harness.floors.failWith(new IllegalStateException("redis down"));

    assertThatThrownBy(() -> harness.service.login(login("login-floor@example.com", backupCode)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(harness.backupRepository.consumeCalls).isZero();
    assertThat(harness.refreshTokens.createCalls).isZero();
    assertThat(harness.floors.recordCalls).isEqualTo(1);

    harness.floors.healthy(1L);
    AuthSession retry = harness.service.login(login("login-floor@example.com", backupCode));

    assertThat(retry.accountId()).isEqualTo(registered.accountId());
    assertThat(harness.backupRepository.consumeCalls).isEqualTo(1);
    assertThat(harness.refreshTokens.createCalls).isEqualTo(1);
  }

  @Test
  void refreshFloorFailureLeavesRefreshAndJtiUsableUntilHealthyRetry() throws Exception {
    Harness harness = new Harness();
    AuthSession baseline = harness.register("refresh-floor@example.com");
    String originalJti = SignedJWT.parse(baseline.accessToken()).getJWTClaimsSet().getJWTID();
    harness.resetSideEffects();
    harness.floors.failWith(new IllegalStateException("redis down"));

    assertThatThrownBy(() -> harness.service.refresh(new RefreshCommand(baseline.refreshToken(), "{}")))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(harness.refreshTokens.revokeCalls).isZero();
    assertThat(harness.refreshTokens.createCalls).isZero();
    assertThat(harness.blacklist.revokeCalls).isZero();
    assertThat(harness.blacklist.isRevoked(originalJti)).isFalse();
    assertThat(harness.refreshTokens.findByHash(harness.codec.hash(baseline.refreshToken())))
        .get()
        .extracting(RefreshTokenRecord::revoked)
        .isEqualTo(false);

    harness.floors.healthy(1L);
    AuthSession retry = harness.service.refresh(new RefreshCommand(baseline.refreshToken(), "{}"));

    assertThat(retry.accountId()).isEqualTo(baseline.accountId());
    assertThat(harness.refreshTokens.revokeCalls).isEqualTo(1);
    assertThat(harness.refreshTokens.createCalls).isEqualTo(1);
    assertThat(harness.blacklist.revokeCalls).isEqualTo(1);
  }

  @Test
  void profileSwitchFloorFailureLeavesOriginalSessionUsableUntilHealthyRetry() throws Exception {
    Harness harness = new Harness();
    AuthSession baseline = harness.register("switch-floor@example.com");
    String originalJti = SignedJWT.parse(baseline.accessToken()).getJWTClaimsSet().getJWTID();
    String selectedProfileId = UUID.randomUUID().toString();
    harness.resetSideEffects();
    harness.floors.failWith(new IllegalStateException("redis down"));

    assertThatThrownBy(
            () -> harness.service.switchActiveProfile(baseline.accessToken(), selectedProfileId, "{}"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(harness.blacklist.revokeCalls).isZero();
    assertThat(harness.refreshTokens.createCalls).isZero();
    assertThat(harness.blacklist.isRevoked(originalJti)).isFalse();
    assertThat(harness.service.validate(baseline.accessToken()).userId()).isEqualTo(baseline.accountId());

    harness.floors.healthy(1L);
    AuthSession retry = harness.service.switchActiveProfile(baseline.accessToken(), selectedProfileId, "{}");

    assertThat(retry.accountId()).isEqualTo(baseline.accountId());
    assertThat(retry.profileId()).isEqualTo(selectedProfileId);
    assertThat(harness.blacklist.revokeCalls).isEqualTo(1);
    assertThat(harness.refreshTokens.createCalls).isEqualTo(1);
  }

  @Test
  void loginRedisAheadUsesPreparedEpochWithCurrentAccountAndProfileClaims() throws Exception {
    Harness harness = new Harness();
    AuthSession registered = harness.register("ahead-floor@example.com");
    harness.resetSideEffects();
    harness.floors.healthy(7L);

    AuthSession session = harness.service.login(login("ahead-floor@example.com", null));
    var claims = SignedJWT.parse(session.accessToken()).getJWTClaimsSet();
    TokenClaims validated = harness.service.validate(session.accessToken());

    assertThat(session.accountId()).isEqualTo(registered.accountId());
    assertThat(session.profileId()).isEqualTo(registered.profileId());
    assertThat(session.accountType()).isEqualTo(registered.accountType());
    assertThat(claims.getStringClaim("user_id")).isEqualTo(registered.accountId());
    assertThat(claims.getStringClaim("profile_id")).isEqualTo(registered.profileId());
    assertThat(claims.getLongClaim("session_epoch")).isEqualTo(7L);
    assertThat(validated.userId()).isEqualTo(registered.accountId());
    assertThat(validated.profileId()).isEqualTo(registered.profileId());
    assertThat(validated.accountType()).isEqualTo(registered.accountType());
    assertThat(harness.floors.recordCalls).isEqualTo(1);
    assertThat(harness.floors.lastAccountId).isEqualTo(UUID.fromString(registered.accountId()));
    assertThat(harness.floors.lastEpoch).isEqualTo(1L);
    assertThat(harness.accounts.advanceCalls).isEqualTo(1);
    assertThat(harness.accounts.lastAdvanceRequested).isEqualTo(7L);
    assertThat(harness.accounts.findById(registered.accountId()).map(Account::sessionEpoch).orElseThrow())
        .isEqualTo(7L);
  }

  @Test
  void verify2FAFloorFailureLeavesEnrollmentPendingUntilHealthyRetry() throws Exception {
    Harness harness = new Harness();
    AuthSession registered = harness.register("verify-floor@example.com");
    harness.service.enable2FA(registered.accessToken(), PASSWORD);
    harness.resetSideEffects();
    harness.floors.failWith(new IllegalStateException("redis down"));

    assertThatThrownBy(() -> harness.service.verify2FA(registered.accessToken(), "000000"))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(harness.accounts.setTotpEnabledCalls).isZero();
    assertThat(harness.refreshTokens.createCalls).isZero();
    assertThat(harness.accounts.findById(registered.accountId()).orElseThrow().totpEnabled()).isFalse();
    assertThat(harness.service.validate(registered.accessToken()).userId()).isEqualTo(registered.accountId());

    harness.resetSideEffects();
    harness.floors.healthy(7L);
    AuthSession retry = harness.service.verify2FA(registered.accessToken(), "000000");
    var claims = SignedJWT.parse(retry.accessToken()).getJWTClaimsSet();
    TokenClaims validated = harness.service.validate(retry.accessToken());

    assertThat(harness.accounts.setTotpEnabledCalls).isEqualTo(1);
    assertThat(harness.refreshTokens.createCalls).isEqualTo(1);
    assertThat(harness.accounts.findById(registered.accountId()).orElseThrow().totpEnabled()).isTrue();
    assertThat(retry.accountId()).isEqualTo(registered.accountId());
    assertThat(retry.profileId()).isEqualTo(registered.profileId());
    assertThat(retry.accountType()).isEqualTo(registered.accountType());
    assertThat(claims.getStringClaim("user_id")).isEqualTo(registered.accountId());
    assertThat(claims.getStringClaim("profile_id")).isEqualTo(registered.profileId());
    assertThat(claims.getStringClaim("account_type")).isEqualTo(registered.accountType());
    assertThat(claims.getLongClaim("session_epoch")).isEqualTo(7L);
    assertThat(validated.userId()).isEqualTo(registered.accountId());
    assertThat(validated.profileId()).isEqualTo(registered.profileId());
    assertThat(validated.accountType()).isEqualTo(registered.accountType());
    assertThat(harness.floors.recordCalls).isEqualTo(1);
    assertThat(harness.accounts.advanceCalls).isEqualTo(1);
    assertThat(harness.accounts.lastAdvanceRequested).isEqualTo(7L);
  }

  @Test
  void convertGuestFloorFailureLeavesPendingIdentityAndBearerUsableUntilHealthyRetry() throws Exception {
    Harness harness = new Harness();
    AuthSession guest = harness.registerGuest();
    String originalJti = SignedJWT.parse(guest.accessToken()).getJWTClaimsSet().getJWTID();
    ConvertGuestCommand command = new ConvertGuestCommand("pending-floor@example.com", null, "New account password 1");
    harness.resetSideEffects();
    harness.floors.failWith(new IllegalStateException("redis down"));

    assertThatThrownBy(() -> harness.service.convertGuest(guest.accessToken(), command))
        .isInstanceOf(SessionEpochFloorUnavailableException.class);

    assertThat(harness.accounts.convertGuestCalls).isZero();
    assertThat(harness.accounts.findByEmail("pending-floor@example.com")).isEmpty();
    assertThat(harness.accounts.findById(guest.accountId()).orElseThrow().type()).isEqualTo("guest");
    assertThat(harness.blacklist.revokeCalls).isZero();
    assertThat(harness.blacklist.isRevoked(originalJti)).isFalse();
    assertThat(harness.refreshTokens.createCalls).isZero();
    assertThat(harness.service.validate(guest.accessToken()).accountType()).isEqualTo("guest");

    harness.resetSideEffects();
    harness.floors.healthy(7L);
    AuthSession retry = harness.service.convertGuest(guest.accessToken(), command);
    var claims = SignedJWT.parse(retry.accessToken()).getJWTClaimsSet();
    TokenClaims validated = harness.service.validate(retry.accessToken());

    assertThat(harness.accounts.convertGuestCalls).isEqualTo(1);
    assertThat(harness.blacklist.revokeCalls).isEqualTo(1);
    assertThat(harness.refreshTokens.createCalls).isEqualTo(1);
    assertThat(retry.accountId()).isEqualTo(guest.accountId());
    assertThat(retry.profileId()).isEqualTo(guest.profileId());
    assertThat(retry.accountType()).isEqualTo("guest");
    assertThat(claims.getStringClaim("user_id")).isEqualTo(guest.accountId());
    assertThat(claims.getStringClaim("profile_id")).isEqualTo(guest.profileId());
    assertThat(claims.getStringClaim("account_type")).isEqualTo("guest");
    assertThat(claims.getLongClaim("session_epoch")).isEqualTo(7L);
    assertThat(validated.userId()).isEqualTo(guest.accountId());
    assertThat(validated.profileId()).isEqualTo(guest.profileId());
    assertThat(validated.accountType()).isEqualTo("guest");
    assertThat(harness.accounts.findByEmail("pending-floor@example.com")).isPresent();
    assertThat(harness.accounts.findById(guest.accountId()).orElseThrow().sessionEpoch()).isEqualTo(7L);
    assertThat(harness.floors.recordCalls).isEqualTo(1);
    assertThat(harness.accounts.advanceCalls).isEqualTo(1);
    assertThat(harness.accounts.lastAdvanceRequested).isEqualTo(7L);
  }

  private static LoginCommand login(String email, String totpCode) {
    return new LoginCommand(email, null, PASSWORD, totpCode, "{}");
  }

  private static final class Harness {
    final RecordingAccounts accounts = new RecordingAccounts();
    final RecordingRefreshTokens refreshTokens = new RecordingRefreshTokens();
    final RecordingBackupCodeRepository backupRepository = new RecordingBackupCodeRepository();
    final RecordingBlacklist blacklist = new RecordingBlacklist();
    final RecordingFloors floors = new RecordingFloors();
    final RefreshTokenCodec codec = new RefreshTokenCodec();
    final TotpService totp = new TotpService(memoryTotpProperties());
    final InMemoryPrimaryProfileProvisioner profiles = new InMemoryPrimaryProfileProvisioner();
    final AuthService service = new AuthService(
        accounts,
        refreshTokens,
        codec,
        new BCryptPasswordHasher(),
        JwtService.forTests("voice-auth", "voice-client", "test-key", Duration.ofMinutes(15), CLOCK),
        blacklist,
        totp,
        new BackupCodeService(backupRepository),
        CLOCK,
        Duration.ofDays(30),
        profiles,
        new InMemoryPhoneHashResolver(accounts, profiles),
        new InMemorySubscriptionTierStore(),
        new NoOpProfileSwitchValidator(),
        new InMemoryE2EKeyBackupRepository(),
        new NoopAuthEventPublisher(),
        new SimpleMeterRegistry(),
        new InMemoryAccountRestoreTokenStore(),
        new NoopMailSender(),
        floors);
    final BackupCodeService backups = new BackupCodeService(backupRepository);

    AuthSession register(String email) {
      return service.register(new RegisterCommand(email, null, PASSWORD, false, "{}"));
    }

    AuthSession registerGuest() {
      return service.register(new RegisterCommand(null, null, PASSWORD, true, "{}"));
    }

    void enableBackupCode(UUID accountId) {
      accounts.saveTotpSecret(accountId, totp.encryptSecret(totp.generateSecret()), true);
    }

    void resetSideEffects() {
      accounts.resetRecording();
      refreshTokens.resetRecording();
      backupRepository.resetRecording();
      blacklist.resetRecording();
      floors.resetRecording();
    }
  }

  private static AuthProperties memoryTotpProperties() {
    AuthProperties properties = new AuthProperties();
    properties.setPersistence(AuthProperties.PersistenceMode.MEMORY);
    properties.getTotp().setTestBypass(true);
    return properties;
  }

  private static final class RecordingAccounts extends InMemoryAccountRepository {
    int advanceCalls;
    long lastAdvanceRequested;
    int setTotpEnabledCalls;
    int convertGuestCalls;

    @Override
    public synchronized long advanceSessionEpochAtLeast(UUID accountId, long requestedEpoch) {
      advanceCalls++;
      lastAdvanceRequested = requestedEpoch;
      return super.advanceSessionEpochAtLeast(accountId, requestedEpoch);
    }

    @Override
    public synchronized void setTotpEnabled(UUID accountId, boolean enabled) {
      setTotpEnabledCalls++;
      super.setTotpEnabled(accountId, enabled);
    }

    @Override
    public synchronized Account convertGuest(UUID accountId, String email, String phone, String passwordHash) {
      convertGuestCalls++;
      return super.convertGuest(accountId, email, phone, passwordHash);
    }

    void resetRecording() {
      advanceCalls = 0;
      lastAdvanceRequested = 0L;
      setTotpEnabledCalls = 0;
      convertGuestCalls = 0;
    }
  }

  private static final class RecordingRefreshTokens extends InMemoryRefreshTokenRepository {
    int createCalls;
    int revokeCalls;

    @Override
    public synchronized RefreshTokenRecord create(
        UUID accountId, String tokenHash, String deviceInfoJson, String accessJti, Instant expiresAt, Instant now) {
      createCalls++;
      return super.create(accountId, tokenHash, deviceInfoJson, accessJti, expiresAt, now);
    }

    @Override
    public synchronized RefreshTokenRecord revoke(String tokenHash, Instant now) {
      revokeCalls++;
      return super.revoke(tokenHash, now);
    }

    void resetRecording() {
      createCalls = 0;
      revokeCalls = 0;
    }
  }

  private static final class RecordingBackupCodeRepository extends InMemoryBackupCodeRepository {
    int consumeCalls;

    @Override
    public synchronized boolean consumeCode(UUID accountId, String codeHash) {
      consumeCalls++;
      return super.consumeCode(accountId, codeHash);
    }

    void resetRecording() {
      consumeCalls = 0;
    }
  }

  private static final class RecordingBlacklist implements TokenBlacklist {
    private final InMemoryTokenBlacklist delegate = new InMemoryTokenBlacklist(CLOCK);
    int revokeCalls;

    @Override
    public void revoke(String jti, Duration ttl) {
      revokeCalls++;
      delegate.revoke(jti, ttl);
    }

    @Override
    public boolean isRevoked(String jti) {
      return delegate.isRevoked(jti);
    }

    void resetRecording() {
      revokeCalls = 0;
    }
  }

  private static final class RecordingFloors implements SessionEpochFloorStore {
    RuntimeException failure;
    long floor = 1L;
    int recordCalls;
    UUID lastAccountId;
    long lastEpoch;

    @Override
    public long recordAtLeast(UUID accountId, long epoch) {
      recordCalls++;
      lastAccountId = accountId;
      lastEpoch = epoch;
      if (failure != null) {
        throw failure;
      }
      return floor;
    }

    @Override
    public long requireFloor(UUID accountId) {
      throw new AssertionError("issuance must not read the floor after recording it");
    }

    void failWith(RuntimeException exception) {
      failure = exception;
    }

    void healthy(long returnedFloor) {
      failure = null;
      floor = returnedFloor;
    }

    void resetRecording() {
      recordCalls = 0;
      lastAccountId = null;
      lastEpoch = 0L;
    }
  }
}
