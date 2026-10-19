package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.*;
import io.grpc.stub.MetadataUtils;
import java.io.File;
import java.security.interfaces.RSAPublicKey;
import java.time.Clock;
import java.time.ZoneOffset;
import java.util.HashSet;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.roleidentityauthority.*;
import voice.identityerasure.v1.*;

class IdentityAuthorityMtlsTest {
  static final String RPC = IdentityAuthorityPrincipalTest.PARENT;
  static File resource(String name) throws Exception {
    return new File(IdentityAuthorityMtlsTest.class.getResource("/principal-tls/" + name).toURI());
  }
  static ResolveAccountHandleRequest request() {
    return ResolveAccountHandleRequest.newBuilder().setProtocolVersion(1)
        .setRequestId(UUID.randomUUID().toString()).setAccountId(UUID.randomUUID().toString()).build();
  }
  static Metadata headers(ResolveAccountHandleRequest request, String issuer) {
    var claims = new IdentityAuthorityPrincipalTest().claims(issuer, RPC);
    claims.put("request_id", request.getRequestId());
    claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
    var metadata = new Metadata();
    metadata.put(AuthPrincipalServerInterceptorTest.AUTH, "Bearer " + AuthPrincipalVerifierTest.token(claims));
    metadata.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, request.getRequestId());
    return metadata;
  }
  static ResolveAccountHandleResponse invoke(ManagedChannel channel, ResolveAccountHandleRequest request, Metadata headers) {
    return AuthUserIdentityAuthorityServiceGrpc.newBlockingStub(
        ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers)))
        .withDeadlineAfter(3, TimeUnit.SECONDS).resolveAccountHandle(request);
  }

  @Test void exactSignedMutualTlsCallSucceedsAndReplayCannotReadAgain() throws Exception {
    try (var fixture = new Fixture()) {
      var request = request(); var metadata = headers(request, "user");
      try (var connection = fixture.connection("server-cert.pem", true, "localhost", false)) {
        assertEquals(request.getAccountId(), invoke(connection.channel, request, metadata).getAccountId());
        assertEquals(Status.Code.UNAUTHENTICATED,
            assertThrows(StatusRuntimeException.class, () -> invoke(connection.channel, request, metadata)).getStatus().getCode());
        assertEquals(1, fixture.reads.get());
      }
    }
  }
  @Test void callerRawMetadataDuplicateAndChangedBodyFailBeforeOwnerLookup() throws Exception {
    try (var fixture = new Fixture(); var connection = fixture.connection("server-cert.pem", true, "localhost", false)) {
      var request = request();
      for (String issuer : java.util.List.of("role", "space", "gateway")) {
        assertThrows(StatusRuntimeException.class, () -> invoke(connection.channel, request, headers(request, issuer)));
      }
      for (String extra : java.util.List.of("x-account-id", "x-voice-profile-id", "authorization")) {
        var metadata = headers(request, "user");
        metadata.put(Metadata.Key.of(extra, Metadata.ASCII_STRING_MARSHALLER), "untrusted");
        assertThrows(StatusRuntimeException.class, () -> invoke(connection.channel, request, metadata));
      }
      assertThrows(StatusRuntimeException.class, () -> invoke(connection.channel,
          request.toBuilder().setAccountId(UUID.randomUUID().toString()).build(), headers(request, "user")));
      assertThrows(StatusRuntimeException.class, () -> invoke(connection.channel, request, new Metadata()));
      assertEquals(0, fixture.reads.get());
    }
  }
  @Test void wrongServerCaSanMissingClientCertificateAndPlaintextCannotReachOwner() throws Exception {
    try (var fixture = new Fixture()) {
      var request = request();
      for (String mode : java.util.List.of("ca", "san", "client", "plaintext")) {
        try (var connection = fixture.connection(mode.equals("ca") ? "wrong-ca-cert.pem" : "server-cert.pem",
            !mode.equals("client"), mode.equals("san") ? "wrong.invalid" : "localhost", mode.equals("plaintext"))) {
          assertThrows(StatusRuntimeException.class, () -> invoke(connection.channel, request, headers(request, "user")), mode);
        }
      }
      assertEquals(0, fixture.reads.get());
    }
  }

  static final class Fixture implements AutoCloseable {
    final AtomicInteger reads = new AtomicInteger();
    final Server server;
    Fixture() throws Exception {
      assertTrue(System.getProperty("os.name").toLowerCase().contains("linux"), "native runner must be Linux");
      var environment = new MockEnvironment().withProperty("AUTH_IDENTITY_AUTHORITY_ENABLED", "true")
          .withProperty("AUTH_IDENTITY_AUTHORITY_GRPC_LISTEN", "127.0.0.1:0")
          .withProperty("AUTH_IDENTITY_AUTHORITY_TLS_CERT_FILE", resource("server-cert.pem").toString())
          .withProperty("AUTH_IDENTITY_AUTHORITY_TLS_KEY_FILE", resource("server-key.pem").toString())
          .withProperty("AUTH_IDENTITY_AUTHORITY_TLS_CLIENT_CA_FILE", resource("server-cert.pem").toString())
          .withProperty("S2S_JWKS_URLS_JSON", "{\"role\":\"https://role/keys\",\"user\":\"https://user/keys\"}");
      environment.setActiveProfiles("test");
      var builder = NettyServerBuilder.forAddress(IdentityAuthorityTransportConfig.load(environment).listen());
      IdentityAuthorityTransport.configure(builder, IdentityAuthorityTransportConfig.load(environment));
      var seen = java.util.concurrent.ConcurrentHashMap.<String>newKeySet();
      var verifier = AuthPrincipalVerifier.identityAuthority(
          (issuer, kid) -> (RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic(),
          (issuer, jti, expires) -> { if (!seen.add(issuer + ":" + jti)) throw new IllegalArgumentException(); },
          Clock.fixed(AuthPrincipalVerifierTest.NOW, ZoneOffset.UTC));
      var service = new AuthParentIdentityGrpcAdapter(account -> {
        reads.incrementAndGet();
        return new AuthIdentityOwnerStore.Observed(account, AccountLifecycleFact.getDefaultInstance(), ObservationWindow.getDefaultInstance());
      });
      server = builder.addService(ServerInterceptors.intercept(service,
          AuthPrincipalServerInterceptor.identityAuthority(verifier))).build().start();
    }
    Connection connection(String trust, boolean clientCertificate, String authority, boolean plaintext) throws Exception {
      var builder = NettyChannelBuilder.forAddress("localhost", server.getPort()).overrideAuthority(authority);
      if (plaintext) builder.usePlaintext();
      else {
        var tls = GrpcSslContexts.forClient().trustManager(resource(trust));
        if (clientCertificate) tls.keyManager(resource("server-cert.pem"), resource("server-key.pem"));
        builder.sslContext(tls.build());
      }
      return new Connection(builder.build());
    }
    public void close() throws Exception { server.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
  }
  record Connection(ManagedChannel channel) implements AutoCloseable {
    public void close() throws Exception { channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
  }
}
