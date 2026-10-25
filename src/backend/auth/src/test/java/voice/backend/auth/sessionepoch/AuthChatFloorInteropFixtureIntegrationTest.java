package voice.backend.auth.sessionepoch;

import static org.junit.jupiter.api.Assertions.*;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.GetChatSessionEpochFloorRequest;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.stub.MetadataUtils;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.Signature;
import java.time.Instant;
import java.util.Base64;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.junit.jupiter.api.io.TempDir;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;

/** Opt-in only: runner supplies fresh, owned PostgreSQL/Redis in a private Linux network. */
@EnabledIfEnvironmentVariable(named = "AUTH_CHAT_FIXTURE_TEST_JDBC_URL", matches = ".+")
class AuthChatFloorInteropFixtureIntegrationTest {
  @TempDir Path root;
  private static final ObjectMapper JSON = new ObjectMapper();

  @Test void standaloneProductionReceiverUsesRealHttpsRedisAndMutualTlsThenExpires() throws Exception {
    try (var material = new ChatFloorFixtureTestMaterial(root)) {
      Path output = Files.createDirectory(root.resolve("run"));
      var environment = material.environment();
      environment.put("S2S_UNKNOWN_KID_COOLDOWN", "1ms");
      var child = new ProcessBuilder(Path.of(System.getProperty("java.home"), "bin", "java").toString(),
          "-cp", System.getProperty("java.class.path"), AuthChatFloorInteropFixture.class.getName(),
          output.toString(), "15");
      child.environment().putAll(environment);
      child.redirectErrorStream(true).redirectOutput(root.resolve("java.log").toFile());
      Process process = child.start();
      ManagedChannel channel = null;
      Path proof = Files.createDirectories(Path.of("target", "chat-floor-interop-proof"));
      try {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(30);
        while (!Files.exists(output.resolve("ready.json")) && process.isAlive() && System.nanoTime() < deadline)
          Thread.sleep(50);
        assertTrue(Files.exists(output.resolve("ready.json")),
            () -> "fixture did not publish readiness: " + readLog(root.resolve("java.log")));
        var ready = JSON.readTree(Files.readAllBytes(output.resolve("ready.json")));
        Files.copy(output.resolve("ready.json"), proof.resolve("ready.json"), java.nio.file.StandardCopyOption.REPLACE_EXISTING);
        assertEquals(1, ready.path("schema_version").asInt());
        assertEquals(environment.get(AuthChatFloorInteropFixture.PREFIX + "RUN_ID"), ready.path("run_id").asText());
        assertEquals(environment.get(AuthChatFloorInteropFixture.PREFIX + "AUTH_SOURCE_SHA"), ready.path("auth_source_sha").asText());
        assertEquals(environment.get(AuthChatFloorInteropFixture.PREFIX + "LAUNCHER_SOURCE_SHA"), ready.path("launcher_source_sha").asText());
        assertEquals(9, ready.path("expected_floor").asLong());
        assertEquals(7, ready.path("durable_epoch").asLong());
        assertEquals(9, ready.path("retained_epoch").asLong());
        assertEquals(15, java.time.Duration.between(Instant.parse(ready.path("started_at").asText()),
            Instant.parse(ready.path("expires_at").asText())).toSeconds());
        assertEquals(AuthPrincipalServerInterceptor.CHAT_SESSION_FLOOR_RPC, ready.path("rpc").asText());
        assertFalse(Files.exists(output.resolve("ready.json.pending")));
        assertEquals(0, material.requests.get(), "ready is not a JWKS success claim");
        channel = NettyChannelBuilder.forAddress("localhost", ready.path("port").asInt())
            .sslContext(GrpcSslContexts.forClient().trustManager(root.resolve("server-cert.pem").toFile())
                .keyManager(root.resolve("chat-cert.pem").toFile(), root.resolve("chat-key.pem").toFile()).build()).build();
        var request = GetChatSessionEpochFloorRequest.newBuilder().setAccountId(ready.path("account_id").asText()).build();
        var signed = signed(material, request, "chat", "chat-current");
        assertEquals(9, call(channel, request, signed).getSessionEpochFloor());
        assertEquals(1, material.requests.get(), "actual production HttpClient fetched actual HTTPS Chat JWKS");
        ManagedChannel actual = channel;
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> call(actual, request, signed)).getStatus().getCode(), "actual Redis JTI replay rejected");
        var unknown = request.toBuilder().setAccountId(ready.path("unknown_account_id").asText()).build();
        var deleted = request.toBuilder().setAccountId(ready.path("deleted_account_id").asText()).build();
        for (var absent : new GetChatSessionEpochFloorRequest[] {unknown, deleted})
          assertEquals(Status.Code.NOT_FOUND, assertThrows(StatusRuntimeException.class,
              () -> call(actual, absent, signed(material, absent, "chat", "chat-current"))).getStatus().getCode());
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> call(actual, unknown, signed(material, request, "chat", "chat-current"))).getStatus().getCode());
        assertEquals(Status.Code.PERMISSION_DENIED, assertThrows(StatusRuntimeException.class,
            () -> call(actual, request, signed(material, request, "space", "space-current"))).getStatus().getCode(),
            "valid foreign issuer with its own distinct HTTPS keys is not permitted on Chat RPC");
        var noClientCertificate = NettyChannelBuilder.forAddress("localhost", ready.path("port").asInt())
            .sslContext(GrpcSslContexts.forClient().trustManager(root.resolve("server-cert.pem").toFile()).build()).build();
        try {
          assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
              () -> call(noClientCertificate, request, signed(material, request, "chat", "chat-current"))).getStatus().getCode());
        } finally { noClientCertificate.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
        material.httpStatus.set(503);
        Thread.sleep(10); // Pass the configured production unknown-kid cooldown.
        int requestsBeforeOutage = material.requests.get();
        assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
            () -> call(actual, request, signed(material, request, "chat", "outage-kid"))).getStatus().getCode());
        assertTrue(material.requests.get() > requestsBeforeOutage, "503 reached actual HTTPS JWKS endpoint");
        var redis = new org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory(
            System.getenv("AUTH_CHAT_FIXTURE_TEST_REDIS_HOST"), 6379);
        try {
          redis.afterPropertiesSet();
          var template = new org.springframework.data.redis.core.StringRedisTemplate(redis);
          template.afterPropertiesSet();
          assertFalse(template.keys("auth:principal:*").isEmpty(), "real replay storage written");
        } finally { redis.destroy(); }
        assertTrue(process.waitFor(25, TimeUnit.SECONDS), "bounded main exits itself");
        assertEquals(0, process.exitValue(), () -> readLog(root.resolve("java.log")));
        assertFalse(Files.exists(output.resolve("ready.json")));
        var closed = JSON.readTree(Files.readAllBytes(output.resolve("closed.json")));
        assertTrue(closed.path("server_terminated").asBoolean());
        assertTrue(closed.path("clients_closed").asBoolean());
        assertTrue(closed.path("ready_removed").asBoolean());
        assertEquals(ready.path("run_id").asText(), closed.path("run_id").asText());
        Files.copy(output.resolve("closed.json"), proof.resolve("closed.json"), java.nio.file.StandardCopyOption.REPLACE_EXISTING);
      } finally {
        if (channel != null) channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
        if (process.isAlive()) {
          process.destroy();
          if (!process.waitFor(10, TimeUnit.SECONDS)) process.destroyForcibly().waitFor(5, TimeUnit.SECONDS);
        }
        Files.copy(root.resolve("java.log"), proof.resolve("java.log"), java.nio.file.StandardCopyOption.REPLACE_EXISTING);
      }
    }
  }

  private record Signed(String requestId, String token) {}
  private static Signed signed(ChatFloorFixtureTestMaterial material, GetChatSessionEpochFloorRequest request,
      String issuer, String kid) throws Exception {
    String requestId = UUID.randomUUID().toString();
    long now = Instant.now().getEpochSecond();
    var claims = new HashMap<String, Object>(Map.of("iss", issuer, "sub", "service:" + issuer,
        "principal_type", "service", "aud", "auth", "rpc", AuthPrincipalServerInterceptor.CHAT_SESSION_FLOOR_RPC,
        "request_id", requestId, "request_hash", AuthPrincipalServerInterceptor.requestHash(request),
        "jti", UUID.randomUUID().toString(), "iat", now, "nbf", now));
    claims.put("exp", now + 20);
    String unsigned = encode(JSON.writeValueAsBytes(Map.of("alg", "RS256", "kid", kid))) + "." + encode(JSON.writeValueAsBytes(claims));
    var signature = Signature.getInstance("SHA256withRSA");
    signature.initSign(("chat".equals(issuer) ? material.signer : material.foreignSigner).getPrivate());
    signature.update(unsigned.getBytes(StandardCharsets.US_ASCII));
    return new Signed(requestId, unsigned + "." + encode(signature.sign()));
  }
  private static app.voice.auth.v1.GetChatSessionEpochFloorResponse call(ManagedChannel channel,
      GetChatSessionEpochFloorRequest request, Signed signed) {
    var headers = new Metadata();
    headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER), "Bearer " + signed.token());
    headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), signed.requestId());
    return AuthServiceGrpc.newBlockingStub(io.grpc.ClientInterceptors.intercept(channel,
        MetadataUtils.newAttachHeadersInterceptor(headers))).withDeadlineAfter(5, TimeUnit.SECONDS).getChatSessionEpochFloor(request);
  }
  private static String encode(byte[] bytes) { return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes); }
  private static String readLog(Path path) {
    try { return Files.readString(path); } catch (Exception failure) { return "log unavailable"; }
  }
}
