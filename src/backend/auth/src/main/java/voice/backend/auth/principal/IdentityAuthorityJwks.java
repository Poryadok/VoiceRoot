package voice.backend.auth.principal;

import java.net.URI;
import java.net.http.HttpClient;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.Duration;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;

/** HTTPS trust for the dedicated identity capability; no legacy caller policy changes. */
public final class IdentityAuthorityJwks implements AuthPrincipalVerifier.KeyResolver, AutoCloseable {
  private final Map<String, URI> endpoints;
  private final HttpClient http;
  private final AuthPrincipalJwksResolver resolver;
  private volatile boolean closed;

  public IdentityAuthorityJwks(Map<String, URI> endpoints, Path ca, Clock clock,
      Duration refresh, Duration hardExpiry, Duration cooldown) {
    this(endpoints, Set.of("role", "user"), ca, clock, refresh, hardExpiry, cooldown);
  }

  /** Separate finite Subscription issuer policy; identity listeners retain their old pair. */
  public static IdentityAuthorityJwks subscriptionLifecycle(URI endpoint, Path ca, Clock clock,
      Duration refresh, Duration hardExpiry, Duration cooldown) {
    var result = new IdentityAuthorityJwks(Map.of("subscription", endpoint), Set.of("subscription"), ca, clock, refresh, hardExpiry, cooldown);
    try { result.resolver.prime("subscription"); return result; }
    catch (RuntimeException unavailable) { result.close(); throw unavailable; }
  }

  private IdentityAuthorityJwks(Map<String, URI> endpoints, Set<String> requiredIssuers, Path ca, Clock clock,
      Duration refresh, Duration hardExpiry, Duration cooldown) {
    HttpClient pending = null;
    try {
      this.endpoints = Map.copyOf(endpoints);
      if (!this.endpoints.keySet().equals(requiredIssuers)) throw new IllegalArgumentException();
      for (URI endpoint : this.endpoints.values()) {
        if (!"https".equals(endpoint.getScheme()) || endpoint.getHost() == null
            || endpoint.getRawUserInfo() != null || endpoint.getRawFragment() != null) throw new IllegalArgumentException();
      }
      Objects.requireNonNull(clock);
      Objects.requireNonNull(refresh);
      Objects.requireNonNull(hardExpiry);
      Objects.requireNonNull(cooldown);
      if (refresh.isNegative() || refresh.isZero() || hardExpiry.compareTo(refresh) < 0
          || cooldown.isNegative() || cooldown.isZero()) throw new IllegalArgumentException();
      var builder = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2))
          .followRedirects(HttpClient.Redirect.NEVER);
      if (ca != null) builder.sslContext(trust(ca));
      pending = builder.build();
      this.http = pending;
      this.resolver = new AuthPrincipalJwksResolver(
          issuer -> AuthPrincipalConfiguration.fetch(http, this.endpoints.get(issuer)),
          clock, refresh, hardExpiry, cooldown);
    } catch (Exception failure) {
      if (pending != null) pending.shutdownNow();
      throw new IllegalArgumentException("invalid identity authority HTTPS trust configuration");
    }
  }

  @Override public RSAPublicKey resolve(String issuer, String kid) {
    if (closed || issuer == null || !endpoints.containsKey(issuer)) throw new PrincipalDependencyUnavailableException();
    return resolver.resolve(issuer, kid);
  }

  private static SSLContext trust(Path ca) throws Exception {
    var keys = KeyStore.getInstance("PKCS12"); keys.load(null, null);
    try (var input = Files.newInputStream(ca)) {
      int count = 0;
      for (var certificate : CertificateFactory.getInstance("X.509").generateCertificates(input)) {
        keys.setCertificateEntry("identity-authority-ca-" + count++, certificate);
      }
      if (count == 0) throw new IllegalArgumentException();
    }
    var managers = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
    managers.init(keys);
    var context = SSLContext.getInstance("TLS");
    context.init(null, managers.getTrustManagers(), null);
    return context;
  }

  @Override public void close() { closed = true; http.shutdownNow(); }
}
