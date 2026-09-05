package voice.backend.auth.service;

import java.time.Instant;
import java.util.Objects;
import java.util.UUID;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.repository.OtpCodeRepository;

/** Explicit memory-profile acceptance boundary; it never invokes User or event publication inline. */
public final class InMemoryGuestConversionOtpAcceptance implements GuestConversionOtpAcceptance {
  private final OtpCodeRepository otpCodes;
  private final GuestConversionOperationRepository operations;

  public InMemoryGuestConversionOtpAcceptance(
      OtpCodeRepository otpCodes, GuestConversionOperationRepository operations) {
    this.otpCodes = Objects.requireNonNull(otpCodes, "otpCodes");
    this.operations = Objects.requireNonNull(operations, "operations");
  }

  @Override
  public synchronized void acceptVerifiedGuestEmailOtp(UUID accountId, OtpCodeRecord otp, Instant now) {
    Objects.requireNonNull(accountId, "accountId");
    Objects.requireNonNull(otp, "otp");
    Objects.requireNonNull(now, "now");
    if (!accountId.equals(otp.accountId())) {
      throw new IllegalArgumentException("OTP does not belong to account");
    }
    operations.createOrResume(accountId, otp.id(), now);
    otpCodes.markUsed(otp.id(), now);
  }
}
