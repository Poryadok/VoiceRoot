package voice.backend.auth.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.repository.LinkedIdentity;
import voice.backend.auth.repository.LinkedIdentityRepository;
import voice.backend.auth.userdb.UserVerificationSync;

/**
 * Platform OAuth linking and personal verification (docs/features/verification.md).
 *
 * <p>Twitch: Helix {@code broadcaster_type == partner}. YouTube V1: channel {@code
 * status.longUploadsStatus} in {allowed, eligible} as YPP proxy.
 */
public class LinkedAccountsService {
  public static final String PLATFORM_TWITCH = "twitch";
  public static final String PLATFORM_YOUTUBE = "youtube";

  private final UserVerificationSync verificationSync;
  private final LinkedIdentityRepository linkedIdentities;
  private final AuthProperties.OAuth oauth;
  private final HttpClient httpClient;
  private final ObjectMapper objectMapper;

  private volatile String twitchApiBaseUrl;
  private volatile String twitchTokenUrl;
  private volatile String youtubeApiBaseUrl;
  private volatile String youtubeTokenUrl;

  public LinkedAccountsService(
      UserVerificationSync verificationSync,
      LinkedIdentityRepository linkedIdentities,
      AuthProperties.OAuth oauth) {
    this.verificationSync = verificationSync;
    this.linkedIdentities = linkedIdentities;
    this.oauth = oauth == null ? new AuthProperties.OAuth() : oauth;
    this.httpClient = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();
    this.objectMapper = new ObjectMapper();
    this.twitchApiBaseUrl = blankTo(this.oauth.getTwitch().getApiBaseUrl(), "https://api.twitch.tv");
    this.twitchTokenUrl =
        blankTo(this.oauth.getTwitch().getTokenUrl(), "https://id.twitch.tv/oauth2/token");
    this.youtubeApiBaseUrl =
        blankTo(this.oauth.getYoutube().getApiBaseUrl(), "https://www.googleapis.com");
    this.youtubeTokenUrl =
        blankTo(this.oauth.getYoutube().getTokenUrl(), "https://oauth2.googleapis.com/token");
  }

  /** Test hook: point Helix + token exchange at a local mock HTTP server. */
  public void setTwitchEndpointsForTests(String apiBaseUrl, String tokenUrl) {
    this.twitchApiBaseUrl = apiBaseUrl;
    this.twitchTokenUrl = tokenUrl;
  }

  /** @deprecated use {@link #setTwitchEndpointsForTests(String, String)} */
  public void setTwitchApiBaseUrlForTests(String baseUrl) {
    setTwitchEndpointsForTests(baseUrl, baseUrl + "/oauth2/token");
  }

  public void setYoutubeEndpointsForTests(String apiBaseUrl, String tokenUrl) {
    this.youtubeApiBaseUrl = apiBaseUrl;
    this.youtubeTokenUrl = tokenUrl;
  }

  public String buildTwitchAuthorizeUrl(String redirectUri, String state) {
    AuthProperties.PlatformOAuth twitch = oauth.getTwitch();
    String clientId = blankTo(twitch.getClientId(), "voice-twitch-dev");
    String authorize =
        blankTo(twitch.getAuthorizeUrl(), "https://id.twitch.tv/oauth2/authorize");
    String scope = URLEncoder.encode("user:read:email", StandardCharsets.UTF_8);
    return authorize
        + "?client_id="
        + url(clientId)
        + "&redirect_uri="
        + url(redirectUri)
        + "&response_type=code"
        + "&scope="
        + scope
        + (state == null || state.isBlank() ? "" : "&state=" + url(state));
  }

  public String buildYoutubeAuthorizeUrl(String redirectUri, String state) {
    AuthProperties.PlatformOAuth youtube = oauth.getYoutube();
    String clientId = blankTo(youtube.getClientId(), "voice-youtube-dev");
    String authorize =
        blankTo(youtube.getAuthorizeUrl(), "https://accounts.google.com/o/oauth2/v2/auth");
    String scope =
        URLEncoder.encode(
            "https://www.googleapis.com/auth/youtube.readonly", StandardCharsets.UTF_8);
    return authorize
        + "?client_id="
        + url(clientId)
        + "&redirect_uri="
        + url(redirectUri)
        + "&response_type=code"
        + "&scope="
        + scope
        + "&access_type=offline"
        + (state == null || state.isBlank() ? "" : "&state=" + url(state));
  }

  public List<Map<String, Object>> listLinkedAccounts(UUID accountId) {
    List<Map<String, Object>> out = new ArrayList<>();
    for (LinkedIdentity row : linkedIdentities.listActiveByAccount(accountId)) {
      Map<String, Object> item = new LinkedHashMap<>();
      item.put("platform", row.platform());
      item.put("external_id", row.externalId());
      item.put("external_login", row.externalLogin() == null ? "" : row.externalLogin());
      item.put("status", row.status());
      out.add(item);
    }
    return out;
  }

