package voice.backend.auth.sessionepoch;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.nimbusds.jose.jwk.RSAKey;
import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsServer;
import java.net.InetSocketAddress;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.Base64;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;

/** Disposable caller-owned credentials and actual HTTPS JWKS, never a Go/Auth provider fake. */
final class ChatFloorFixtureTestMaterial implements AutoCloseable {
  final Path root;
  final KeyPair signer;
  final KeyPair foreignSigner;
  final AtomicInteger requests = new AtomicInteger();
  final AtomicInteger httpStatus = new AtomicInteger(200);
  final HttpsServer https;
  final java.util.concurrent.ExecutorService executor = Executors.newCachedThreadPool(
      Thread.ofPlatform().daemon().factory());

  ChatFloorFixtureTestMaterial(Path root) throws Exception {
    this.root = root;
    for (String name : new String[] {"server-cert.pem", "server-key.pem"})
      try (var input = getClass().getResourceAsStream("/principal-tls/" + name)) {
        Files.copy(input, root.resolve(name));
      }
    Files.writeString(root.resolve("chat.ext"),
        "basicConstraints=CA:FALSE\nkeyUsage=digitalSignature\nextendedKeyUsage=clientAuth\nsubjectAltName=URI:urn:voice:test:chat-floor\n");
    command("openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=chat-fixture",
        "-keyout", root.resolve("chat-key.pem").toString(), "-out", root.resolve("chat.csr").toString());
    command("openssl", "x509", "-req", "-in", root.resolve("chat.csr").toString(),
        "-CA", root.resolve("server-cert.pem").toString(), "-CAkey", root.resolve("server-key.pem").toString(),
        "-set_serial", "225", "-days", "2", "-extfile", root.resolve("chat.ext").toString(),
        "-out", root.resolve("chat-cert.pem").toString());
    final java.security.cert.Certificate certificate;
    try (var input = Files.newInputStream(root.resolve("server-cert.pem"))) {
      certificate = CertificateFactory.getInstance("X.509").generateCertificate(input);
    }
    char[] password = "public-fixture-trust".toCharArray();
    var trust = KeyStore.getInstance("PKCS12");
    trust.load(null, password);
    trust.setCertificateEntry("fixture-ca", certificate);
    try (var output = Files.newOutputStream(root.resolve("trust.p12"))) { trust.store(output, password); }
    Files.writeString(root.resolve("trust-password.txt"), new String(password));
    Files.writeString(root.resolve("jdbc-password.txt"), "test");
    var keys = KeyStore.getInstance("PKCS12");
    keys.load(null, password);
    String pem = Files.readString(root.resolve("server-key.pem"));
    var key = KeyFactory.getInstance("RSA").generatePrivate(new PKCS8EncodedKeySpec(
        Base64.getMimeDecoder().decode(pem.replace("-----BEGIN PRIVATE KEY-----", "")
            .replace("-----END PRIVATE KEY-----", ""))));
    keys.setKeyEntry("https", key, password, new java.security.cert.Certificate[] {certificate});
    var managers = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
    managers.init(keys, password);
    var context = SSLContext.getInstance("TLS");
    context.init(managers.getKeyManagers(), null, null);
    signer = KeyPairGenerator.getInstance("RSA").generateKeyPair();
    foreignSigner = KeyPairGenerator.getInstance("RSA").generateKeyPair();
    var next = KeyPairGenerator.getInstance("RSA").generateKeyPair();
    var jwk = new RSAKey.Builder((java.security.interfaces.RSAPublicKey) signer.getPublic())
        .keyID("chat-current").keyUse(com.nimbusds.jose.jwk.KeyUse.SIGNATURE)
        .algorithm(com.nimbusds.jose.JWSAlgorithm.RS256).build().toJSONObject();
    var nextJwk = new RSAKey.Builder((java.security.interfaces.RSAPublicKey) next.getPublic())
        .keyID("chat-next").keyUse(com.nimbusds.jose.jwk.KeyUse.SIGNATURE)
        .algorithm(com.nimbusds.jose.JWSAlgorithm.RS256).build().toJSONObject();
    byte[] body = new ObjectMapper().writeValueAsBytes(Map.of("keys", java.util.List.of(jwk, nextJwk)));
    var spaceKey = new RSAKey.Builder((java.security.interfaces.RSAPublicKey) foreignSigner.getPublic())
        .keyID("space-current").keyUse(com.nimbusds.jose.jwk.KeyUse.SIGNATURE)
        .algorithm(com.nimbusds.jose.JWSAlgorithm.RS256).build().toJSONObject();
    var spaceNext = new RSAKey.Builder((java.security.interfaces.RSAPublicKey) KeyPairGenerator.getInstance("RSA").generateKeyPair().getPublic())
        .keyID("space-next").keyUse(com.nimbusds.jose.jwk.KeyUse.SIGNATURE)
        .algorithm(com.nimbusds.jose.JWSAlgorithm.RS256).build().toJSONObject();
    byte[] foreignBody = new ObjectMapper().writeValueAsBytes(Map.of("keys", java.util.List.of(spaceKey, spaceNext)));
    https = HttpsServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    https.setHttpsConfigurator(new HttpsConfigurator(context));
    https.setExecutor(executor);
    https.createContext("/chat", exchange -> {
      requests.incrementAndGet();
      exchange.sendResponseHeaders(httpStatus.get(), body.length);
      try (var output = exchange.getResponseBody()) { output.write(body); }
    });
    https.createContext("/space", exchange -> {
      exchange.sendResponseHeaders(200, foreignBody.length);
      try (var output = exchange.getResponseBody()) { output.write(foreignBody); }
    });
    https.start();
  }

