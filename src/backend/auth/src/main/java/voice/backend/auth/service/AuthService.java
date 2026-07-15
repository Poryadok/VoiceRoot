package voice.backend.auth.service;

import io.micrometer.core.instrument.MeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.E2EKeyBackupRecord;
import voice.backend.auth.repository.E2EKeyBackupRepository;
import voice.backend.auth.repository.RefreshTokenRecord;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.security.TokenBlacklist;

public class AuthService {
  /** Max opaque encrypted blob size for E2E key backup (512 KiB). */
  public static final int E2E_KEY_BACKUP_MAX_BLOB_BYTES = 512 * 1024;

  private final AccountRepository accounts;
  private final RefreshTokenRepository refreshTokens;
  private final RefreshTokenCodec refreshTokenCodec;
  private final BCryptPasswordHasher passwordHasher;
  private final JwtService jwtService;
  private final TokenBlacklist tokenBlacklist;
  private final TotpService totpService;
  private final BackupCodeService backupCodeService;
  private final Clock clock;
  private final Duration refreshTtl;
  private final PrimaryProfileProvisioner primaryProfileProvisioner;
  private final PhoneHashResolver phoneHashResolver;
  private final SubscriptionTierResolver subscriptionTierResolver;
  private final ProfileSwitchValidator profileSwitchValidator;
  private final E2EKeyBackupRepository e2eKeyBackups;
  private final AuthEventPublisher authEventPublisher;
  private final MeterRegistry meterRegistry;

  public AuthService(
      AccountRepository accounts,
      RefreshTokenRepository refreshTokens,
      RefreshTokenCodec refreshTokenCodec,
      BCryptPasswordHasher passwordHasher,
      JwtService jwtService,
      TokenBlacklist tokenBlacklist,
      TotpService totpService,
      BackupCodeService backupCodeService,
      Clock clock,
      Duration refreshTtl,
      PrimaryProfileProvisioner primaryProfileProvisioner,
      PhoneHashResolver phoneHashResolver,
      SubscriptionTierResolver subscriptionTierResolver,
      ProfileSwitchValidator profileSwitchValidator,
      E2EKeyBackupRepository e2eKeyBackups,
      AuthEventPublisher authEventPublisher,
      MeterRegistry meterRegistry) {
    this.accounts = accounts;
    this.refreshTokens = refreshTokens;
    this.refreshTokenCodec = refreshTokenCodec;
    this.passwordHasher = passwordHasher;
    this.jwtService = jwtService;
    this.tokenBlacklist = tokenBlacklist;
    this.totpService = totpService;
    this.backupCodeService = backupCodeService;
    this.clock = clock;
    this.refreshTtl = refreshTtl;
    this.primaryProfileProvisioner = primaryProfileProvisioner;
    this.phoneHashResolver = phoneHashResolver;
    this.subscriptionTierResolver = subscriptionTierResolver;
    this.profileSwitchValidator = profileSwitchValidator;
    this.e2eKeyBackups = e2eKeyBackups;
    this.authEventPublisher = authEventPublisher;
    this.meterRegistry = meterRegistry;
  }

  public AuthService withClock(Clock newClock) {
    return new AuthService(
        accounts,
        refreshTokens,
        refreshTokenCodec,
        passwordHasher,
        jwtService.withClock(newClock),
        tokenBlacklist,
        totpService,
        backupCodeService,
        newClock,
        refreshTtl,
        primaryProfileProvisioner,
        phoneHashResolver,
        subscriptionTierResolver,
        profileSwitchValidator,
        e2eKeyBackups,
        authEventPublisher,
        meterRegistry);
  }

  public AuthSession register(RegisterCommand command) {
    String email = normalize(command.email());
    String phone = normalize(command.phone());
    if (!command.guest() && email == null && phone == null) {
      throw new AuthException("validation_failed");
    }
    if (command.password() == null || command.password().length() < 8) {
      throw new AuthException("validation_failed");
    }
    Account account;
    try {
      account = accounts.create(email, phone, passwordHasher.hash(command.password()), command.guest() ? "guest" : "regular");
    } catch (IllegalArgumentException ex) {
      throw new AuthException("registration_conflict");
    }
    touchLastOnline(account);
    return issueSession(account, command.deviceInfoJson());
  }

  public AuthSession login(LoginCommand command) {
    try {
      Account account = findLoginAccount(command.email(), command.phone());
      if (!passwordHasher.matches(command.password(), account.passwordHash())) {
        throw new AuthException("invalid_credentials");
      }
      ensureActive(account);
      if (account.totpEnabled()) {
        String code = command.totpCode();
        if (code == null || code.isBlank()) {
          throw new AuthException("totp_required");
        }
        boolean validTotp = account.totpSecret() != null && totpService.verifyEncrypted(account.totpSecret(), code.trim());
        if (!validTotp && !backupCodeService.consume(account.id(), code.trim())) {
          throw new AuthException("invalid_totp");
        }
      }
      touchLastOnline(account);
      AuthSession session = issueSession(account, command.deviceInfoJson());
      recordAuthLoginMetric(true);
      return session;
    } catch (RuntimeException ex) {
      recordAuthLoginMetric(false);
      throw ex;
    }
  }