  public VerificationResult completeTwitchCallback(
      UUID accountId, UUID profileId, String code, String redirectUri) {
    if (code == null || code.isBlank()) {
      throw new AuthException("validation_failed");
    }
    TokenPair tokens = exchangeTwitchCode(code, redirectUri);
    TwitchUser user = fetchTwitchUser(tokens.accessToken());
    if (!"partner".equalsIgnoreCase(user.broadcasterType())) {
      throw new AuthException("verification_denied");
    }
    persistLink(
        accountId,
        profileId,
        PLATFORM_TWITCH,
        user.id(),
        user.login(),
        tokens.accessToken(),
        tokens.refreshToken());
    verificationSync.setPersonalVerification(profileId, PLATFORM_TWITCH);
    return new VerificationResult("personal", PLATFORM_TWITCH);
  }

  public VerificationResult completeYoutubeCallback(
      UUID accountId, UUID profileId, String code, String redirectUri) {
    if (code == null || code.isBlank()) {
      throw new AuthException("validation_failed");
    }
    TokenPair tokens = exchangeYoutubeCode(code, redirectUri);
    YoutubeChannel channel = fetchYoutubeChannel(tokens.accessToken());
    if (!isYoutubePartner(channel.longUploadsStatus())) {
      throw new AuthException("verification_denied");
    }
    persistLink(
        accountId,
        profileId,
        PLATFORM_YOUTUBE,
        channel.id(),
        channel.title(),
        tokens.accessToken(),
        tokens.refreshToken());
    verificationSync.setPersonalVerification(profileId, PLATFORM_YOUTUBE);
    return new VerificationResult("personal", PLATFORM_YOUTUBE);
  }

  public void unlinkTwitch(UUID accountId, UUID profileId) {
    unlinkPlatform(accountId, profileId, PLATFORM_TWITCH);
  }

  public void unlinkYoutube(UUID accountId, UUID profileId) {
    unlinkPlatform(accountId, profileId, PLATFORM_YOUTUBE);
  }

  private void unlinkPlatform(UUID accountId, UUID profileId, String platform) {
    linkedIdentities.revoke(accountId, platform);
    // Clear badge only when no other active verifying platform remains.
    boolean stillVerified =
        linkedIdentities.listActiveByAccount(accountId).stream()
            .anyMatch(
                row ->
                    PLATFORM_TWITCH.equals(row.platform())
                        || PLATFORM_YOUTUBE.equals(row.platform()));
    if (!stillVerified) {
      verificationSync.clearVerification(profileId);
    }
  }

  /** Re-check partner status for all active links; clear badge when lost. */
  public int refreshVerificationStatuses() {
    int cleared = 0;
    for (LinkedIdentity row : linkedIdentities.listAllActive()) {
      if (row.accessTokenEncrypted() == null || row.profileId() == null) {
        continue;
      }
      String accessToken = new String(row.accessTokenEncrypted(), StandardCharsets.UTF_8);
      boolean stillEligible;
      try {
        if (PLATFORM_TWITCH.equals(row.platform())) {
          stillEligible =
              "partner".equalsIgnoreCase(fetchTwitchUser(accessToken).broadcasterType());
        } else if (PLATFORM_YOUTUBE.equals(row.platform())) {
          stillEligible = isYoutubePartner(fetchYoutubeChannel(accessToken).longUploadsStatus());
        } else {
          continue;
        }
      } catch (AuthException ex) {
        stillEligible = false;
      }
      if (!stillEligible) {
        linkedIdentities.revoke(row.accountId(), row.platform());
        verificationSync.clearVerification(row.profileId());
        cleared++;
      }
    }
    return cleared;
  }

  private void persistLink(
      UUID accountId,
      UUID profileId,
      String platform,
      String externalId,
      String login,
      String accessToken,
      String refreshToken) {
    linkedIdentities.upsertActive(
        accountId,
        profileId,
        platform,
        externalId,
        login,
        accessToken == null ? null : accessToken.getBytes(StandardCharsets.UTF_8),
        refreshToken == null ? null : refreshToken.getBytes(StandardCharsets.UTF_8));
  }

  private TokenPair exchangeTwitchCode(String code, String redirectUri) {
    if (isMockCode(code) || !oauth.getTwitch().isConfigured()) {
      return new TokenPair("mock-access-token", null);
    }
    return exchangeAuthorizationCode(
        twitchTokenUrl,
        oauth.getTwitch().getClientId(),
        oauth.getTwitch().getClientSecret(),
        code,
        redirectUri);
  }

