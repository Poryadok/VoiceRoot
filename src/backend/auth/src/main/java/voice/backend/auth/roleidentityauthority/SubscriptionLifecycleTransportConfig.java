package voice.backend.auth.roleidentityauthority;

import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import org.springframework.core.env.Environment;

/** Complete dedicated capability configuration; absence never borrows an ordinary listener. */
public record SubscriptionLifecycleTransportConfig(InetSocketAddress listen, Path certificate, Path key,
    Path clientCa, String subscriptionPeerUri, URI jwks, Path jwksCa) {
  private static final String PREFIX = "AUTH_SUBSCRIPTION_LIFECYCLE_";
  static SubscriptionLifecycleTransportConfig load(Environment environment) {
    var names = java.util.List.of("LISTEN_ADDR","TLS_CERT_FILE","TLS_KEY_FILE","TLS_CLIENT_CA_FILE","SUBSCRIPTION_PEER_URI","JWKS_URL","JWKS_CA_FILE");
    if (names.stream().noneMatch(name -> environment.containsProperty(PREFIX+name))) return null;
    try {
      URI address = URI.create("tcp://"+environment.getProperty(PREFIX+"LISTEN_ADDR", ""));
      int port = address.getPort();
      if (address.getHost() == null || port < 1 || port > 65535 || address.getRawUserInfo() != null || address.getRawQuery() != null || address.getRawFragment() != null
          || (address.getRawPath() != null && !address.getRawPath().isEmpty())) throw new IllegalArgumentException();
      URI endpoint = URI.create(environment.getProperty(PREFIX+"JWKS_URL", ""));
      if (!"https".equals(endpoint.getScheme()) || endpoint.getHost() == null || endpoint.getRawUserInfo() != null || endpoint.getRawFragment() != null) throw new IllegalArgumentException();
      return new SubscriptionLifecycleTransportConfig(new InetSocketAddress(address.getHost(),port), file(environment,"TLS_CERT_FILE"), file(environment,"TLS_KEY_FILE"),
          file(environment,"TLS_CLIENT_CA_FILE"), environment.getRequiredProperty(PREFIX+"SUBSCRIPTION_PEER_URI"), endpoint,
          environment.containsProperty(PREFIX+"JWKS_CA_FILE") ? file(environment,"JWKS_CA_FILE") : null);
    } catch (RuntimeException invalid) { throw new IllegalArgumentException("invalid Subscription lifecycle transport configuration"); }
  }
  private static Path file(Environment environment, String name) {
    Path value = Path.of(environment.getRequiredProperty(PREFIX+name));
    if (!Files.isRegularFile(value) || !Files.isReadable(value)) throw new IllegalArgumentException();
    return value;
  }
}