  Map<String, String> environment() throws Exception {
    String prefix = AuthChatFloorInteropFixture.PREFIX;
    var values = new HashMap<String, String>();
    values.put(prefix + "RUN_ID", UUID.randomUUID().toString());
    values.put(prefix + "INPUT_ROOT", root.toString());
    values.put(prefix + "AUTH_SOURCE_SHA", System.getenv().getOrDefault("AUTH_CHAT_FIXTURE_TEST_SOURCE_SHA",
        "b8a71768d3a147fabcec01dd6c05b996314f3e3e"));
    values.put(prefix + "LAUNCHER_SOURCE_SHA", values.get(prefix + "AUTH_SOURCE_SHA"));
    values.put(prefix + "HOST", "localhost");
    values.put(prefix + "TLS_SERVER_NAME", "localhost");
    values.put(prefix + "CA_FILE", root.resolve("server-cert.pem").toString());
    values.put(prefix + "CLIENT_CERT_FILE", root.resolve("chat-cert.pem").toString());
    values.put(prefix + "CLIENT_KEY_FILE", root.resolve("chat-key.pem").toString());
    values.put(prefix + "JWKS_TRUSTSTORE_FILE", root.resolve("trust.p12").toString());
    values.put(prefix + "JWKS_TRUSTSTORE_PASSWORD_FILE", root.resolve("trust-password.txt").toString());
    values.put(prefix + "JDBC_PASSWORD_FILE", root.resolve("jdbc-password.txt").toString());
    values.put(prefix + "JDBC_URL", System.getenv().getOrDefault("AUTH_CHAT_FIXTURE_TEST_JDBC_URL",
        "jdbc:postgresql://127.0.0.1:1/unused"));
    values.put(prefix + "JDBC_USERNAME", "test");
    values.put(prefix + "REDIS_HOST", System.getenv().getOrDefault("AUTH_CHAT_FIXTURE_TEST_REDIS_HOST", "127.0.0.1"));
    values.put("AUTH_GRPC_TLS_CERT_FILE", root.resolve("server-cert.pem").toString());
    values.put("AUTH_GRPC_TLS_KEY_FILE", root.resolve("server-key.pem").toString());
    values.put("AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE", root.resolve("server-cert.pem").toString());
    values.put("S2S_JWKS_URLS_JSON", new ObjectMapper().writeValueAsString(Map.of(
        "gateway", "https://localhost:" + https.getAddress().getPort() + "/unused-gateway",
        "space", "https://localhost:" + https.getAddress().getPort() + "/space",
        "chat", "https://localhost:" + https.getAddress().getPort() + "/chat")));
    return values;
  }

  private static void command(String... command) throws Exception {
    var process = new ProcessBuilder(command).redirectOutput(ProcessBuilder.Redirect.DISCARD)
        .redirectError(ProcessBuilder.Redirect.DISCARD).start();
    if (!process.waitFor(15, TimeUnit.SECONDS) || process.exitValue() != 0) {
      process.destroyForcibly();
      throw new IllegalStateException("test certificate creation failed");
    }
  }

  @Override public void close() { https.stop(0); executor.shutdownNow(); }
}