  private TokenPair exchangeYoutubeCode(String code, String redirectUri) {
    if (isMockCode(code) || !oauth.getYoutube().isConfigured()) {
      return new TokenPair("mock-access-token", null);
    }
    return exchangeAuthorizationCode(
        youtubeTokenUrl,
        oauth.getYoutube().getClientId(),
        oauth.getYoutube().getClientSecret(),
        code,
        redirectUri);
  }

  private TokenPair exchangeAuthorizationCode(
      String tokenUrl, String clientId, String clientSecret, String code, String redirectUri) {
    try {
      String body =
          "client_id="
              + url(clientId)
              + "&client_secret="
              + url(clientSecret)
              + "&code="
              + url(code)
              + "&grant_type=authorization_code"
              + "&redirect_uri="
              + url(redirectUri == null ? "" : redirectUri);
      HttpRequest request =
          HttpRequest.newBuilder()
              .uri(URI.create(tokenUrl))
              .header("Content-Type", "application/x-www-form-urlencoded")
              .POST(HttpRequest.BodyPublishers.ofString(body))
              .timeout(Duration.ofSeconds(5))
              .build();
      HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() != 200) {
        throw new AuthException("oauth_failed");
      }
      JsonNode root = objectMapper.readTree(response.body());
      String access = root.path("access_token").asText(null);
      if (access == null || access.isBlank()) {
        throw new AuthException("oauth_failed");
      }
      String refresh = root.path("refresh_token").asText(null);
      return new TokenPair(access, refresh);
    } catch (AuthException ex) {
      throw ex;
    } catch (Exception ex) {
      throw new AuthException("oauth_unavailable");
    }
  }

  private TwitchUser fetchTwitchUser(String accessToken) {
    try {
      HttpRequest request =
          HttpRequest.newBuilder()
              .uri(URI.create(twitchApiBaseUrl + "/helix/users"))
              .header("Authorization", "Bearer " + accessToken)
              .header("Client-Id", blankTo(oauth.getTwitch().getClientId(), "test-client"))
              .GET()
              .timeout(Duration.ofSeconds(5))
              .build();
      HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() != 200) {
        throw new AuthException("oauth_unavailable");
      }
      JsonNode data = objectMapper.readTree(response.body()).path("data");
      if (!data.isArray() || data.isEmpty()) {
        throw new AuthException("verification_denied");
      }
      JsonNode user = data.get(0);
      return new TwitchUser(
          user.path("id").asText(""),
          user.path("login").asText(""),
          user.path("broadcaster_type").asText(""));
    } catch (AuthException ex) {
      throw ex;
    } catch (Exception ex) {
      throw new AuthException("oauth_unavailable");
    }
  }

  private YoutubeChannel fetchYoutubeChannel(String accessToken) {
    try {
      HttpRequest request =
          HttpRequest.newBuilder()
              .uri(
                  URI.create(
                      youtubeApiBaseUrl
                          + "/youtube/v3/channels?part=status,snippet&mine=true"))
              .header("Authorization", "Bearer " + accessToken)
              .GET()
              .timeout(Duration.ofSeconds(5))
              .build();
      HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() != 200) {
        throw new AuthException("oauth_unavailable");
      }
      JsonNode items = objectMapper.readTree(response.body()).path("items");
      if (!items.isArray() || items.isEmpty()) {
        throw new AuthException("verification_denied");
      }
      JsonNode channel = items.get(0);
      return new YoutubeChannel(
          channel.path("id").asText(""),
          channel.path("snippet").path("title").asText(""),
          channel.path("status").path("longUploadsStatus").asText(""));
    } catch (AuthException ex) {
      throw ex;
    } catch (Exception ex) {
      throw new AuthException("oauth_unavailable");
    }
  }

  static boolean isYoutubePartner(String longUploadsStatus) {
    if (longUploadsStatus == null) {
      return false;
    }
    String v = longUploadsStatus.trim().toLowerCase();
    return "allowed".equals(v) || "eligible".equals(v);
  }

  private static boolean isMockCode(String code) {
    return "mock-code".equals(code);
  }

  private static String blankTo(String value, String fallback) {
    return value == null || value.isBlank() ? fallback : value;
  }

  private static String url(String value) {
    return URLEncoder.encode(value == null ? "" : value, StandardCharsets.UTF_8);
  }

  public record VerificationResult(String verificationType, String badge) {}

  private record TokenPair(String accessToken, String refreshToken) {}

  private record TwitchUser(String id, String login, String broadcasterType) {}

  private record YoutubeChannel(String id, String title, String longUploadsStatus) {}
}
