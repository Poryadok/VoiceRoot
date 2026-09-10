package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import com.google.protobuf.Struct;
import com.google.protobuf.Value;
import io.grpc.*;
import io.grpc.protobuf.ProtoUtils;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;

class AuthPrincipalServerInterceptorTest {
  static final Metadata.Key<String> AUTH = Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
  static final Metadata.Key<String> REQUEST_ID = Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);
  final AuthPrincipalVerifierTest fixture = new AuthPrincipalVerifierTest();
  final Struct request = Struct.newBuilder().putFields("operation", Value.newBuilder().setStringValue("issue").build()).build();
  final AuthPrincipalServerInterceptor interceptor = new AuthPrincipalServerInterceptor(fixture.verifier);

  Metadata validHeaders() {
    var claims = AuthPrincipalVerifierTest.claims();
    return headers(claims);
  }

  Metadata headers(Map<String,Object> claims) {
    claims.put("request_hash", AuthPrincipalServerInterceptor.requestHash(request));
    Metadata headers = new Metadata();
    headers.put(AUTH, "Bearer " + AuthPrincipalVerifierTest.token(claims));
    headers.put(REQUEST_ID, "request-1");
    return headers;
  }

  @Test void acceptsSignedBoundRequestAndTracingMetadata() {
    Metadata headers = validHeaders();
    headers.put(Metadata.Key.of("traceparent", Metadata.ASCII_STRING_MARSHALLER), "trace-only");
    var call = new RecordingCall(AuthPrincipalServerInterceptor.ISSUE_RPC);
    var messages = new AtomicInteger();
    var halfCloses = new AtomicInteger();
    assertNull(VerifiedPrincipal.current());
    var listener = interceptor.interceptCall(call, headers, (c, h) -> {
      assertCurrentPrincipal();
      return new ServerCall.Listener<Struct>() {
        @Override public void onMessage(Struct message) {
          assertCurrentPrincipal(); assertEquals(request, message); messages.incrementAndGet();
        }
        @Override public void onHalfClose() { assertCurrentPrincipal(); halfCloses.incrementAndGet(); }
      };
    });
    assertNull(VerifiedPrincipal.current());
    listener.onMessage(request);
    assertNull(VerifiedPrincipal.current());
    listener.onHalfClose();
    assertNull(VerifiedPrincipal.current());
    assertNull(call.closed);
    assertEquals(1, messages.get());
    assertEquals(1, halfCloses.get());
  }

  void assertCurrentPrincipal() {
    var principal = VerifiedPrincipal.current();
    assertNotNull(principal);
    assertEquals(AuthPrincipalVerifierTest.ACCOUNT, principal.accountId());
    assertEquals(AuthPrincipalVerifierTest.PROFILE, principal.profileId());
    assertEquals("gateway", principal.issuer());
    assertEquals(2, principal.sessionEpoch());
  }

  @Test void verifiedButDisallowedCallerIsPermissionDeniedBeforeHandler() {
    var gatewayService = AuthPrincipalVerifierTest.claims();
    gatewayService.put("principal_type", "service"); gatewayService.put("sub", "service:gateway");
    gatewayService.remove("account_id"); gatewayService.remove("profile_id"); gatewayService.remove("session_epoch");
    assertDenied(interceptor, headers(gatewayService), request,
        AuthPrincipalServerInterceptor.ISSUE_RPC, Status.Code.PERMISSION_DENIED);

    var delegatedConsume = AuthPrincipalVerifierTest.claims();
    delegatedConsume.put("rpc", AuthPrincipalServerInterceptor.CONSUME_RPC);
    assertDenied(interceptor, headers(delegatedConsume), request,
        AuthPrincipalServerInterceptor.CONSUME_RPC, Status.Code.PERMISSION_DENIED);

    var spaceIssue = AuthPrincipalVerifierTest.claims();
    spaceIssue.put("iss", "space"); spaceIssue.put("principal_type", "service"); spaceIssue.put("sub", "service:space");
    spaceIssue.remove("account_id"); spaceIssue.remove("profile_id"); spaceIssue.remove("session_epoch");
    assertDenied(interceptor, headers(spaceIssue), request,
        AuthPrincipalServerInterceptor.ISSUE_RPC, Status.Code.PERMISSION_DENIED);
  }

  @Test void unknownIssuerIsUnauthenticatedBeforeHandler() {
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("iss", "unknown");
    assertDenied(interceptor, headers(claims), request);
  }

  @Test void duplicateAuthorityMetadataIsDeniedBeforeHandlerEvenWhenValuesMatch() {
    Metadata headers = validHeaders();
    headers.put(AUTH, headers.get(AUTH));
    assertDenied(interceptor, headers, request);
    headers = validHeaders();
    headers.put(REQUEST_ID, "request-1");
    assertDenied(interceptor, headers, request);
  }

  @Test void everyRawIdentityMetadataIsDeniedEvenAlongsideValidCredential() {
    for (String name : List.of("x-voice-account-id", "x-voice-profile-id", "x-voice-anything", "x-profile-id",
        "x-account-id", "x-user-id", "x-actor-id", "x-internal-caller")) {
      Metadata headers = validHeaders();
      headers.put(Metadata.Key.of(name, Metadata.ASCII_STRING_MARSHALLER), "untrusted");
      assertDenied(interceptor, headers, request);
    }
    Metadata headers = validHeaders();
    headers.put(Metadata.Key.of("x-voice-identity-bin", Metadata.BINARY_BYTE_MARSHALLER), new byte[] {1});
    assertDenied(interceptor, headers, request);
  }

  @Test void missingBlankAndNonBearerAuthorityIsDenied() {
    Metadata headers = validHeaders(); headers.removeAll(AUTH);
    assertDenied(interceptor, headers, request);
    headers = validHeaders(); headers.removeAll(REQUEST_ID);
    assertDenied(interceptor, headers, request);
    for (String id : List.of("", " ")) {
      headers = validHeaders(); headers.removeAll(REQUEST_ID); headers.put(REQUEST_ID, id);
      assertDenied(interceptor, headers, request);
    }
    for (String credential : List.of("", "Bearer ", "Basic abc", "bearer abc")) {
      headers = validHeaders(); headers.removeAll(AUTH); headers.put(AUTH, credential);
      assertDenied(interceptor, headers, request);
    }
  }

  @Test void wrongRequestBindingCannotReachHandler() {
    assertDenied(interceptor, validHeaders(), Struct.getDefaultInstance());
    Metadata headers = validHeaders(); headers.removeAll(REQUEST_ID); headers.put(REQUEST_ID, "other-request");
    assertDenied(interceptor, headers, request);
  }

  @Test void requestThatExpiresBeforeClientHalfCloseNeverCreatesHandler() {
    var clock = new AuthPrincipalJwksResolverTest.MutableClock();
    var verifier = new AuthPrincipalVerifier(
        (issuer, kid) -> (java.security.interfaces.RSAPublicKey) AuthPrincipalVerifierTest.KEY.getPublic(),
        account -> 2L, (issuer, jti, expires) -> {}, clock);
    var subject = new AuthPrincipalServerInterceptor(verifier);
    var call = new RecordingCall(AuthPrincipalServerInterceptor.ISSUE_RPC);
    var handlers = new AtomicInteger();
    var listener = subject.interceptCall(call, validHeaders(), (c, h) -> {
      handlers.incrementAndGet(); return new ServerCall.Listener<Struct>() {};
    });
    listener.onMessage(request);
    clock.advance(30);
    listener.onHalfClose();
    assertNotNull(call.closed);
    assertEquals(Status.Code.UNAUTHENTICATED, call.closed.getCode());
    assertEquals(0, handlers.get(), "a stalled unary request must not create a handler before final verification");
    assertNull(VerifiedPrincipal.current());
  }

  @Test void absentVerifierFailsClosedForAllThreeProtectedMethods() {
    var disabled = new AuthPrincipalServerInterceptor(null);
    for (String rpc : List.of(AuthPrincipalServerInterceptor.ISSUE_RPC, AuthPrincipalServerInterceptor.CONSUME_RPC, AuthPrincipalServerInterceptor.LOOKUP_RPC)) {
      var call = new RecordingCall(rpc);
      var handlers = new AtomicInteger();
      var listener = disabled.interceptCall(call, validHeaders(), (c, h) -> {
        handlers.incrementAndGet(); return new ServerCall.Listener<Struct>() {};
      });
      listener.onMessage(request); listener.onHalfClose();
      assertNotNull(call.closed); assertEquals(Status.Code.UNAUTHENTICATED, call.closed.getCode());
      assertEquals(0, handlers.get());
    }
  }

  @Test void lookupRejectsMissingDuplicateRawAndMismatchedAuthorityBeforeHandler() {
    String rpc = AuthPrincipalServerInterceptor.LOOKUP_RPC;
    var claims = AuthPrincipalVerifierTest.claims();
    claims.put("rpc", rpc); claims.put("iss", "space"); claims.put("sub", "service:space"); claims.put("principal_type", "service");
    claims.remove("account_id"); claims.remove("profile_id"); claims.remove("session_epoch");
    assertDenied(interceptor, new Metadata(), request, rpc, Status.Code.UNAUTHENTICATED);
    var duplicate = headers(claims); duplicate.put(AUTH, duplicate.get(AUTH));
    assertDenied(interceptor, duplicate, request, rpc, Status.Code.UNAUTHENTICATED);
    var raw = headers(claims);
    raw.put(Metadata.Key.of("x-account-id", Metadata.ASCII_STRING_MARSHALLER), AuthPrincipalVerifierTest.ACCOUNT.toString());
    assertDenied(interceptor, raw, request, rpc, Status.Code.UNAUTHENTICATED);
    assertDenied(interceptor, headers(claims), Struct.getDefaultInstance(), rpc, Status.Code.UNAUTHENTICATED);
    var delegated = AuthPrincipalVerifierTest.claims(); delegated.put("rpc", rpc);
    assertDenied(interceptor, headers(delegated), request, rpc, Status.Code.PERMISSION_DENIED);
  }
  @Test void unrelatedRpcPreservesExistingAuthenticationPath() {
    var call = new RecordingCall("/voice.auth.v1.AuthService/Login");
    var handlers = new AtomicInteger();
    var listener = new AuthPrincipalServerInterceptor(null).interceptCall(call, new Metadata(), (c, h) -> {
      handlers.incrementAndGet(); return new ServerCall.Listener<Struct>() {};
    });
    listener.onMessage(request); listener.onHalfClose();
    assertNull(call.closed); assertEquals(1, handlers.get());
  }

  void assertDenied(AuthPrincipalServerInterceptor subject, Metadata headers, Struct message) {
    assertDenied(subject, headers, message, AuthPrincipalServerInterceptor.ISSUE_RPC, Status.Code.UNAUTHENTICATED);
  }

  void assertDenied(AuthPrincipalServerInterceptor subject, Metadata headers, Struct message, String rpc, Status.Code code) {
    var call = new RecordingCall(rpc);
    var handlers = new AtomicInteger();
    var listener = subject.interceptCall(call, headers, (c, h) -> {
      handlers.incrementAndGet(); return new ServerCall.Listener<Struct>() {};
    });
    listener.onMessage(message); listener.onHalfClose();
    assertNotNull(call.closed, "invalid authority must close the call");
    assertEquals(code, call.closed.getCode());
    assertEquals(0, handlers.get(), "invalid authority must be rejected before handler creation");
    assertNull(VerifiedPrincipal.current(), "denied call must not leak a principal");
  }

  static final class RecordingCall extends ServerCall<Struct, Struct> {
    final MethodDescriptor<Struct, Struct> method;
    Status closed;
    RecordingCall(String rpc) {
      method = MethodDescriptor.<Struct, Struct>newBuilder().setType(MethodDescriptor.MethodType.UNARY)
          .setFullMethodName(rpc.startsWith("/") ? rpc.substring(1) : rpc)
          .setRequestMarshaller(ProtoUtils.marshaller(Struct.getDefaultInstance()))
          .setResponseMarshaller(ProtoUtils.marshaller(Struct.getDefaultInstance())).build();
    }
    @Override public void request(int count) {}
    @Override public void sendHeaders(Metadata headers) {}
    @Override public void sendMessage(Struct message) {}
    @Override public void close(Status status, Metadata trailers) { closed = status; }
    @Override public boolean isCancelled() { return false; }
    @Override public MethodDescriptor<Struct, Struct> getMethodDescriptor() { return method; }
  }
}
