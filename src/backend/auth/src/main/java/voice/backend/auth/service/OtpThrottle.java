package voice.backend.auth.service;

/** Redis-backed OTP send/verify rate limits (docs/features/auth-and-contacts.md). */
public interface OtpThrottle {
  void checkCanSend(String key);

  void recordSend(String key);

  void checkCanVerify(String key);

  void recordFailedVerify(String key);
}
