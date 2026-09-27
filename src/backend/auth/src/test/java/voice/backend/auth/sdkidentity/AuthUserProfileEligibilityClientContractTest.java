package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.junit.jupiter.api.Assertions.fail;

import app.voice.user.v1.GetSdkProfileEligibilityRequest;
import app.voice.user.v1.GetSdkProfileEligibilityResponse;
import app.voice.user.v1.UserServiceGrpc;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jwt.SignedJWT;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.ServerInterceptors;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.lang.reflect.InvocationTargetException;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.core.io.ClassPathResource;
import io.grpc.ManagedChannel;

/** End-to-end TLS test for Auth's request-bound User eligibility call. */
class AuthUserProfileEligibilityClientContractTest {
  private static final String RPC = "/voice.user.v1.UserService/GetSdkProfileEligibility";
  private static final Instant NOW = Instant.parse("2026-09-27T12:00:00Z");
  private static final Metadata.Key<String> AUTHORIZATION =
      Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
  private static final Metadata.Key<String> REQUEST_ID =
      Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);

  @TempDir Path keys;
  private final UUID accountId = UUID.fromString("0bd64a51-8720-4846-a4ea-3ff980a41d83");
  private final UUID profileId = UUID.fromString("d371d30f-7059-46e7-9880-3fbc4c61f8e9");
  private final AtomicReference<GetSdkProfileEligibilityResponse> response = new AtomicReference<>();
  private final AtomicReference<Status> rpcFailure = new AtomicReference<>();
  private final AtomicReference<GetSdkProfileEligibilityRequest> seenRequest = new AtomicReference<>();
  private final AtomicReference<String> seenMethod = new AtomicReference<>();
  private final List<String> authorizationHeaders = new ArrayList<>();
  private final List<String> requestIds = new ArrayList<>();
  private Server server;
  private ManagedChannel channel;

  @BeforeEach
  void startTlsUserListener() throws Exception {
    response.set(eligibleResponse());
    server = NettyServerBuilder.forPort(0)
        .useTransportSecurity(resource("principal-tls/server-cert.pem"), resource("principal-tls/server-key.pem"))
        .addService(ServerInterceptors.intercept(new UserServiceGrpc.UserServiceImplBase() {
          @Override
          public void getSdkProfileEligibility(GetSdkProfileEligibilityRequest req,
              StreamObserver<GetSdkProfileEligibilityResponse> observer) {
            seenRequest.set(req);
            Status failure = rpcFailure.get();
            if (failure != null) observer.onError(failure.asRuntimeException());
            else observer.onNext(response.get());
            if (failure == null) observer.onCompleted();
          }
        }, new ServerInterceptor() {
          @Override
          public <ReqT, RespT> io.grpc.ServerCall.Listener<ReqT> interceptCall(
              ServerCall<ReqT, RespT> call, Metadata headers, ServerCallHandler<ReqT, RespT> next) {
            seenMethod.set(call.getMethodDescriptor().getFullMethodName());
            synchronized (authorizationHeaders) {
              authorizationHeaders.clear();
              var values = headers.getAll(AUTHORIZATION);
              if (values != null) values.forEach(authorizationHeaders::add);
              requestIds.clear();
              var ids = headers.getAll(REQUEST_ID);
              if (ids != null) ids.forEach(requestIds::add);
            }
            return next.startCall(call, headers);
          }
        })).build().start();
    channel = NettyChannelBuilder.forAddress("localhost", server.getPort())
        .sslContext(GrpcSslContexts.forClient().trustManager(resource("principal-tls/server-cert.pem")).build())
        .build();
  }

  @AfterEach
  void stopTlsUserListener() throws Exception {
    if (channel != null) channel.shutdownNow().awaitTermination(5, java.util.concurrent.TimeUnit.SECONDS);
    if (server != null) server.shutdownNow().awaitTermination(5, java.util.concurrent.TimeUnit.SECONDS);
  }

  @Test
  void eligibleReadUsesTlsExactRpcSingleBearerRequestIdAndGoCompatibleDeterministicProtoHash() throws Exception {
    Object issuer = issuer();
    Object client = client(issuer);

    SdkProfileEligibility.Profile actual = inspect(client);

    assertThat(actual).isEqualTo(new SdkProfileEligibility.Profile(accountId, profileId, 7, false, false));
    // gRPC Java exposes the full method without the canonical leading slash used in JWT binding.
    assertThat("/" + seenMethod.get()).isEqualTo(RPC);
    assertThat(seenRequest.get()).isEqualTo(GetSdkProfileEligibilityRequest.newBuilder()
        .setAccountId(accountId.toString()).setProfileId(profileId.toString()).build());
    assertThat(authorizationHeaders).hasSize(1).allSatisfy(value -> assertThat(value).startsWith("Bearer "));
    assertThat(requestIds).hasSize(1);
    assertThat(requestIds.getFirst()).matches("[0-9a-fA-F-]{36}");

    SignedJWT token = SignedJWT.parse(authorizationHeaders.getFirst().substring("Bearer ".length()));
    assertThat(token.getHeader().getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
    var claims = token.getJWTClaimsSet();
    assertThat(claims.getIssuer()).isEqualTo("auth");
    assertThat(claims.getSubject()).isEqualTo("service:auth");
    assertThat(claims.getAudience()).containsExactly("user");
    assertThat(claims.getClaim("principal_type")).isEqualTo("service");
    assertThat(claims.getClaim("rpc")).isEqualTo(RPC);
    assertThat(claims.getClaim("request_id")).isEqualTo(requestIds.getFirst());
    assertThat(claims.getClaim("request_hash")).isEqualTo("sha256:" + sha256(seenRequest.get().toByteArray()))
        .isEqualTo("sha256:755e97464a52e2dfddfdec444f215743dfe49e1da2f5719a08e2bb90b4ae3709");
    assertThat(claims.getExpirationTime().toInstant()).isEqualTo(NOW.plusSeconds(30));
  }

  @Test
  void malformedEligibilityResponsesAndUserFailuresMapToExistingDenial() throws Exception {
    Object client = client(issuer());
    for (GetSdkProfileEligibilityResponse invalid : List.of(
        eligibleResponse().toBuilder().setAccountId(UUID.randomUUID().toString()).build(),
        eligibleResponse().toBuilder().setProfileId(UUID.randomUUID().toString()).build(),
        eligibleResponse().toBuilder().setProfileRevision(0).build(),
        eligibleResponse().toBuilder().setDeleted(true).build(),
        eligibleResponse().toBuilder().setFrozen(true).build())) {
      response.set(invalid);
      assertDenied(client);
    }
    for (Status failure : List.of(Status.NOT_FOUND, Status.FAILED_PRECONDITION, Status.UNAVAILABLE,
        Status.DEADLINE_EXCEEDED)) {
      response.set(null);
      rpcFailure.set(failure);
      assertDenied(client);
    }
  }

  private void assertDenied(Object client) {
    assertThatThrownBy(() -> inspect(client)).isInstanceOf(SdkIdentityDeniedException.class)
        .hasMessage("invalid_sdk_identity");
  }

  private SdkProfileEligibility.Profile inspect(Object client) throws Exception {
    return (SdkProfileEligibility.Profile) invoke(client, "inspect",
        new Class<?>[] {UUID.class, UUID.class}, accountId, profileId);
  }

  private Object client(Object issuer) throws Exception {
    Class<?> type = implementationType("voice.backend.auth.sdkidentity.AuthUserProfileEligibilityClient");
    return type.getConstructor(UserServiceGrpc.UserServiceBlockingStub.class, issuer.getClass())
        .newInstance(UserServiceGrpc.newBlockingStub(channel), issuer);
  }

  private Object issuer() throws Exception {
    Path current = keys.resolve("current.pem");
    Path next = keys.resolve("next.pem");
    var generator = java.security.KeyPairGenerator.getInstance("RSA");
    generator.initialize(2048);
    for (Path file : List.of(current, next)) {
      byte[] der = generator.generateKeyPair().getPrivate().getEncoded();
      java.nio.file.Files.writeString(file, "-----BEGIN PRIVATE KEY-----\n"
          + java.util.Base64.getMimeEncoder(64, new byte[] {'\n'}).encodeToString(der)
          + "\n-----END PRIVATE KEY-----\n");
    }
    Class<?> type = implementationType("voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer");
    return type.getMethod("load", Path.class, String.class, Clock.class)
        .invoke(null, keys, "current", Clock.fixed(NOW, ZoneOffset.UTC));
  }

  private static Object invoke(Object target, String name, Class<?>[] parameterTypes, Object... args)
      throws Exception {
    try {
      return target.getClass().getMethod(name, parameterTypes).invoke(target, args);
    } catch (InvocationTargetException wrapped) {
      if (wrapped.getCause() instanceof Exception cause) throw cause;
      throw wrapped;
    }
  }

  private static Class<?> implementationType(String name) {
    try {
      return Class.forName(name);
    } catch (ClassNotFoundException absentImplementation) {
      return fail(name + " is not implemented yet", absentImplementation);
    }
  }

  private static GetSdkProfileEligibilityResponse eligibleResponse() {
    return GetSdkProfileEligibilityResponse.newBuilder().setAccountId(accountIdStatic())
        .setProfileId(profileIdStatic()).setProfileRevision(7).build();
  }

  private static String accountIdStatic() { return "0bd64a51-8720-4846-a4ea-3ff980a41d83"; }
  private static String profileIdStatic() { return "d371d30f-7059-46e7-9880-3fbc4c61f8e9"; }

  private static String sha256(byte[] value) throws Exception {
    return java.util.HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(value));
  }

  private static java.io.File resource(String name) throws Exception {
    return new ClassPathResource(name).getFile();
  }
}
