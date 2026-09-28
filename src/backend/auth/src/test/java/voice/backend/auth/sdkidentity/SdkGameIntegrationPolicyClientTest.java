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
    assertThat(policy.providers()).containsExactly("google");
    assertThat(policy.displayName()).isEqualTo("Example Game");
    assertThat(policy.redirectUris()).containsExactly("voicegame://auth/callback");
    assertThat(policy.playerScopes()).containsExactlyInAnyOrder("game.identity.read", "game.chat.send");
  }

  @Test
  void resolvesPersistedBindingChallengeOverAuthenticatedExactPathAndResponse() throws Exception {
    UUID challenge = UUID.fromString("30000000-0000-4000-8000-000000000003");
    UUID device = UUID.fromString("40000000-0000-4000-8000-000000000004");
    UUID sourceAccount = UUID.fromString("50000000-0000-4000-8000-000000000005");
    UUID sourceActor = UUID.fromString("60000000-0000-4000-8000-000000000006");
    UUID targetAccount = UUID.fromString("70000000-0000-4000-8000-000000000007");
    UUID targetProfile = UUID.fromString("80000000-0000-4000-8000-000000000008");
    String path = "/internal/v1/bindings/challenges/" + challenge;
    String body = "{\"challenge_id\":\"" + challenge + "\",\"nonce\":\"" + "n".repeat(43)
        + "\",\"application_id\":\"" + APP + "\",\"environment_id\":\"" + ENV
        + "\",\"provider\":\"google\",\"redirect_uri_sha256\":\"" + "a".repeat(64)
        + "\",\"pkce_challenge\":\"" + "p".repeat(43) + "\",\"device_key_id\":\"" + device
        + "\",\"device_key_thumbprint\":\"" + "t".repeat(43) + "\",\"operation_id\":\""
        + UUID.randomUUID() + "\",\"expires_at\":\"2026-09-26T12:39:56Z\",\"status\":\"pending\",\"source_account_id\":\""
        + sourceAccount + "\",\"source_actor_id\":\"" + sourceActor + "\",\"source_device_id\":\"" + device
        + "\",\"source_generation\":1,\"target_account_id\":\"" + targetAccount
        + "\",\"target_profile_id\":\"" + targetProfile
        + "\",\"profile_revision\":3,\"consent_revision\":4,\"policy_revision\":5,\"scopes\":[\"game.chat.read\"]}";
    AtomicReference<String> authenticatedPath = new AtomicReference<>();
    start(exchange -> {
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      authenticatedPath.set(exchange.getRequestURI().getRawPath());
      assertThat(exchange.getRequestMethod()).isEqualTo("GET");
      assertThat(exchange.getRequestHeaders().getFirst("X-Voice-Workload")).isEqualTo("auth");
      assertThat(exchange.getRequestHeaders().getFirst("X-Voice-Signature"))
          .isEqualTo(requestSignature(path, timestamp, nonce));
      respond(exchange, 200, body, timestamp, nonce, responseSignature(path, timestamp, nonce, body));
    });

    var resolved = client(baseUrl()).resolveBindingChallenge(challenge);

    assertThat(authenticatedPath.get()).isEqualTo(path);
    assertThat(resolved.challengeId()).isEqualTo(challenge);
    assertThat(resolved.applicationId()).isEqualTo(APP);
    assertThat(resolved.provider()).isEqualTo("google");
  }

  @Test
  void createsConsentBoundChallengeWithSignedExactBodyAndVerifiesReturnedGISFacts() throws Exception {
    UUID device = UUID.fromString("40000000-0000-4000-8000-000000000004");
    UUID sourceAccount = UUID.fromString("50000000-0000-4000-8000-000000000005");
    UUID sourceActor = UUID.fromString("60000000-0000-4000-8000-000000000006");
    UUID targetAccount = UUID.fromString("70000000-0000-4000-8000-000000000007");
    UUID targetProfile = UUID.fromString("80000000-0000-4000-8000-000000000008");
    UUID operation = UUID.fromString("90000000-0000-4000-8000-000000000009");
    UUID challenge = UUID.fromString("a0000000-0000-4000-8000-00000000000a");
    String path = "/internal/v1/bindings/challenges";
    String body = "{\"application_id\":\"" + APP + "\",\"environment_id\":\"" + ENV
        + "\",\"provider\":\"google\",\"redirect_uri_sha256\":\"" + "a".repeat(64)
        + "\",\"pkce_challenge\":\"" + "p".repeat(43) + "\",\"device_key_id\":\"" + device
        + "\",\"device_key_thumbprint\":\"" + "t".repeat(43) + "\",\"operation_id\":\"" + operation
        + "\",\"expires_at\":\"2026-09-26T12:38:56Z\",\"source_account_id\":\"" + sourceAccount
        + "\",\"source_actor_id\":\"" + sourceActor + "\",\"source_device_id\":\"" + device
        + "\",\"source_generation\":2,\"target_account_id\":\"" + targetAccount
        + "\",\"target_profile_id\":\"" + targetProfile
        + "\",\"profile_revision\":11,\"consent_revision\":12,\"policy_revision\":7,\"scopes\":[\"game.chat.read\",\"game.chat.send\"]}";
    String responseBody = "{\"challenge_id\":\"" + challenge + "\",\"nonce\":\"" + "n".repeat(43)
        + "\",\"expires_at\":\"2026-09-26T12:38:56Z\"}";
    AtomicReference<String> received = new AtomicReference<>();
    start(exchange -> {
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      String sent = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
      received.set(sent);
      assertThat(exchange.getRequestMethod()).isEqualTo("POST");
      assertThat(exchange.getRequestURI().getRawPath()).isEqualTo(path);
      assertThat(exchange.getRequestHeaders().getFirst("X-Voice-Workload")).isEqualTo("auth");
      assertThat(exchange.getRequestHeaders().getFirst("X-Voice-Signature")).isEqualTo(bodyRequestSignature("POST", path, timestamp, nonce, sent));
      respond(exchange, 200, responseBody, timestamp, nonce, responseSignature(path, timestamp, nonce, responseBody));
    });

    var created = client(baseUrl()).createBindingChallenge(new SdkBindingChallengeAuthority.CreateRequest(APP, ENV,
        "google", "a".repeat(64), "p".repeat(43), device, "t".repeat(43), operation,
        Instant.parse("2026-09-26T12:38:56Z"), sourceAccount, sourceActor, device, 2, targetAccount, targetProfile,
        11, 12, 7, List.of("game.chat.read", "game.chat.send")));

    assertThat(received.get()).isEqualTo(body);
    assertThat(created.challengeId()).isEqualTo(challenge);
    assertThat(created.sourceAccountId()).isEqualTo(sourceAccount);
    assertThat(created.targetProfileId()).isEqualTo(targetProfile);
    assertThat(created.scopes()).containsExactly("game.chat.read", "game.chat.send");
  }

  @Test
  void issuesExecutionPermitWithVersionTwoAssertionBoundSigningVectorAndStrictResponse() throws Exception {
    UUID binding = UUID.fromString("30000000-0000-4000-8000-000000000003");
    UUID operation = UUID.fromString("40000000-0000-4000-8000-000000000004");
    UUID permit = UUID.fromString("50000000-0000-4000-8000-000000000005");
    UUID assertionId = UUID.fromString("60000000-0000-4000-8000-000000000006");
    String assertion = compactAssertion(assertionId);
    String path = "/internal/v1/game-integrations/bindings/" + binding + "/execution-permits";
    String body = "{\"operation_id\":\"" + operation + "\"}";
    String responseBody = "{\"permit_id\":\"" + permit + "\",\"binding_id\":\"" + binding
        + "\",\"application_id\":\"" + APP + "\",\"environment_id\":\"" + ENV
        + "\",\"binding_revision\":7,\"assertion_jti\":\"" + assertionId
        + "\",\"operation_id\":\"" + operation + "\",\"expires_at\":\"2026-09-26T12:34:58Z\"}";
    AtomicReference<String> received = new AtomicReference<>();
    AtomicReference<String> requestNonce = new AtomicReference<>();
    start(exchange -> {
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      requestNonce.set(nonce);
      String sentBody = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
      received.set(exchange.getRequestMethod() + "\n" + exchange.getRequestURI().getRawPath() + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Workload") + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Workload-Version") + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Device-Authority") + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Signature") + "\n" + sentBody);
      respond(exchange, 200, responseBody, timestamp, nonce, responseSignature(path, timestamp, nonce, responseBody));
    });

    var issued = client(baseUrl()).issue(binding, operation, assertion);

    String[] request = received.get().split("\n", -1);
    assertThat(request[0]).isEqualTo("POST");
    assertThat(request[1]).isEqualTo(path);
    assertThat(request[2]).isEqualTo("auth");
    assertThat(request[3]).isEqualTo("2");
    assertThat(request[4]).isEqualTo(assertion);
    assertThat(request[5]).isEqualTo(executionPermitRequestSignature(path,
        Long.toString(CLOCK.instant().getEpochSecond()), requestNonce.get(), body, assertion));
    assertThat(request[6]).isEqualTo(body);
    assertThat(issued.permitId()).isEqualTo(permit);
    assertThat(issued.bindingId()).isEqualTo(binding);
    assertThat(issued.operationId()).isEqualTo(operation);
    assertThat(issued.assertionJti()).isEqualTo(assertionId);
  }

  @Test
  void completionUsesExactV1BodySignatureAndRejectsDivergentOrUnknownReceiptFields() throws Exception {
    UUID permit = UUID.fromString("50000000-0000-4000-8000-000000000005");
    UUID operation = UUID.fromString("40000000-0000-4000-8000-000000000004");
    String path = "/internal/v1/game-integrations/execution-permits/" + permit + "/completion";
    String body = "{\"operation_id\":\"" + operation + "\",\"outcome\":\"committed\"}";
    String responseBody = "{\"permit_id\":\"" + permit + "\",\"operation_id\":\"" + operation
        + "\",\"outcome\":\"committed\",\"status\":\"completed\"}";
    AtomicReference<String> received = new AtomicReference<>();
    AtomicReference<String> requestNonce = new AtomicReference<>();
    start(exchange -> {
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      requestNonce.set(nonce);
      String sentBody = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
      received.set(exchange.getRequestMethod() + "\n" + exchange.getRequestURI().getRawPath() + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Workload") + "\n"
          + exchange.getRequestHeaders().getFirst("X-Voice-Signature") + "\n" + sentBody);
      respond(exchange, 200, responseBody, timestamp, nonce, responseSignature(path, timestamp, nonce, responseBody));
    });

    var completion = client(baseUrl()).complete(permit, operation, "committed");

    String[] request = received.get().split("\n", -1);
    assertThat(request[0]).isEqualTo("POST");
    assertThat(request[1]).isEqualTo(path);
    assertThat(request[2]).isEqualTo("auth");
    assertThat(request[3]).isEqualTo(bodyRequestSignature("POST", path,
        Long.toString(CLOCK.instant().getEpochSecond()), requestNonce.get(), body));
    assertThat(request[4]).isEqualTo(body);
    assertThat(completion.permitId()).isEqualTo(permit);
    assertThat(completion.operationId()).isEqualTo(operation);
    assertThat(completion.outcome()).isEqualTo("committed");

    stopServer();
    server = null;
    String extraFieldReceipt = responseBody.substring(0, responseBody.length() - 1) + ",\"extra\":true}";
    start(exchange -> {
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      respond(exchange, 200, extraFieldReceipt, timestamp, nonce,
          responseSignature(path, timestamp, nonce, extraFieldReceipt));
    });
    assertThatThrownBy(() -> client(baseUrl()).complete(permit, operation, "committed"))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void rejectsSignedExecutionPermitWithUnknownFieldWrongOperationOrExpiryOutsideWindow() throws Exception {
    UUID binding = UUID.fromString("30000000-0000-4000-8000-000000000003");
    UUID operation = UUID.fromString("40000000-0000-4000-8000-000000000004");
    UUID assertionId = UUID.fromString("60000000-0000-4000-8000-000000000006");
    UUID permit = UUID.fromString("50000000-0000-4000-8000-000000000005");
    String assertion = compactAssertion(assertionId);
    String path = "/internal/v1/game-integrations/bindings/" + binding + "/execution-permits";
    List<String> invalidReceipts = List.of(
        executionPermitReceipt(permit, binding, operation, assertionId, "2026-09-26T12:34:58Z")
            .replace("}", ",\"unexpected\":true}"),
        executionPermitReceipt(permit, binding, UUID.fromString("70000000-0000-4000-8000-000000000007"), assertionId,
            "2026-09-26T12:34:58Z"),
        executionPermitReceipt(permit, binding, operation, assertionId, "2026-09-26T12:34:56.500Z"),
        executionPermitReceipt(permit, binding, operation, assertionId, "2026-09-26T12:35:00.751Z"));

    for (String body : invalidReceipts) {
      start(exchange -> {
        String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
        String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
        respond(exchange, 200, body, timestamp, nonce, responseSignature(path, timestamp, nonce, body));
      });
      assertThatThrownBy(() -> client(baseUrl()).issue(binding, operation, assertion))
          .isInstanceOf(SdkIdentityDeniedException.class);
      stopServer();
      server = null;
    }
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
  void rejectsInactiveOrEmptyProviderPolicyEvenWhenResponseIsSigned() throws Exception {
    for (String providers : List.of("[]", "[\"apple\"]", "[\"google\",\"apple\"]")) {
      String body = BODY.replace("[\"google\"]", providers);
      start(exchange -> {
        String path = exchange.getRequestURI().getRawPath();
        String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
        String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
        respond(exchange, 200, body, timestamp, nonce, responseSignature(path, timestamp, nonce, body));
      });
      assertDenied(client(baseUrl())::resolve);
      stopServer();
      server = null;
    }
  }

  @Test
  void rejectsMalformedPolicyJsonWithValidResponseSignature() throws Exception {
    String body = "{not-json}\n";
    start(exchange -> {
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      respond(exchange, 200, body, timestamp, nonce, responseSignature(path, timestamp, nonce, body));
    });

    assertDenied(client(baseUrl())::resolve);
  }

  @Test
  void unavailableRegistryDeniesPolicyResolution() throws Exception {
    start(exchange -> {
      exchange.sendResponseHeaders(503, -1);
      exchange.close();
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

  private static String bodyRequestSignature(String method, String path, String timestamp, String nonce, String body) throws Exception {
    String bodyHash = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(body.getBytes(StandardCharsets.UTF_8)));
    return hmac("v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash);
  }

  private static String executionPermitRequestSignature(String path, String timestamp, String nonce,
      String body, String assertion) throws Exception {
    String bodyHash = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest(body.getBytes(StandardCharsets.UTF_8)));
    String assertionHash = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
        .digest(assertion.getBytes(StandardCharsets.UTF_8)));
    return hmac("v2\nPOST\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash + "\n" + assertionHash);
  }

  private static String compactAssertion(UUID jti) {
    Base64.Encoder encoder = Base64.getUrlEncoder().withoutPadding();
    String header = encoder.encodeToString("{\"alg\":\"RS256\"}".getBytes(StandardCharsets.UTF_8));
    String payload = encoder.encodeToString(("{\"jti\":\"" + jti + "\"}").getBytes(StandardCharsets.UTF_8));
    return header + "." + payload + ".AA";
  }

  private static String executionPermitReceipt(UUID permit, UUID binding, UUID operation, UUID assertionId,
      String expiresAt) {
    return "{\"permit_id\":\"" + permit + "\",\"binding_id\":\"" + binding
        + "\",\"application_id\":\"" + APP + "\",\"environment_id\":\"" + ENV
        + "\",\"binding_revision\":7,\"assertion_jti\":\"" + assertionId
        + "\",\"operation_id\":\"" + operation + "\",\"expires_at\":\"" + expiresAt + "\"}";
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
