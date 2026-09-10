package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import com.sun.net.httpserver.HttpServer;
import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.security.spec.PKCS8EncodedKeySpec;
import java.time.Duration;
import java.util.Base64;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class AuthPrincipalJwksHttpTest {
  HttpsServer server;
  HttpClient trusted;
  final ExecutorService executor = Executors.newCachedThreadPool(Thread.ofPlatform().daemon().factory());
  final AtomicInteger requests = new AtomicInteger();
  final byte[] jwks = AuthPrincipalJwksResolverTest.set(
      AuthPrincipalJwksResolverTest.jwk("current", AuthPrincipalJwksResolverTest.CURRENT),
      AuthPrincipalJwksResolverTest.jwk("next", AuthPrincipalJwksResolverTest.NEXT));

  @BeforeEach void startHttps() throws Exception {
    SSLContext context = fixtureContext();
    server = HttpsServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.setHttpsConfigurator(new HttpsConfigurator(context));
    server.setExecutor(executor);
    server.createContext("/jwks", exchange -> {
      requests.incrementAndGet();
      exchange.getResponseHeaders().set("Content-Type", "application/json");
      exchange.sendResponseHeaders(200, jwks.length);
      try (var output = exchange.getResponseBody()) { output.write(jwks); }
    });
    server.start();
    trusted = HttpClient.newBuilder().sslContext(context).connectTimeout(Duration.ofSeconds(2))
        .followRedirects(HttpClient.Redirect.NEVER).build();
  }

  @AfterEach void stopHttps() {
    if (trusted != null) trusted.shutdownNow();
    if (server != null) server.stop(0);
    executor.shutdownNow();
  }

  @Test void fetchesExactJwksBytesOverTrustedHttps() {
    assertArrayEquals(jwks, AuthPrincipalConfiguration.fetch(trusted, endpoint("/jwks")));
    assertEquals(1, requests.get());
  }

  @Test void rejectsUntrustedCertificateAndWrongHostnameBeforeHttpHandler() {
    HttpClient untrusted = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build();
    try {
      assertThrows(IllegalArgumentException.class,
          () -> AuthPrincipalConfiguration.fetch(untrusted, endpoint("/jwks")));
    } finally { untrusted.shutdownNow(); }
    URI wrongHostname = URI.create("https://127.0.0.1:" + server.getAddress().getPort() + "/jwks");
    assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fetch(trusted, wrongHostname));
    assertEquals(0, requests.get());
  }

  @Test void rejectsRedirectWithoutFetchingDestination() {
    server.createContext("/redirect", exchange -> {
      exchange.getResponseHeaders().set("Location", endpoint("/jwks").toString());
      exchange.sendResponseHeaders(302, -1); exchange.close();
    });
    assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fetch(trusted, endpoint("/redirect")));
    assertEquals(0, requests.get(), "redirect target must never be requested");
  }

  @Test void rejectsNon200EvenWhenBodyContainsValidJwks() {
    server.createContext("/unavailable", exchange -> {
      exchange.sendResponseHeaders(503, jwks.length);
      try (var output = exchange.getResponseBody()) { output.write(jwks); }
    });
    assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fetch(trusted, endpoint("/unavailable")));
  }

  @Test void acceptsLimitButRejectsOversizedFixedAndChunkedBodies() {
    byte[] limit = new byte[65536];
    server.createContext("/limit", exchange -> {
      exchange.sendResponseHeaders(200, limit.length);
      try (var output = exchange.getResponseBody()) { output.write(limit); }
    });
    assertArrayEquals(limit, AuthPrincipalConfiguration.fetch(trusted, endpoint("/limit")));
    for (String path : new String[] {"/large", "/chunked"}) {
      server.createContext(path, exchange -> {
        byte[] oversized = new byte[65537];
        exchange.sendResponseHeaders(200, path.equals("/chunked") ? 0 : oversized.length);
        try (var output = exchange.getResponseBody()) { output.write(oversized); }
        catch (IOException clientCancelled) { /* Body limit may close the connection. */ }
      });
      assertThrows(IllegalArgumentException.class,
          () -> AuthPrincipalConfiguration.fetch(trusted, endpoint(path)));
    }
  }

  @Test void slowEndpointFailsWithinTwoSecondFetchBudget() {
    server.createContext("/slow", exchange -> {
      try { Thread.sleep(5000); }
      catch (InterruptedException cancelled) { Thread.currentThread().interrupt(); }
      finally { exchange.close(); }
    });
    assertTimeoutPreemptively(Duration.ofSeconds(3), () -> assertThrows(IllegalArgumentException.class,
        () -> AuthPrincipalConfiguration.fetch(trusted, endpoint("/slow"))));
  }

  @Test void refusesPlaintextEndpointBeforeSendingRequest() throws Exception {
    HttpServer plaintext = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    plaintext.setExecutor(executor);
    plaintext.createContext("/jwks", exchange -> {
      requests.incrementAndGet();
      exchange.sendResponseHeaders(200, jwks.length);
      try (var output = exchange.getResponseBody()) { output.write(jwks); }
    });
    plaintext.start();
    try {
      URI endpoint = URI.create("http://localhost:" + plaintext.getAddress().getPort() + "/jwks");
      assertThrows(IllegalArgumentException.class, () -> AuthPrincipalConfiguration.fetch(trusted, endpoint));
      assertEquals(0, requests.get());
    } finally { plaintext.stop(0); }
  }

  URI endpoint(String path) { return URI.create("https://localhost:" + server.getAddress().getPort() + path); }

  static SSLContext fixtureContext() throws Exception {
    X509Certificate certificate;
    try (var input = AuthPrincipalJwksHttpTest.class.getResourceAsStream("/principal-tls/server-cert.pem")) {
      certificate = (X509Certificate) CertificateFactory.getInstance("X.509").generateCertificate(input);
    }
    String pem;
    try (var input = AuthPrincipalJwksHttpTest.class.getResourceAsStream("/principal-tls/server-key.pem")) {
      pem = new String(input.readAllBytes(), StandardCharsets.US_ASCII);
    }
    byte[] der = Base64.getMimeDecoder().decode(pem.replace("-----BEGIN PRIVATE KEY-----", "")
        .replace("-----END PRIVATE KEY-----", ""));
    var key = KeyFactory.getInstance("RSA").generatePrivate(new PKCS8EncodedKeySpec(der));
    char[] password = "public-test-fixture".toCharArray();
    KeyStore keys = KeyStore.getInstance("PKCS12"); keys.load(null, password);
    keys.setKeyEntry("server", key, password, new java.security.cert.Certificate[] {certificate});
    KeyManagerFactory keyManagers = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
    keyManagers.init(keys, password);
    KeyStore trust = KeyStore.getInstance("PKCS12"); trust.load(null, password);
    trust.setCertificateEntry("server", certificate);
    TrustManagerFactory trustManagers = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
    trustManagers.init(trust);
    SSLContext context = SSLContext.getInstance("TLS");
    context.init(keyManagers.getKeyManagers(), trustManagers.getTrustManagers(), null);
    return context;
  }
}
