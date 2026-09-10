package voice.backend.auth.principal;

import io.grpc.ServerInterceptors;
import io.grpc.ServerServiceDefinition;
import java.util.Set;

/** Separate service definitions prevent private proof methods entering the legacy auth path. */
public final class AuthPrincipalServices {
  private AuthPrincipalServices() {}

  public static ServerServiceDefinition legacyService(ServerServiceDefinition all) {
    return ServerInterceptors.intercept(all, new AuthPrincipalServerInterceptor(null));
  }

  public static ServerServiceDefinition proofService(
      ServerServiceDefinition all, AuthPrincipalServerInterceptor verifier) {
    Set<String> allowed = Set.of(
        AuthPrincipalServerInterceptor.ISSUE_RPC.substring(1),
        AuthPrincipalServerInterceptor.CONSUME_RPC.substring(1),
        AuthPrincipalServerInterceptor.LOOKUP_RPC.substring(1));
    var proof = ServerServiceDefinition.builder(all.getServiceDescriptor().getName());
    int methods = 0;
    for (var method : all.getMethods()) {
      if (allowed.contains(method.getMethodDescriptor().getFullMethodName())) {
        proof.addMethod(method);
        methods++;
      }
    }
    if (methods != allowed.size()) throw new IllegalArgumentException("Auth proof RPC implementation is incomplete");
    return ServerInterceptors.intercept(proof.build(), verifier);
  }
}
