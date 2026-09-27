package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

class SdkGameIntegrationPolicyClientTest {
  private static final UUID APP = UUID.fromString("10000000-0000-4000-8000-000000000001");
  private static final UUID ENV = UUID.fromString("20000000-0000-4000-8000-000000000002");
  private static final Clock CLOCK = Clock.fixed(Instant.parse("2026-09-26T12:34:56Z"), ZoneOffset.UTC);
  private static final byte[] KEY = "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII);
  private static final String KEY_B64 = Base64.getEncoder().encodeToString(KEY);
  private static final String BODY = "{\"application_id\":\"10000000-0000-4000-8000-000000000001\","
      + "\"environment_id\":\"20000000-0000-4000-8000-000000000002\",\"revision\":7,"
      + "\"display_name\":\"Example Game\",\"redirect_uris\":[\"voicegame://auth/callback\"],"
      + "\"allowed_origins\":[],\"providers\":[\"google\"],"
      + "\"player_scopes\":[\"game.identity.read\",\"game.chat.send\"]}\n";

  private HttpServer server;

  @AfterEach
  void stopServer() {
    if (server != null) server.stop(0);
  }

  @Test
  void signsFixedGetAndVerifiesEchoedResponseBeforeReturningPolicy() throws Exception {
    AtomicReference<String> received = new AtomicReference<>();
    start(exchange -> {
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      received.set(exchange.getRequestMethod() + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Workload") + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Signature") + "\n"
          + new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
      respond(exchange, 200, BODY, timestamp, nonce, responseSignature(path, timestamp, nonce, BODY));
    });

    SdkAuthorizationPolicy.Policy policy = client(baseUrl()).resolve(APP, ENV);

    assertThat(received.get()).startsWith("GET\n/internal/v1/authorizations/environments/" + ENV
        + "\n" + CLOCK.instant().getEpochSecond() + "\n");
    String[] request = received.get().split("\n", -1);
    assertThat(request[4]).isEqualTo("auth");
    assertThat(request[5]).isEqualTo(requestSignature(
        "/internal/v1/authorizations/environments/" + ENV, request[2], request[3]));
    assertThat(request[6]).isEmpty();
    assertThat(policy.applicationId()).isEqualTo(APP);
    assertThat(policy.environmentId()).isEqualTo(ENV);
    assertThat(policy.revision()).isEqualTo(7);
    assertThat(policy.displayName()).isEqualTo("Example Game");
    assertThat(policy.redirectUris()).containsExactly("voicegame://auth/callback");
    assertThat(policy.playerScopes()).containsExactlyInAnyOrder("game.identity.read", "game.chat.send");
  }

  @Test
  void rejectsResponseSignatureThatDoesNotBindExactBodyBytes() throws Exception {
    start(exchange -> {
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      String signed = BODY;
      String sent = BODY.replace("Example Game", "Changed Game");
      respond(exchange, 200, sent, timestamp, nonce, responseSignature(path, timestamp, nonce, signed));
    });

    assertDenied(client(baseUrl())::resolve);
  }

