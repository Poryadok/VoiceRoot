package voice.backend.auth.mail;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Discards outbound email; OTP codes remain in DB for verify in dev/test. */
public class NoopMailSender implements MailSender {
  private static final Logger log = LoggerFactory.getLogger(NoopMailSender.class);

  @Override
  public void sendOtpEmail(String toEmail, String subject, String body) {
    log.debug("noop mail to={} subject={} body={}", toEmail, subject, body);
  }
}
