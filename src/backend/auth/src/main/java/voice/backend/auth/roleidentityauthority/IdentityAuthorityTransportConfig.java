package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.springframework.core.env.Environment;

/** Separate explicit capability. Parsing neither opens a listener nor fetches trust material. */
public record IdentityAuthorityTransportConfig(InetSocketAddress listen, Path certificate, Path key,
    Path clientCa, Path jwksCa, Map<String, URI> jwks,
    Duration refresh, Duration hardExpiry, Duration cooldown) {
  private static final String PREFIX = "AUTH_IDENTITY_AUTHORITY_";
  private static final List<String> SETTINGS = List.of("GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE",
      "TLS_CLIENT_CA_FILE", "JWKS_CA_FILE");

  public IdentityAuthorityTransportConfig { jwks = Map.copyOf(jwks); }

  public static IdentityAuthorityTransportConfig load(Environment environment) {
    try {
      String enabled = environment.getProperty(PREFIX + "ENABLED", "false");
      if (!Set.of("true", "false").contains(enabled)) throw new IllegalArgumentException();
      if (!enabled.equals("true")) {
        if (SETTINGS.stream().anyMatch(name -> environment.containsProperty(PREFIX + name))) {
          throw new IllegalArgumentException();
        }
        return null;
      }
      String address = environment.getProperty(PREFIX + "GRPC_LISTEN", ":9094");
      URI parsed = URI.create("tcp://" + (address.startsWith(":") ? "0.0.0.0" + address : address));
      int port = parsed.getPort();
      boolean test = environment.getActiveProfiles().length > 0 && Arrays.stream(environment.getActiveProfiles())
          .allMatch(Set.of("test", "local")::contains);
      if (parsed.getHost() == null || parsed.getRawUserInfo() != null || parsed.getRawQuery() != null
          || parsed.getRawFragment() != null || !parsed.getPath().isEmpty()
          || port < 0 || port > 65535 || (port == 0 && !test)) throw new IllegalArgumentException();
      var mapper = new ObjectMapper().enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
          .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
      var root = mapper.readTree(environment.getProperty("S2S_JWKS_URLS_JSON", ""));
      if (root == null || !root.isObject()) throw new IllegalArgumentException();
      Map<String, URI> endpoints = new LinkedHashMap<>();
      for (String issuer : List.of("role", "user")) {
        var value = root.get(issuer);
        if (value == null || !value.isTextual()) throw new IllegalArgumentException();
        URI endpoint = URI.create(value.textValue());
        if (!"https".equals(endpoint.getScheme()) || endpoint.getHost() == null
            || endpoint.getRawUserInfo() != null || endpoint.getRawFragment() != null) throw new IllegalArgumentException();
        endpoints.put(issuer, endpoint);
      }
      Duration refresh = duration(environment, "S2S_JWKS_REFRESH_AFTER", Duration.ofSeconds(30));
      Duration hard = duration(environment, "S2S_JWKS_HARD_EXPIRY", Duration.ofMinutes(2));
      Duration cooldown = duration(environment, "S2S_UNKNOWN_KID_COOLDOWN", Duration.ofSeconds(5));
      if (hard.compareTo(refresh) < 0) throw new IllegalArgumentException();
      return new IdentityAuthorityTransportConfig(new InetSocketAddress(parsed.getHost(), port),
          file(environment, "TLS_CERT_FILE"), file(environment, "TLS_KEY_FILE"),
          file(environment, "TLS_CLIENT_CA_FILE"),
          environment.containsProperty(PREFIX + "JWKS_CA_FILE") ? file(environment, "JWKS_CA_FILE") : null,
          endpoints, refresh, hard, cooldown);
    } catch (Exception ex) {
      throw new IllegalArgumentException("invalid identity authority transport configuration");
    }
  }

  private static Path file(Environment environment, String name) {
    String value = environment.getProperty(PREFIX + name, "");
    if (value.isBlank()) throw new IllegalArgumentException();
    Path file = Path.of(value);
    if (!Files.isRegularFile(file) || !Files.isReadable(file)) throw new IllegalArgumentException();
    return file;
  }

  private static Duration duration(Environment environment, String name, Duration fallback) {
    String value = environment.getProperty(name);
    if (value == null) return fallback;
    var matcher = java.util.regex.Pattern.compile("([0-9]+)(ms|s|m|h)").matcher(value);
    if (!matcher.matches()) throw new IllegalArgumentException();
    long amount = Long.parseLong(matcher.group(1));
    Duration duration = switch (matcher.group(2)) {
      case "ms" -> Duration.ofMillis(amount);
      case "s" -> Duration.ofSeconds(amount);
      case "m" -> Duration.ofMinutes(amount);
      default -> Duration.ofHours(amount);
    };
    if (duration.isZero()) throw new IllegalArgumentException();
    return duration;
  }
}
