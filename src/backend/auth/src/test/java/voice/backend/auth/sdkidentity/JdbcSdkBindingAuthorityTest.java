package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyMap;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Base64;
import java.util.HexFormat;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;

class JdbcSdkBindingAuthorityTest {
  private static final UUID APP = UUID.fromString("10000000-0000-4000-8000-000000000001");
  private static final UUID ENV = UUID.fromString("20000000-0000-4000-8000-000000000002");
  private static final UUID ACCOUNT = UUID.fromString("30000000-0000-4000-8000-000000000003");
  private static final UUID DEVICE = UUID.fromString("40000000-0000-4000-8000-000000000004");
  private static final UUID ACTOR = UUID.fromString("50000000-0000-4000-8000-000000000005");
  private static final UUID BINDING = UUID.fromString("60000000-0000-4000-8000-000000000006");
  private static final byte[] KEY = "0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.US_ASCII);
  private HttpServer server;

  @AfterEach
  void stopServer() { if (server != null) server.stop(0); }

  @Test
  void selectsOneActiveAuthGrantAndConfirmsMatchingCurrentGISAuthority() throws Exception {
    AtomicInteger requests = new AtomicInteger();
    start(authorityBody(APP, ENV, BINDING, "active", 3), requests);
    NamedParameterJdbcTemplate jdbc = jdbcWithRows(List.of(grantRow(BINDING, ACTOR, DEVICE)));

    var current = new JdbcSdkBindingAuthority(jdbc, client()).currentBinding(APP, ENV, ACCOUNT, DEVICE);

    assertThat(current).contains(new SdkBindingAuthority.Binding(ACTOR, BINDING));
    assertThat(requests.get()).isEqualTo(1);
    verify(jdbc).queryForList(anyString(), anyMap());
  }

  @Test
  void missingOrAmbiguousCurrentGrantDeniesWithoutCallingGIS() throws Exception {
    AtomicInteger requests = new AtomicInteger();
    start(authorityBody(APP, ENV, BINDING, "active", 3), requests);
    NamedParameterJdbcTemplate missing = jdbcWithRows(List.of());
    NamedParameterJdbcTemplate ambiguous = jdbcWithRows(List.of(
        grantRow(BINDING, ACTOR, DEVICE), grantRow(UUID.randomUUID(), ACTOR, DEVICE)));

    assertThat(new JdbcSdkBindingAuthority(missing, client()).currentBinding(APP, ENV, ACCOUNT, DEVICE)).isEmpty();
    assertThat(new JdbcSdkBindingAuthority(ambiguous, client()).currentBinding(APP, ENV, ACCOUNT, DEVICE)).isEmpty();
    assertThat(requests.get()).isZero();
  }

  @Test
  void localActorOrDeviceMismatchDeniesWithoutCallingGIS() throws Exception {
    AtomicInteger requests = new AtomicInteger();
    start(authorityBody(APP, ENV, BINDING, "active", 3), requests);
    NamedParameterJdbcTemplate wrongDevice = jdbcWithRows(List.of(grantRow(BINDING, ACTOR, UUID.randomUUID())));
    NamedParameterJdbcTemplate missingActor = jdbcWithRows(List.of(grantRow(BINDING, null, DEVICE)));

    assertThat(new JdbcSdkBindingAuthority(wrongDevice, client()).currentBinding(APP, ENV, ACCOUNT, DEVICE)).isEmpty();
    assertThat(new JdbcSdkBindingAuthority(missingActor, client()).currentBinding(APP, ENV, ACCOUNT, DEVICE)).isEmpty();
    assertThat(requests.get()).isZero();
  }

