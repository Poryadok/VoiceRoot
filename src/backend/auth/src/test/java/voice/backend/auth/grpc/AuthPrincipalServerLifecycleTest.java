package voice.backend.auth.grpc;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

import com.google.protobuf.Struct;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.protobuf.ProtoUtils;
import io.grpc.stub.ClientCalls;
import io.grpc.stub.MetadataUtils;
import io.grpc.stub.ServerCalls;
import java.net.ServerSocket;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalVerifier;
import voice.backend.auth.principal.VerifiedPrincipal;

class AuthPrincipalServerLifecycleTest {
  static final String SERVICE = "voice.auth.v1.AuthService";
  static final Struct REQUEST = Struct.getDefaultInstance();
  static final MethodDescriptor<Struct,Struct> ISSUE = method(AuthPrincipalServerInterceptor.ISSUE_RPC);
  static final MethodDescriptor<Struct,Struct> CONSUME = method(AuthPrincipalServerInterceptor.CONSUME_RPC);
  static final MethodDescriptor<Struct,Struct> LOOKUP = method(AuthPrincipalServerInterceptor.LOOKUP_RPC);
  static final MethodDescriptor<Struct,Struct> FLOOR = method(AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC);
  static final MethodDescriptor<Struct,Struct> LOGIN = method("/" + SERVICE + "/Login");
  final AuthGrpcService service = mock(AuthGrpcService.class);
  final AuthPrincipalVerifier verifier = mock(AuthPrincipalVerifier.class);
  final AtomicInteger proofCalls = new AtomicInteger(), loginCalls = new AtomicInteger();
  final AtomicReference<VerifiedPrincipal> observed = new AtomicReference<>();
  final VerifiedPrincipal principal = new VerifiedPrincipal("delegated_user", "gateway", UUID.randomUUID(), UUID.randomUUID(), 2);
  final VerifiedPrincipal spacePrincipal = new VerifiedPrincipal("service", "space", null, null, 0);
  final VerifiedPrincipal voicePrincipal = new VerifiedPrincipal("service", "voice", null, null, 0);
  final AuthProperties properties = new AuthProperties();

  AuthPrincipalServerLifecycleTest() {
    properties.getGrpc().setPort(0);
    when(verifier.verify(anyString(), anyString(), anyString(), anyString())).thenReturn(principal);
    when(verifier.verify(anyString(), eq(AuthPrincipalServerInterceptor.LOOKUP_RPC), anyString(), anyString()))
        .thenReturn(spacePrincipal);
    when(verifier.verify(anyString(), eq(AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC), anyString(), anyString()))
        .thenReturn(voicePrincipal);
    when(service.bindService()).thenReturn(definition(true));
  }

