package voice.backend.auth.service;

import io.micrometer.core.instrument.MeterRegistry;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import voice.backend.auth.userdb.PhoneHashResolver;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import voice.backend.auth.userdb.ProfileSwitchValidator;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountDeletionState;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.E2EKeyBackupRecord;
import voice.backend.auth.repository.E2EKeyBackupRepository;
import voice.backend.auth.repository.RefreshTokenRecord;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.events.AuthEventPublisher;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.sessionepoch.SessionEpochFloorMissingException;
import voice.backend.auth.sessionepoch.SessionEpochFloorUnavailableException;
import voice.backend.auth.sessionepoch.PreparedSessionEpoch;
import voice.backend.auth.sessionepoch.SessionEpochIssuanceGate;

public class AuthService {
  /** Max opaque encrypted blob size for E2E key backup (512 KiB). */
  public static final int E2E_KEY_BACKUP_MAX_BLOB_BYTES = 512 * 1024;
  static final Duration ACCOUNT_RESTORE_GRACE = Duration.ofDays(30);

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
  private final AccountRestoreTokenStore restoreTokenStore;
  private final MailSender mailSender;
  private final SessionEpochFloorStore sessionEpochFloors;
  private final SessionEpochIssuanceGate sessionEpochIssuanceGate;
  private AccountDeletionOperationRepository deletionOperations;
  private AccountDeletionRestoreTokenCodec deletionTokenCodec;
  private AccountDeletionEventPublisher deletionEventPublisher;
  private AccountDeletionOperationStarter deletionStarter;
  private AccountDeletionPendingFloorWorker deletionFloorWorker;
  private AccountDeletionPendingEventWorker deletionEventWorker;

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
      MeterRegistry meterRegistry,
      AccountRestoreTokenStore restoreTokenStore,
      MailSender mailSender,
      SessionEpochFloorStore sessionEpochFloors) {
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
    this.restoreTokenStore = restoreTokenStore;
    this.mailSender = mailSender;
    this.sessionEpochFloors = sessionEpochFloors;
    this.sessionEpochIssuanceGate = new SessionEpochIssuanceGate(accounts, sessionEpochFloors);
  }

  public AuthService withClock(Clock newClock) {
    AuthService copy = new AuthService(
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
        meterRegistry,
        restoreTokenStore,
        mailSender,
        sessionEpochFloors);
    if (deletionOperations != null && deletionTokenCodec != null && deletionEventPublisher != null
        && deletionStarter != null && deletionFloorWorker != null && deletionEventWorker != null) {
      copy.configureAccountDeletion(
          deletionOperations, deletionTokenCodec, deletionEventPublisher, deletionStarter,
          deletionFloorWorker, deletionEventWorker);
    }
    return copy;
  }

  /** Injects the Auth-owned durable deletion outbox without changing legacy unit-test constructors. */
  public void configureAccountDeletion(
      AccountDeletionOperationRepository deletionOperations,
      AccountDeletionRestoreTokenCodec deletionTokenCodec,
      AccountDeletionEventPublisher deletionEventPublisher,
      AccountDeletionOperationStarter deletionStarter,
      AccountDeletionPendingFloorWorker deletionFloorWorker,
      AccountDeletionPendingEventWorker deletionEventWorker) {
    this.deletionOperations = java.util.Objects.requireNonNull(deletionOperations, "deletionOperations");
    this.deletionTokenCodec = java.util.Objects.requireNonNull(deletionTokenCodec, "deletionTokenCodec");
    this.deletionEventPublisher =
        java.util.Objects.requireNonNull(deletionEventPublisher, "deletionEventPublisher");
    this.deletionStarter = java.util.Objects.requireNonNull(deletionStarter, "deletionStarter");
    this.deletionFloorWorker = java.util.Objects.requireNonNull(deletionFloorWorker, "deletionFloorWorker");
    this.deletionEventWorker = java.util.Objects.requireNonNull(deletionEventWorker, "deletionEventWorker");
  }

  public AuthSession register(RegisterCommand command) {
    String email = normalize(command.email());
    String phone = normalize(command.phone());
    if (command.guest() && (email != null || phone != null)) {
      throw new AuthException("validation_failed");
    }
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
      PreparedSessionEpoch prepared = sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
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
      AuthSession session = issueSession(account, prepared, command.deviceInfoJson());
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
      Account account = accounts.findById(current.accountId().toString()).orElseThrow(() -> new AuthException("invalid_token"));
      ensureActive(account);
      PreparedSessionEpoch prepared = sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
      refreshTokens.revoke(current.tokenHash(), Instant.now(clock));
      tokenBlacklist.revoke(current.accessJti(), jwtService.accessTtl());
      touchLastOnline(account);
      AuthSession session = issueSession(account, prepared, command.deviceInfoJson());
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
    return issueOAuthAccessToken(accountId, profileId, prepareOAuthAccessToken(accountId));
  }

  /** Prepares an active OAuth account's epoch before an authorization-code consume. */
  public PreparedSessionEpoch prepareOAuthAccessToken(String accountId) {
    Account account = accounts.findById(accountId).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    return sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
  }

  /** Signs an OAuth access token from a previously prepared epoch without another floor write. */
  public String issueOAuthAccessToken(
      String accountId, String profileId, PreparedSessionEpoch prepared) {
    Account account = accounts.findById(accountId).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    if (!account.id().equals(prepared.accountId())) {
      throw new IllegalStateException("prepared session epoch account mismatch");
    }
    String expectedProfileId = requireProfileId(profileId);
    String ensuredProfileId = requireProfileId(primaryProfileProvisioner.ensurePrimaryProfile(
        account.id(), displayHint(account), "guest".equals(account.type())));
    if (!expectedProfileId.equals(ensuredProfileId)) {
      throw new AuthException("malformed_user_response");
    }
    String tier = subscriptionTierResolver.resolveTier(account.id());
    return jwtService.issue(
        account.id().toString(), ensuredProfileId, List.of("user"), tier, account.type(), prepared.sessionEpoch());
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
    Map<String, String> resolved = phoneHashResolver.resolvePrimaryProfileIdsByPhoneHashes(phoneHashes);
    if (resolved == null) {
      throw new AuthException("malformed_user_response");
    }
    for (Map.Entry<String, String> entry : resolved.entrySet()) {
      if (entry.getKey() == null || entry.getKey().isBlank()) {
        throw new AuthException("malformed_user_response");
      }
      requireProfileId(entry.getValue());
    }
    return resolved;
  }

  /** Internal S2S: return account ids that are soft-deleted (deleted_at set). */
  public List<String> filterDeletedAccountIds(Collection<String> accountIds) {
    if (accountIds == null || accountIds.isEmpty()) {
      return List.of();
    }
    List<UUID> parsed = new ArrayList<>();
    for (String raw : accountIds) {
      if (raw == null || raw.isBlank()) {
        continue;
      }
      try {
        parsed.add(UUID.fromString(raw.trim()));
      } catch (IllegalArgumentException ignored) {
        // skip invalid ids
      }
    }
    if (parsed.isEmpty()) {
      return List.of();
    }
    return accounts.findDeletedAmong(parsed).stream().map(UUID::toString).toList();
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
    PreparedSessionEpoch prepared = sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
    accounts.setTotpEnabled(account.id(), true);
    Account fresh = accounts.findById(account.id().toString()).orElse(account);
    return issueSession(fresh, prepared, "{}");
  }

  public AuthSession switchActiveProfile(String accessToken, String profileId, String deviceInfoJson) {
    TokenClaims claims = validate(accessToken);
    UUID accountId = UUID.fromString(claims.userId());
    UUID targetProfile = UUID.fromString(profileId);
    profileSwitchValidator.validateOwnedSwitchable(
        accountId,
        UUID.fromString(claims.profileId()),
        targetProfile,
        claims.subscriptionTier());
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    PreparedSessionEpoch prepared = sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
    tokenBlacklist.revoke(claims.jti(), jwtService.ttl(claims));
    return issueSessionForProfile(account, prepared, profileId, deviceInfoJson == null ? "{}" : deviceInfoJson);
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
    if (email == null || phone != null) {
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
    PreparedSessionEpoch prepared = sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
    Account converted;
    try {
      converted = accounts.convertGuest(account.id(), email, phone, passwordHash);
    } catch (IllegalArgumentException ex) {
      throw new AuthException("registration_conflict");
    }
    tokenBlacklist.revoke(claims.jti(), jwtService.ttl(claims));
    return issueSession(converted, prepared, "{}");
  }

  public DeleteAccountResult deleteAccount(String accessToken, String password) {
    return deleteAccount(accessToken, password, null);
  }

  public DeleteAccountResult deleteAccount(String accessToken, String password, String totpCode) {
    TokenClaims claims = validateForAccountDeletion(accessToken);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    if (!passwordHasher.matches(password, account.passwordHash())) {
      throw new AuthException("invalid_credentials");
    }
    verifyDeletionSecondFactor(account, totpCode);
    if ("deleted".equals(account.status())) {
      return finishAccountDeletion(claims, account, operationForDeletedAccount(account));
    }
    ensureActive(account);
    return finishAccountDeletion(claims, startOperation(account));
  }

  private void verifyDeletionSecondFactor(Account account, String totpCode) {
    if (!account.totpEnabled()) {
      return;
    }
    if (totpCode == null || totpCode.isBlank()) {
      throw new AuthException("totp_required");
    }
    String code = totpCode.trim();
    boolean validTotp =
        account.totpSecret() != null && totpService.verifyEncrypted(account.totpSecret(), code);
    if (!validTotp && !backupCodeService.consume(account.id(), code)) {
      throw new AuthException("invalid_totp");
    }
  }

  private boolean hasSealedDeletionEpoch(Account account) {
    try {
      return sessionEpochFloors.requireFloor(account.id()) >= account.sessionEpoch();
    } catch (SessionEpochFloorMissingException ignored) {
      return false;
    }
  }

  private AccountDeletionStartResult startOperation(Account account) {
    requireDeletionOperations();
    UUID proposedOperationId = UUID.randomUUID();
    String proposedToken = deletionTokenCodec.derive(account.id(), proposedOperationId);
    return deletionStarter.startOrResume(
        account, proposedOperationId, refreshTokenCodec.hash(proposedToken), Instant.now(clock));
  }

  private AccountDeletionOperation operationForDeletedAccount(Account account) {
    requireDeletionOperations();
    return deletionOperations
        .findByAccountAndEpoch(account.id(), account.sessionEpoch())
        .orElseThrow(
            () ->
                new SessionEpochFloorUnavailableException(
                    "account deletion completion has not been durably recorded"));
  }

  private DeleteAccountResult finishAccountDeletion(
      TokenClaims claims, Account account, AccountDeletionOperation operation) {
    String restoreToken = deletionTokenCodec.derive(account.id(), operation.operationId());
    if (!refreshTokenCodec.hash(restoreToken).equals(operation.restoreTokenHash())) {
      throw new IllegalStateException("account deletion restore token verifier mismatch");
    }
    if (operation.state() == AccountDeletionState.PENDING_FLOOR) {
      deletionFloorWorker.recoverOperation(operation.operationId(), Duration.ofSeconds(30));
      operation = operationForDeletedAccount(account);
      if (operation.state() == AccountDeletionState.PENDING_FLOOR) {
        throw new SessionEpochFloorUnavailableException("account deletion epoch floor is not durably sealed");
      }
    }
    Instant now = Instant.now(clock);
    refreshTokens.revokeAllForAccount(account.id(), now);
    tokenBlacklist.revoke(claims.jti(), jwtService.ttl(claims));
    restoreTokenStore.store(restoreToken, account.id(), AccountRestoreTokenStore.RESTORE_TTL);
    if (account.email() != null && !account.email().isBlank()) {
      mailSender.sendOtpEmail(
          account.email(),
          "Restore your Voice account",
          "Your account was scheduled for deletion. Restore within 30 days using token: " + restoreToken);
    }
    if (operation.state() == AccountDeletionState.PENDING_EVENT) {
      deletionEventWorker.recoverOperation(operation.operationId(), Duration.ofSeconds(30));
      operation = operationForDeletedAccount(account);
      if (operation.state() != AccountDeletionState.COMPLETED) {
        throw new IllegalStateException("account deletion event has not been durably acknowledged");
      }
    }
    return new DeleteAccountResult(restoreToken);
  }

  private DeleteAccountResult finishAccountDeletion(
      TokenClaims claims, AccountDeletionStartResult started) {
    return finishAccountDeletion(claims, started.account(), started.operation());
  }

  private void requireDeletionOperations() {
    if (deletionOperations == null || deletionTokenCodec == null || deletionEventPublisher == null
        || deletionStarter == null || deletionFloorWorker == null || deletionEventWorker == null) {
      throw new IllegalStateException("account deletion durable operation repository is not configured");
    }
  }


  private TokenClaims validateForAccountDeletion(String accessToken) {
    if (accessToken == null || accessToken.isBlank()) {
      throw new AuthException("invalid_token");
    }
    TokenClaims claims = jwtService.validate(stripBearer(accessToken));
    if (tokenBlacklist.isRevoked(claims.jti())) {
      throw new AuthException("token_revoked");
    }
    return claims;
  }

  public AuthSession restoreAccount(String restoreToken) {
    if (restoreToken == null || restoreToken.isBlank()) {
      throw new AuthException("invalid_token");
    }
    String token = restoreToken.trim();
    UUID peekedAccountId =
        restoreTokenStore
            .peek(token)
            .orElseThrow(() -> new AuthException("invalid_token"));
    Account account =
        accounts
            .findById(peekedAccountId.toString())
            .orElseThrow(() -> new AuthException("invalid_token"));
    ensureRestorable(account);
    PreparedSessionEpoch prepared = sessionEpochIssuanceGate.prepare(account.id(), account.sessionEpoch());
    UUID consumedAccountId =
        restoreTokenStore
            .consume(token)
            .orElseThrow(() -> new AuthException("invalid_token"));
    if (!peekedAccountId.equals(consumedAccountId)) {
      throw new AuthException("invalid_token");
    }
    account =
        accounts
            .findById(consumedAccountId.toString())
            .orElseThrow(() -> new AuthException("invalid_token"));
    ensureRestorable(account);
    if (!accounts.restoreDeleted(account.id())) {
      throw new AuthException("validation_failed");
    }
    Account restored = accounts.findById(account.id().toString()).orElse(account);
    authEventPublisher.publishAccountRestored(restored.id());
    return issueSession(restored, prepared, "{}");
  }

  private void ensureRestorable(Account account) {
    if (!"deleted".equals(account.status()) || account.deletedAt() == null) {
      throw new AuthException("validation_failed");
    }
    Instant precheckNow = Instant.now(clock);
    if (account.deletedAt().plus(ACCOUNT_RESTORE_GRACE).isBefore(precheckNow)) {
      throw new AuthException("account_inactive");
    }
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

  public GuestReminderState getGuestReminder(String accessToken) {
    TokenClaims claims = validate(accessToken);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    if (!"guest".equals(account.type())) {
      throw new AuthException("validation_failed");
    }
    Instant lastShown = accounts.getGuestReminderLastShownAt(account.id()).orElse(null);
    boolean shouldShow = lastShown == null || lastShown.isBefore(Instant.now(clock).minus(Duration.ofHours(24)));
    return new GuestReminderState(lastShown, shouldShow);
  }

  public GuestReminderState markGuestReminderShown(String accessToken) {
    TokenClaims claims = validate(accessToken);
    Account account = accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    if (!"guest".equals(account.type())) {
      throw new AuthException("validation_failed");
    }
    Instant now = Instant.now(clock);
    accounts.markGuestReminderShown(account.id(), now);
    return new GuestReminderState(now, false);
  }

  public List<ActiveSession> listSessions(String accessToken) {
    TokenClaims claims = validate(accessToken);
    UUID accountId = UUID.fromString(claims.userId());
    return refreshTokens.listActiveByAccount(accountId).stream()
        .map(
            record ->
                new ActiveSession(
                    record.id().toString(),
                    record.deviceInfoJson(),
                    record.createdAt(),
                    record.expiresAt(),
                    claims.jti() != null && claims.jti().equals(record.accessJti())))
        .toList();
  }

  public void revokeSession(String accessToken, String sessionId) {
    TokenClaims claims = validate(accessToken);
    UUID accountId = UUID.fromString(claims.userId());
    UUID id;
    try {
      id = UUID.fromString(sessionId);
    } catch (IllegalArgumentException ex) {
      throw new AuthException("validation_failed");
    }
    RefreshTokenRecord record =
        refreshTokens.findById(id).orElseThrow(() -> new AuthException("not_found"));
    if (!record.accountId().equals(accountId)) {
      throw new AuthException("not_found");
    }
    Instant now = Instant.now(clock);
    refreshTokens.revokeById(id, now);
    if (record.accessJti() != null && !record.accessJti().isBlank()) {
      tokenBlacklist.revoke(record.accessJti(), jwtService.accessTtl());
    }
  }

  private AuthSession issueSession(Account account, String deviceInfoJson) {
    String profileId = primaryProfileProvisioner.ensurePrimaryProfile(
        account.id(), displayHint(account), "guest".equals(account.type()));
    return issueSessionForProfile(account, profileId, deviceInfoJson);
  }

  private AuthSession issueSession(Account account, PreparedSessionEpoch prepared, String deviceInfoJson) {
    String profileId = primaryProfileProvisioner.ensurePrimaryProfile(
        account.id(), displayHint(account), "guest".equals(account.type()));
    return issueSessionForProfile(account, prepared, profileId, deviceInfoJson);
  }

  private AuthSession issueSessionForProfile(Account account, String profileId, String deviceInfoJson) {
    return issueSessionForProfile(account, new PreparedSessionEpoch(account.id(), account.sessionEpoch()), profileId, deviceInfoJson);
  }

  private AuthSession issueSessionForProfile(
      Account account, PreparedSessionEpoch prepared, String profileId, String deviceInfoJson) {
    if (!account.id().equals(prepared.accountId())) {
      throw new IllegalStateException("prepared session epoch account mismatch");
    }
    profileId = requireProfileId(profileId);
    if (deviceInfoJson == null || deviceInfoJson.isBlank()) {
      deviceInfoJson = "{}";
    }
    String tier = subscriptionTierResolver.resolveTier(account.id());
    String accessToken =
        jwtService.issue(
            account.id().toString(), profileId, List.of("user"), tier, account.type(), prepared.sessionEpoch());
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

  private static String requireProfileId(String profileId) {
    try {
      return UUID.fromString(profileId).toString();
    } catch (RuntimeException ex) {
      throw new AuthException("malformed_user_response");
    }
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
