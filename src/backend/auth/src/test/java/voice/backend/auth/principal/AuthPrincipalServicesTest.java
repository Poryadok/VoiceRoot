package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import com.google.protobuf.Struct;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.protobuf.ProtoUtils;
import io.grpc.stub.ClientCalls;
import io.grpc.stub.MetadataUtils;
import io.grpc.stub.ServerCalls;
import java.io.File;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;

class AuthPrincipalServicesTest {
  static final String SERVICE = "voice.auth.v1.AuthService";
  static final Struct REQUEST = Struct.getDefaultInstance();
  static final MethodDescriptor<Struct,Struct> ISSUE = method(AuthPrincipalServerInterceptor.ISSUE_RPC);
  static final MethodDescriptor<Struct,Struct> CONSUME = method(AuthPrincipalServerInterceptor.CONSUME_RPC);
  static final MethodDescriptor<Struct,Struct> LOOKUP = method(AuthPrincipalServerInterceptor.LOOKUP_RPC);
  static final MethodDescriptor<Struct,Struct> FLOOR = method(AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC);
  static final MethodDescriptor<Struct,Struct> LOGIN = method("/" + SERVICE + "/Login");
  final AtomicInteger proofCalls = new AtomicInteger(), loginCalls = new AtomicInteger();
  final AtomicReference<VerifiedPrincipal> principal = new AtomicReference<>();
  final AuthPrincipalVerifierTest fixture = new AuthPrincipalVerifierTest();

  ServerServiceDefinition all() {
    return ServerServiceDefinition.builder(SERVICE)
        .addMethod(ISSUE, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); principal.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(CONSUME, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); principal.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(LOOKUP, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); principal.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(FLOOR, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); principal.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(LOGIN, ServerCalls.asyncUnaryCall((request, observer) -> {
          loginCalls.incrementAndGet(); observer.onNext(request); observer.onCompleted();
        })).build();
  }

  @Test void proofServiceContainsOnlyTheThreeExistingProofMethods() {
    var service = AuthPrincipalServices.proofService(all(), new AuthPrincipalServerInterceptor(fixture.verifier));
    assertEquals(Set.of(ISSUE.getFullMethodName(), CONSUME.getFullMethodName(), LOOKUP.getFullMethodName()), service.getMethods().stream()
        .map(method -> method.getMethodDescriptor().getFullMethodName()).collect(Collectors.toSet()));
    var floor = AuthPrincipalServices.sessionFloorService(all(), new AuthPrincipalServerInterceptor(fixture.verifier));
    assertEquals(Set.of(FLOOR.getFullMethodName()), floor.getMethods().stream()
        .map(method -> method.getMethodDescriptor().getFullMethodName()).collect(Collectors.toSet()));
    for (var missing : Set.of(ISSUE, CONSUME, LOOKUP)) {
      var builder = ServerServiceDefinition.builder(SERVICE);
      for (var present : Set.of(ISSUE, CONSUME, LOOKUP)) {
        if (present != missing) builder.addMethod(present,
            ServerCalls.asyncUnaryCall((request, observer) -> observer.onCompleted()));
      }
      var incomplete = builder.build();
      assertThrows(IllegalArgumentException.class,
          () -> AuthPrincipalServices.proofService(incomplete, new AuthPrincipalServerInterceptor(fixture.verifier)));
    }
  }

