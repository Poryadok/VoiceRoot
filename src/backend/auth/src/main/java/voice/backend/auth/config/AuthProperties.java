package voice.backend.auth.config;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "auth")
public class AuthProperties {
  private final Jwt jwt = new Jwt();
  private final Refresh refresh = new Refresh();
  private final Redis redis = new Redis();
  private final Grpc grpc = new Grpc();
  private final UserDb userDb = new UserDb();
  private final Totp totp = new Totp();
  private PersistenceMode persistence = PersistenceMode.JDBC;

  public UserDb getUserDb() {
    return userDb;
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

  public static class UserDb {
    private String jdbcUrl = "";
    private String username = "";
    private String password = "";

    public boolean isConfigured() {
      return jdbcUrl != null && !jdbcUrl.isBlank();
    }

    public String getJdbcUrl() {
      return jdbcUrl;
    }

    public void setJdbcUrl(String jdbcUrl) {
      this.jdbcUrl = jdbcUrl;
    }

    public String getUsername() {
      return username;
    }

    public void setUsername(String username) {
      this.username = username;
    }

    public String getPassword() {
      return password;
    }

    public void setPassword(String password) {
      this.password = password;
    }

    public String resolveUsername() {
      return username == null || username.isBlank() ? "voice" : username;
    }

    public String resolvePassword() {
      return password == null ? "" : password;
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