  @Test void startsSeparateListenersRegistersOnlyProofsPrivatelyAndStopsBoth() throws Exception {
    AuthGrpcServer server = server(new AuthPrincipalServerInterceptor(verifier));
    ManagedChannel legacy = null, privateChannel = null, floorChannel = null;
    try {
      server.start();
      assertTrue(server.isRunning());
      assertTrue(server.legacyPort() > 0); assertTrue(server.principalPort() > 0); assertTrue(server.sessionFloorPort() > 0);
      assertNotEquals(server.legacyPort(), server.principalPort());
      assertNotEquals(server.legacyPort(), server.sessionFloorPort());
      assertNotEquals(server.principalPort(), server.sessionFloorPort());
      legacy = channel(server.legacyPort()); privateChannel = channel(server.principalPort());
      floorChannel = channel(server.sessionFloorPort());
      assertStatus(legacy, ISSUE, Status.Code.UNAUTHENTICATED);
      assertStatus(legacy, LOOKUP, Status.Code.UNAUTHENTICATED);
      assertStatus(legacy, FLOOR, Status.Code.UNIMPLEMENTED);
      assertEquals(0, proofCalls.get());
      assertEquals(REQUEST, call(legacy, LOGIN));
      assertEquals(1, loginCalls.get());
      assertEquals(REQUEST, call(privateChannel, ISSUE));
      assertEquals(1, proofCalls.get()); assertEquals(principal, observed.get());
      verify(verifier).verify("test-credential", AuthPrincipalServerInterceptor.ISSUE_RPC,
          "lifecycle-request", AuthPrincipalServerInterceptor.requestHash(REQUEST));
      assertEquals(REQUEST, call(privateChannel, LOOKUP));
      assertEquals(2, proofCalls.get()); assertEquals(spacePrincipal, observed.get());
      verify(verifier).verify("test-credential", AuthPrincipalServerInterceptor.LOOKUP_RPC,
          "lifecycle-request", AuthPrincipalServerInterceptor.requestHash(REQUEST));
      assertStatus(privateChannel, FLOOR, Status.Code.UNIMPLEMENTED);
      assertEquals(REQUEST, call(floorChannel, FLOOR));
      assertEquals(voicePrincipal, observed.get());
      assertEquals(3, proofCalls.get());
      verify(verifier).verify("test-credential", AuthPrincipalServerInterceptor.VOICE_SESSION_FLOOR_RPC,
          "lifecycle-request", AuthPrincipalServerInterceptor.requestHash(REQUEST));
      assertStatus(floorChannel, ISSUE, Status.Code.UNIMPLEMENTED);
      assertStatus(privateChannel, LOGIN, Status.Code.UNIMPLEMENTED);
      for(var sourceRpc : java.util.List.of("ReadSnapshot","ReadRevision")) {
        var source = method("/voice.authority.v1.AuthoritySourceService/" + sourceRpc);
        assertStatus(legacy, source, Status.Code.UNIMPLEMENTED);
        assertStatus(privateChannel, source, Status.Code.UNIMPLEMENTED);
      }
      assertEquals(1, loginCalls.get());
      int legacyPort = server.legacyPort();
      int principalPort = server.principalPort(); int floorPort = server.sessionFloorPort();
      server.stop();
      assertFalse(server.isRunning());
      // shutdownNow cancels calls on an established transport. Verify listener shutdown through
      // new connection attempts, whose contract is UNAVAILABLE rather than transport cancellation.
      close(legacy); close(privateChannel); close(floorChannel);
      legacy = channel(legacyPort); privateChannel = channel(principalPort); floorChannel = channel(floorPort);
      assertStatus(legacy, LOGIN, Status.Code.UNAVAILABLE);
      assertStatus(privateChannel, ISSUE, Status.Code.UNAVAILABLE);
      assertStatus(privateChannel, LOOKUP, Status.Code.UNAVAILABLE);
      assertStatus(floorChannel, FLOOR, Status.Code.UNAVAILABLE);
       assertEquals(1, loginCalls.get()); assertEquals(3, proofCalls.get());
    } finally { server.stop(); close(legacy); close(privateChannel); close(floorChannel); }
  }

  @Test void absentVerifierHasNoPrivateListenerAndProofsRemainDeniedOnLegacy() throws Exception {
    AuthGrpcServer server = server(new AuthPrincipalServerInterceptor(null));
    ManagedChannel legacy = null;
    try {
      server.start();
      assertTrue(server.isRunning()); assertEquals(-1, server.principalPort());
      legacy = channel(server.legacyPort());
      assertStatus(legacy, ISSUE, Status.Code.UNAUTHENTICATED);
      assertStatus(legacy, CONSUME, Status.Code.UNAUTHENTICATED);
      assertStatus(legacy, LOOKUP, Status.Code.UNAUTHENTICATED);
      assertEquals(REQUEST, call(legacy, LOGIN));
      assertEquals(0, proofCalls.get()); assertEquals(1, loginCalls.get());
      verifyNoInteractions(verifier);
    } finally { server.stop(); close(legacy); }
  }

  @Test void incompletePrivateServiceFailsStartupWithoutLeakingLegacyListener() throws Exception {
    // Reserve an ephemeral port briefly so rollback can be checked by rebinding the exact port.
    int port;
    try (var reservation = new ServerSocket(0)) { port = reservation.getLocalPort(); }
    properties.getGrpc().setPort(port);
    when(service.bindService()).thenReturn(definition(false));
    AuthGrpcServer server = server(new AuthPrincipalServerInterceptor(verifier));
    try {
      assertThrows(RuntimeException.class, server::start);
      assertFalse(server.isRunning());
      try (var rebound = new ServerSocket(port)) { assertEquals(port, rebound.getLocalPort()); }
      assertEquals(0, proofCalls.get()); assertEquals(0, loginCalls.get());
    } finally { server.stop(); }
  }

