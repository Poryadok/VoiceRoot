package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import java.util.regex.Pattern;
import java.util.stream.Stream;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.convention.TestBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.mail.ResendMailSender;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.repository.RefreshTokenRepository;

/** Real mail HTTP adapter + disposable Node provider; capture never proves delivery. */
@SpringBootTest
@AutoConfigureMockMvc(print = org.springframework.boot.test.autoconfigure.web.servlet.MockMvcPrint.NONE)
@ActiveProfiles("test")
@org.junit.jupiter.api.parallel.Execution(org.junit.jupiter.api.parallel.ExecutionMode.SAME_THREAD)
class MailTransportAcceptanceTest {
  private static final ObjectMapper JSON = new ObjectMapper();
  private static final Fixture MAIL = new Fixture();
  private static final MutableClock TIME = new MutableClock();
  private static final String PASSWORD = "Original password 99!";
  private static final String NEW_PASSWORD = "Replacement password 99!";

  @TestBean(name = "capturingMailSender", methodName = "realSender") MailSender sender;
  @TestBean(name = "clock", methodName = "testClock") Clock clock;
  @Autowired MockMvc http;
  @Autowired AccountRepository accounts;
  @Autowired RefreshTokenRepository refreshTokens;
  @Autowired voice.backend.auth.security.RefreshTokenCodec refreshCodec;

  static MailSender realSender() {
    return new ResendMailSender("fixture-only-key", "Voice <noreply@voice.test>", MAIL.base + "/emails");
  }

  static Clock testClock() { return TIME; }

  @BeforeEach void resetClockAndCheckTransport() {
    TIME.now.set(Instant.now().truncatedTo(java.time.temporal.ChronoUnit.SECONDS));
    assertThat(sender).isExactlyInstanceOf(ResendMailSender.class);
    assertThat(clock).isSameAs(TIME);
  }

  @AfterAll static void stopFixture() throws Exception { MAIL.close(); }

  @Test void deliveredRegistrationAndResetKeepIdentityAndRevokeRefresh() throws Exception {
    Identity user = register();
    assertPending(user);
    JsonNode submission = capture(user);
    assertThat(MAIL.get("/test/mail/status?id=" + submission.path("id").asText()).path("status").asText()).isEqualTo("accepted");
    assertThat(MAIL.status(inboxPath(user))).isEqualTo(404);
    String code = deliver(user);
    JsonNode verified = body(submitCode(user, "email_verify", code, 200)).path("session");
    assertThat(verified.path("account_type").asText()).isEqualTo("regular");
    assertThat(verified.path("account_id").asText()).isEqualTo(user.session.path("account_id").asText());
    assertThat(verified.path("profile_id").asText()).isEqualTo(user.session.path("profile_id").asText());
    Identity regular = new Identity(user.email, verified);
    send(regular, "password_reset", 204);
    String resetCode = deliver(regular);
    submitCode(regular, "password_reset", resetCode, 204);
    assertThat(refreshTokens.listActiveByAccount(UUID.fromString(verified.path("account_id").asText()))).isEmpty();
    login(regular, PASSWORD, 401);
    login(regular, NEW_PASSWORD, 200);
    request("/api/v1/auth/refresh", Map.of("refresh_token", verified.path("refresh_token").asText()), null, 401);
    // A used reset code cannot be presented as fresh delivered proof.
    assertError(submitCode(regular, "password_reset", resetCode, 401), "invalid_otp");
  }

  static Stream<Arguments> expiryCases() {
    return Stream.of("email_verify", "password_reset").flatMap(type ->
        Stream.of(599L, 600L, 601L).map(seconds -> Arguments.of(type, seconds)));
  }

  @ParameterizedTest @MethodSource("expiryCases")
  void otpDeadlineUsesFreshDeliveredCodeAndActualAuthClock(String type, long seconds) throws Exception {
    Identity user = register();
    if (type.equals("password_reset")) send(user, type, 204);
    String code = deliver(user);
    Instant issuedAt = TIME.instant();
    TIME.now.set(issuedAt.plusSeconds(seconds));
    if (seconds < 600) {
      submitCode(user, type, code, type.equals("email_verify") ? 200 : 204);
    } else {
      assertError(submitCode(user, type, code, 401), "invalid_otp");
      assertUnchanged(user);
      // The unchanged refresh remains usable; JWT itself is still within 15 minutes.
      request("/api/v1/auth/refresh", Map.of("refresh_token", user.session.path("refresh_token").asText()), null, 200);
    }
  }

  static Stream<Arguments> failureCases() {
    return Stream.of("email_verify", "password_reset").flatMap(type ->
        Stream.of("REJECT_AT_SUBMISSION", "ACCEPT_THEN_BOUNCE").map(mode -> Arguments.of(type, mode)));
  }

