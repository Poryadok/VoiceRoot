package voice.backend.auth.config;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "auth")
public class AuthProperties {
  private final Jwt jwt = new Jwt();
  private final Refresh refresh = new Refresh();
  private final Redis redis = new Redis();
  private final Grpc grpc = new Grpc();
  private final UserGrpc userGrpc = new UserGrpc();
  private final Totp totp = new Totp();
  private final OAuth oauth = new OAuth();
  private final Nats nats = new Nats();
  private final Resend resend = new Resend();
  private final AccountDeletion accountDeletion = new AccountDeletion();
  private final SessionEpoch sessionEpoch = new SessionEpoch();
  private PersistenceMode persistence = PersistenceMode.JDBC;

  public UserGrpc getUserGrpc() {
    return userGrpc;
  }

  public PersistenceMode getPersistence() {
    return persistence;
  }

  public void setPersistence(PersistenceMode persistence) {
    this.persistence = persistence;
  }

  public enum PersistenceMode {
    MEMORY,
    JDBC
  }

  public Jwt getJwt() {
    return jwt;
  }

  public Refresh getRefresh() {
    return refresh;
  }

  public Redis getRedis() {
    return redis;
  }

  public Grpc getGrpc() {
    return grpc;
  }

  public Totp getTotp() {
    return totp;
  }

  public OAuth getOauth() {
    return oauth;
  }

  public Nats getNats() {
    return nats;
  }

  public Resend getResend() {
    return resend;
  }

  public AccountDeletion getAccountDeletion() {
    return accountDeletion;
  }

  public SessionEpoch getSessionEpoch() {
    return sessionEpoch;
  }

  public static class SessionEpoch {
    private final Seed seed = new Seed();

    public Seed getSeed() {
      return seed;
    }

    public static class Seed {
      private int pageSize = 256;

      public int getPageSize() {
        return pageSize;
      }

      public void setPageSize(int pageSize) {
        this.pageSize = pageSize;
      }
    }
  }

  /** Dedicated HMAC material for deterministic account-restore tokens. */
  public static class AccountDeletion {
    private String tokenSecret = "";

    public String getTokenSecret() {
      return tokenSecret;
    }

    public void setTokenSecret(String tokenSecret) {
      this.tokenSecret = tokenSecret;
    }
  }

  public static class Resend {
    private String apiKey = "";
    private String from = "Voice <onboarding@resend.dev>";

    public String getApiKey() {
      return apiKey;
    }

    public void setApiKey(String apiKey) {
      this.apiKey = apiKey;
    }

    public String getFrom() {
      return from;
    }

    public void setFrom(String from) {
      this.from = from;
    }
  }

  public static class Nats {
    private String url = "";

    public String getUrl() {
      return url;
    }

    public void setUrl(String url) {
      this.url = url;
    }
  }

  public static class OAuth {
    /** @deprecated prefer {@link #twitch}.apiBaseUrl */
    private String twitchApiBaseUrl = "https://api.twitch.tv";

    private String publicApiBaseUrl = "http://127.0.0.1:18080";
    private final PlatformOAuth twitch = new PlatformOAuth();
    private final PlatformOAuth youtube = new PlatformOAuth();
    private final DeveloperPortalOAuth developerPortal = new DeveloperPortalOAuth();
    private final AdminOAuth admin = new AdminOAuth();

    public OAuth() {
      twitch.setApiBaseUrl("https://api.twitch.tv");
      twitch.setTokenUrl("https://id.twitch.tv/oauth2/token");
      twitch.setAuthorizeUrl("https://id.twitch.tv/oauth2/authorize");
      youtube.setApiBaseUrl("https://www.googleapis.com");
      youtube.setTokenUrl("https://oauth2.googleapis.com/token");
      youtube.setAuthorizeUrl("https://accounts.google.com/o/oauth2/v2/auth");
    }

    public String getTwitchApiBaseUrl() {
      if (twitch.getApiBaseUrl() != null && !twitch.getApiBaseUrl().isBlank()) {
        return twitch.getApiBaseUrl();
      }
      return twitchApiBaseUrl;
    }

    public void setTwitchApiBaseUrl(String twitchApiBaseUrl) {
      this.twitchApiBaseUrl = twitchApiBaseUrl;
      if (twitchApiBaseUrl != null && !twitchApiBaseUrl.isBlank()) {
        twitch.setApiBaseUrl(twitchApiBaseUrl);
      }
    }

    public PlatformOAuth getTwitch() {
      return twitch;
    }

    public PlatformOAuth getYoutube() {
      return youtube;
    }

    public String getPublicApiBaseUrl() {
      return publicApiBaseUrl;
    }

    public void setPublicApiBaseUrl(String publicApiBaseUrl) {
      this.publicApiBaseUrl = publicApiBaseUrl;
    }

    public DeveloperPortalOAuth getDeveloperPortal() {
      return developerPortal;
    }

    public AdminOAuth getAdmin() {
      return admin;
    }
  }

  /** Twitch / YouTube OAuth client settings for linked-account verification. */
  public static class PlatformOAuth {
    private String clientId = "";
    private String clientSecret = "";
    private String apiBaseUrl = "";
    private String tokenUrl = "";
    private String authorizeUrl = "";

