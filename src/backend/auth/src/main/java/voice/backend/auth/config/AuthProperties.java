package voice.backend.auth.config;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "auth")
public class AuthProperties {
  private final Jwt jwt = new Jwt();
  private final Refresh refresh = new Refresh();
  private final Redis redis = new Redis();
  private final Grpc grpc = new Grpc();
  private PersistenceMode persistence = PersistenceMode.JDBC;

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
}
