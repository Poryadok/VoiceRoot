package voice.backend.auth.principal;

import com.google.protobuf.ByteString;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Proxy;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Function;
import javax.sql.DataSource;
import org.junit.jupiter.api.Test;
import voice.backend.auth.roleidentityauthority.*;
import voice.identityerasure.v1.*;

/** Focused protocol/owner-input proof; no PostgreSQL or independent CURRENT claim. */
public final class AuthParentHandlesAdapterTest {
  static final UUID HANDLE = UUID.fromString("00000000-0000-0000-0000-000000000001");
  static final String REQUEST = "00000000-0000-0000-0000-000000000002";
  static final OwnerCut CUT = OwnerCut.newBuilder().setOwner("auth")
      .setStreamId("00000000-0000-0000-0000-000000000003")
      .setHistoryGeneration("00000000-0000-0000-0000-000000000004")
      .setThroughRevision(8).setSelectorKeysetRevision(1).addSelectorKeyVersions("1")
      .setFactCount(2).setDigest(ByteString.copyFrom(new byte[32])).setComplete(true).build();
  static final ObservationWindow WINDOW = ObservationWindow.newBuilder()
      .setObservedAt(time(100)).setValidUntil(time(110)).build();

  static ObserveAccountHandlesRequest request() {
    return ObserveAccountHandlesRequest.newBuilder().setProtocolVersion(1).setRequestId(REQUEST)
        .addOwnerHandles(HANDLE.toString()).setRequiredAuthCut(CUT).build();
  }

  static Capture invoke(AuthParentIdentityGrpcAdapter service, ObserveAccountHandlesRequest request,
      String issuer, String requestId) {
    var capture = new Capture();
    Context context = Context.current().withValue(AuthPrincipalServerInterceptor.REQUEST_ID_CONTEXT, requestId);
    if (issuer != null) context = context.withValue(VerifiedPrincipal.CONTEXT,
        new VerifiedPrincipal(VerifiedPrincipal.SERVICE, issuer, null, null, 0));
    context.run(() -> service.observeAccountHandles(request, capture));
    return capture;
  }

  @Test void unverifiedAndWrongPurposeCallersAreDeniedBeforeAnyOwnerCall() {
    var service = new AuthParentIdentityGrpcAdapter(account -> { throw new AssertionError("raw lookup"); });
    for (String issuer : new String[] {null, "role", "space", "gateway"}) {
      require(code(invoke(service, request(), issuer, REQUEST)) == Status.Code.PERMISSION_DENIED,
          "inherited UNIMPLEMENTED is not a purpose-bound implementation");
    }
  }

  @Test void malformedBatchAndRequestBindingAreRejectedBeforeOwnerCall() {
    var service = new AuthParentIdentityGrpcAdapter(account -> { throw new AssertionError("raw lookup"); });
    var base = request();
    for (var invalid : List.of(base.toBuilder().setProtocolVersion(0).build(),
        base.toBuilder().addOwnerHandles(HANDLE.toString()).build(),
        base.toBuilder().setOwnerHandles(0, "invalid").build(),
        base.toBuilder().setRequiredAuthCut(CUT.toBuilder().setComplete(false)).build(),
        base.toBuilder().setRecovery(RecoveryBinding.newBuilder().setRestoreTarget("partial")).build())) {
      require(code(invoke(service, invalid, "user", REQUEST)) == Status.Code.INVALID_ARGUMENT, "invalid batch");
    }
    require(code(invoke(service, base, "user", UUID.randomUUID().toString())) == Status.Code.INVALID_ARGUMENT,
        "request binding");
  }

  @Test void completeCohortIsForwardedAndExactCutEchoedWithoutTruncation() throws Exception {
    var reads = new AtomicInteger();
    var request = request().toBuilder().clearOwnerHandles();
    for (int i = 1; i <= 137; i++) request.addOwnerHandles(new UUID(0, i).toString());
    var input = request.build();
    var service = fixture((handles, required, recovery) -> {
      reads.incrementAndGet();
      require(required.equals(CUT) && recovery == null, "required cut or recovery changed");
      require(handles.size() == 137, "cohort truncated");
      var facts = new ArrayList<AccountLifecycleFact>();
      for (UUID handle : handles) facts.add(AccountLifecycleFact.newBuilder().setOwnerHandle(handle.toString())
          .setLifecycleRevision(1).setState(AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_ACTIVE)
          .setEligibility(Eligibility.ELIGIBILITY_ELIGIBLE)
          .setDisposition(ErasureDisposition.ERASURE_DISPOSITION_NOT_IRREVERSIBLE).setOwnerCut(CUT).build());
      return observed(facts, CUT);
    });
    var capture = invoke(service, input, "user", REQUEST);
    require(capture.error == null && capture.completed && reads.get() == 1, "batch did not complete");
    require(capture.value.getAccountsCount() == 137 && capture.value.getOwnerCut().equals(CUT)
        && capture.value.getWindow().equals(WINDOW) && !capture.value.hasRecovery(), "bound response");
    require(capture.value.getRequestId().equals(REQUEST), "request echo");
    for (int i = 0; i < 137; i++) require(capture.value.getAccounts(i).getOwnerHandle()
        .equals(input.getOwnerHandles(i)), "handle omitted/reordered");
  }

