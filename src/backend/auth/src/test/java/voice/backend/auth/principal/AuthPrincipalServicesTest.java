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
        .addMethod(LOGIN, ServerCalls.asyncUnaryCall((request, observer) -> {
          loginCalls.incrementAndGet(); observer.onNext(request); observer.onCompleted();
        })).build();
  }

  @Test void privateServiceContainsExactlyTheThreeProofMethods() {
    var service = AuthPrincipalServices.proofService(all(), new AuthPrincipalServerInterceptor(fixture.verifier));
    assertEquals(Set.of(ISSUE.getFullMethodName(), CONSUME.getFullMethodName(), LOOKUP.getFullMethodName()), service.getMethods().stream()
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
      assertEquals(REQUEST, call(channel, LOGIN));
      assertEquals(0, proofCalls.get()); assertEquals(1, loginCalls.get());
    } finally { stop(channel, server); }
  }

  @Test void trustedTlsAndBoundPrincipalReachPrivateHandlerAndLoginIsNotExposed() throws Exception {
    Server server = tlsServer();
    ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(GrpcSslContexts.forClient().trustManager(resource("server-cert.pem")).build()).build();
    try {
      assertEquals(REQUEST, call(channel, ISSUE));
      assertEquals(1, proofCalls.get());
      assertNotNull(principal.get());
      assertEquals(AuthPrincipalVerifierTest.ACCOUNT, principal.get().accountId());
      assertEquals(REQUEST, call(channel, CONSUME));
      assertEquals(2, proofCalls.get());
      assertEquals("space", principal.get().issuer());
      assertNull(principal.get().accountId());
      assertEquals(REQUEST, call(channel, LOOKUP));
      assertEquals(3, proofCalls.get());
      assertEquals("space", principal.get().issuer());
      assertNull(principal.get().accountId());
      var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, LOGIN));
      assertEquals(Status.Code.UNIMPLEMENTED, failure.getStatus().getCode());
      assertEquals(0, loginCalls.get());
    } finally { stop(channel, server); }
  }

  @Test void wrongCaWrongServerNameAndPlaintextNeverReachTlsHandler() throws Exception {
    Server server = tlsServer();
    try {
      for (int scenario = 0; scenario < 3; scenario++) {
        var builder = NettyChannelBuilder.forAddress("localhost", server.getPort());
        if (scenario == 2) builder.usePlaintext();
        else {
          builder.sslContext(GrpcSslContexts.forClient()
              .trustManager(resource(scenario == 0 ? "wrong-ca-cert.pem" : "server-cert.pem")).build());
          if (scenario == 1) builder.overrideAuthority("wrong-server.invalid");
        }
        ManagedChannel channel = builder.build();
        try {
          var failure = assertThrows(StatusRuntimeException.class, () -> call(channel, ISSUE));
          assertEquals(Status.Code.UNAVAILABLE, failure.getStatus().getCode());
          assertEquals(0, proofCalls.get());
        } finally { channel.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
      }
    } finally { server.shutdownNow().awaitTermination(5, TimeUnit.SECONDS); }
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

  Struct call(ManagedChannel channel, MethodDescriptor<Struct,Struct> method) {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("rpc", "/" + method.getFullMethodName());
    claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(REQUEST));
    if (method == CONSUME || method == LOOKUP) {
      claims.put("iss", "space"); claims.put("sub", "service:space"); claims.put("principal_type", "service");
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
