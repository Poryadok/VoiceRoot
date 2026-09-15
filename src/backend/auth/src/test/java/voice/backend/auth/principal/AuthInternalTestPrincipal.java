package voice.backend.auth.principal;

import app.voice.auth.v1.AuthServiceGrpc;
import com.google.protobuf.MessageLite;
import io.grpc.Metadata;
import io.grpc.stub.MetadataUtils;

/** Signed in-process credentials for domain integration tests; keys exist only in memory. */
public final class AuthInternalTestPrincipal {
  private AuthInternalTestPrincipal() {}

  public static AuthPrincipalServerInterceptor interceptor() {
    return new AuthPrincipalServerInterceptor(new AuthPrincipalVerifierTest().verifier);
  }

  public static AuthServiceGrpc.AuthServiceBlockingStub client(
      AuthServiceGrpc.AuthServiceBlockingStub client, String issuer, String rpc, MessageLite request) {
    var headers = new Metadata();
    headers.put(AuthPrincipalServerInterceptorTest.AUTH,
        "Bearer " + AuthPrincipalVerifierTest.token(AuthInternalCallerBoundaryTest.claims(issuer, rpc, request)));
    headers.put(AuthPrincipalServerInterceptorTest.REQUEST_ID, "request-1");
    return client.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers));
  }
}
