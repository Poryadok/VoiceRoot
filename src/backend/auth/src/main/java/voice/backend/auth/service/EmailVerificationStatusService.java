package voice.backend.auth.service;

import java.time.Instant;
import org.springframework.stereotype.Service;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.OtpCodeRepository;

/** Authoritative, session-scoped recovery state for pending email identities. */
@Service
public final class EmailVerificationStatusService {
  private final AccountRepository accounts;
  private final OtpCodeRepository otpCodes;
  private final GuestConversionOperationRepository operations;

  public EmailVerificationStatusService(
      AccountRepository accounts,
      OtpCodeRepository otpCodes,
      GuestConversionOperationRepository operations) {
    this.accounts = accounts;
    this.otpCodes = otpCodes;
    this.operations = operations;
  }

  public EmailVerificationStatus status(TokenClaims claims) {
    Account account =
        accounts.findById(claims.userId()).orElseThrow(() -> new AuthException("invalid_token"));
    if ("regular".equals(account.type())) {
      return new EmailVerificationStatus("REGULAR", "NONE");
    }
    if (operations.findByAccountId(account.id())
        .map(operation -> operation.state() == GuestConversionState.PENDING_USER)
        .orElse(false)) {
      return new EmailVerificationStatus("PROMOTION_PENDING", "NONE");
    }
    if (!accounts.isRegularEmailVerificationPending(account.id())) {
      return new EmailVerificationStatus("GUEST", "NONE");
    }
    boolean active = otpCodes.findLatestValid(account.id(), "email_verify", Instant.now()).isPresent();
    return new EmailVerificationStatus("EMAIL_PENDING", active ? "ACTIVE" : "NONE");
  }

  public record EmailVerificationStatus(String state, String codeState) {}
}
