package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.cert.X509Certificate;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.web.MockFilterChain;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.mock.env.MockEnvironment;
import org.springframework.boot.web.embedded.tomcat.TomcatServletWebServerFactory;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class AuthGameBindingMtlsConfigurationTest {
  private static final String CLIENT_ID = "spiffe://voice/service/gameintegration";

  @TempDir Path files;

  @Test
  void listenerIsDisabledByDefault() {
    var settings = AuthGameBindingMtlsConfiguration.settings(new MockEnvironment());

    assertThat(settings.enabled()).isFalse();
  }

  @Test
  void rejectsPartialOrInvalidListenerConfiguration() throws Exception {
    var partial = new MockEnvironment().withProperty("AUTH_GAME_BINDING_MTLS_PORT", "9443")
        .withProperty("AUTH_GAME_BINDING_MTLS_SERVER_CERT_FILE", "server.pem");

    assertThatThrownBy(() -> AuthGameBindingMtlsConfiguration.settings(partial))
        .isInstanceOf(IllegalStateException.class);

    var invalidPort = completeEnvironment();
    invalidPort.setProperty("AUTH_GAME_BINDING_MTLS_PORT", "70000");
    assertThatThrownBy(() -> AuthGameBindingMtlsConfiguration.settings(invalidPort))
        .isInstanceOf(IllegalStateException.class);

    var invalidCertificate = invalidPort;
    invalidCertificate.setProperty("AUTH_GAME_BINDING_MTLS_PORT", "9443");
    Files.writeString(files.resolve("server.pem"), "invalid cert");
    assertThatThrownBy(() -> AuthGameBindingMtlsConfiguration.settings(invalidCertificate))
        .isInstanceOf(IllegalStateException.class);
  }

  @Test
  void rejectsJsseTruststoreContainingAuthoritiesOutsideTheConfiguredClientCa() throws Exception {
    var environment = completeEnvironment();
    Path truststorePath = Path.of(environment.getProperty("AUTH_GAME_BINDING_MTLS_TRUSTSTORE_FILE"));
    KeyStore unrelatedTruststore = KeyStore.getInstance("PKCS12");
    unrelatedTruststore.load(null, "test-password".toCharArray());
    try (OutputStream output = Files.newOutputStream(truststorePath)) {
      unrelatedTruststore.store(output, "test-password".toCharArray());
    }

    assertThatThrownBy(() -> AuthGameBindingMtlsConfiguration.settings(environment))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("matching JSSE truststore/client CA");
  }

  @Test
  void enablesOnlyWithAllFilesAndExactClientServiceIdentity() throws Exception {
    var settings = AuthGameBindingMtlsConfiguration.settings(completeEnvironment());

    assertThat(settings.enabled()).isTrue();
    assertThat(settings.port()).isEqualTo(9443);
    assertThat(settings.allowedClientUriSan()).isEqualTo(CLIENT_ID);
    assertThat(settings.serverCertFile()).isEqualTo(files.resolve("server.pem"));
    assertThat(settings.serverKeyFile()).isEqualTo(files.resolve("server-key.pem"));
    assertThat(settings.clientCaFile()).isEqualTo(files.resolve("client-ca.pem"));
    assertThat(settings.truststoreFile()).isEqualTo(files.resolve("client-ca.p12"));
    assertThat(settings.truststorePassword()).isEqualTo("test-password");
  }

  @Test
  void privateRoutesRequireVerifiedClientCertificateWithExactUriSan() throws Exception {
    var settings = AuthGameBindingMtlsConfiguration.settings(completeEnvironment());
    var filter = new AuthGameBindingClientIdentityFilter(new AuthGameBindingMtlsConfiguration.AuthGameBindingMtlsSettings(
        settings.enabled(), settings.port(), settings.serverCertFile(), settings.serverKeyFile(), settings.clientCaFile(),
        settings.allowedClientUriSan()));

    var missing = new MockHttpServletRequest("POST", "/internal/v1/auth/game-bindings/handoffs/claim");
    var missingResponse = new MockHttpServletResponse();
    filter.doFilterInternal(missing, missingResponse, new MockFilterChain());
    assertThat(missingResponse.getStatus()).isEqualTo(401);

    X509Certificate wrong = certificate("spiffe://voice/service/not-gis");
    var wrongRequest = new MockHttpServletRequest("POST", "/internal/v1/auth/game-bindings/handoffs/claim");
    wrongRequest.setAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE, new X509Certificate[] {wrong});
    var wrongResponse = new MockHttpServletResponse();
    filter.doFilterInternal(wrongRequest, wrongResponse, new MockFilterChain());
    assertThat(wrongResponse.getStatus()).isEqualTo(401);

    X509Certificate accepted = certificate(CLIENT_ID);
    var allowedRequest = new MockHttpServletRequest("POST", "/internal/v1/auth/game-bindings/handoffs/claim");
    allowedRequest.setAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE,
        new X509Certificate[] {accepted});
    var allowedResponse = new MockHttpServletResponse();
    var allowedChain = new MockFilterChain();
    filter.doFilterInternal(allowedRequest, allowedResponse, allowedChain);
    assertThat(allowedResponse.getStatus()).isEqualTo(200);
    assertThat(allowedChain.getRequest()).isSameAs(allowedRequest);
  }

  @Test
  void enabledListenerUsesSeparateHttpsConnectorWithMandatoryClientCertificates() throws Exception {
    var settings = AuthGameBindingMtlsConfiguration.settings(completeEnvironment());
    var authSettings = new AuthGameBindingMtlsConfiguration.AuthGameBindingMtlsSettings(settings.enabled(),
        settings.port(), settings.serverCertFile(), settings.serverKeyFile(), settings.clientCaFile(),
        settings.truststoreFile(), settings.truststorePassword(), settings.allowedClientUriSan(), "");
    var factory = new TomcatServletWebServerFactory();

    new AuthGameBindingMtlsConfiguration().authGameBindingMtlsConnector(authSettings).customize(factory);

    assertThat(factory.getAdditionalTomcatConnectors()).hasSize(1);
    var connector = factory.getAdditionalTomcatConnectors().get(0);
    assertThat(connector.getPort()).isEqualTo(9443);
    assertThat(connector.getScheme()).isEqualTo("https");
    assertThat(connector.getSecure()).isTrue();
    assertThat(connector.getProperty("SSLEnabled")).isEqualTo(true);
    var ssl = connector.findSslHostConfigs()[0];
    assertThat(ssl.getCertificateVerification()).hasToString("REQUIRED");
    assertThat(ssl.getCaCertificateFile()).isEqualTo(settings.clientCaFile().toString());
    assertThat(ssl.getTruststoreFile()).isEqualTo(settings.truststoreFile().toString());
    assertThat(ssl.getTruststoreType()).isEqualTo("PKCS12");
    assertThat(ssl.getTruststorePassword()).isEqualTo(settings.truststorePassword());
  }

  private static X509Certificate certificate(String uriSan) throws Exception {
    X509Certificate certificate = mock(X509Certificate.class);
    when(certificate.getSubjectAlternativeNames()).thenReturn(List.of(List.of(6, uriSan)));
    return certificate;
  }

  private MockEnvironment completeEnvironment() throws Exception {
    Path cert = copyFixture("server-cert.pem", "server.pem");
    Path key = copyFixture("server-key.pem", "server-key.pem");
    Path ca = copyFixture("server-cert.pem", "client-ca.pem");
    Path truststore = files.resolve("client-ca.p12");
    var certificates = java.security.cert.CertificateFactory.getInstance("X.509");
    java.security.cert.X509Certificate allowedCa;
    try (var input = Files.newInputStream(ca)) {
      allowedCa = (java.security.cert.X509Certificate) certificates.generateCertificate(input);
    }
    KeyStore clientTruststore = KeyStore.getInstance("PKCS12");
    clientTruststore.load(null, "test-password".toCharArray());
    clientTruststore.setCertificateEntry("allowed-client-ca", allowedCa);
    try (OutputStream output = Files.newOutputStream(truststore)) {
      clientTruststore.store(output, "test-password".toCharArray());
    }
    return new MockEnvironment().withProperty("AUTH_GAME_BINDING_MTLS_PORT", "9443")
        .withProperty("AUTH_GAME_BINDING_MTLS_SERVER_CERT_FILE", cert.toString())
        .withProperty("AUTH_GAME_BINDING_MTLS_SERVER_KEY_FILE", key.toString())
        .withProperty("AUTH_GAME_BINDING_MTLS_CLIENT_CA_FILE", ca.toString())
        .withProperty("AUTH_GAME_BINDING_MTLS_TRUSTSTORE_FILE", truststore.toString())
        .withProperty("AUTH_GAME_BINDING_MTLS_TRUSTSTORE_PASSWORD", "test-password")
        .withProperty("AUTH_GAME_BINDING_MTLS_ALLOWED_CLIENT_URI_SAN", CLIENT_ID);
  }

  private Path copyFixture(String resource, String target) throws Exception {
    try (var input = AuthGameBindingMtlsConfigurationTest.class.getResourceAsStream("/principal-tls/" + resource)) {
      Files.copy(input, files.resolve(target));
      return files.resolve(target);
    }
  }
}
