package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import app.voice.auth.v1.*;
import com.google.protobuf.MessageLite;
import io.grpc.*;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.MetadataUtils;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.OtpService;

class AuthInternalCallerBoundaryTest {
  static final String PHONE = "/voice.auth.v1.AuthService/ResolvePhoneHashes";
  static final String STATUS = "/voice.auth.v1.AuthService/SetAccountStatus";
  static final ResolvePhoneHashesRequest PHONE_REQUEST = ResolvePhoneHashesRequest.newBuilder()
      .addPhoneHashes("sha256-private-phone").build();
  static final SetAccountStatusRequest STATUS_REQUEST = SetAccountStatusRequest.newBuilder()
      .setAccountId("26bc045a-7b92-41d3-a520-2c3ce2c62bd1").setStatus("suspended")
      .setReason("moderation decision").build();
  final AuthPrincipalVerifierTest fixture = new AuthPrincipalVerifierTest();
  final AuthService auth = mock(AuthService.class);
  final AuthGrpcService service = new AuthGrpcService(auth, mock(OtpService.class));

  @Test void exactServiceCallerMatrixUsesFreshBoundCredentials() {
    for (var entry : Map.of(PHONE, "social", STATUS, "moderation").entrySet()) {
      String rpc = entry.getKey(), issuer = entry.getValue();
      var claims = claims(issuer, rpc, request(rpc));
      String token = AuthPrincipalVerifierTest.token(claims);
      var principal = fixture.verifier.verify(token, rpc, "request-1", hash(rpc));
      assertEquals(issuer, principal.issuer());
      assertEquals("service", principal.kind());
      assertNull(principal.accountId());
      assertCode(Status.Code.UNAUTHENTICATED,
          () -> fixture.verifier.verify(token, rpc, "request-1", hash(rpc)));
      for (String wrong : List.of("gateway", "space", "social", "moderation")) {
        if (wrong.equals(issuer)) continue;
        var denied = claims(wrong, rpc, request(rpc));
        assertCode(Status.Code.PERMISSION_DENIED, () -> fixture.verifier.verify(
            AuthPrincipalVerifierTest.token(denied), rpc, "request-1", hash(rpc)));
      }
      var delegated = AuthPrincipalVerifierTest.claims();
      delegated.put("rpc", rpc); delegated.put("request_hash", hash(rpc));
      assertCode(Status.Code.PERMISSION_DENIED, () -> fixture.verifier.verify(
          AuthPrincipalVerifierTest.token(delegated), rpc, "request-1", hash(rpc)));
    }
  }

  @Test void bothInternalMethodsDenyUnsignedRawDuplicateAndChangedRequestsBeforeDomainWork() throws Exception {
    try (var endpoint = endpoint(false)) {
      for (String rpc : List.of(PHONE, STATUS)) {
        String issuer = rpc.equals(PHONE) ? "social" : "moderation";
        assertCode(Status.Code.UNAUTHENTICATED, () -> invoke(endpoint.client, rpc));
        Metadata raw = new Metadata();
        raw.put(Metadata.Key.of("x-voice-internal", Metadata.ASCII_STRING_MARSHALLER), "true");
        assertCode(Status.Code.UNAUTHENTICATED, () -> invoke(attach(endpoint.client, raw), rpc));
        Metadata signedRaw = headers(issuer, rpc, request(rpc));
        signedRaw.put(Metadata.Key.of("x-voice-internal", Metadata.ASCII_STRING_MARSHALLER), "true");
        assertCode(Status.Code.UNAUTHENTICATED, () -> invoke(attach(endpoint.client, signedRaw), rpc));
        Metadata duplicate = headers(issuer, rpc, request(rpc));
        duplicate.put(AuthPrincipalServerInterceptorTest.AUTH, duplicate.get(AuthPrincipalServerInterceptorTest.AUTH));
        assertCode(Status.Code.UNAUTHENTICATED, () -> invoke(attach(endpoint.client, duplicate), rpc));
        Metadata changed = headers(issuer, rpc, rpc.equals(PHONE)
            ? PHONE_REQUEST.toBuilder().addPhoneHashes("other").build()
            : STATUS_REQUEST.toBuilder().setStatus("active").build());
        assertCode(Status.Code.UNAUTHENTICATED, () -> invoke(attach(endpoint.client, changed), rpc));
      }
      verifyNoInteractions(auth);
    }
  }

  @Test void signedSocialLookupAndModerationStatusReachRealAdapter() throws Exception {
    when(auth.resolvePhoneHashes(PHONE_REQUEST.getPhoneHashesList()))
        .thenReturn(Map.of("sha256-private-phone", "profile-result"));
    try (var endpoint = endpoint(false)) {
      var social = attach(endpoint.client, headers("social", PHONE, PHONE_REQUEST));
      var result = social.resolvePhoneHashes(PHONE_REQUEST);
      assertEquals("profile-result", result.getMatches(0).getProfileId());
      attach(endpoint.client, headers("moderation", STATUS, STATUS_REQUEST)).setAccountStatus(STATUS_REQUEST);
      verify(auth).resolvePhoneHashes(PHONE_REQUEST.getPhoneHashesList());
      verify(auth).setAccountStatus(STATUS_REQUEST.getAccountId(), "suspended");
      verifyNoMoreInteractions(auth);
    }
  }

  @Test void legacyListenerRejectsEvenSignedInternalCallers() throws Exception {
    try (var endpoint = endpoint(true)) {
      for (String rpc : List.of(PHONE, STATUS)) {
        var signed = attach(endpoint.client, headers(rpc.equals(PHONE) ? "social" : "moderation", rpc, request(rpc)));
        assertCode(Status.Code.UNAUTHENTICATED, () -> invoke(signed, rpc));
      }
      verifyNoInteractions(auth);
    }
  }