  public synchronized AuthSession refresh(RefreshCommand command) {
    try {
      RefreshTokenRecord current = refreshRecord(command.refreshToken());
      ensureUsableRefresh(current);
      refreshTokens.revoke(current.tokenHash(), Instant.now(clock));
      Account account = accounts.findById(current.accountId().toString()).orElseThrow(() -> new AuthException("invalid_token"));
      ensureActive(account);
      tokenBlacklist.revoke(current.accessJti(), jwtService.accessTtl());
      touchLastOnline(account);
      AuthSession session = issueSession(account, command.deviceInfoJson());
      recordAuthLoginMetric(true);
      return session;
    } catch (RuntimeException ex) {
      recordAuthLoginMetric(false);
      throw ex;
    }
  }

  public void logout(LogoutCommand command) {
    RefreshTokenRecord current = refreshRecord(command.refreshToken());
    refreshTokens.revoke(current.tokenHash(), Instant.now(clock));
    tokenBlacklist.revoke(current.accessJti(), jwtService.accessTtl());
    if (command.accessToken() != null && !command.accessToken().isBlank()) {
      TokenClaims claims = jwtService.validate(stripBearer(command.accessToken()));
      tokenBlacklist.revoke(claims.jti(), jwtService.ttl(claims));
    }
  }

  public TokenClaims validate(String accessToken) {
    if (accessToken == null || accessToken.isBlank()) {
      throw new AuthException("invalid_token");
    }
    TokenClaims claims = jwtService.validate(stripBearer(accessToken));
    if (tokenBlacklist.isRevoked(claims.jti())) {
      throw new AuthException("token_revoked");
    }
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    return claims;
  }

  public String jwksJson() {
    return jwtService.jwksJson();
  }

  /** Issues a user access JWT for OAuth authorization_code grant (no refresh token). */
  public String issueOAuthAccessToken(String accountId, String profileId) {
    Account account = accounts.findById(accountId).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    String tier = subscriptionTierResolver.resolveTier(account.id());
    return jwtService.issue(account.id().toString(), profileId, List.of("user"), tier, account.type());
  }

  public long accessTokenTtlSeconds() {
    return jwtService.accessTtl().toSeconds();
  }

  public void setAccountStatus(String accountId, String status) {
    if (accountId == null || accountId.isBlank()) {
      throw new AuthException("invalid_account");
    }
    if (!"active".equals(status) && !"suspended".equals(status)) {
      throw new AuthException("invalid_status");
    }
    UUID id = UUID.fromString(accountId);
    accounts.findById(accountId).orElseThrow(() -> new AuthException("invalid_account"));
    accounts.setStatus(id, status);
  }

  /** Internal S2S: map stored phone hashes to primary profile IDs. */
  public Map<String, String> resolvePhoneHashes(Collection<String> phoneHashes) {
    if (phoneHashResolver == null) {
      return Map.of();
    }
    return phoneHashResolver.resolvePrimaryProfileIdsByPhoneHashes(phoneHashes);
  }

  public TotpEnrollment enable2FA(String accessToken, String password) {
    TokenClaims claims = validate(accessToken);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    if (!passwordHasher.matches(password, account.passwordHash())) {
      throw new AuthException("invalid_credentials");
    }
    String secret = totpService.generateSecret();
    byte[] encrypted = totpService.encryptSecret(secret);
    accounts.saveTotpSecret(account.id(), encrypted, false);
    List<String> backupCodes = backupCodeService.generateAndStore(account.id());
    String label = displayHint(account);
    String hint = "Saved";
    if (backupCodes.size() > 1) {
      hint = "Saved " + backupCodes.size() + " codes";
    }
    return new TotpEnrollment(
        totpService.buildTotpUriFromSecret(label, secret),
        hint,
        backupCodes);
  }

  public AuthSession verify2FA(String accessToken, String totpCode) {
    TokenClaims claims = validate(accessToken);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    if (account.totpSecret() == null || account.totpSecret().length == 0) {
      throw new AuthException("totp_not_enrolled");
    }
    if (!totpService.verifyEncrypted(account.totpSecret(), totpCode)) {
      throw new AuthException("invalid_totp");
    }
    accounts.setTotpEnabled(account.id(), true);
    Account fresh = accounts.findById(account.id().toString()).orElse(account);
    return issueSession(fresh, "{}");
  }

  public AuthSession switchActiveProfile(String accessToken, String profileId, String deviceInfoJson) {
    TokenClaims claims = validate(accessToken);
    UUID accountId = UUID.fromString(claims.userId());
    UUID targetProfile = UUID.fromString(profileId);
    profileSwitchValidator.validateOwnedSwitchable(accountId, targetProfile);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    tokenBlacklist.revoke(claims.jti(), jwtService.ttl(claims));
    return issueSessionForProfile(account, profileId, deviceInfoJson == null ? "{}" : deviceInfoJson);
  }

