package voice.backend.auth.linkoauth;

import java.net.URI;
import java.time.Duration;
import java.util.*;

/** Immutable registration snapshot. A changed snapshot invalidates existing attempts. */
public record LinkedOAuthSettings(String revision, Duration attemptTtl, Duration resultTtl,
    byte[] encryptionKey, Map<String, Provider> providers) {
  public LinkedOAuthSettings {
    if (revision == null || !revision.matches("[A-Za-z0-9._-]{1,128}")) throw new IllegalArgumentException("linked OAuth revision required");
    bounded(attemptTtl, 300); bounded(resultTtl, 60);
    if (encryptionKey == null || encryptionKey.length != 32 || Arrays.equals(encryptionKey, new byte[32]))
      throw new IllegalArgumentException("linked OAuth requires a nonzero AES-256 key");
    encryptionKey = encryptionKey.clone(); providers = Map.copyOf(providers);
  }
  @Override public byte[] encryptionKey() { return encryptionKey.clone(); }
  // Prevent accidental credential logging by record-generated toString.
  @Override public String toString() { return "LinkedOAuthSettings[redacted]"; }
  public record Provider(boolean enabled, boolean s256, String authorizationUrl, String tokenUrl,
      String identityUrl, String clientId, String clientSecret, String callbackUri, Map<String,String> returns) {
    public Provider { returns = Map.copyOf(returns); }
    @Override public String toString() { return "LinkedOAuthProvider[redacted]"; }
  }
  public void validateProduction() {
    for (var entry : providers.entrySet()) {
      if (!Set.of("twitch", "youtube").contains(entry.getKey())) throw new IllegalArgumentException("unknown linked OAuth provider");
      Provider p = entry.getValue(); if (!p.enabled()) continue;
      if (!p.s256() || p.clientId() == null || p.clientId().isBlank() || p.clientSecret() == null || p.clientSecret().isBlank() || p.returns().isEmpty())
        throw new IllegalArgumentException("incomplete linked OAuth registration");
      exactHttps(p.authorizationUrl()); exactHttps(p.tokenUrl()); exactHttps(p.identityUrl()); exactHttps(p.callbackUri());
      if (!URI.create(p.callbackUri()).getPath().equals("/api/v1/auth/linked-accounts/" + entry.getKey() + "/oauth-callback"))
        throw new IllegalArgumentException("invalid linked OAuth callback path");
      for (var r : p.returns().entrySet()) {
        if (r.getKey().equals("web")) exactHttps(r.getValue());
        else if (r.getKey().equals("windows")) exactWindows(r.getValue());
        else throw new IllegalArgumentException("unqualified linked OAuth surface");
      }
    }
  }
  static void exactWindows(String value) {
    URI u=URI.create(value);
    if(!"voice".equals(u.getScheme()) || u.getHost()==null || u.getRawUserInfo()!=null || u.getPort()!=-1
        || u.getRawQuery()!=null || u.getRawFragment()!=null || u.getPath()==null || u.getPath().isEmpty()
        || value.contains("%") || value.contains("\\") || value.contains("*") || !u.normalize().equals(u))
      throw new IllegalArgumentException("linked OAuth Windows return must be an exact registered Voice route");
  }
  static void exactHttps(String value) {
    URI u;
    try { u = URI.create(value); } catch (RuntimeException e) { throw new IllegalArgumentException("invalid linked OAuth endpoint"); }
    if (!"https".equals(u.getScheme()) || u.getHost() == null || u.getRawUserInfo() != null || u.getRawQuery() != null
        || u.getRawFragment() != null || u.getPort() == 0 || u.getPath() == null || u.getPath().isEmpty()
        || value.contains("%") || value.contains("\\") || value.contains("*") || !u.normalize().equals(u))
      throw new IllegalArgumentException("linked OAuth endpoint must be exact HTTPS");
  }
  private static void bounded(Duration value, int max) {
    if (value == null || value.isNegative() || value.isZero() || value.compareTo(Duration.ofSeconds(max)) > 0)
      throw new IllegalArgumentException("invalid linked OAuth lifetime");
  }
}
