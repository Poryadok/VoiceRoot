package voice.backend.auth.mail;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Delivers transactional auth email via the Resend HTTP API. */
public class ResendMailSender implements MailSender {
  private static final Logger log = LoggerFactory.getLogger(ResendMailSender.class);
  private static final URI DEFAULT_ENDPOINT = URI.create("https://api.resend.com/emails");

  private final String apiKey;
  private final String from;
  private final HttpClient httpClient;
  private final ObjectMapper objectMapper;
  private final URI endpoint;

  public ResendMailSender(String apiKey, String from) {
    this(apiKey, from, HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build(), new ObjectMapper(), DEFAULT_ENDPOINT);
  }

  ResendMailSender(
      String apiKey, String from, HttpClient httpClient, ObjectMapper objectMapper, URI endpoint) {
    if (apiKey == null || apiKey.isBlank()) {
      throw new IllegalArgumentException("Resend API key is required");
    }
    if (from == null || from.isBlank()) {
      throw new IllegalArgumentException("Resend from address is required");
    }
    this.apiKey = apiKey;
    this.from = from;
    this.httpClient = httpClient;
    this.objectMapper = objectMapper;
    this.endpoint = endpoint == null ? DEFAULT_ENDPOINT : endpoint;
  }

  @Override
  public void sendOtpEmail(String toEmail, String subject, String body) {
    if (toEmail == null || toEmail.isBlank()) {
      throw new IllegalArgumentException("toEmail is required");
    }
    try {
      byte[] payload =
          objectMapper.writeValueAsBytes(
              Map.of(
                  "from", from,
                  "to", List.of(toEmail.trim()),
                  "subject", subject == null ? "" : subject,
                  "text", body == null ? "" : body));
      HttpRequest request =
          HttpRequest.newBuilder(endpoint)
              .timeout(Duration.ofSeconds(15))
              .header("Authorization", "Bearer " + apiKey)
              .header("Content-Type", "application/json")
              .POST(HttpRequest.BodyPublishers.ofByteArray(payload))
              .build();
      HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() < 200 || response.statusCode() >= 300) {
        log.warn("resend send failed status={} body={}", response.statusCode(), truncate(response.body()));
        throw new IllegalStateException("resend_send_failed");
      }
      log.debug("resend mail accepted to={} status={}", toEmail, response.statusCode());
    } catch (InterruptedException ex) {
      Thread.currentThread().interrupt();
      throw new IllegalStateException("resend_send_interrupted", ex);
    } catch (IOException ex) {
      throw new IllegalStateException("resend_send_failed", ex);
    }
  }

  private static String truncate(String body) {
    if (body == null) {
      return "";
    }
    return body.length() <= 200 ? body : body.substring(0, 200);
  }
}
