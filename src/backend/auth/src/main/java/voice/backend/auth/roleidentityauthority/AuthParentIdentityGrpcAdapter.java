package voice.backend.auth.roleidentityauthority;

import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.Objects;
import java.util.UUID;
import java.util.function.Function;
import java.util.List;
import java.util.ArrayList;
import java.util.HashSet;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.VerifiedPrincipal;
import voice.identityerasure.v1.AuthUserIdentityAuthorityServiceGrpc;
import voice.identityerasure.v1.ResolveAccountHandleRequest;
import voice.identityerasure.v1.ResolveAccountHandleResponse;
import voice.identityerasure.v1.ObserveAccountHandlesRequest;
import voice.identityerasure.v1.ObserveAccountHandlesResponse;
import voice.identityerasure.v1.OwnerCut;
import voice.identityerasure.v1.RecoveryBinding;

/** User-only parent observation. Register only behind the closed identity listener. */
public final class AuthParentIdentityGrpcAdapter
    extends AuthUserIdentityAuthorityServiceGrpc.AuthUserIdentityAuthorityServiceImplBase {
  private final Function<UUID, AuthIdentityOwnerStore.Observed> observer;
  @FunctionalInterface public interface HandleObserver {
    AuthIdentityOwnerStore.ObservedHandles observe(List<UUID> handles, OwnerCut requiredCut, RecoveryBinding recovery);
  }
  private final HandleObserver handleObserver;

  public AuthParentIdentityGrpcAdapter(AuthIdentityOwnerStore store) {
    this(Objects.requireNonNull(store)::resolveAccount, store::observeAccountHandles);
  }

  public AuthParentIdentityGrpcAdapter(Function<UUID, AuthIdentityOwnerStore.Observed> observer) {
    this(observer, null);
  }

  /** Trusted in-process owner injection, never supplied by a request. */
  public AuthParentIdentityGrpcAdapter(Function<UUID, AuthIdentityOwnerStore.Observed> observer,
      HandleObserver handleObserver) {
    this.observer = Objects.requireNonNull(observer);
    this.handleObserver = handleObserver;
  }

  @Override public void observeAccountHandles(ObserveAccountHandlesRequest request,
      StreamObserver<ObserveAccountHandlesResponse> response) {
    var principal = VerifiedPrincipal.current();
    if (principal == null || !VerifiedPrincipal.SERVICE.equals(principal.kind()) || !"user".equals(principal.issuer())) {
      response.onError(Status.PERMISSION_DENIED.withDescription("principal caller not permitted").asRuntimeException());
      return;
    }
    final List<UUID> handles;
    try {
      if (request.getProtocolVersion() != 1 || !request.getUnknownFields().asMap().isEmpty()
          || !request.getRequestId().equals(AuthPrincipalServerInterceptor.verifiedRequestId())) {
        throw new IllegalArgumentException();
      }
      canonicalId(request.getRequestId());
      var parsed = new ArrayList<UUID>();
      var seen = new HashSet<UUID>();
      for (String text : request.getOwnerHandlesList()) {
        UUID handle = canonicalId(text);
        if (!seen.add(handle)) throw new IllegalArgumentException();
        parsed.add(handle);
      }
      handles = List.copyOf(parsed);
      AuthIdentityOwnerStore.validateRequiredCut(request.hasRequiredAuthCut() ? request.getRequiredAuthCut() : null);
      if (request.hasRecovery()) AuthIdentityOwnerStore.validateRecoveryBinding(request.getRecovery());
    } catch (RuntimeException invalid) {
      response.onError(Status.INVALID_ARGUMENT.withDescription("invalid owner request").asRuntimeException());
      return;
    }
    final AuthIdentityOwnerStore.ObservedHandles observed;
    try {
      if (handleObserver == null) throw new IllegalStateException();
      observed = Objects.requireNonNull(handleObserver.observe(handles,
          request.hasRequiredAuthCut() ? request.getRequiredAuthCut() : null,
          request.hasRecovery() ? request.getRecovery() : null));
      if (observed.accounts().size() != handles.size()) throw new IllegalStateException();
      for (int i = 0; i < handles.size(); i++) {
        var fact = observed.accounts().get(i);
        if (!fact.getOwnerHandle().equals(handles.get(i).toString())
            || !fact.getOwnerCut().equals(observed.ownerCut())) throw new IllegalStateException();
      }
      // This source has no independently CURRENT recovery producer. Even a
      // faulty injected owner must not turn a supplied binding into an echo PASS.
      if (request.hasRecovery()) throw new IllegalStateException();
    } catch (RuntimeException unavailable) {
      response.onError(Status.UNAVAILABLE.withDescription("identity owner unavailable").asRuntimeException());
      return;
    }
    response.onNext(ObserveAccountHandlesResponse.newBuilder().setProtocolVersion(1)
        .setRequestId(request.getRequestId()).setWindow(observed.window())
        .addAllAccounts(observed.accounts()).setOwnerCut(observed.ownerCut()).build());
    response.onCompleted();
  }

  @Override public void resolveAccountHandle(ResolveAccountHandleRequest request,
      StreamObserver<ResolveAccountHandleResponse> response) {
    var principal = VerifiedPrincipal.current();
    if (principal == null || !VerifiedPrincipal.SERVICE.equals(principal.kind())
        || !"user".equals(principal.issuer())) {
      response.onError(Status.PERMISSION_DENIED.withDescription("principal caller not permitted").asRuntimeException());
      return;
    }
    final UUID account;
    try {
      if (request.getProtocolVersion() != 1 || !request.getUnknownFields().asMap().isEmpty()
          || !request.getRequestId().equals(AuthPrincipalServerInterceptor.verifiedRequestId())) {
        throw new IllegalArgumentException();
      }
      canonicalId(request.getRequestId());
      account = canonicalId(request.getAccountId());
    } catch (RuntimeException ex) {
      response.onError(Status.INVALID_ARGUMENT.withDescription("invalid owner request").asRuntimeException());
      return;
    }
    final AuthIdentityOwnerStore.Observed observed;
    try {
      observed = Objects.requireNonNull(observer.apply(account));
      if (!account.equals(observed.accountId()) || observed.lifecycle() == null || observed.window() == null) {
        throw new IllegalStateException();
      }
    } catch (RuntimeException ex) {
      // Never expose raw identity, database, keyset, or network diagnostic text.
      response.onError(Status.UNAVAILABLE.withDescription("identity owner unavailable").asRuntimeException());
      return;
    }
    response.onNext(ResolveAccountHandleResponse.newBuilder().setProtocolVersion(1)
        .setRequestId(request.getRequestId()).setAccountId(account.toString())
        .setWindow(observed.window()).setLifecycle(observed.lifecycle()).build());
    response.onCompleted();
  }

  private static UUID canonicalId(String value) {
    UUID id = UUID.fromString(value);
    if (!id.toString().equals(value) || id.equals(new UUID(0, 0))) throw new IllegalArgumentException();
    return id;
  }
}