  @Test void legacyListenerRejectsAllThreeProofMethodsIncludingSignedLookupButStillServesLogin() throws Exception {
    Server server = NettyServerBuilder.forPort(0).addService(AuthPrincipalServices.legacyService(all())).build().start();
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort()).usePlaintext().build();
    try {
      for (var method : Set.of(ISSUE, CONSUME, LOOKUP)) {
        var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, method));
        assertEquals(Status.Code.UNAUTHENTICATED, failure.getStatus().getCode());
      }
      var floorFailure = assertThrows(StatusRuntimeException.class, () -> call(channel, FLOOR));
      assertEquals(Status.Code.UNIMPLEMENTED, floorFailure.getStatus().getCode());
      assertEquals(REQUEST, call(channel, LOGIN));
      assertEquals(0, proofCalls.get()); assertEquals(1, loginCalls.get());
    } finally { stop(channel, server); }
  }

  @Test void trustedMutualTlsAndBoundVoicePrincipalReachOnlyFloorHandler() throws Exception {
    Server server = floorTlsServer("server-cert.pem");
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(tlsClientContext(true, "server-cert.pem")).build();
    try {
      assertEquals(REQUEST, call(channel, FLOOR));
      assertEquals(1, proofCalls.get());
      assertNotNull(principal.get());
      assertEquals("voice", principal.get().issuer());
      assertNull(principal.get().accountId());
      var proofFailure = assertThrows(StatusRuntimeException.class, () -> call(channel, ISSUE));
      assertEquals(Status.Code.UNIMPLEMENTED, proofFailure.getStatus().getCode());
      assertEquals(1, proofCalls.get());
      var wrongService = assertThrows(StatusRuntimeException.class, () -> call(channel, FLOOR, "space"));
      assertEquals(Status.Code.PERMISSION_DENIED, wrongService.getStatus().getCode());
      assertEquals(1, proofCalls.get());
      var wrongMethod = assertThrows(StatusRuntimeException.class,
          () -> call(channel, FLOOR, "voice", AuthPrincipalServerInterceptor.ISSUE_RPC));
      assertEquals(Status.Code.UNAUTHENTICATED, wrongMethod.getStatus().getCode());
      assertEquals(1, proofCalls.get());
      var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, LOGIN));
      assertEquals(Status.Code.UNIMPLEMENTED, failure.getStatus().getCode());
      assertEquals(0, loginCalls.get());
    } finally { stop(channel, server); }
  }

  @Test void existingProofListenerStillAcceptsItsPriorServerTlsClientWithoutClientCertificate() throws Exception {
    Server server = tlsServer();
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(GrpcSslContexts.forClient().trustManager(resource("server-cert.pem")).build()).build();
    try {
      assertEquals(REQUEST, call(channel, ISSUE));
      assertEquals(1, proofCalls.get());
      var floorFailure = assertThrows(StatusRuntimeException.class, () -> call(channel, FLOOR));
      assertEquals(Status.Code.UNIMPLEMENTED, floorFailure.getStatus().getCode());
      assertEquals(1, proofCalls.get());
    } finally { stop(channel, server); }
  }

  @Test void missingClientCertificateAndWrongServerTrustNeverReachTlsHandler() throws Exception {
    Server server = floorTlsServer("server-cert.pem");
    try {
      for (int scenario = 0; scenario < 4; scenario++) {
        var builder = NettyChannelBuilder.forAddress("localhost", server.getPort());
        if (scenario == 3) builder.usePlaintext();
        else {
          boolean presentClientCertificate = scenario != 2;
          String trust = scenario == 0 ? "wrong-ca-cert.pem" : "server-cert.pem";
          builder.sslContext(tlsClientContext(presentClientCertificate, trust));
          if (scenario == 1) builder.overrideAuthority("wrong-server.invalid");
        }
        ManagedChannel channel = builder.build();
        try {
          var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, FLOOR));
          assertEquals(Status.Code.UNAVAILABLE, failure.getStatus().getCode());
          assertEquals(0, proofCalls.get());
        } finally { channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
      }
    } finally { server.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
  }

  @Test void clientCertificateOutsideTheConfiguredCaNeverReachesPrivateHandler() throws Exception {
    Server server = floorTlsServer("wrong-ca-cert.pem");
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(tlsClientContext(true, "server-cert.pem")).build();
    try {
      var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, FLOOR));
      assertEquals(Status.Code.UNAVAILABLE, failure.getStatus().getCode());
      assertEquals(0, proofCalls.get());
    } finally { stop(channel, server); }
  }

  Server tlsServer() throws Exception {
    var builder = NettyServerBuilder.forPort(0);
    var environment = new MockEnvironment().withProperty("AUTH_GRPC_TLS_CERT_FILE", resource("server-cert.pem").getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE", resource("server-key.pem").getAbsolutePath());
    environment.setActiveProfiles("production");
    AuthPrincipalTransport.configure(builder, environment, true);
    return builder.addService(AuthPrincipalServices.proofService(all(), new AuthPrincipalServerInterceptor(fixture.verifier)))
        .build().start();
  }

  Server floorTlsServer(String clientCa) throws Exception {
    var builder = NettyServerBuilder.forPort(0);
    var environment = new MockEnvironment().withProperty("AUTH_GRPC_TLS_CERT_FILE", resource("server-cert.pem").getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE", resource("server-key.pem").getAbsolutePath())
        .withProperty("AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE", resource(clientCa).getAbsolutePath());
    environment.setActiveProfiles("production");
    AuthPrincipalTransport.configureSessionFloor(builder, environment, true);
    return builder.addService(AuthPrincipalServices.sessionFloorService(all(),
        new AuthPrincipalServerInterceptor(fixture.verifier))).build().start();
  }

  static io.grpc.netty.shaded.io.netty.handler.ssl.SslContext tlsClientContext(boolean withClientCertificate, String trust)
      throws Exception {
    var builder = GrpcSslContexts.forClient().trustManager(resource(trust));
    if (withClientCertificate) builder.keyManager(resource("server-cert.pem"), resource("server-key.pem"));
    return builder.build();
  }

  Struct call(ManagedChannel channel, MethodDescriptor<Struct,Struct> method) {
    return call(channel, method, null, null);
  }

  Struct call(ManagedChannel channel, MethodDescriptor<Struct,Struct> method, String issuerOverride) {
    return call(channel, method, issuerOverride, null);
  }

  Struct call(ManagedChannel channel, MethodDescriptor<Struct,Struct> method,
      String issuerOverride, String rpcOverride) {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("rpc", rpcOverride == null ? "/" + method.getFullMethodName() : rpcOverride);
    claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(REQUEST));
    if (method == CONSUME || method == LOOKUP || method == FLOOR) {
      String issuer = method == FLOOR ? "voice" : "space";
      claims.put("iss", issuer); claims.put("sub", "service:" + issuer); claims.put("principal_type", "service");
      claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    }
    if (issuerOverride != null) {
      claims.put("iss", issuerOverride); claims.put("sub", "service:" + issuerOverride);
      claims.put("principal_type", "service");
      claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    }
    Metadata headers = new Metadata();
    headers.put(AuthPrincipalServerInterceptorTest.AUTH, "Bearer " + AuthPrincipalVerifierTest.token(claims));
    headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, "request-1");
    Channel signed = ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers));
    return ClientCalls.blockingUnaryCall(signed, method, CallOptions.DEFAULT.withDeadlineAfter(5, TimeUnit.SECONDS), REQUEST);
  }

  static File resource(String name) throws Exception {
    return new File(AuthPrincipalServicesTest.class.getResource("/principal-tls/" + name).toURI());
  }
  static MethodDescriptor<Struct,Struct> method(String rpc) {
    return MethodDescriptor.<Struct,Struct>newBuilder().setType(MethodDescriptor.MethodType.UNARY)
        .setFullMethodName(rpc.substring(1)).setRequestMarshaller(ProtoUtils.marshaller(REQUEST))
        .setResponseMarshaller(ProtoUtils.marshaller(REQUEST)).build();
  }
  static void stop(ManagedChannel channel, Server server) throws Exception {
    channel.shutdownNow(); server.shutdownNow();
    channel.awaitTermination(5, TimeUnit.SECONDS); server.awaitTermination(5, TimeUnit.SECONDS);
  }
}
