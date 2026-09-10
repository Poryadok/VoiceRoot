package voice.backend.auth.principal;

import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.File;
import java.util.Arrays;
import java.util.Set;
import org.springframework.core.env.Environment;

/** TLS policy for the dedicated Auth proof listener. */
public final class AuthPrincipalTransport {
  private AuthPrincipalTransport() {}

  public static void configure(NettyServerBuilder builder, Environment environment, boolean enabled) {
    String certificate = environment.getProperty("AUTH_GRPC_TLS_CERT_FILE");
    String key = environment.getProperty("AUTH_GRPC_TLS_KEY_FILE");
    if (certificate != null || key != null) {
      if (certificate == null || certificate.isBlank() || key == null || key.isBlank()
          || !new File(certificate).isFile() || !new File(key).isFile()) {
        throw new IllegalArgumentException("Auth principal TLS certificate and key files are required");
      }
      builder.useTransportSecurity(new File(certificate), new File(key));
    } else if (enabled && !local(environment)) {
      throw new IllegalArgumentException("Auth principal listener requires TLS");
    }
  }

  static boolean local(Environment environment) {
    String[] profiles = environment.getActiveProfiles();
    return profiles.length > 0 && Arrays.stream(profiles).allMatch(Set.of("local", "test")::contains);
  }

  public static int port(Environment environment, int legacyPort) {
    try {
      int port = Integer.parseInt(environment.getProperty("AUTH_PRINCIPAL_GRPC_PORT", "9091"));
      if (port < 0 || port > 65535 || (port == 0 && !local(environment))
          || (port > 0 && port == legacyPort)) throw new IllegalArgumentException();
      return port;
    } catch (RuntimeException ex) {
      throw new IllegalArgumentException("invalid Auth principal listener port");
    }
  }
}