  @Test void wrongOwnerOutputAndPrivateFailureAreCoarseUnavailable() throws Exception {
    var wrong = fixture((handles, cut, recovery) -> observed(List.of(AccountLifecycleFact.newBuilder()
        .setOwnerHandle(UUID.randomUUID().toString()).setOwnerCut(CUT).build()), CUT));
    require(code(invoke(wrong, request(), "user", REQUEST)) == Status.Code.UNAVAILABLE, "wrong binding");
    var failed = fixture((handles, cut, recovery) -> { throw new IllegalStateException("private fixture detail"); });
    var capture = invoke(failed, request(), "user", REQUEST);
    require(code(capture) == Status.Code.UNAVAILABLE && !capture.error.getMessage().contains("private fixture"),
        "private diagnostic leak");
  }

  @Test void recoveryWithoutIndependentAuthorityAndInvalidCutNeverAcquireDatabase() throws Exception {
    DataSource source = (DataSource) Proxy.newProxyInstance(getClass().getClassLoader(),
        new Class<?>[] {DataSource.class}, (proxy, method, args) -> { throw new AssertionError("DB touched"); });
    byte[] key = new byte[32]; key[0] = 1;
    var store = new AuthIdentityOwnerStore(source, new RoleIdentitySelectors(1, 1, Map.of(1, key)));
    var method = AuthIdentityOwnerStore.class.getMethod("observeAccountHandles", List.class, OwnerCut.class,
        RecoveryBinding.class);
    var recovery = RecoveryBinding.newBuilder().setRestoreTarget("fixture-target").setRestoreGeneration("fixture-gen")
        .setChallenge(ByteString.copyFromUtf8("challenge")).setRoleStreamId("fixture-role-stream")
        .setRoleThroughRevision(1).setRoleCheckpointDigest(ByteString.copyFrom(new byte[32]))
        .setRoleKeysetRevision("fixture-keyset").setInventoryGeneration(1)
        .setInventoryDigest(ByteString.copyFrom(new byte[32])).build();
    expectFailure(method, store, CUT, recovery, IllegalStateException.class);
    expectFailure(method, store, CUT.toBuilder().setDigest(ByteString.EMPTY).build(), null,
        IllegalArgumentException.class);
  }

  static void expectFailure(java.lang.reflect.Method method, Object store, OwnerCut cut,
      RecoveryBinding recovery, Class<?> expected) throws Exception {
    try { method.invoke(store, List.of(HANDLE), cut, recovery); throw new AssertionError("must refuse"); }
    catch (InvocationTargetException failure) { require(expected.isInstance(failure.getCause()), "wrong refusal"); }
  }

  interface BatchFixture { Object call(List<UUID> handles, OwnerCut cut, RecoveryBinding recovery) throws Exception; }
  static AuthParentIdentityGrpcAdapter fixture(BatchFixture fixture) throws Exception {
    Class<?> observer = Class.forName("voice.backend.auth.roleidentityauthority.AuthParentIdentityGrpcAdapter$HandleObserver");
    Object callback = Proxy.newProxyInstance(observer.getClassLoader(), new Class<?>[] {observer},
        (proxy, method, args) -> fixture.call((List<UUID>) args[0], (OwnerCut) args[1], (RecoveryBinding) args[2]));
    return (AuthParentIdentityGrpcAdapter) AuthParentIdentityGrpcAdapter.class
        .getConstructor(Function.class, observer).newInstance((Function<UUID, AuthIdentityOwnerStore.Observed>)
            account -> { throw new AssertionError("raw lookup"); }, callback);
  }
  static Object observed(List<AccountLifecycleFact> facts, OwnerCut cut) throws Exception {
    return Class.forName("voice.backend.auth.roleidentityauthority.AuthIdentityOwnerStore$ObservedHandles")
        .getConstructor(List.class, ObservationWindow.class, OwnerCut.class).newInstance(facts, WINDOW, cut);
  }
  static com.google.protobuf.Timestamp time(long seconds) {
    return com.google.protobuf.Timestamp.newBuilder().setSeconds(seconds).build();
  }
  static Status.Code code(Capture capture) { return Status.fromThrowable(capture.error).getCode(); }
  static void require(boolean ok, String message) { if (!ok) throw new AssertionError(message); }
  static final class Capture implements StreamObserver<ObserveAccountHandlesResponse> {
    ObserveAccountHandlesResponse value; Throwable error; boolean completed;
    public void onNext(ObserveAccountHandlesResponse value) { this.value = value; }
    public void onError(Throwable error) { this.error = error; }
    public void onCompleted() { completed = true; }
  }
  public static void main(String[] args) throws Exception {
    require(System.getProperty("os.name").equals("Linux"), "Linux required");
    var suite = new AuthParentHandlesAdapterTest();
    suite.unverifiedAndWrongPurposeCallersAreDeniedBeforeAnyOwnerCall();
    suite.malformedBatchAndRequestBindingAreRejectedBeforeOwnerCall();
    suite.completeCohortIsForwardedAndExactCutEchoedWithoutTruncation();
    suite.wrongOwnerOutputAndPrivateFailureAreCoarseUnavailable();
    suite.recoveryWithoutIndependentAuthorityAndInvalidCutNeverAcquireDatabase();
    System.out.println("PASS 5 ObserveAccountHandles protocol/input checks; no PostgreSQL/CURRENT claim");
  }
}
