package voice.backend.auth.service;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.RefreshTokenRecord;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.JwtService;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.security.TokenBlacklist;

public class AuthService {
  private final AccountRepository accounts;
  private final RefreshTokenRepository refreshTokens;
  private final RefreshTokenCodec refreshTokenCodec;
  private final BCryptPasswordHasher passwordHasher;
  private final JwtService jwtService;
  private final TokenBlacklist tokenBlacklist;
  private final Clock clock;
  private final Duration refreshTtl;

  public AuthService(
      AccountRepository accounts,
      RefreshTokenRepository refreshTokens,
      RefreshTokenCodec refreshTokenCodec,
      BCryptPasswordHasher passwordHasher,
      JwtService jwtService,
      TokenBlacklist tokenBlacklist,
      Clock clock,
      Duration refreshTtl) {
    this.accounts = accounts;
    this.refreshTokens = refreshTokens;
    this.refreshTokenCodec = refreshTokenCodec;
    this.passwordHasher = passwordHasher;
    this.jwtService = jwtService;
    this.tokenBlacklist = tokenBlacklist;
    this.clock = clock;
    this.refreshTtl = refreshTtl;
  }

  public AuthService withClock(Clock newClock) {
    return new AuthService(
        accounts,
        refreshTokens,
        refreshTokenCodec,
        passwordHasher,
        jwtService.withClock(newClock),
        tokenBlacklist,
        newClock,
        refreshTtl);
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
      throw new AuthException("invalid_credentials");
    }
    return issueSession(account, command.deviceInfoJson());
  }

  public AuthSession login(LoginCommand command) {
    Account account = findLoginAccount(command.email(), command.phone());
    if (!passwordHasher.matches(command.password(), account.passwordHash())) {
      throw new AuthException("invalid_credentials");
    }
    ensureActive(account);
    return issueSession(account, command.deviceInfoJson());
  }

  public synchronized AuthSession refresh(RefreshCommand command) {
    RefreshTokenRecord current = refreshRecord(command.refreshToken());
    ensureUsableRefresh(current);
    refreshTokens.revoke(current.tokenHash(), Instant.now(clock));
    Account account = accounts.findById(current.accountId().toString()).orElseThrow(() -> new AuthException("invalid_token"));
    ensureActive(account);
    tokenBlacklist.revoke(current.accessJti(), jwtService.accessTtl());
    return issueSession(account, command.deviceInfoJson());
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

  private AuthSession issueSession(Account account, String deviceInfoJson) {
    String profileId = "profile-" + account.id();
    String accessToken = jwtService.issue(account.id().toString(), profileId, List.of("user"), "free");
    TokenClaims claims = jwtService.validate(accessToken);
    String refreshToken = refreshTokenCodec.generate();
    refreshTokens.create(
        account.id(),
        refreshTokenCodec.hash(refreshToken),
        deviceInfoJson,
        claims.jti(),
        Instant.now(clock).plus(refreshTtl),
        Instant.now(clock));
    return new AuthSession(accessToken, refreshToken, jwtService.accessTtl().toSeconds(), account.id().toString());
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
}
