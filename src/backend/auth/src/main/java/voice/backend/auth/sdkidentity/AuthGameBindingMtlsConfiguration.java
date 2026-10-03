package voice.backend.auth.sdkidentity;

import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.util.Base64;
import java.util.List;
import org.apache.catalina.connector.Connector;
import org.apache.tomcat.util.net.SSLHostConfig;
import org.apache.tomcat.util.net.SSLHostConfigCertificate;
import org.springframework.boot.web.embedded.tomcat.TomcatServletWebServerFactory;
import org.springframework.boot.web.server.WebServerFactoryCustomizer;
import org.springframework.boot.web.servlet.FilterRegistrationBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;

/** Dedicated, opt-in mTLS listener for private GIS binding handoff calls. */
@Configuration(proxyBeanMethods = false)
public class AuthGameBindingMtlsConfiguration {
  private static final String PORT = "AUTH_GAME_BINDING_MTLS_PORT";
  private static final String SERVER_CERT = "AUTH_GAME_BINDING_MTLS_SERVER_CERT_FILE";
  private static final String SERVER_KEY = "AUTH_GAME_BINDING_MTLS_SERVER_KEY_FILE";
  private static final String CLIENT_CA = "AUTH_GAME_BINDING_MTLS_CLIENT_CA_FILE";
  private static final String TRUSTSTORE_FILE = "AUTH_GAME_BINDING_MTLS_TRUSTSTORE_FILE";
  private static final String TRUSTSTORE_PASSWORD = "AUTH_GAME_BINDING_MTLS_TRUSTSTORE_PASSWORD";
  private static final String CLIENT_SAN = "AUTH_GAME_BINDING_MTLS_ALLOWED_CLIENT_URI_SAN";
  private static final String MESSAGING_CLIENT_SAN = "AUTH_GAME_MESSAGE_EXECUTION_PERMIT_ALLOWED_CLIENT_URI_SAN";

  record Settings(boolean enabled, int port, Path serverCertFile, Path serverKeyFile,
                  Path clientCaFile, Path truststoreFile, String truststorePassword,
                  String allowedClientUriSan, String allowedMessagingClientUriSan) {
    Settings(boolean enabled, int port, Path serverCertFile, Path serverKeyFile, Path clientCaFile,
        String allowedClientUriSan) {
      this(enabled, port, serverCertFile, serverKeyFile, clientCaFile, null, null, allowedClientUriSan, "");
    }
    Settings(boolean enabled, int port, Path serverCertFile, Path serverKeyFile, Path clientCaFile,
        String allowedClientUriSan, String allowedMessagingClientUriSan) {
      this(enabled, port, serverCertFile, serverKeyFile, clientCaFile, null, null,
          allowedClientUriSan, allowedMessagingClientUriSan);
    }
  }

  static Settings settings(Environment environment) {
    String rawPort = property(environment, "voice.auth.game-binding.mtls.port", PORT);
    String cert = property(environment, "voice.auth.game-binding.mtls.server-cert-file", SERVER_CERT);
    String key = property(environment, "voice.auth.game-binding.mtls.server-key-file", SERVER_KEY);
    String ca = property(environment, "voice.auth.game-binding.mtls.client-ca-file", CLIENT_CA);
    String truststore = property(environment, "voice.auth.game-binding.mtls.truststore-file", TRUSTSTORE_FILE);
    String truststorePassword = property(environment, "voice.auth.game-binding.mtls.truststore-password", TRUSTSTORE_PASSWORD);
    String san = property(environment, "voice.auth.game-binding.mtls.allowed-client-uri-san", CLIENT_SAN);
    String messagingSan = property(environment, "voice.auth.game-message.mtls.allowed-client-uri-san", MESSAGING_CLIENT_SAN);
    boolean configured = List.of(cert, key, ca, truststore, truststorePassword, san, messagingSan)
        .stream().anyMatch(value -> !value.isBlank());
    if (rawPort == null || rawPort.isBlank()) {
      if (configured) throw invalid();
      return new Settings(false, 0, null, null, null, null, null, null, null);
    }
    final int port;
    try { port = Integer.parseInt(rawPort); }
    catch (NumberFormatException malformed) { throw invalid(); }
    if (port == 0 && !configured) return new Settings(false, 0, null, null, null, null, null, null, null);
    if (port < 1 || port > 65535 || cert.isBlank() || key.isBlank() || ca.isBlank()
        || truststore.isBlank() || truststorePassword.isBlank() || !validUriSan(san)) throw invalid();
    Path certPath = readableFile(cert);
    Path keyPath = readableFile(key);
    Path caPath = readableFile(ca);
    Path truststorePath = readableFile(truststore);
    validateCertificateMaterial(certPath, keyPath, caPath, truststorePath, truststorePassword);
    if (!messagingSan.isBlank() && !validUriSan(messagingSan)) throw invalid();
    return new Settings(true, port, certPath, keyPath, caPath, truststorePath, truststorePassword, san, messagingSan);
  }