  @ParameterizedTest @MethodSource("failureCases")
  void rejectionAndBouncePreserveStateAndRecoverByOrdinaryResend(String type, String mode) throws Exception {
    Identity user = register();
    // Registration has already sent email_verify, so observe its normal cooldown.
    TIME.advance(61);
    MAIL.post("/test/mail/mode", Map.of("to", user.email, "mode", mode), 200);
    int attempts = MAIL.get("/test/mail/attempts?to=" + encode(user.email)).path("count").asInt();
    String previousId = capture(user).path("id").asText();
    MvcResult result = send(user, type, mode.equals("REJECT_AT_SUBMISSION") ? 503 : 204);
    assertThat(MAIL.get("/test/mail/attempts?to=" + encode(user.email)).path("count").asInt()).isEqualTo(attempts + 1);
    if (mode.equals("REJECT_AT_SUBMISSION")) {
      assertError(result, "auth_unavailable");
      assertThat(capture(user).path("id").asText()).as("rejection creates no accepted message").isEqualTo(previousId);
    } else {
      JsonNode accepted = capture(user);
      assertThat(accepted.path("id").asText()).isNotEqualTo(previousId);
      MAIL.post("/test/mail/settle", Map.of("id", accepted.path("id").asText()), 200);
      assertThat(MAIL.get("/test/mail/status?id=" + accepted.path("id").asText()).path("status").asText()).isEqualTo("bounced");
    }
    assertThat(MAIL.status(inboxPath(user))).isEqualTo(404);
    assertUnchanged(user);
    // Failed mail does not promise rollback of the OTP/throttle reservation.
    send(user, type, 429);
    if (mode.equals("ACCEPT_THEN_BOUNCE")) {
      TIME.advance(600);
      // Inspector-only content is used solely to test ordinary OTP expiry,
      // never as received mail or a successful recipient action.
      String intercepted = code(capture(user));
      assertError(submitCode(user, type, intercepted, 401), "invalid_otp");
      assertUnchanged(user);
    } else TIME.advance(61);
    MAIL.post("/test/mail/mode", Map.of("to", user.email, "mode", "ACCEPT_THEN_DELIVER"), 200);
    send(user, type, 204);
    String delivered = deliver(user);
    submitCode(user, type, delivered, type.equals("email_verify") ? 200 : 204);
    if (type.equals("email_verify")) {
      assertThat(login(user, PASSWORD, 200).path("session").path("account_type").asText()).isEqualTo("regular");
    } else {
      login(user, PASSWORD, 401);
      login(user, NEW_PASSWORD, 200);
    }
  }

  @Test void resendCooldownUsesTheInjectedClockWithoutWallClockWaiting() throws Exception {
    Identity user = register();
    TIME.advance(59);
    send(user, "email_verify", 429);
    TIME.advance(1);
    send(user, "email_verify", 204);
  }

  private Identity register() throws Exception {
    String email = "mail-" + UUID.randomUUID() + "@voice.test";
    JsonNode session = body(request("/api/v1/auth/register", Map.of("email", email, "password", PASSWORD, "device_info_json", "{}"), null, 200)).path("session");
    assertThat(session.path("access_token").asText()).isNotBlank();
    Identity user = new Identity(email, session);
    assertThat(MAIL.get("/test/mail/attempts?to=" + encode(email)).path("count").asInt()).isEqualTo(1);
    return user;
  }

  private void assertPending(Identity user) throws Exception {
    MvcResult result = http.perform(get("/api/v1/auth/verification-status").header("Authorization", "Bearer " + user.token())).andReturn();
    assertThat(result.getResponse().getStatus()).isEqualTo(200);
    assertThat(body(result).path("state").asText()).isEqualTo("EMAIL_PENDING");
  }

  private void assertUnchanged(Identity user) throws Exception {
    assertPending(user);
    assertThat(login(user, PASSWORD, 200).path("session").path("account_type").asText()).isEqualTo("guest");
    login(user, NEW_PASSWORD, 401);
    assertThat(accounts.findByEmail(user.email).orElseThrow().type()).isEqualTo("guest");
    assertThat(refreshTokens.findByHash(refreshCodec.hash(user.session.path("refresh_token").asText()))
        .orElseThrow().revoked()).as("original refresh session remains active").isFalse();
  }

  private MvcResult send(Identity user, String type, int status) throws Exception {
    return request("/api/v1/auth/otp/send", type.equals("email_verify") ? Map.of("otp_type", type) : Map.of("otp_type", type, "email", user.email), type.equals("email_verify") ? user.token() : null, status);
  }

  private MvcResult submitCode(Identity user, String type, String code, int status) throws Exception {
    return type.equals("email_verify")
        ? request("/api/v1/auth/otp/verify", Map.of("otp_type", type, "code", code), user.token(), status)
        : request("/api/v1/auth/password/reset", Map.of("email", user.email, "code", code, "new_password", NEW_PASSWORD), null, status);
  }

