package voice.backend.auth.service;

/** Auth-owned OTP throttling. Production implementations must fail closed when state is unavailable. */
public interface OtpThrottle {
  /** Atomically reserves the documented resend cooldown before a code is created or sent. */
  void reserveSend(String key);

  /** Atomically admits one OTP verification attempt before the code is inspected. */
  void admitVerify(String key);
}
