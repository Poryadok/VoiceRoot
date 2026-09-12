package voice.backend.auth.principal;

import io.grpc.ServerInterceptors;
import io.grpc.ServerServiceDefinition;
import java.util.Set;

/** Separate service definitions prevent private proof methods entering the legacy auth path. */
public final class AuthPrincipalServices {
  private AuthPrincipalServices() {}

  public static ServerServiceDefinition legacyService(ServerServiceDefinition all) {
    Set<String> protectedDeletion = Set.of(
        AuthPrincipalServerInterceptor.SPACE_DELETE_CONSUME_RPC.substring(1),
        AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC.substring(1),
        AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC.substring(1));
    var legacy = ServerServiceDefinition.builder(all.getServiceDescriptor().getName());
    for (var method : all.getMethods()) {
      if (!protectedDeletion.contains(method.getMethodDescriptor().getFullMethodName())) {
        legacy.addMethod(method);
      }
    }
    return ServerInterceptors.intercept(legacy.build(), new AuthPrincipalServerInterceptor(null));
  }

  public static ServerServiceDefinition proofService(
      ServerServiceDefinition all, AuthPrincipalServerInterceptor verifier) {
    Set<String> ownership = Set.of(
        AuthPrincipalServerInterceptor.ISSUE_RPC.substring(1),
        AuthPrincipalServerInterceptor.CONSUME_RPC.substring(1),
        AuthPrincipalServerInterceptor.LOOKUP_RPC.substring(1));
    Set<String> deletion = Set.of(
        AuthPrincipalServerInterceptor.SPACE_DELETE_CONSUME_RPC.substring(1),
        AuthPrincipalServerInterceptor.SPACE_DELETE_LOOKUP_RPC.substring(1),
        AuthPrincipalServerInterceptor.SPACE_DELETE_ACK_RPC.substring(1));
    Set<String> present = all.getMethods().stream()
        .map(method -> method.getMethodDescriptor().getFullMethodName())
        .collect(java.util.stream.Collectors.toUnmodifiableSet());
    if (!present.containsAll(ownership)) {
      throw new IllegalArgumentException("Auth proof RPC implementation is incomplete");
    }
    boolean hasDeletionSurface = present.stream().anyMatch(name ->
        name.endsWith("SpaceDeletionProof") || name.endsWith("SpaceDeletionProofReceipt"));
    if (hasDeletionSurface && !present.containsAll(deletion)) {
      throw new IllegalArgumentException("Auth deletion proof RPC implementation is incomplete");
    }
    var proof = ServerServiceDefinition.builder(all.getServiceDescriptor().getName());
    int methods = 0;
    for (var method : all.getMethods()) {
      String name = method.getMethodDescriptor().getFullMethodName();
      if (ownership.contains(name) || (hasDeletionSurface && deletion.contains(name))) {
        proof.addMethod(method);
        methods++;
      }
    }
    int expected = ownership.size() + (hasDeletionSurface ? deletion.size() : 0);
    if (methods != expected) throw new IllegalArgumentException("Auth proof RPC implementation is incomplete");
    return ServerInterceptors.intercept(proof.build(), verifier);
  }
}
