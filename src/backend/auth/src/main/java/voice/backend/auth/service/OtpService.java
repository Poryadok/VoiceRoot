package voice.backend.auth.service;

import java.security.SecureRandom;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Locale;
import java.util.UUID;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.repository.OtpCodeRepository;
import voice.backend.auth.security.RefreshTokenCodec;

public class OtpService {
  static final Duration OTP_TTL = Duration.ofMinutes(10);

  private final AccountRepository accounts;
  private final OtpCodeRepository otpCodes;
  private final RefreshTokenCodec codec;
  private final MailSender mailSender;
  private final OtpThrottle throttle;
  private final Clock clock;
  private final SecureRandom random = new SecureRandom();

  public OtpService(
      AccountRepository accounts,
      OtpCodeRepository otpCodes,
      RefreshTokenCodec codec,
      MailSender mailSender,
      OtpThrottle throttle,
      Clock clock) {
    this.accounts = accounts;
    this.otpCodes = otpCodes;
    this.codec = codec;
    this.mailSender = mailSender;
    this.throttle = throttle;
    this.clock = clock;
  }

  public void sendOtp(SendOtpCommand command, AuthService authService) {
    String type = normalizeType(command.otpType());
    Account account = resolveAccount(command, authService);
    ensureActive(account);
    if (account.email() == null || account.email().isBlank()) {
      throw new AuthException("validation_failed");
    }
    String throttleKey = account.id().toString();
    throttle.checkCanSend(throttleKey);
    String code = generateCode();
    Instant now = Instant.now(clock);
    otpCodes.create(account.id(), codec.hash(code), type, now.plus(OTP_TTL), now);
    mailSender.sendOtpEmail(
        account.email(),
        subjectFor(type),
        "Your Voice verification code is " + code + " (expires in 10 minutes).");
    throttle.recordSend(throttleKey);
  }

  public void verifyOtp(VerifyOtpCommand command, AuthService authService) {
    String type = normalizeType(command.otpType());
    if (command.code() == null || command.code().isBlank()) {
      throw new AuthException("validation_failed");
    }
    Account account = resolveAccount(command, authService);
    String throttleKey = account.id().toString();
    throttle.checkCanVerify(throttleKey);
    Instant now = Instant.now(clock);
    OtpCodeRecord record =
        otpCodes
            .findLatestValid(account.id(), type, now)
            .orElseThrow(() -> new AuthException("invalid_otp"));
    if (!codec.hash(command.code().trim()).equals(record.codeHash())) {
      throttle.recordFailedVerify(throttleKey);
      throw new AuthException("invalid_otp");
    }
    otpCodes.markUsed(record.id(), now);
  }

  private Account resolveAccount(SendOtpCommand command, AuthService authService) {
    if (command.accessToken() != null && !command.accessToken().isBlank()) {
      TokenClaims claims = authService.validate(command.accessToken());
      return accounts
          .findById(claims.userId())
          .orElseThrow(() -> new AuthException("invalid_token"));
    }
    return resolveAccountByIdentifier(command.email(), command.phone());
  }

  private Account resolveAccount(VerifyOtpCommand command, AuthService authService) {
    if (command.accessToken() != null && !command.accessToken().isBlank()) {
      TokenClaims claims = authService.validate(command.accessToken());
      return accounts
          .findById(claims.userId())
          .orElseThrow(() -> new AuthException("invalid_token"));
    }
    return resolveAccountByIdentifier(command.email(), command.phone());
  }

  private Account resolveAccountByIdentifier(String email, String phone) {
    if (email != null && !email.isBlank()) {
      return accounts
          .findByEmail(email.trim().toLowerCase(Locale.ROOT))
          .orElseThrow(() -> new AuthException("invalid_credentials"));
    }
    if (phone != null && !phone.isBlank()) {
      return accounts
          .findByPhone(phone.trim())
          .orElseThrow(() -> new AuthException("invalid_credentials"));
    }
    throw new AuthException("validation_failed");
  }

  private static void ensureActive(Account account) {
    if (!"active".equals(account.status())) {
      throw new AuthException("account_inactive");
    }
  }

  private String generateCode() {
    int value = 100_000 + random.nextInt(900_000);
    return Integer.toString(value);
  }

  static String normalizeType(String otpType) {
    if (otpType == null || otpType.isBlank()) {
      throw new AuthException("validation_failed");
    }
    return switch (otpType.trim().toLowerCase(Locale.ROOT)) {
      case "email_verify", "otp_type_email_verify" -> "email_verify";
      case "password_reset", "otp_type_password_reset" -> "password_reset";
      default -> throw new AuthException("validation_failed");
    };
  }

  private static String subjectFor(String type) {
    return switch (type) {
      case "email_verify" -> "Verify your Voice email";
      case "password_reset" -> "Reset your Voice password";
      default -> "Voice verification code";
    };
  }
}
