package voice.backend.auth.principal;

import static org.junit.jupiter.api.Assertions.*;

import io.grpc.Context;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import voice.backend.auth.roleidentityauthority.AuthIdentityOwnerStore;
import voice.backend.auth.roleidentityauthority.AuthParentIdentityGrpcAdapter;
import voice.identityerasure.v1.*;

class AuthParentAdapterTest {
  static final String REQUEST = UUID.randomUUID().toString();
  static final UUID ACCOUNT = UUID.randomUUID();
  final AtomicInteger reads = new AtomicInteger();
  final AuthParentIdentityGrpcAdapter service = new AuthParentIdentityGrpcAdapter(account -> {
    reads.incrementAndGet();
    return new AuthIdentityOwnerStore.Observed(account,
        AccountLifecycleFact.newBuilder().setOwnerHandle(UUID.randomUUID().toString()).build(),
        ObservationWindow.getDefaultInstance());
  });
  ResolveAccountHandleRequest request() {
    return ResolveAccountHandleRequest.newBuilder().setProtocolVersion(1).setRequestId(REQUEST)
        .setAccountId(ACCOUNT.toString()).build();
  }
  Capture invoke(ResolveAccountHandleRequest request, String issuer, String requestId) {
    var capture = new Capture();
    Context context = Context.current().withValue(AuthPrincipalServerInterceptor.REQUEST_ID_CONTEXT, requestId);
    if (issuer != null) context = context.withValue(VerifiedPrincipal.CONTEXT,
        new VerifiedPrincipal(VerifiedPrincipal.SERVICE, issuer, null, null, 0));
    context.run(() -> service.resolveAccountHandle(request, capture));
    return capture;
  }
  @Test void deniesUnverifiedOrRoleCallerBeforeReadingOwnerState() {
    for (String issuer : new String[] {null, "role", "space"}) {
      var capture = invoke(request(), issuer, REQUEST);
      assertEquals(Status.Code.PERMISSION_DENIED, Status.fromThrowable(capture.error).getCode());
    }
    assertEquals(0, reads.get());
  }
  @Test void rejectsProtocolUnknownFieldsSelectorAndRequestIdMismatchBeforeReading() {
    var base = request();
    for (var invalid : java.util.List.of(
        base.toBuilder().setProtocolVersion(0).build(), base.toBuilder().setRequestId("request-1").build(),
        base.toBuilder().setAccountId("00000000-0000-0000-0000-000000000000").build(),
        base.toBuilder().setAccountId("invalid").build(),
        base.toBuilder().setUnknownFields(com.google.protobuf.UnknownFieldSet.newBuilder()
            .addField(99, com.google.protobuf.UnknownFieldSet.Field.newBuilder().addVarint(1).build()).build()).build())) {
      assertEquals(Status.Code.INVALID_ARGUMENT, Status.fromThrowable(invoke(invalid, "user", REQUEST).error).getCode());
    }
    assertEquals(Status.Code.INVALID_ARGUMENT,
        Status.fromThrowable(invoke(base, "user", UUID.randomUUID().toString()).error).getCode());
    assertEquals(0, reads.get());
  }
  @Test void echoesOnlyExactRequestedAccountAndBoundObservation() {
    var capture = invoke(request(), "user", REQUEST);
    assertNull(capture.error); assertTrue(capture.completed); assertNotNull(capture.value);
    assertEquals(1, capture.value.getProtocolVersion()); assertEquals(REQUEST, capture.value.getRequestId());
    assertEquals(ACCOUNT.toString(), capture.value.getAccountId()); assertEquals(1, reads.get());
  }
  static final class Capture implements StreamObserver<ResolveAccountHandleResponse> {
    ResolveAccountHandleResponse value; Throwable error; boolean completed;
    public void onNext(ResolveAccountHandleResponse value) { this.value = value; }
    public void onError(Throwable error) { this.error = error; }
    public void onCompleted() { completed = true; }
  }
}
