package voice.backend.auth.sdkidentity;

import app.voice.user.v1.UserServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import java.io.File;
import java.io.FileInputStream;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.cert.Certificate;
import java.security.cert.CertificateFactory;
import java.time.Clock;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.core.env.Environment;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/** Enables Auth's dedicated principal issuer and TLS User eligibility client. */
@Configuration(proxyBeanMethods = false)
public class AuthUserPrincipalConfiguration {
  private static final String KEYS_DIR = "AUTH_PRINCIPAL_SIGNING_KEYS_DIR";
  private static final String ACTIVE_KID = "AUTH_PRINCIPAL_ACTIVE_KID";
  private static final String GRPC_ADDR = "AUTH_USER_PRINCIPAL_GRPC_ADDR";
  private static final String TLS_CA_FILE = "AUTH_USER_PRINCIPAL_TLS_CA_FILE";
  private static final String TLS_SERVER_NAME = "AUTH_USER_PRINCIPAL_TLS_SERVER_NAME";

  @Bean
  PrincipalSettings authUserPrincipalSettings(Environment environment, Clock clock) {
    boolean enabled = environment.getProperty("auth.sdk-authorization.enabled", Boolean.class, false);
    boolean anySettingPresent = List.of(KEYS_DIR, ACTIVE_KID, GRPC_ADDR, TLS_CA_FILE, TLS_SERVER_NAME)
        .stream().anyMatch(environment::containsProperty);
    if (!enabled && !anySettingPresent) return new PrincipalSettings(false, null, null, null, null);

    String directory = environment.getProperty(KEYS_DIR, "");
    String activeKid = environment.getProperty(ACTIVE_KID, "");
    String address = environment.getProperty(GRPC_ADDR, "");
    String caFile = environment.getProperty(TLS_CA_FILE, "");
    String serverName = environment.getProperty(TLS_SERVER_NAME, "");
    if (directory.isBlank() || activeKid.isBlank() || address.isBlank()) {
      throw new IllegalStateException("Auth principal issuer requires signing keys, active key ID, and User TLS endpoint");
    }
    if (environment.containsProperty(TLS_CA_FILE) && caFile.isBlank()) {
      throw new IllegalStateException("Auth User principal TLS CA file must not be blank");
    }
    if (environment.containsProperty(TLS_SERVER_NAME) && serverName.isBlank()) {
      throw new IllegalStateException("Auth User principal TLS server name must not be blank");
    }

    AuthUserPrincipalIssuer validatedIssuer = AuthUserPrincipalIssuer.load(Path.of(directory), activeKid, clock);
    try {
      NettyChannelBuilder.forTarget(address.trim());
      if (!caFile.isBlank()) trustManager(new File(caFile));
    } catch (Exception invalidConfiguration) {
      throw new IllegalStateException("invalid Auth-to-User principal TLS configuration", invalidConfiguration);
    }
    return new PrincipalSettings(enabled, enabled ? validatedIssuer : null, address, caFile, serverName);
  }

  record PrincipalSettings(boolean enabled, AuthUserPrincipalIssuer issuer, String address, String caFile,
      String serverName) {}

  @Bean
  @ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
  AuthUserPrincipalIssuer authUserPrincipalIssuer(
      PrincipalSettings settings) {
    return settings.issuer();
  }

  @Bean(destroyMethod = "shutdownNow")
  @ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
  ManagedChannel authUserPrincipalChannel(
      PrincipalSettings settings) {
    try {
      NettyChannelBuilder builder = NettyChannelBuilder.forTarget(settings.address().trim());
      var ssl = GrpcSslContexts.forClient();
      if (settings.caFile() != null && !settings.caFile().isBlank()) {
        ssl.trustManager(trustManager(new File(settings.caFile())));
      }
      builder.sslContext(ssl.build());
      if (settings.serverName() != null && !settings.serverName().isBlank()) {
        builder.overrideAuthority(settings.serverName().trim());
      }
      return builder.build();
    } catch (Exception invalidTls) {
      throw new IllegalStateException("invalid Auth-to-User principal TLS configuration", invalidTls);
    }
  }

  private static X509TrustManager trustManager(File additionalCa) throws Exception {
    TrustManagerFactory systemFactory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
    systemFactory.init((KeyStore) null);
    X509TrustManager system = findX509(systemFactory.getTrustManagers());

    Collection<? extends Certificate> certificates;
    try (FileInputStream input = new FileInputStream(additionalCa)) {
      certificates = CertificateFactory.getInstance("X.509").generateCertificates(input);
    }
    if (certificates.isEmpty()) throw new IllegalArgumentException("additional TLS CA file is empty");
    KeyStore additionalStore = KeyStore.getInstance(KeyStore.getDefaultType());
    additionalStore.load(null, null);
    int index = 0;
    for (Certificate certificate : certificates) additionalStore.setCertificateEntry("auth-user-ca-" + index++, certificate);
    TrustManagerFactory additionalFactory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
    additionalFactory.init(additionalStore);
    X509TrustManager additional = findX509(additionalFactory.getTrustManagers());
    return new CompositeTrustManager(system, additional);
  }

  private static X509TrustManager findX509(javax.net.ssl.TrustManager[] managers) {
    for (javax.net.ssl.TrustManager manager : managers) {
      if (manager instanceof X509TrustManager x509) return x509;
    }
    throw new IllegalStateException("X.509 trust manager is unavailable");
  }

  private record CompositeTrustManager(X509TrustManager system, X509TrustManager additional)
      implements X509TrustManager {
    @Override public void checkClientTrusted(java.security.cert.X509Certificate[] chain, String authType)
        throws java.security.cert.CertificateException {
      try { system.checkClientTrusted(chain, authType); }
      catch (java.security.cert.CertificateException rejected) { additional.checkClientTrusted(chain, authType); }
    }
    @Override public void checkServerTrusted(java.security.cert.X509Certificate[] chain, String authType)
        throws java.security.cert.CertificateException {
      try { system.checkServerTrusted(chain, authType); }
      catch (java.security.cert.CertificateException rejected) { additional.checkServerTrusted(chain, authType); }
    }
    @Override public java.security.cert.X509Certificate[] getAcceptedIssuers() {
      List<java.security.cert.X509Certificate> issuers = new ArrayList<>();
      issuers.addAll(List.of(system.getAcceptedIssuers()));
      issuers.addAll(List.of(additional.getAcceptedIssuers()));
      return issuers.toArray(java.security.cert.X509Certificate[]::new);
    }
  }

  @Bean
  @ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
  SdkProfileEligibility authUserProfileEligibilityClient(ManagedChannel authUserPrincipalChannel,
      AuthUserPrincipalIssuer authUserPrincipalIssuer,
      @Value("${auth.user-grpc.deadline:PT15S}") String deadlineValue) {
    Duration deadline;
    try {
      deadline = Duration.parse(deadlineValue);
    } catch (RuntimeException malformed) {
      throw new IllegalArgumentException("auth.user-grpc.deadline must be positive", malformed);
    }
    if (deadline.isZero() || deadline.isNegative()) {
      throw new IllegalArgumentException("auth.user-grpc.deadline must be positive");
    }
    return new AuthUserProfileEligibilityClient(
        UserServiceGrpc.newBlockingStub(authUserPrincipalChannel), authUserPrincipalIssuer, deadline);
  }
}
