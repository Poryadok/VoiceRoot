package voice.backend.auth.mail;

/** Outbound email channel (Resend in production; noop in dev/test). */
public interface MailSender {
  void sendOtpEmail(String toEmail, String subject, String body);
}
