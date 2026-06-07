package voice.backend.auth.grpc;

import io.grpc.ForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import java.time.Duration;
import java.time.Instant;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.stereotype.Component;
import voice.backend.auth.web.RequestIdFilter;

@Component
public class RequestIdServerInterceptor implements ServerInterceptor {
  private static final Logger log = LoggerFactory.getLogger(RequestIdServerInterceptor.class);
  private static final Metadata.Key<String> REQUEST_ID_KEY =
      Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);

  @Override
  public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
      ServerCall<ReqT, RespT> call,
      Metadata headers,
      ServerCallHandler<ReqT, RespT> next) {
    String requestId = headers.get(REQUEST_ID_KEY);
    if (requestId == null || requestId.isBlank()) {
      requestId = RequestIdFilter.generateRequestId();
    }
    MDC.put(RequestIdFilter.MDC_KEY, requestId);
    Instant start = Instant.now();
    ServerCall.Listener<ReqT> delegate = next.startCall(call, headers);
    return new ForwardingServerCallListener.SimpleForwardingServerCallListener<>(delegate) {
      @Override
      public void onComplete() {
        logGrpcCall(call, start, Status.OK);
        try {
          super.onComplete();
        } finally {
          MDC.remove(RequestIdFilter.MDC_KEY);
        }
      }

      @Override
      public void onCancel() {
        logGrpcCall(call, start, Status.CANCELLED);
        try {
          super.onCancel();
        } finally {
          MDC.remove(RequestIdFilter.MDC_KEY);
        }
      }
    };
  }

  private static void logGrpcCall(ServerCall<?, ?> call, Instant start, Status status) {
    long durationMs = Duration.between(start, Instant.now()).toMillis();
    log.info(
        "grpc request method={} grpc_code={} duration_ms={} event=grpc_call",
        call.getMethodDescriptor().getFullMethodName(),
        status.getCode(),
        durationMs);
  }
}