  private static boolean validUriSan(String san) {
    if (san == null || san.isBlank() || san.length() > 512) return false;
    try {
      java.net.URI uri = java.net.URI.create(san);
      return uri.isAbsolute() && uri.getScheme() != null && uri.getRawUserInfo() == null
          && uri.getRawQuery() == null && uri.getRawFragment() == null;
    } catch (RuntimeException malformed) { return false; }
  }

  @Bean
  AuthGameBindingMtlsSettings authGameBindingMtlsSettings(Environment environment) {
    Settings settings = settings(environment);
    return new AuthGameBindingMtlsSettings(settings.enabled(), settings.port(), settings.serverCertFile(),
        settings.serverKeyFile(), settings.clientCaFile(), settings.truststoreFile(), settings.truststorePassword(),
        settings.allowedClientUriSan(),
        settings.allowedMessagingClientUriSan());
  }

  @Bean
  WebServerFactoryCustomizer<TomcatServletWebServerFactory> authGameBindingMtlsConnector(
      AuthGameBindingMtlsSettings settings) {
    return factory -> {
      if (!settings.enabled()) return;
      Connector connector = new Connector();
      connector.setPort(settings.port());
      connector.setScheme("https");
      connector.setSecure(true);
      connector.setProperty("SSLEnabled", "true");
      SSLHostConfig ssl = new SSLHostConfig();
      ssl.setCertificateVerification("required");
      // caCertificateFile is used by Tomcat's OpenSSL provider. The Temurin runtime
      // uses JSSE, which reads the client trust roots from truststoreFile instead.
      ssl.setCaCertificateFile(settings.clientCaFile().toString());
      ssl.setTruststoreFile(settings.truststoreFile().toString());
      ssl.setTruststoreType("PKCS12");
      ssl.setTruststorePassword(settings.truststorePassword());
      SSLHostConfigCertificate certificate = new SSLHostConfigCertificate(
          ssl, SSLHostConfigCertificate.Type.UNDEFINED);
      certificate.setCertificateFile(settings.serverCertFile().toString());
      certificate.setCertificateKeyFile(settings.serverKeyFile().toString());
      ssl.addCertificate(certificate);
      connector.addSslHostConfig(ssl);
      factory.addAdditionalTomcatConnectors(connector);
    };
  }

  @Bean
  FilterRegistrationBean<AuthGameBindingClientIdentityFilter> authGameBindingClientIdentityFilter(
      AuthGameBindingMtlsSettings settings) {
    FilterRegistrationBean<AuthGameBindingClientIdentityFilter> registration = new FilterRegistrationBean<>();
    registration.setFilter(new AuthGameBindingClientIdentityFilter(settings));
    registration.addUrlPatterns("/internal/v1/auth/game-bindings/handoffs/*");
    registration.setOrder(Integer.MIN_VALUE + 32);
    return registration;
  }

  @Bean
  FilterRegistrationBean<AuthGameMessagePermitClientIdentityFilter> authGameMessagePermitClientIdentityFilter(
      AuthGameBindingMtlsSettings settings) {
    FilterRegistrationBean<AuthGameMessagePermitClientIdentityFilter> registration = new FilterRegistrationBean<>();
    registration.setFilter(new AuthGameMessagePermitClientIdentityFilter(settings));
    registration.addUrlPatterns("/api/v1/auth/sdk/game-message/execution-permits",
        "/api/v1/auth/sdk/game-message/execution-permits/*");
    registration.setOrder(Integer.MIN_VALUE + 33);
    return registration;
  }

