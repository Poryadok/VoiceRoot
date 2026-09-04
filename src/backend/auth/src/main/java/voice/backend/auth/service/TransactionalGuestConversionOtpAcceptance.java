package voice.backend.auth.service;

import java.time.Instant;
import java.util.Objects;
import java.util.UUID;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.repository.OtpCodeRepository;

/** Auth-datasource transaction boundary for guest OTP consumption and durable conversion creation. */
public final class TransactionalGuestConversionOtpAcceptance
    implements GuestConversionOtpAcceptance {
  private final TransactionTemplate transactions;
  private final OtpCodeRepository otpCodes;
  private final GuestConversionOperationRepository operations;

  public TransactionalGuestConversionOtpAcceptance(
      TransactionTemplate transactions,
      OtpCodeRepository otpCodes,
      GuestConversionOperationRepository operations) {
    this.transactions = Objects.requireNonNull(transactions, "transactions");
    this.otpCodes = Objects.requireNonNull(otpCodes, "otpCodes");
    this.operations = Objects.requireNonNull(operations, "operations");
  }

  @Override
  public void acceptVerifiedGuestEmailOtp(UUID accountId, OtpCodeRecord otp, Instant now) {
    Objects.requireNonNull(accountId, "accountId");
    Objects.requireNonNull(otp, "otp");
    Objects.requireNonNull(now, "now");
    if (!accountId.equals(otp.accountId())) {
      throw new IllegalArgumentException("OTP does not belong to account");
    }
    transactions.executeWithoutResult(
        ignored -> {
          otpCodes.markUsed(otp.id(), now);
          operations.createOrResume(accountId, otp.id(), now);
        });
  }
}