  private JsonNode login(Identity user, String password, int status) throws Exception {
    return body(request("/api/v1/auth/login", Map.of("email", user.email, "password", password, "device_info_json", "{}"), null, status));
  }

  private MvcResult request(String path, Object body, String token, int expected) throws Exception {
    var builder = post(path).contentType(MediaType.APPLICATION_JSON).content(JSON.writeValueAsBytes(body));
    if (token != null) builder.header("Authorization", "Bearer " + token);
    MvcResult result = http.perform(builder).andReturn();
    // Avoid dumping HTTP responses containing tokens/OTP into test logs.
    assertThat(result.getResponse().getStatus()).as(path).isEqualTo(expected);
    return result;
  }

  private static JsonNode body(MvcResult result) throws Exception { return JSON.readTree(result.getResponse().getContentAsString()); }
  private static void assertError(MvcResult result, String error) throws Exception { assertThat(body(result).path("error").asText()).isEqualTo(error); }
  private static String encode(String value) { return URLEncoder.encode(value, StandardCharsets.UTF_8); }
  private static String inboxPath(Identity user) { return "/test/mail/inbox?to=" + encode(user.email); }
  private static JsonNode capture(Identity user) throws Exception { return MAIL.get("/emails/latest?to=" + encode(user.email)); }
  private static String code(JsonNode message) {
    var match = Pattern.compile("\\b([0-9]{6})\\b").matcher(message.path("text").asText());
    assertThat(match.find()).isTrue();
    return match.group(1);
  }
  private static String deliver(Identity user) throws Exception {
    String id = capture(user).path("id").asText();
    MAIL.post("/test/mail/settle", Map.of("id", id), 200);
    JsonNode delivered = MAIL.get(inboxPath(user));
    assertThat(delivered.path("id").asText()).isEqualTo(id);
    assertThat(delivered.path("status").asText()).isEqualTo("delivered");
    return code(delivered);
  }

  record Identity(String email, JsonNode session) { String token() { return session.path("access_token").asText(); } }

  static final class MutableClock extends Clock {
    final AtomicReference<Instant> now = new AtomicReference<>(Instant.now());
    void advance(long seconds) { now.updateAndGet(time -> time.plusSeconds(seconds)); }
    @Override public Instant instant() { return now.get(); }
    @Override public ZoneId getZone() { return ZoneOffset.UTC; }
    @Override public Clock withZone(ZoneId zone) { if (!zone.equals(ZoneOffset.UTC)) throw new IllegalArgumentException("UTC only"); return this; }
  }

  static final class Fixture implements AutoCloseable {
    final Process process;
    final String base;
    final HttpClient client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();
    Fixture() {
      Process started = null;
      try {
        Path script = Path.of("../../../docker/verification-stub/server.mjs").toAbsolutePath().normalize();
        var builder = new ProcessBuilder("node", script.toString()).redirectError(ProcessBuilder.Redirect.INHERIT);
        builder.environment().put("PORT", "0");
        started = builder.start();
        Process child = started;
        String line = CompletableFuture.supplyAsync(() -> {
          try { return child.inputReader().readLine(); }
          catch (Exception ex) { throw new IllegalStateException(ex); }
        }).get(10, TimeUnit.SECONDS);
        if (line == null || !line.matches("verification-stub listening [0-9]+")) throw new IllegalStateException("mail fixture failed to start");
        process = started;
        base = "http://127.0.0.1:" + line.substring(line.lastIndexOf(' ') + 1);
      } catch (Exception ex) {
        if (started != null) started.destroyForcibly();
        throw new IllegalStateException("mail fixture unavailable", ex);
      }
    }
    HttpResponse<String> exchange(String path, Object body) throws Exception {
      var request = HttpRequest.newBuilder(URI.create(base + path)).timeout(Duration.ofSeconds(5));
      if (body == null) request.GET();
      else request.header("Content-Type", "application/json").POST(HttpRequest.BodyPublishers.ofByteArray(JSON.writeValueAsBytes(body)));
      return client.send(request.build(), HttpResponse.BodyHandlers.ofString());
    }
    int status(String path) throws Exception { return exchange(path, null).statusCode(); }
    JsonNode get(String path) throws Exception { var response = exchange(path, null); assertThat(response.statusCode()).as(path).isEqualTo(200); return JSON.readTree(response.body()); }
    void post(String path, Object body, int expected) throws Exception { assertThat(exchange(path, body).statusCode()).as(path).isEqualTo(expected); }
    @Override public void close() throws Exception { process.destroy(); if (!process.waitFor(5, TimeUnit.SECONDS)) { process.destroyForcibly(); process.waitFor(5, TimeUnit.SECONDS); } }
  }
}