  @Test void bindingChecksApplyToNewCallerIssuers() {
    for (String rpc : List.of(PHONE, STATUS)) {
      for (String field : List.of("aud", "rpc", "request_id", "request_hash", "sub")) {
        var claims = claims(rpc.equals(PHONE) ? "social" : "moderation", rpc, request(rpc));
        claims.put(field, "wrong");
        assertCode(Status.Code.UNAUTHENTICATED, () -> fixture.verifier.verify(
            AuthPrincipalVerifierTest.token(claims), rpc, "request-1", hash(rpc)));
      }
    }
  }

  @Test void bareAdapterRequiresVerifiedPrincipalAndRejectsWrongCaller() {
    var phone = new RecordingObserver<ResolvePhoneHashesResponse>();
    service.resolvePhoneHashes(PHONE_REQUEST, phone);
    assertNotNull(phone.error);
    assertEquals(Status.Code.UNAUTHENTICATED, Status.fromThrowable(phone.error).getCode());
    var status = new RecordingObserver<SetAccountStatusResponse>();
    service.setAccountStatus(STATUS_REQUEST, status);
    assertNotNull(status.error);
    assertEquals(Status.Code.UNAUTHENTICATED, Status.fromThrowable(status.error).getCode());
    var wrong = new RecordingObserver<SetAccountStatusResponse>();
    Context.current().withValue(VerifiedPrincipal.CONTEXT,
        new VerifiedPrincipal("service", "social", null, null, 0))
        .run(() -> service.setAccountStatus(STATUS_REQUEST, wrong));
    assertNotNull(wrong.error);
    assertEquals(Status.Code.PERMISSION_DENIED, Status.fromThrowable(wrong.error).getCode());
    var wrongPhone = new RecordingObserver<ResolvePhoneHashesResponse>();
    Context.current().withValue(VerifiedPrincipal.CONTEXT,
        new VerifiedPrincipal("service", "moderation", null, null, 0))
        .run(() -> service.resolvePhoneHashes(PHONE_REQUEST, wrongPhone));
    assertNotNull(wrongPhone.error);
    assertEquals(Status.Code.PERMISSION_DENIED, Status.fromThrowable(wrongPhone.error).getCode());
    verifyNoInteractions(auth);
  }

  @Test void productionPrivateSurfaceContainsOnlyProtectedMethods() {
    var definition = AuthPrincipalServices.proofService(service.bindService(), new AuthPrincipalServerInterceptor(fixture.verifier));
    var names = definition.getMethods().stream().map(method -> "/" + method.getMethodDescriptor().getFullMethodName())
        .collect(java.util.stream.Collectors.toSet());
    assertEquals(java.util.Set.of(PHONE, STATUS,
        AuthPrincipalServerInterceptor.ISSUE_RPC, AuthPrincipalServerInterceptor.CONSUME_RPC,
        AuthPrincipalServerInterceptor.LOOKUP_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_CONSUME_RPC,
        AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC, AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC), names);
  }

  static final class RecordingObserver<T> implements io.grpc.stub.StreamObserver<T> {
    Throwable error;
    public void onNext(T response) {}
    public void onError(Throwable failure) { error = failure; }
    public void onCompleted() {}
  }

  Endpoint endpoint(boolean legacy) throws Exception {
    var definition = legacy ? AuthPrincipalServices.legacyService(service.bindService())
        : AuthPrincipalServices.proofService(service.bindService(), new AuthPrincipalServerInterceptor(fixture.verifier));
    String name = InProcessServerBuilder.generateName();
    Server server = InProcessServerBuilder.forName(name).directExecutor().addService(definition).build().start();
    ManagedChannel channel = InProcessChannelBuilder.forName(name).directExecutor().build();
    return new Endpoint(server, channel, AuthServiceGrpc.newBlockingStub(channel).withDeadlineAfter(3, TimeUnit.SECONDS));
  }
  Metadata headers(String issuer, String rpc, MessageLite message) {
    Metadata metadata = new Metadata();
    metadata.put(AuthPrincipalServerInterceptorTest.AUTH, "Bearer " + AuthPrincipalVerifierTest.token(claims(issuer, rpc, message)));
    metadata.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, "request-1");
    return metadata;
  }
  static Map<String,Object> claims(String issuer, String rpc, MessageLite message) {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("iss", issuer); claims.put("sub", "service:" + issuer); claims.put("principal_type", "service");
    claims.put("rpc", rpc); claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(message));
    claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    return claims;
  }
  static MessageLite request(String rpc) { return rpc.equals(PHONE) ? PHONE_REQUEST : STATUS_REQUEST; }
  static String hash(String rpc) { return AuthPrincipalServerInterceptor.requestHash(request(rpc)); }
  static AuthServiceGrpc.AuthServiceBlockingStub attach(AuthServiceGrpc.AuthServiceBlockingStub client, Metadata headers) {
    return client.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));
  }
  static void invoke(AuthServiceGrpc.AuthServiceBlockingStub client, String rpc) {
    if (rpc.equals(PHONE)) client.resolvePhoneHashes(PHONE_REQUEST); else client.setAccountStatus(STATUS_REQUEST);
  }
  static void assertCode(Status.Code code, org.junit.jupiter.api.function.Executable action) {
    assertEquals(code, assertThrows(StatusRuntimeException.class, action).getStatus().getCode());
  }
  record Endpoint(Server server, ManagedChannel channel, AuthServiceGrpc.AuthServiceBlockingStub client) implements AutoCloseable {
    public void close() throws Exception {
      channel.shutdownNow(); server.shutdownNow();
      channel.awaitTermination(5, TimeUnit.SECONDS); server.awaitTermination(5, TimeUnit.SECONDS);
    }
  }
}