  private static String property(Environment environment, String property, String env) {
    String value = environment.getProperty(property);
    if (value == null) value = environment.getProperty(env);
    return value == null ? "" : value.trim();
  }

  private static Path readableFile(String raw) {
    try {
      Path path = Path.of(raw).toAbsolutePath().normalize();
      if (!Files.isRegularFile(path) || !Files.isReadable(path)) throw invalid();
      return path;
    } catch (RuntimeException invalidPath) { throw invalid(); }
  }

  private static void validateCertificateMaterial(Path certPath, Path keyPath, Path caPath,
      Path truststorePath, String truststorePassword) {
    try {
      CertificateFactory certificates = CertificateFactory.getInstance("X.509");
      X509Certificate server;
      try (var input = Files.newInputStream(certPath)) {
        server = (X509Certificate) certificates.generateCertificate(input);
      }
      server.checkValidity();
      String pem = Files.readString(keyPath).trim();
      if (!pem.startsWith("-----BEGIN PRIVATE KEY-----") || !pem.endsWith("-----END PRIVATE KEY-----")) {
        throw invalid();
      }
      byte[] der = Base64.getDecoder().decode(pem.substring("-----BEGIN PRIVATE KEY-----".length(),
          pem.length() - "-----END PRIVATE KEY-----".length()).replaceAll("\\s", ""));
      var privateKey = KeyFactory.getInstance("RSA").generatePrivate(new PKCS8EncodedKeySpec(der));
      if (!(privateKey instanceof RSAPrivateCrtKey rsaPrivate)
          || !(server.getPublicKey() instanceof RSAPublicKey rsaPublic)
          || rsaPrivate.getModulus().bitLength() < 2048
          || !rsaPrivate.getModulus().equals(rsaPublic.getModulus())
          || !rsaPrivate.getPublicExponent().equals(rsaPublic.getPublicExponent())) throw invalid();
      java.util.Set<X509Certificate> allowedAuthorities = new java.util.HashSet<>();
      try (var input = Files.newInputStream(caPath)) {
        for (var authority : certificates.generateCertificates(input)) {
          allowedAuthorities.add((X509Certificate) authority);
        }
      }
      java.util.Set<X509Certificate> trustedAuthorities = new java.util.HashSet<>();
      var truststore = java.security.KeyStore.getInstance("PKCS12");
      try (var input = Files.newInputStream(truststorePath)) {
        truststore.load(input, truststorePassword.toCharArray());
      }
      var aliases = truststore.aliases();
      while (aliases.hasMoreElements()) {
        var trusted = truststore.getCertificate(aliases.nextElement());
        if (trusted instanceof X509Certificate x509) trustedAuthorities.add(x509);
      }
      if (allowedAuthorities.isEmpty() || trustedAuthorities.isEmpty()
          || !allowedAuthorities.equals(trustedAuthorities)) throw invalid();
    } catch (Exception malformed) {
      throw invalid();
    }
  }

  private static IllegalStateException invalid() {
    return new IllegalStateException("Auth game-binding mTLS requires a valid port, server cert/key, matching JSSE truststore/client CA and exact GIS URI SAN");
  }

  record AuthGameBindingMtlsSettings(boolean enabled, int port, Path serverCertFile, Path serverKeyFile,
                                     Path clientCaFile, Path truststoreFile, String truststorePassword,
                                     String allowedClientUriSan,
                                     String allowedMessagingClientUriSan) {
    AuthGameBindingMtlsSettings(boolean enabled, int port, Path serverCertFile, Path serverKeyFile,
        Path clientCaFile, String allowedClientUriSan) {
      this(enabled, port, serverCertFile, serverKeyFile, clientCaFile, null, null, allowedClientUriSan, "");
    }
    AuthGameBindingMtlsSettings(boolean enabled, int port, Path serverCertFile, Path serverKeyFile,
        Path clientCaFile, String allowedClientUriSan, String allowedMessagingClientUriSan) {
      this(enabled, port, serverCertFile, serverKeyFile, clientCaFile, null, null,
          allowedClientUriSan, allowedMessagingClientUriSan);
    }
  }
}
