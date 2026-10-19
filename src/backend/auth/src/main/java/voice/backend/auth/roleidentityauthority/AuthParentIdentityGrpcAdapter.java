package voice.backend.auth.roleidentityauthority;

import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.Objects;
import java.util.UUID;
import java.util.function.Function;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.VerifiedPrincipal;
import voice.identityerasure.v1.AuthUserIdentityAuthorityServiceGrpc;
import voice.identityerasure.v1.ResolveAccountHandleRequest;
import voice.identityerasure.v1.ResolveAccountHandleResponse;

/** User-only parent observation. Register only behind the closed identity listener. */
public final class AuthParentIdentityGrpcAdapter
    extends AuthUserIdentityAuthorityServiceGrpc.AuthUserIdentityAuthorityServiceImplBase {
  private final Function<UUID, AuthIdentityOwnerStore.Observed> observer;

  public AuthParentIdentityGrpcAdapter(AuthIdentityOwnerStore store) {
    this(Objects.requireNonNull(store)::resolveAccount);
  }

  public AuthParentIdentityGrpcAdapter(Function<UUID, AuthIdentityOwnerStore.Observed> observer) {
    this.observer = Objects.requireNonNull(observer);
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