  @Test void missingFloorClientCaFailsProductionStartupBeforeBindingAnyListener() throws Exception {
    int legacyPort = freePort();
    int principalPort = freePortExcept(legacyPort);
    int floorPort = freePortExcept(legacyPort, principalPort);
    properties.getGrpc().setPort(legacyPort);
    var cert = new java.io.File(getClass().getResource("/principal-tls/server-cert.pem").toURI());
    var key = new java.io.File(getClass().getResource("/principal-tls/server-key.pem").toURI());
    var environment = new MockEnvironment()
        .withProperty("AUTH_PRINCIPAL_GRPC_PORT", Integer.toString(principalPort))
        .withProperty("AUTH_SESSION_FLOOR_GRPC_PORT", Integer.toString(floorPort))
        .withProperty("AUTH_GRPC_TLS_CERT_FILE", cert.getAbsolutePath())
        .withProperty("AUTH_GRPC_TLS_KEY_FILE", key.getAbsolutePath());
    environment.setActiveProfiles("production");
    AuthGrpcServer server = new AuthGrpcServer(service, properties, new RequestIdServerInterceptor(),
        new AuthorizationServerInterceptor(), new AuthPrincipalServerInterceptor(verifier), environment);

    assertThrows(IllegalStateException.class, server::start);
    assertFalse(server.isRunning());
    for (int port : java.util.List.of(legacyPort, principalPort, floorPort)) {
      try (var rebound = new ServerSocket(port)) { assertEquals(port, rebound.getLocalPort()); }
    }
    verifyNoInteractions(verifier);
    assertEquals(0, proofCalls.get());
    assertEquals(0, loginCalls.get());
  }

  AuthGrpcServer server(AuthPrincipalServerInterceptor interceptor) {
    var environment = new MockEnvironment().withProperty("AUTH_PRINCIPAL_GRPC_PORT", "0")
        .withProperty("AUTH_SESSION_FLOOR_GRPC_PORT", "0");
    environment.setActiveProfiles("test");
    return new AuthGrpcServer(service, properties, new RequestIdServerInterceptor(),
        new AuthorizationServerInterceptor(), interceptor, environment);
  }

  ServerServiceDefinition definition(boolean includeConsume) {
    var definition = ServerServiceDefinition.builder(SERVICE)
        .addMethod(ISSUE, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); observed.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(LOOKUP, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); observed.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(FLOOR, ServerCalls.asyncUnaryCall((request, observer) -> {
          proofCalls.incrementAndGet(); observed.set(VerifiedPrincipal.current());
          observer.onNext(request); observer.onCompleted();
        }))
        .addMethod(LOGIN, ServerCalls.asyncUnaryCall((request, observer) -> {
          loginCalls.incrementAndGet(); observer.onNext(request); observer.onCompleted();
        }));
    if (includeConsume) definition.addMethod(CONSUME, ServerCalls.asyncUnaryCall((request, observer) -> {
      proofCalls.incrementAndGet(); observer.onNext(request); observer.onCompleted();
    }));
    return definition.build();
  }

  static Struct call(ManagedChannel channel, MethodDescriptor<Struct,Struct> method) {
    Metadata headers = new Metadata();
    headers.put(Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER), "Bearer test-credential");
    headers.put(Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER), "lifecycle-request");
    Channel signed = ClientInterceptors.intercept(channel, MetadataUtils.newAttachHeadersInterceptor(headers));
    return ClientCalls.blockingUnaryCall(signed, method, CallOptions.DEFAULT.withDeadlineAfter(3, TimeUnit.SECONDS), REQUEST);
  }
  static void assertStatus(ManagedChannel channel, MethodDescriptor<Struct,Struct> method, Status.Code code) {
    var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, method));
    assertEquals(code, failure.getStatus().getCode());
  }
  static MethodDescriptor<Struct,Struct> method(String rpc) {
    return MethodDescriptor.<Struct,Struct>newBuilder().setType(MethodDescriptor.MethodType.UNARY)
        .setFullMethodName(rpc.substring(1)).setRequestMarshaller(ProtoUtils.marshaller(REQUEST))
        .setResponseMarshaller(ProtoUtils.marshaller(REQUEST)).build();
  }
  static ManagedChannel channel(int port) {
    return NettyChannelBuilder.forAddress("localhost", port).usePlaintext().build();
  }
  static int freePort() throws Exception { return freePortExcept(); }
  static int freePortExcept(int... excluded) throws Exception {
    for (int attempt = 0; attempt < 20; attempt++) {
      try (var socket = new ServerSocket(0)) {
        int port = socket.getLocalPort();
        if (java.util.Arrays.stream(excluded).noneMatch(value -> value == port)) return port;
      }
    }
    throw new IllegalStateException("could not allocate distinct test ports");
  }
  static void close(ManagedChannel channel) throws Exception {
    if (channel != null) channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS);
  }
}