  @Test
  void wrongOrRevokedGISAuthorityDeniesEvenWhenAuthHasOneActiveGrant() throws Exception {
    for (String body : List.of(
        authorityBody(UUID.randomUUID(), ENV, BINDING, "active", 3),
        authorityBody(APP, ENV, UUID.randomUUID(), "active", 3),
        authorityBody(APP, ENV, BINDING, "revoked", 3),
        authorityBody(APP, ENV, BINDING, "active", 0))) {
      AtomicInteger requests = new AtomicInteger();
      start(body, requests);
      NamedParameterJdbcTemplate jdbc = jdbcWithRows(List.of(grantRow(BINDING, ACTOR, DEVICE)));
      assertThat(new JdbcSdkBindingAuthority(jdbc, client()).currentBinding(APP, ENV, ACCOUNT, DEVICE)).isEmpty();
      assertThat(requests.get()).isEqualTo(1);
      stopServer();
      server = null;
    }
  }

  private NamedParameterJdbcTemplate jdbcWithRows(List<Map<String, Object>> rows) {
    NamedParameterJdbcTemplate jdbc = mock(NamedParameterJdbcTemplate.class);
    when(jdbc.queryForList(anyString(), anyMap())).thenReturn(rows);
    return jdbc;
  }

  private Map<String, Object> grantRow(UUID bindingId, UUID actorId, UUID deviceId) {
    Map<String, Object> row = new HashMap<>();
    row.put("binding_id", bindingId);
    row.put("actor_id", actorId);
    row.put("device_id", deviceId);
    return row;
  }

  private SdkGameIntegrationPolicyClient client() {
    return new SdkGameIntegrationPolicyClient("http://127.0.0.1:" + server.getAddress().getPort(),
        Base64.getEncoder().encodeToString(KEY), true, Clock.fixed(Instant.parse("2026-09-26T12:34:56Z"), ZoneOffset.UTC),
        HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build(), Duration.ofSeconds(2));
  }

  private void start(String body, AtomicInteger requests) throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext("/", exchange -> {
      requests.incrementAndGet();
      String path = exchange.getRequestURI().getRawPath();
      String timestamp = exchange.getRequestHeaders().getFirst("X-Voice-Timestamp");
      String nonce = exchange.getRequestHeaders().getFirst("X-Voice-Nonce");
      respond(exchange, body, timestamp, nonce, responseSignature(path, timestamp, nonce, body));
    });
    server.start();
  }

  private static String authorityBody(UUID app, UUID env, UUID binding, String status, long revision) {
    return "{\"application_id\":\"" + app + "\",\"environment_id\":\"" + env + "\",\"binding_id\":\""
        + binding + "\",\"status\":\"" + status + "\",\"binding_revision\":" + revision + ",\"character_context\":[]}\n";
  }

  private static void respond(HttpExchange exchange, String body, String timestamp, String nonce, String signature) throws IOException {
    exchange.getResponseHeaders().add("Cache-Control", "no-store");
    exchange.getResponseHeaders().add("Content-Type", "application/json");
    exchange.getResponseHeaders().add("X-Voice-Response-Timestamp", timestamp);
    exchange.getResponseHeaders().add("X-Voice-Response-Nonce", nonce);
    exchange.getResponseHeaders().add("X-Voice-Response-Signature", signature);
    byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
    exchange.sendResponseHeaders(200, bytes.length);
    exchange.getResponseBody().write(bytes);
    exchange.close();
  }

  private static String responseSignature(String path, String timestamp, String nonce, String body) {
    byte[] digest = sha256(body.getBytes(StandardCharsets.UTF_8));
    return hmac("v1\n200\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + HexFormat.of().formatHex(digest));
  }

  private static String hmac(String message) {
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(KEY, "HmacSHA256"));
      return Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(message.getBytes(StandardCharsets.UTF_8)));
    } catch (Exception impossible) { throw new IllegalStateException(impossible); }
  }

  private static byte[] sha256(byte[] input) {
    try { return java.security.MessageDigest.getInstance("SHA-256").digest(input); }
    catch (Exception impossible) { throw new IllegalStateException(impossible); }
  }
}
