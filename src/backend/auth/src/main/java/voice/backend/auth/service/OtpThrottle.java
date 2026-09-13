package voice.backend.auth.service;

/** Auth-owned OTP send/verify throttling. Implementations must fail closed on unavailable state. */
public interface OtpThrottle {
  /** Atomically reserves the documented resend cooldown before a code is created or sent. */
  void reserveSend(String key);

  /** Rejects verification when the documented failure window is already exhausted. */
  void checkCanVerify(String key);

  /** Atomically records a failed verification attempt. */
  void recordFailedVerify(String key);
}
