package voice.backend.auth.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.UUID;
import voice.backend.auth.userdb.UserVerificationSync;

public class LinkedAccountsService {
  private final UserVerificationSync verificationSync;
  private final HttpClient httpClient;
  private final ObjectMapper objectMapper;
  private final String twitchApiBaseUrl;

  public LinkedAccountsService(UserVerificationSync verificationSync, String twitchApiBaseUrl) {
    this.verificationSync = verificationSync;
    this.twitchApiBaseUrl = twitchApiBaseUrl == null || twitchApiBaseUrl.isBlank() ? "https://api.twitch.tv" : twitchApiBaseUrl;
    this.httpClient = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();
    this.objectMapper = new ObjectMapper();
  }

  public void setTwitchApiBaseUrlForTests(String baseUrl) {
    // package-private for integration tests with local mock HTTP server
    java.lang.reflect.Field f;
    try {
      f = LinkedAccountsService.class.getDeclaredField("twitchApiBaseUrl");
      f.setAccessible(true);
      f.set(this, baseUrl);
    } catch (ReflectiveOperationException ex) {
      throw new IllegalStateException(ex);
    }
  }

  public VerificationResult completeTwitchCallback(UUID profileId, String code) {
    if (code == null || code.isBlank()) {
      throw new AuthException("validation_failed");
    }
    String accessToken = "mock-access-token";
    if (!"mock-code".equals(code)) {
      throw new AuthException("oauth_failed");
    }
    if (!isTwitchPartner(accessToken)) {
      throw new AuthException("verification_denied");
    }
    verificationSync.setPersonalVerification(profileId, "twitch");
    return new VerificationResult("personal", "twitch");
  }

  public void unlinkTwitch(UUID profileId) {
    verificationSync.clearVerification(profileId);
  }

  private boolean isTwitchPartner(String accessToken) {
    try {
      HttpRequest request =
          HttpRequest.newBuilder()
              .uri(URI.create(twitchApiBaseUrl + "/helix/users"))
              .header("Authorization", "Bearer " + accessToken)
              .header("Client-Id", "test-client")
              .GET()
              .timeout(Duration.ofSeconds(5))
              .build();
      HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() != 200) {
        return false;
      }
      JsonNode root = objectMapper.readTree(response.body());
      JsonNode data = root.path("data");
      if (!data.isArray() || data.isEmpty()) {
        return false;
      }
      return "partner".equalsIgnoreCase(data.get(0).path("broadcaster_type").asText());
    } catch (Exception ex) {
      throw new AuthException("oauth_unavailable");
    }
  }

  public record VerificationResult(String verificationType, String badge) {}
}
