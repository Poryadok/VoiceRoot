package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import app.voice.auth.v1.*;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.stub.MetadataUtils;
import java.security.interfaces.RSAPublicKey;
import java.time.*;
import java.util.*;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.repository.*;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;

class AuthBotSessionFloorGrpcTest {
  static final String RPC = "/voice.auth.v1.AuthService/GetBotSessionEpochFloor";
  static final String PEER = "urn:voice:test:bot-floor";
  static final UUID ACCOUNT = UUID.fromString("00000000-0000-0000-0000-000000000271");
  static final GetBotSessionEpochFloorRequest REQUEST = GetBotSessionEpochFloorRequest.newBuilder()
      .setAccountId(ACCOUNT.toString()).build();

  @Test void actualBotMutualTlsAndSignedBodyReadPositiveMaximumWithoutWrites() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      assertEquals(9, f.call(REQUEST, f.claims(REQUEST)).getSessionEpochFloor());
      when(f.floors.requireFloor(ACCOUNT)).thenReturn(4L);
      assertEquals(7, f.call(REQUEST, f.claims(REQUEST)).getSessionEpochFloor());
      verify(f.accounts, times(2)).findById(ACCOUNT.toString());
      verify(f.floors, times(2)).requireFloor(ACCOUNT);
      verifyNoMoreInteractions(f.accounts, f.floors);
    }
  }

  @Test void sameCaForeignPeerAndMissingConfigurationNeverReadOwners() throws Exception {
    for (String peer : Arrays.asList(null, "", "not a URI", "urn:voice:test:foreign", "urn:voice:*", "/relative",
        "urn:voice:test:bot-floor#fragment", "https://user@example.test/bot", "https://example.test/a/../bot")) {
      try (var f = new Fixture(peer, "bot", true)) {
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
        verifyNoInteractions(f.accounts, f.floors);
      }
    }
    try (var f = new Fixture(PEER, "server", true)) {
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void plaintextEvenWithBotConfigNeverReadsOwners() throws Exception {
    try (var f = new Fixture(PEER, "bot", false)) {
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void rejectsMalformedAccountsAndUnknownFieldsBeforeOwnerReads() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      for (String id : List.of("", "not-an-account", "00000000-0000-0000-0000-000000000000",
          "8A78BD68-75E9-4F21-9387-3BDC2F6116AB")) {
        var request = REQUEST.toBuilder().setAccountId(id).build();
        assertEquals(Status.Code.INVALID_ARGUMENT, assertThrows(StatusRuntimeException.class,
            () -> f.call(request, f.claims(request))).getStatus().getCode());
      }
      var request = REQUEST.toBuilder().setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder()
          .addField(99, com.google.protobuf.UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build()).build();
      assertEquals(Status.Code.INVALID_ARGUMENT, assertThrows(StatusRuntimeException.class,
          () -> f.call(request, f.claims(request))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void wrongBindingsMixedIdentityTemporalViolationsAndReplayNeverReadOwners() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      for (String field : List.of("sub", "principal_type", "aud", "rpc", "request_id", "request_hash", "jti")) {
        var c = f.claims(REQUEST); c.put(field, "wrong");
        if (field.equals("jti")) c.remove(field);
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c), field).getStatus().getCode());
      }
      for (String field : List.of("account_id", "profile_id", "session_epoch", "account_type", "bot_id", "bot_actor_id",
          "delegation", "actor_profile_id", "installation_id", "bot_scope", "arbitrary_extension"))
          for (Object value : Arrays.asList(null, "", "untrusted")) {
        var c = f.claims(REQUEST); c.put(field, value);
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c), field).getStatus().getCode());
      }
      for (var change : List.of(Map.entry("exp", AuthPrincipalVerifierTest.NOW.getEpochSecond()),
          Map.entry("exp", AuthPrincipalVerifierTest.NOW.plusSeconds(31).getEpochSecond()),
          Map.entry("iat", AuthPrincipalVerifierTest.NOW.plusSeconds(6).getEpochSecond()),
          Map.entry("nbf", AuthPrincipalVerifierTest.NOW.plusSeconds(6).getEpochSecond()))) {
        var c = f.claims(REQUEST); c.put(change.getKey(), change.getValue());
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c)).getStatus().getCode());
      }
      verifyNoInteractions(f.accounts, f.floors);
      var valid = f.claims(REQUEST);
      assertEquals(9, f.call(REQUEST, valid).getSessionEpochFloor());
      clearInvocations(f.accounts, f.floors);
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, valid)).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void readErrorsAndMissingOrDeletedAccountsNeverFabricateFloor() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      when(f.accounts.findById(ACCOUNT.toString())).thenReturn(Optional.empty());
      assertEquals(Status.Code.NOT_FOUND, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      for (Account deleted : List.of(account(7, "deleted", null), account(7, "active", Instant.now()))) {
        when(f.accounts.findById(ACCOUNT.toString())).thenReturn(Optional.of(deleted));
        assertEquals(Status.Code.NOT_FOUND, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      }
      verifyNoInteractions(f.floors);
      when(f.accounts.findById(ACCOUNT.toString())).thenReturn(Optional.of(account(7, "active", null)));
      for (long invalid : new long[]{0, -1}) {
        when(f.floors.requireFloor(ACCOUNT)).thenReturn(invalid);
        assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      }
      when(f.floors.requireFloor(ACCOUNT)).thenThrow(new IllegalStateException("floor outage"));
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      when(f.accounts.findById(ACCOUNT.toString())).thenThrow(new IllegalStateException("database outage"));
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
    }
  }

  @Test void wrongServicesAndDuplicateOrRawMetadataCannotReadOwners() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      var delegated = AuthPrincipalVerifierTest.claims();
      delegated.put("rpc", RPC); delegated.put("request_hash", AuthPrincipalServerInterceptor.requestHash(REQUEST));
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, delegated)).getStatus().getCode());
      for (String issuer : List.of("role", "space", "voice", "gateway")) {
        var c = f.claims(REQUEST); c.put("iss", issuer); c.put("sub", "service:" + issuer);
        assertEquals(Status.Code.PERMISSION_DENIED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, c)).getStatus().getCode());
      }
      for (String header : List.of("authorization", "x-request-id", "x-account-id", "x-profile-id", "x-voice-bot-id")) {
        var extra = new Metadata();
        extra.put(Metadata.Key.of(header, Metadata.ASCII_STRING_MARSHALLER), "untrusted");
        assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
            () -> f.call(REQUEST, f.claims(REQUEST), extra)).getStatus().getCode());
      }
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void dependencyOutagesAreUnavailableAndDoNotReadOwnerSources() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      f.keyOutage = true;
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      f.keyOutage = false; f.replayOutage = true;
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void exactGeneratedGoRequestHashVectorIsPreserved() {
    // Independently executed shared Go principal.RequestHash, BE170 consumer247.
    assertEquals("0a2430303030303030302d303030302d303030302d303030302d303030303030303030323731",
        java.util.HexFormat.of().formatHex(REQUEST.toByteArray()));
    assertEquals("sha256:5e3a01dc6a24fdf4ed5b82af5e85daa41e759a7e0ec243852110fa8468d8b692",
        AuthPrincipalServerInterceptor.requestHash(REQUEST));
  }

  @Test void botCannotBorrowAnyOldProtectedMethod() throws Exception {
    try (var f = new Fixture(PEER, "bot", true)) {
      var verifier = new AuthPrincipalVerifierTest().verifier;
      for (String rpc : List.of(AuthPrincipalServerInterceptor.ISSUE_RPC, AuthPrincipalServerInterceptor.CONSUME_RPC,
          AuthPrincipalServerInterceptor.LOOKUP_RPC, AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC,
          AuthPrincipalServerInterceptor.ROLE_SESSION_FLOOR_RPC, AuthPrincipalServerInterceptor.SPACE_SESSION_FLOOR_RPC,
          AuthPrincipalServerInterceptor.SPACE_ADMISSION_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_CONSUME_RPC,
          AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC)) {
        var c = f.claims(REQUEST); c.put("rpc", rpc);
        assertEquals(Status.Code.PERMISSION_DENIED, assertThrows(StatusRuntimeException.class,
            () -> verifier.verify(AuthPrincipalVerifierTest.token(c), rpc, "request-1",
                AuthPrincipalServerInterceptor.requestHash(REQUEST)), rpc).getStatus().getCode());
      }
      var old = f.claims(REQUEST);
      old.put("iss", "voice"); old.put("sub", "service:voice");
      old.put("rpc", AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC); old.put("actor_profile_id", null);
      assertEquals("voice", verifier.verify(AuthPrincipalVerifierTest.token(old),
          AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC, "request-1",
          AuthPrincipalServerInterceptor.requestHash(REQUEST)).issuer(), "closed claim shape is Bot-method only");
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void missingClientCertificateAndDisabledVerifierCannotReachBotHandler() throws Exception {
    try (var f = new Fixture(PEER, null, true)) {
      assertEquals(Status.Code.UNAVAILABLE, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
    try (var f = new Fixture(PEER, "bot", true, false)) {
      assertEquals(Status.Code.UNAUTHENTICATED, assertThrows(StatusRuntimeException.class,
          () -> f.call(REQUEST, f.claims(REQUEST))).getStatus().getCode());
      verifyNoInteractions(f.accounts, f.floors);
    }
  }

  @Test void ordinaryAndProofDefinitionsExcludeBotMethod() {
    var service = new AuthGrpcService(null, null).bindService();
    for (var definition : List.of(AuthPrincipalServices.legacyService(service),
        AuthPrincipalServices.proofService(service, new AuthPrincipalServerInterceptor(null)))) {
      assertFalse(definition.getMethods().stream().anyMatch(m ->
          m.getMethodDescriptor().getFullMethodName().equals(RPC.substring(1))));
    }
  }

  static class Fixture implements AutoCloseable {
    final AccountRepository accounts = mock(AccountRepository.class);
    final SessionEpochFloorStore floors = mock(SessionEpochFloorStore.class);
    final Set<String> replay = new HashSet<>();
    volatile boolean keyOutage, replayOutage;
    final Server server;
    final ManagedChannel channel;
    Fixture(String peer, String certificate, boolean tls) throws Exception {
      this(peer, certificate, tls, true);
    }
    Fixture(String peer, String certificate, boolean tls, boolean enabled) throws Exception {
      when(accounts.findById(ACCOUNT.toString())).thenReturn(Optional.of(account(7, "active", null)));
      when(floors.requireFloor(ACCOUNT)).thenReturn(9L);
      var resolver = new AuthPrincipalJwksResolver(issuer -> AuthPrincipalJwksResolverTest.set(
          AuthPrincipalJwksResolverTest.jwk("current", (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic()),
          AuthPrincipalJwksResolverTest.jwk("next", AuthPrincipalJwksResolverTest.NEXT)),
          Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC), Duration.ofSeconds(5), Duration.ofSeconds(10), Duration.ofSeconds(1));
      var verifier = new AuthPrincipalVerifier((issuer, kid) -> {
            if (keyOutage) throw new PrincipalDependencyUnavailableException();
            return resolver.resolve(issuer, kid);
          },
          id -> { throw new AssertionError("service proof must not read delegated epoch"); },
          (issuer, jti, expires) -> {
            if (replayOutage) throw new PrincipalDependencyUnavailableException();
            if (!replay.add(issuer + ":" + jti)) throw new IllegalArgumentException();
          },
          Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));
      var interceptor = new AuthPrincipalServerInterceptor(enabled ? verifier : null, () -> {}, peer);
      var service = new AuthGrpcService(null, null);
      service.setVoiceSessionFloorSources(accounts, floors);
      var builder = NettyServerBuilder.forPort(0);
      if (tls) {
        var env = new MockEnvironment()
            .withProperty("AUTH_GRPC_TLS_CERT_FILE", resource("server-cert.pem").toString())
            .withProperty("AUTH_GRPC_TLS_KEY_FILE", resource("server-key.pem").toString())
            .withProperty("AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE", resource("server-cert.pem").toString());
        env.setActiveProfiles("production");
        AuthPrincipalTransport.configureSessionFloor(builder, env, true);
      }
      server = builder.addService(AuthPrincipalServices.sessionFloorService(service.bindService(), interceptor)).build().start();
      var client = NettyChannelBuilder.forAddress("localhost", server.getPort());
      if (tls) {
        var ssl = GrpcSslContexts.forClient().trustManager(resource("server-cert.pem"));
        if (certificate != null) ssl.keyManager(resource(certificate + "-cert.pem"), resource(certificate + "-key.pem"));
        client.sslContext(ssl.build());
      }
      else client.usePlaintext();
      channel = client.build();
      clearInvocations(accounts, floors);
    }
    Map<String,Object> claims(com.google.protobuf.MessageLite request) {
      var c = AuthPrincipalVerifierTest.claims();
      c.put("iss", "bot"); c.put("sub", "service:bot"); c.put("principal_type", "service");
      c.put("rpc", RPC); c.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
      c.remove("account_id"); c.remove("profile_id"); c.remove("session_epoch");
      return c;
    }
    GetBotSessionEpochFloorResponse call(GetBotSessionEpochFloorRequest request, Map<String,Object> claims) {
      return call(request, claims, new Metadata());
    }
    GetBotSessionEpochFloorResponse call(GetBotSessionEpochFloorRequest request, Map<String,Object> claims, Metadata extra) {
      Metadata headers = new Metadata();
      headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER),
          "Bearer " + AuthPrincipalVerifierTest.token(claims));
      headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), "request-1");
      headers.merge(extra);
      return AuthServiceGrpc.newBlockingStub(ClientInterceptors.intercept(channel,
          MetadataUtils.newAttachHeadersInterceptor(headers))).withDeadlineAfter(5, TimeUnit.SECONDS).getBotSessionEpochFloor(request);
    }
    @Override public void close() throws Exception {
      channel.shutdownNow(); server.shutdownNow();
      channel.awaitTermination(5, TimeUnit.SECONDS); server.awaitTermination(5, TimeUnit.SECONDS);
    }
  }
  static Account account(long epoch, String status, Instant deleted) {
    return new Account(ACCOUNT, null, null, null, "regular", status, null, false, epoch,
        Instant.parse("2026-01-01T00:00:00Z"), deleted);
  }
  static java.io.File resource(String name) throws Exception { return AuthPrincipalServicesTest.resource(name); }
}
