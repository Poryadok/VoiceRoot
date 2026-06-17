package voice.backend.auth.grpc;

import io.grpc.Context;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import org.springframework.stereotype.Component;

@Component
public class AuthorizationServerInterceptor implements ServerInterceptor {
  static final Context.Key<String> AUTHORIZATION = Context.key("authorization");
  private static final Metadata.Key<String> AUTHORIZATION_KEY =
      Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);

  @Override
  public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
      ServerCall<ReqT, RespT> call, Metadata headers, ServerCallHandler<ReqT, RespT> next) {
    String authorization = headers.get(AUTHORIZATION_KEY);
    Context ctx = Context.current().withValue(AUTHORIZATION, authorization);
    return io.grpc.Contexts.interceptCall(ctx, call, headers, next);
  }
}
