package voice.backend.auth.sessionepoch;

import voice.backend.auth.grpc.AuthGrpcService;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.Mockito.doReturn;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import app.voice.auth.v1.AuthServiceGrpc;
import app.voice.auth.v1.GetSpaceSessionEpochFloorRequest;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.MetadataUtils;
import java.nio.charset.StandardCharsets;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.Signature;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Base64;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalServices;
import voice.backend.auth.principal.AuthPrincipalVerifier;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class AuthSpaceSessionFloorGrpcTest {
  private static final String RPC = AuthPrincipalServerInterceptor.SPACE_SESSION_FLOOR_RPC;
  private static final ObjectMapper JSON = new ObjectMapper();

  @Test void returnsMaximumOfDurableAccountEpochAndPersistedFloor() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.of(account(accountId, 4)));
    when(floors.requireFloor(accountId)).thenReturn(6L);

    var response = invoke(accountId, accounts, floors);

    assertEquals(6L, response.getSessionEpochFloor());
  }

  @Test void unavailableAndNonpositiveAuthorityFailClosed() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.of(account(accountId, 4)));
    when(floors.requireFloor(accountId)).thenThrow(new IllegalStateException("storage unavailable"));
    var unavailable = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
    assertEquals(Status.Code.UNAVAILABLE, unavailable.getStatus().getCode());

    doReturn(0L).when(floors).requireFloor(accountId);
    unavailable = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
    assertEquals(Status.Code.UNAVAILABLE, unavailable.getStatus().getCode());
  }

  @Test void higherDurableEpochCannotBeLoweredByRedisFloor() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.of(account(accountId, 9)));
    when(floors.requireFloor(accountId)).thenReturn(4L);

    assertEquals(9L, invoke(accountId, accounts, floors).getSessionEpochFloor());
  }

  @Test void durableReadFailureCannotAuthorizeFromHealthyRedisAlone() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenThrow(new IllegalStateException("database unavailable"));
    when(floors.requireFloor(accountId)).thenReturn(9L);

    var unavailable = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));

    assertEquals(Status.Code.UNAVAILABLE, unavailable.getStatus().getCode());
    verifyNoInteractions(floors);
  }

  @Test void nonpositiveDurableEpochCannotAuthorizeFromHealthyRedisAlone() throws Exception {
    UUID accountId = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(floors.requireFloor(accountId)).thenReturn(9L);

    for (long durableEpoch : new long[] {0L, -1L}) {
      when(accounts.findById(accountId.toString()))
          .thenReturn(java.util.Optional.of(account(accountId, durableEpoch)));
      var unavailable = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
      assertEquals(Status.Code.UNAVAILABLE, unavailable.getStatus().getCode());
    }
  }

  @Test void malformedAccountIdIsRejectedBeforeAuthorityAccess() throws Exception {
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);

    for (String accountId : new String[] {"", "not-an-account", "8A78BD68-75E9-4F21-9387-3BDC2F6116AB", "00000000-0000-0000-0000-000000000000"}) {
      var malformed = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
      assertEquals(Status.Code.INVALID_ARGUMENT, malformed.getStatus().getCode());
    }
    verifyNoInteractions(accounts, floors);
  }

  @Test void absentSourcesAndUnknownAccountFailClosed() throws Exception {
    UUID accountId = UUID.randomUUID();
    var absent = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, null, null));
    assertEquals(Status.Code.UNAVAILABLE, absent.getStatus().getCode());

    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(accountId.toString())).thenReturn(java.util.Optional.empty());
    var missing = assertThrows(StatusRuntimeException.class, () -> invoke(accountId, accounts, floors));
    assertEquals(Status.Code.NOT_FOUND, missing.getStatus().getCode());
  }

  @Test void unknownFieldsFailBeforeAccountRead() throws Exception {
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    var request = GetSpaceSessionEpochFloorRequest.newBuilder().setAccountId(UUID.randomUUID().toString())
        .setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder()
            .addField(99, com.google.protobuf.UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build()).build();
    var failure = assertThrows(StatusRuntimeException.class, () -> invoke(request, accounts, floors));
    assertEquals(Status.Code.INVALID_ARGUMENT, failure.getStatus().getCode());
    verifyNoInteractions(accounts, floors);
  }

  @Test void deletedAccountsAreNotReturnedAsLiveAuthority() throws Exception {
    UUID id = UUID.randomUUID();
    AccountRepository accounts = mock(AccountRepository.class);
    SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    when(accounts.findById(id.toString())).thenReturn(java.util.Optional.of(
        new Account(id, null, null, null, "regular", "deleted", null, false, 7,
            Instant.parse("2026-01-01T00:00:00Z"), Instant.now())));
    var failure = assertThrows(StatusRuntimeException.class, () -> invoke(id, accounts, floors));
    assertEquals(Status.Code.NOT_FOUND, failure.getStatus().getCode());
    verifyNoInteractions(floors);
  }

  private static java.io.File resource(String name) throws Exception {
    return new java.io.File(AuthSpaceSessionFloorGrpcTest.class.getResource("/principal-tls/" + name).toURI());
  }

  @Test void realPostgresAndRedisAuthorityIsReadFreshOverSignedMutualTls() throws Exception {
    try (var postgres = new org.testcontainers.containers.PostgreSQLContainer<>("postgres:16-alpine");
        var redis = new org.testcontainers.containers.GenericContainer<>(
            org.testcontainers.utility.DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379)) {
      String externalJdbc = System.getenv("AUTH_SPACE_FLOOR_TEST_JDBC_URL");
      boolean external = externalJdbc != null;
      if (!external) { postgres.start(); redis.start(); }
      var source = new org.springframework.jdbc.datasource.DriverManagerDataSource(
          external ? externalJdbc : postgres.getJdbcUrl(),
          external ? "test" : postgres.getUsername(), external ? "test" : postgres.getPassword());
      org.flywaydb.core.Flyway.configure().dataSource(source).load().migrate();
      var jdbc = new org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate(source);
      var accounts = new voice.backend.auth.repository.JdbcAccountRepository(jdbc);
      var account = accounts.create("space-floor-" + UUID.randomUUID() + "@example.test", null, "hash", "regular");
      var connection = new org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory(
          new org.springframework.data.redis.connection.RedisStandaloneConfiguration(
              external ? System.getenv("AUTH_SPACE_FLOOR_TEST_REDIS_HOST") : redis.getHost(),
              external ? 6379 : redis.getMappedPort(6379)));
      connection.afterPropertiesSet();
      try {
        var template = new org.springframework.data.redis.core.StringRedisTemplate(connection);
        template.afterPropertiesSet();
        var floors = new voice.backend.auth.sessionepoch.RedisSessionEpochFloorStore(
            new voice.backend.auth.sessionepoch.StringRedisSessionEpochCommands(template), java.time.Duration.ofSeconds(2));
        var missing = assertThrows(StatusRuntimeException.class, () -> invoke(account.id(), accounts, floors));
        assertEquals(Status.Code.UNAVAILABLE, missing.getStatus().getCode());
        floors.recordAtLeast(account.id(), 6);
        assertEquals(6L, invoke(account.id(), accounts, floors).getSessionEpochFloor());
        accounts.advanceSessionEpochAtLeast(account.id(), 9);
        assertEquals(9L, invoke(account.id(), accounts, floors).getSessionEpochFloor());
        floors.recordAtLeast(account.id(), 13);
        assertEquals(13L, invoke(account.id(), accounts, floors).getSessionEpochFloor());
        assertEquals(-1L, template.getExpire(floors.keyFor(account.id())));
        template.opsForValue().set(floors.keyFor(account.id()), "invalid");
        var corrupt = assertThrows(StatusRuntimeException.class, () -> invoke(account.id(), accounts, floors));
        assertEquals(Status.Code.UNAVAILABLE, corrupt.getStatus().getCode());
        var absent = assertThrows(StatusRuntimeException.class, () -> invoke(UUID.randomUUID(), accounts, floors));
        assertEquals(Status.Code.NOT_FOUND, absent.getStatus().getCode());
        // The external fixture remains alive; close only this test's client.
        if (external) connection.destroy(); else redis.stop();
        var offline = assertThrows(StatusRuntimeException.class, () -> invoke(account.id(), accounts, floors));
        assertEquals(Status.Code.UNAVAILABLE, offline.getStatus().getCode());
      } finally { connection.destroy(); }
    }
  }

  private static Account account(UUID id, long epoch) {
    return new Account(id, null, null, null, "regular", "active", null, false,
        epoch, Instant.parse("2026-01-01T00:00:00Z"), null);
  }

  private static app.voice.auth.v1.GetSpaceSessionEpochFloorResponse invoke(
      UUID accountId, AccountRepository accounts, SessionEpochFloorStore floors) throws Exception {
    return invoke(accountId.toString(), accounts, floors);
  }

  private static app.voice.auth.v1.GetSpaceSessionEpochFloorResponse invoke(
      String accountId, AccountRepository accounts, SessionEpochFloorStore floors) throws Exception {
    return invoke(GetSpaceSessionEpochFloorRequest.newBuilder().setAccountId(accountId).build(), accounts, floors);
  }

  private static app.voice.auth.v1.GetSpaceSessionEpochFloorResponse invoke(
      GetSpaceSessionEpochFloorRequest request, AccountRepository accounts, SessionEpochFloorStore floors) throws Exception {
    KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
    generator.initialize(2048);
    KeyPair keyPair = generator.generateKeyPair();
    var clock = Clock.systemUTC();
    Set<String> replay = new HashSet<>();
    var verifier = new AuthPrincipalVerifier(
        (issuer, kid) -> "current".equals(kid) && "space".equals(issuer)
            ? (RSAPublicKey) keyPair.getPublic() : null,
        account -> { throw new AssertionError("service principal must not query user epoch here"); },
        (issuer, jti, expires) -> {
          if (!replay.add(issuer + ":" + jti)) throw new IllegalArgumentException("replayed request");
        }, clock);
    var service = new AuthGrpcService(null, null);
    if (accounts != null && floors != null) service.setVoiceSessionFloorSources(accounts, floors);
    var protectedService = AuthPrincipalServices.sessionFloorService(
        service.bindService(), new AuthPrincipalServerInterceptor(verifier));
    var builder = NettyServerBuilder.forPort(0);
    var env = new org.springframework.mock.env.MockEnvironment()
        .withProperty("AUTH_GRPC_TLS_CERT_FILE", resource("server-cert.pem").getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE", resource("server-key.pem").getAbsolutePath())
        .withProperty("AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE", resource("server-cert.pem").getAbsolutePath());
    env.setActiveProfiles("production");
    voice.backend.auth.principal.AuthPrincipalTransport.configureSessionFloor(builder, env, true);
    Server server = builder.addService(protectedService).build().start();
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts.forClient()
                .trustManager(resource("server-cert.pem"))
                .keyManager(resource("server-cert.pem"), resource("server-key.pem")).build()).build();
    try {
      String requestId = UUID.randomUUID().toString();
      String token = token(keyPair, request, requestId, clock.instant());
      Metadata headers = new Metadata();
      headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER), "Bearer " + token);
      headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), requestId);
      var signed = io.grpc.ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers));
      return AuthServiceGrpc.newBlockingStub(signed)
          .withDeadlineAfter(5, TimeUnit.SECONDS)
          .getSpaceSessionEpochFloor(request);
    } finally {
      channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
      server.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
    }
  }

  private static String token(KeyPair keyPair, GetSpaceSessionEpochFloorRequest request,
      String requestId, Instant now) throws Exception {
    String header = encode(JSON.writeValueAsBytes(Map.of("alg", "RS256", "kid", "current")));
    String claims = encode(JSON.writeValueAsBytes(Map.ofEntries(
        Map.entry("iss", "space"), Map.entry("sub", "service:space"),
        Map.entry("principal_type", "service"), Map.entry("aud", "auth"),
        Map.entry("rpc", RPC), Map.entry("request_id", requestId),
        Map.entry("request_hash", AuthPrincipalServerInterceptor.requestHash(request)),
        Map.entry("jti", UUID.randomUUID().toString()),
        Map.entry("iat", now.getEpochSecond()), Map.entry("nbf", now.getEpochSecond()),
        Map.entry("exp", now.plusSeconds(20).getEpochSecond()))));
    String unsigned = header + "." + claims;
    Signature signature = Signature.getInstance("SHA256withRSA");
    signature.initSign(keyPair.getPrivate());
    signature.update(unsigned.getBytes(StandardCharsets.US_ASCII));
    return unsigned + "." + encode(signature.sign());
  }

  private static String encode(byte[] value) {
    return Base64.getUrlEncoder().withoutPadding().encodeToString(value);
  }
}