  @Test
  void requestTimeoutCoversBodyThatStallsAfterHeaders() throws Exception {
    start(exchange -> {
      byte[] bytes = BODY.getBytes(StandardCharsets.UTF_8);
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      exchange.getResponseHeaders().add("Cache-Control", "no-store");
      exchange.getResponseHeaders().add("Content-Type", "application/json");
      exchange.getResponseHeaders().add("X-Voice-Response-Timestamp", timestamp);
      exchange.getResponseHeaders().add("X-Voice-Response-Nonce", nonce);
      exchange.getResponseHeaders().add("X-Voice-Response-Signature", responseSignature(path, timestamp, nonce, BODY));
      exchange.sendResponseHeaders(200, bytes.length);
      try {
        Thread.sleep(1_500);
        exchange.getResponseBody().write(bytes);
      } catch (InterruptedException interrupted) {
        Thread.currentThread().interrupt();
      } catch (IOException disconnected) {
        // Expected when the client enforces its response-body deadline.
      } finally {
        exchange.close();
      }
    });

    SdkGameIntegrationPolicyClient timedClient = new SdkGameIntegrationPolicyClient(baseUrl(), KEY_B64,
        true, CLOCK, HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NEVER).build(),
        Duration.ofMillis(250));
    long startedAt = System.nanoTime();
    assertDenied(timedClient::resolve);
    long elapsedMillis = Duration.ofNanos(System.nanoTime() - startedAt).toMillis();
    assertThat(elapsedMillis).isLessThan(1_000);
  }

  @Test
  void rejectsDuplicateResponseProofHeaders() throws Exception {
    start(exchange -> {
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      exchange.getResponseHeaders().add("Cache-Control", "no-store");
      exchange.getResponseHeaders().add("Content-Type", "application/json");
      exchange.getResponseHeaders().add("X-Voice-Response-Timestamp", timestamp);
      exchange.getResponseHeaders().add("X-Voice-Response-Nonce", nonce);
      exchange.getResponseHeaders().add("X-Voice-Response-Signature", responseSignature(path, timestamp, nonce, BODY));
      exchange.getResponseHeaders().add("X-Voice-Response-Signature", responseSignature(path, timestamp, nonce, BODY));
      byte[] bytes = BODY.getBytes(StandardCharsets.UTF_8);
      exchange.sendResponseHeaders(200, bytes.length);
      exchange.getResponseBody().write(bytes);
      exchange.close();
    });

    assertDenied(client(baseUrl())::resolve);
  }

  @Test
  void rejectsUnknownFieldsEvenWhenBodySignatureIsValid() throws Exception {
    String body = BODY.replace("\"revision\":7,", "\"revision\":7,\"unexpected\":true,");
    start(exchange -> {
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      respond(exchange, 200, body, timestamp, nonce, responseSignature(path, timestamp, nonce, body));
    });

    assertDenied(client(baseUrl())::resolve);
  }

  @Test
  void rejectsRedirectAndInvalidTrustedBaseWithoutFollowingOrSending() throws Exception {
    AtomicReference<Integer> hits = new AtomicReference<>(0);
    start(exchange -> {
      hits.set(hits.get() + 1);
      exchange.getResponseHeaders().set("Location", "http://127.0.0.1:" + server.getAddress().getPort() + "/attacker");
      exchange.sendResponseHeaders(302, -1);
      exchange.close();
    });
    SdkGameIntegrationPolicyClient redirecting = client(baseUrl());
    assertDenied(redirecting::resolve);
    assertThat(hits.get()).isEqualTo(1);

    SdkGameIntegrationPolicyClient queryBase = client(baseUrl() + "?host=attacker");
    assertDenied(queryBase::resolve);
    SdkGameIntegrationPolicyClient remoteHttp = client("http://attacker.example");
    assertDenied(remoteHttp::resolve);
    assertThat(hits.get()).isEqualTo(1);
  }

  private SdkGameIntegrationPolicyClient client(String baseUrl) {
    return new SdkGameIntegrationPolicyClient(baseUrl, KEY_B64, true, CLOCK,
        HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2))
            .followRedirects(HttpClient.Redirect.NEVER).build(), Duration.ofSeconds(2));
  }

  private String baseUrl() {
    return "http://127.0.0.1:" + server.getAddress().getPort();
  }

  private void start(Handler handler) throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext("/", exchange -> {
      try { handler.handle(exchange); }
      catch (IOException failure) { throw failure; }
      catch (Exception failure) { throw new IOException(failure); }
    });
    server.start();
  }

  private static void respond(HttpExchange exchange, int status, String body, String timestamp,
      String nonce, String signature) throws IOException {
    exchange.getResponseHeaders().add("Cache-Control", "no-store");
    exchange.getResponseHeaders().add("Content-Type", "application/json");
    exchange.getResponseHeaders().add("X-Voice-Response-Timestamp", timestamp);
    exchange.getResponseHeaders().add("X-Voice-Response-Nonce", nonce);
    exchange.getResponseHeaders().add("X-Voice-Response-Signature", signature);
    byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
    exchange.sendResponseHeaders(status, bytes.length);
    exchange.getResponseBody().write(bytes);
    exchange.close();
  }

  private static String requestSignature(String path, String timestamp, String nonce) throws Exception {
    String emptyHash = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest());
    String message = "v1\nGET\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + emptyHash;
    return hmac(message);
  }

  private static String responseSignature(String path, String timestamp, String nonce, String body) throws Exception {
    String bodyHash = HexFormat.of().formatHex(
        MessageDigest.getInstance("SHA-256").digest(body.getBytes(StandardCharsets.UTF_8)));
    return hmac("v1\n200\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash);
  }

  private static String hmac(String message) throws Exception {
    Mac mac = Mac.getInstance("HmacSHA256");
    mac.init(new SecretKeySpec(KEY, "HmacSHA256"));
    return Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(message.getBytes(StandardCharsets.UTF_8)));
  }

  private static void assertDenied(Resolver resolver) {
    assertThatThrownBy(() -> resolver.resolve(APP, ENV)).isInstanceOf(SdkIdentityDeniedException.class)
        .hasMessage("invalid_sdk_identity");
  }

  @FunctionalInterface private interface Handler { void handle(HttpExchange exchange) throws Exception; }
  @FunctionalInterface private interface Resolver { SdkAuthorizationPolicy.Policy resolve(UUID app, UUID env); }
}