  public AuthSession convertGuest(String accessToken, ConvertGuestCommand command) {
    TokenClaims claims = validate(accessToken);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    if (!"guest".equals(account.type())) {
      throw new AuthException("validation_failed");
    }
    if (command.password() == null || command.password().length() < 8) {
      throw new AuthException("validation_failed");
    }
    String passwordHash = passwordHasher.hash(command.password());
    String email = normalize(command.email());
    String phone = normalize(command.phone());
    if (email == null && phone == null) {
      throw new AuthException("validation_failed");
    }
    if (email != null) {
      accounts
          .findByEmail(email)
          .filter(existing -> !existing.id().equals(account.id()))
          .ifPresent(ignored -> {
            throw new AuthException("registration_conflict");
          });
    }
    if (phone != null) {
      accounts
          .findByPhone(phone)
          .filter(existing -> !existing.id().equals(account.id()))
          .ifPresent(ignored -> {
            throw new AuthException("registration_conflict");
          });
    }
    Account converted;
    try {
      converted = accounts.convertGuest(account.id(), email, phone, passwordHash);
    } catch (IllegalArgumentException ex) {
      throw new AuthException("registration_conflict");
    }
    tokenBlacklist.revoke(claims.jti(), jwtService.ttl(claims));
    authEventPublisher.publishGuestConverted(converted.id());
    return issueSession(converted, "{}");
  }

  public void putE2EKeyBackup(String accessToken, String encryptedBlob, String passwordHint) {
    if (encryptedBlob == null || encryptedBlob.isBlank()) {
      throw new AuthException("validation_failed");
    }
    if (encryptedBlob.length() > E2E_KEY_BACKUP_MAX_BLOB_BYTES) {
      throw new AuthException("validation_failed");
    }
    TokenClaims claims = validate(accessToken);
    e2eKeyBackups.put(UUID.fromString(claims.userId()), encryptedBlob, passwordHint);
  }

  public E2EKeyBackupRecord getE2EKeyBackup(String accessToken) {
    TokenClaims claims = validate(accessToken);
    return e2eKeyBackups
        .get(UUID.fromString(claims.userId()))
        .orElseThrow(() -> new AuthException("not_found"));
  }

  private AuthSession issueSession(Account account, String deviceInfoJson) {
    String profileId = primaryProfileProvisioner.ensurePrimaryProfile(account.id(), displayHint(account));
    return issueSessionForProfile(account, profileId, deviceInfoJson);
  }

  private AuthSession issueSessionForProfile(Account account, String profileId, String deviceInfoJson) {
    String tier = subscriptionTierResolver.resolveTier(account.id());
    String accessToken = jwtService.issue(account.id().toString(), profileId, List.of("user"), tier, account.type());
    TokenClaims claims = jwtService.validate(accessToken);
    String refreshToken = refreshTokenCodec.generate();
    refreshTokens.create(
        account.id(),
        refreshTokenCodec.hash(refreshToken),
        deviceInfoJson,
        claims.jti(),
        Instant.now(clock).plus(refreshTtl),
        Instant.now(clock));
    return new AuthSession(
        accessToken,
        refreshToken,
        jwtService.accessTtl().toSeconds(),
        account.id().toString(),
        profileId,
        account.type());
  }

  private void touchLastOnline(Account account) {
    accounts.touchLastOnlineAt(account.id(), Instant.now(clock));
  }

  private static String displayHint(Account account) {
    if (account.email() != null && !account.email().isBlank()) {
      return account.email();
    }
    if (account.phone() != null && !account.phone().isBlank()) {
      return account.phone();
    }
    return account.id().toString();
  }

  private RefreshTokenRecord refreshRecord(String token) {
    if (!refreshTokenCodec.isWellFormed(token)) {
      throw new AuthException("invalid_token");
    }
    return refreshTokens.findByHash(refreshTokenCodec.hash(token)).orElseThrow(() -> new AuthException("invalid_token"));
  }

  private void ensureUsableRefresh(RefreshTokenRecord record) {
    if (record.revoked()) {
      throw new AuthException("token_revoked");
    }
    if (!record.expiresAt().isAfter(Instant.now(clock))) {
      throw new AuthException("token_expired");
    }
  }

  private Account findLoginAccount(String email, String phone) {
    return accounts.findByEmail(normalize(email))
        .or(() -> accounts.findByPhone(normalize(phone)))
        .orElseThrow(() -> new AuthException("invalid_credentials"));
  }

  private void ensureActive(Account account) {
    if (!"active".equals(account.status())) {
      throw new AuthException("account_inactive");
    }
  }

  private String normalize(String value) {
    if (value == null || value.isBlank()) {
      return null;
    }
    return value.trim().toLowerCase();
  }

  private String stripBearer(String token) {
    if (token.startsWith("Bearer ")) {
      return token.substring("Bearer ".length());
    }
    return token;
  }

  private void recordAuthLoginMetric(boolean success) {
    meterRegistry
        .counter("auth_login_total", "result", success ? "success" : "failure")
        .increment();
  }
}