    public boolean isConfigured() {
      return clientId != null
          && !clientId.isBlank()
          && clientSecret != null
          && !clientSecret.isBlank();
    }

    public String getClientId() {
      return clientId;
    }

    public void setClientId(String clientId) {
      this.clientId = clientId;
    }

    public String getClientSecret() {
      return clientSecret;
    }

    public void setClientSecret(String clientSecret) {
      this.clientSecret = clientSecret;
    }

    public String getApiBaseUrl() {
      return apiBaseUrl;
    }

    public void setApiBaseUrl(String apiBaseUrl) {
      this.apiBaseUrl = apiBaseUrl;
    }

    public String getTokenUrl() {
      return tokenUrl;
    }

    public void setTokenUrl(String tokenUrl) {
      this.tokenUrl = tokenUrl;
    }

    public String getAuthorizeUrl() {
      return authorizeUrl;
    }

    public void setAuthorizeUrl(String authorizeUrl) {
      this.authorizeUrl = authorizeUrl;
    }
  }

  public static class OAuthClientSettings {
    private boolean enabled = false;
    private String clientId = "";
    private String clientSecret = "";
    private java.util.List<String> redirectUris = java.util.List.of();
    private Duration authorizationCodeTtl = Duration.ofSeconds(60);

    public boolean isEnabled() {
      return enabled;
    }

    public void setEnabled(boolean enabled) {
      this.enabled = enabled;
    }

    public String getClientId() {
      return clientId;
    }

    public void setClientId(String clientId) {
      this.clientId = clientId;
    }

    public String getClientSecret() {
      return clientSecret;
    }

    public void setClientSecret(String clientSecret) {
      this.clientSecret = clientSecret;
    }

    public java.util.List<String> getRedirectUris() {
      return redirectUris;
    }

    public void setRedirectUris(java.util.List<String> redirectUris) {
      this.redirectUris = redirectUris;
    }

    public Duration getAuthorizationCodeTtl() {
      return authorizationCodeTtl;
    }

    public void setAuthorizationCodeTtl(Duration authorizationCodeTtl) {
      this.authorizationCodeTtl = authorizationCodeTtl;
    }
  }

  public static class DeveloperPortalOAuth extends OAuthClientSettings {}

  public static class AdminOAuth extends OAuthClientSettings {}

  public static class Jwt {
    private String issuer = "voice-auth";
    private String audience = "voice-client";
    private String keyId = "local-key";
    private Duration accessTtl = Duration.ofMinutes(15);
    private String privateKeyPem = "";
    private String privateKeyLocation = "";

    public String getPrivateKeyPem() {
      return privateKeyPem;
    }

    public void setPrivateKeyPem(String privateKeyPem) {
      this.privateKeyPem = privateKeyPem;
    }

    public String getPrivateKeyLocation() {
      return privateKeyLocation;
    }

    public void setPrivateKeyLocation(String privateKeyLocation) {
      this.privateKeyLocation = privateKeyLocation;
    }

    public String getIssuer() {
      return issuer;
    }

    public void setIssuer(String issuer) {
      this.issuer = issuer;
    }

    public String getAudience() {
      return audience;
    }

    public void setAudience(String audience) {
      this.audience = audience;
    }

    public String getKeyId() {
      return keyId;
    }

    public void setKeyId(String keyId) {
      this.keyId = keyId;
    }

    public Duration getAccessTtl() {
      return accessTtl;
    }

    public void setAccessTtl(Duration accessTtl) {
      this.accessTtl = accessTtl;
    }
  }

  public static class Refresh {
    private Duration ttl = Duration.ofDays(30);

    public Duration getTtl() {
      return ttl;
    }

    public void setTtl(Duration ttl) {
      this.ttl = ttl;
    }
  }

  public static class Redis {
    private String blacklistPrefix = "jwt:blacklist:";

    public String getBlacklistPrefix() {
      return blacklistPrefix;
    }

    public void setBlacklistPrefix(String blacklistPrefix) {
      this.blacklistPrefix = blacklistPrefix;
    }
  }

  public static class Grpc {
    private int port = 9090;

    public int getPort() {
      return port;
    }

    public void setPort(int port) {
      this.port = port;
    }
  }

  public static class UserGrpc {
    private String addr = "";
    private Duration deadline = Duration.ofSeconds(15);

    public boolean isConfigured() {
      return addr != null && !addr.isBlank();
    }

    public String getAddr() {
      return addr;
    }

    public void setAddr(String addr) {
      this.addr = addr;
    }

    public Duration getDeadline() {
      return deadline;
    }

    public void setDeadline(Duration deadline) {
      if (deadline == null || deadline.isZero() || deadline.isNegative()) {
        throw new IllegalArgumentException("auth.user-grpc.deadline must be positive");
      }
      this.deadline = deadline;
    }
  }

  public static class Totp {
    private boolean testBypass = false;
    private String encryptionKey = "";

    public boolean isTestBypass() {
      return testBypass;
    }

    public void setTestBypass(boolean testBypass) {
      this.testBypass = testBypass;
    }

    public String getEncryptionKey() {
      return encryptionKey;
    }

    public void setEncryptionKey(String encryptionKey) {
      this.encryptionKey = encryptionKey;
    }
  }
}
