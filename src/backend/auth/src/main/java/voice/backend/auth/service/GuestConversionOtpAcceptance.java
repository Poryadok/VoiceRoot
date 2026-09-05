package voice.backend.auth.service;

import java.time.Instant;
import java.util.UUID;
import voice.backend.auth.repository.OtpCodeRecord;

/** Atomically accepts a verified guest email OTP and starts durable conversion work. */
public interface GuestConversionOtpAcceptance {
  void acceptVerifiedGuestEmailOtp(UUID accountId, OtpCodeRecord otp, Instant now);
}
