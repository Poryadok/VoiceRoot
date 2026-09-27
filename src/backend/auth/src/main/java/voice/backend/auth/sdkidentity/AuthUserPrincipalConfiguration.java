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
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/** Enables Auth's dedicated principal issuer and TLS User eligibility client. */
@Configuration(proxyBeanMethods = false)
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
public class AuthUserPrincipalConfiguration {
  @Bean
  AuthUserPrincipalIssuer authUserPrincipalIssuer(
      @Value("${AUTH_PRINCIPAL_SIGNING_KEYS_DIR:}") String directory,
      @Value("${AUTH_PRINCIPAL_ACTIVE_KID:}") String activeKid, Clock clock) {
    if (directory == null || directory.isBlank() || activeKid == null || activeKid.isBlank()) {
      throw new IllegalStateException("Auth principal signing keys are required when T14 authorization is enabled");
    }
    return AuthUserPrincipalIssuer.load(Path.of(directory), activeKid, clock);
  }

  @Bean(destroyMethod = "shutdownNow")
  ManagedChannel authUserPrincipalChannel(
      @Value("${AUTH_USER_PRINCIPAL_GRPC_ADDR:}") String address,
      @Value("${AUTH_USER_PRINCIPAL_TLS_CA_FILE:}") String caFile,
      @Value("${AUTH_USER_PRINCIPAL_TLS_SERVER_NAME:}") String serverName) {
    if (address == null || address.isBlank()) {
      throw new IllegalStateException("Auth User principal TLS endpoint is required when T14 authorization is enabled");
    }
    try {
      NettyChannelBuilder builder = NettyChannelBuilder.forTarget(address.trim());
      var ssl = GrpcSslContexts.forClient();
      if (caFile != null && !caFile.isBlank()) ssl.trustManager(trustManager(new File(caFile)));
      builder.sslContext(ssl.build());
      if (serverName != null && !serverName.isBlank()) builder.overrideAuthority(serverName.trim());
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
