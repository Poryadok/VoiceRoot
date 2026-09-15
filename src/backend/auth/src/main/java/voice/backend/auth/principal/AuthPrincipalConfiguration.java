package voice.backend.auth.principal;

import com.fasterxml.jackson.databind.JsonNode;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.data.redis.core.StringRedisTemplate;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

@Configuration
public class AuthPrincipalConfiguration {
  @Bean
  AuthPrincipalServerInterceptor authPrincipalServerInterceptor(
      Environment environment, SessionEpochFloorStore epochs, ObjectProvider<StringRedisTemplate> redis) {
    return fromEnvironment(environment, epochs, redis.getIfAvailable());
  }

  public static AuthPrincipalServerInterceptor fromEnvironment(
      Environment environment, SessionEpochFloorStore epochs, StringRedisTemplate redis) {
    for (String alias : Set.of("S2S_SIGNING_KEY_PEM", "S2S_SIGNING_KID")) {
      if (environment.containsProperty(alias)) throw new IllegalArgumentException("unsupported principal signing alias");
    }
    String raw = environment.getProperty("S2S_JWKS_URLS_JSON");
    if (raw == null) {
      for (String property : Set.of("S2S_JWKS_REFRESH_AFTER", "S2S_JWKS_HARD_EXPIRY", "S2S_UNKNOWN_KID_COOLDOWN", "S2S_JWKS_CA_FILE")) {
        if (environment.containsProperty(property)) throw new IllegalArgumentException("incomplete principal configuration");
      }
      return new AuthPrincipalServerInterceptor(null);
    }
    Map<String, URI> endpoints = endpoints(raw);
    Duration refresh = duration(environment, "S2S_JWKS_REFRESH_AFTER", Duration.ofSeconds(30));
    Duration hard = duration(environment, "S2S_JWKS_HARD_EXPIRY", Duration.ofMinutes(2));
    Duration cooldown = duration(environment, "S2S_UNKNOWN_KID_COOLDOWN", Duration.ofSeconds(5));
    if (hard.compareTo(refresh) < 0 || epochs == null) throw new IllegalArgumentException("invalid principal dependencies/cache policy");
    Clock clock = Clock.systemUTC();
    var clientBuilder = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2))
        .followRedirects(HttpClient.Redirect.NEVER);
    if (environment.containsProperty("S2S_JWKS_CA_FILE")) {
      clientBuilder.sslContext(privateCaContext(environment.getProperty("S2S_JWKS_CA_FILE")));
    }
    HttpClient client = clientBuilder.build();
    var resolver = new AuthPrincipalJwksResolver(issuer -> fetch(client, endpoints.get(issuer)),
        clock, refresh, hard, cooldown);
    AuthPrincipalVerifier.ReplayGuard replay;
    Runnable closeReplay;
    if ("memory".equalsIgnoreCase(environment.getProperty("auth.persistence", "jdbc"))) {
      if (!AuthPrincipalTransport.local(environment)) throw new IllegalArgumentException("memory principal replay requires local/test profile");
      Map<String, Instant> entries = new ConcurrentHashMap<>();
      replay = (issuer, jti, expires) -> {
        Instant now = clock.instant();
        entries.entrySet().removeIf(entry -> !entry.getValue().isAfter(now));
        if (!expires.isAfter(now) || entries.size() >= 100000
            || entries.putIfAbsent(issuer + ":" + jti, expires) != null) {
          throw new IllegalArgumentException("principal replay rejected");
        }
      };
      closeReplay = () -> {};
    } else {
      if (redis == null) throw new IllegalArgumentException("principal replay Redis is required");
      var sharedReplay = new RedisPrincipalReplayGuard(redis, clock);
      replay = sharedReplay;
      closeReplay = sharedReplay::close;
    }
    return new AuthPrincipalServerInterceptor(
        new AuthPrincipalVerifier(resolver::resolve, epochs::requireFloor, replay, clock),
        () -> { closeReplay.run(); client.shutdownNow(); });
  }

  private static Map<String, URI> endpoints(String raw) {
    try {
      JsonNode root = AuthPrincipalVerifier.JSON.readTree(raw);
      if (root == null || !root.isObject() || root.isEmpty()) throw new IllegalArgumentException();
      if (root.has("gateway") != root.has("space") || root.has("social") != root.has("moderation")) {
        throw new IllegalArgumentException();
      }
      Map<String, URI> endpoints = new HashMap<>();
      var fields = root.fields();
      while (fields.hasNext()) {
        var field = fields.next();
        if (!Set.of("gateway", "space", "social", "moderation").contains(field.getKey())
            || !field.getValue().isTextual()) throw new IllegalArgumentException();
        URI uri = URI.create(field.getValue().textValue());
        if (!"https".equals(uri.getScheme()) || uri.getHost() == null || uri.getRawUserInfo() != null || uri.getRawFragment() != null) throw new IllegalArgumentException();
        endpoints.put(field.getKey(), uri);
      }
      return Map.copyOf(endpoints);
    } catch (Exception ex) {
      throw new IllegalArgumentException("invalid principal HTTPS JWKS endpoints");
    }
  }

  private static Duration duration(Environment environment, String name, Duration fallback) {
    String value = environment.getProperty(name);
    if (value == null) return fallback;
    try {
      var matcher = java.util.regex.Pattern.compile("([0-9]+)(ms|s|m|h)").matcher(value);
      if (!matcher.matches()) throw new IllegalArgumentException();
      long amount = Long.parseLong(matcher.group(1));
      Duration parsed = switch (matcher.group(2)) {
        case "ms" -> Duration.ofMillis(amount);
        case "s" -> Duration.ofSeconds(amount);
        case "m" -> Duration.ofMinutes(amount);
        default -> Duration.ofHours(amount);
      };
      if (parsed.isZero()) throw new IllegalArgumentException();
      return parsed;
    } catch (Exception ex) {
      throw new IllegalArgumentException("invalid " + name);
    }
  }

  private static javax.net.ssl.SSLContext privateCaContext(String path) {
    try {
      if (path == null || path.isBlank()) throw new IllegalArgumentException();
      var file = java.nio.file.Path.of(path);
      if (!java.nio.file.Files.isRegularFile(file) || java.nio.file.Files.size(file) > 1024 * 1024) {
        throw new IllegalArgumentException();
      }
      java.util.Collection<? extends java.security.cert.Certificate> certificates;
      try (var input = java.nio.file.Files.newInputStream(file)) {
        certificates = java.security.cert.CertificateFactory.getInstance("X.509").generateCertificates(input);
      }
      if (certificates.isEmpty()) throw new IllegalArgumentException();
      var trust = java.security.KeyStore.getInstance(java.security.KeyStore.getDefaultType());
      trust.load(null, null);
      var defaults = javax.net.ssl.TrustManagerFactory.getInstance(javax.net.ssl.TrustManagerFactory.getDefaultAlgorithm());
      defaults.init((java.security.KeyStore) null);
      int index = 0;
      for (var manager : defaults.getTrustManagers()) {
        if (manager instanceof javax.net.ssl.X509TrustManager x509) {
          for (var certificate : x509.getAcceptedIssuers()) trust.setCertificateEntry("system-" + index++, certificate);
        }
      }
      for (var certificate : certificates) trust.setCertificateEntry("private-" + index++, certificate);
      var managers = javax.net.ssl.TrustManagerFactory.getInstance(javax.net.ssl.TrustManagerFactory.getDefaultAlgorithm());
      managers.init(trust);
      var context = javax.net.ssl.SSLContext.getInstance("TLS");
      context.init(null, managers.getTrustManagers(), null);
      return context;
    } catch (Exception invalid) {
      throw new IllegalArgumentException("invalid principal JWKS CA bundle");
    }
  }

  static byte[] fetch(HttpClient client, URI endpoint) {
    if (endpoint == null || !"https".equals(endpoint.getScheme()) || endpoint.getHost() == null
        || endpoint.getRawUserInfo() != null || endpoint.getRawFragment() != null) {
      throw new IllegalArgumentException("invalid principal HTTPS endpoint");
    }
    var request = HttpRequest.newBuilder(endpoint).timeout(Duration.ofSeconds(2)).GET().build();
    var result = client.sendAsync(request, HttpResponse.BodyHandlers.limiting(HttpResponse.BodyHandlers.ofByteArray(), 65536));
    try {
      var response = result.get(2, TimeUnit.SECONDS);
      if (response.statusCode() != 200) throw new IllegalArgumentException();
      return response.body();
    } catch (Exception ex) {
      result.cancel(true);
      if (ex instanceof InterruptedException) Thread.currentThread().interrupt();
      throw new IllegalArgumentException("principal JWKS unavailable");
    }
  }
}
