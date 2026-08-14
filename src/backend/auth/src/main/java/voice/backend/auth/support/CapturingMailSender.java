package voice.backend.auth.support;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import voice.backend.auth.mail.MailSender;

/** Test-profile mail sender that records OTP bodies for assertions. */
public class CapturingMailSender implements MailSender {
  private static final Pattern CODE = Pattern.compile("\\b(\\d{6})\\b");

  private final List<String> bodies = new ArrayList<>();

  @Override
  public synchronized void sendOtpEmail(String toEmail, String subject, String body) {
    bodies.add(body);
  }

  public synchronized String lastCode() {
    if (bodies.isEmpty()) {
      return null;
    }
    Matcher matcher = CODE.matcher(bodies.get(bodies.size() - 1));
    if (!matcher.find()) {
      return null;
    }
    return matcher.group(1);
  }

  public synchronized void clear() {
    bodies.clear();
  }
}
