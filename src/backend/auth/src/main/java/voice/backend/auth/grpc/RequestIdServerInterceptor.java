package voice.backend.auth.grpc;

import io.grpc.ForwardingServerCall;
import io.grpc.ForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.atomic.AtomicBoolean;
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
  private static final String[] GRPC_MDC_KEYS = {
    "event", "grpc_method", "grpc_code", "duration_ms", "error"
  };

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
    AtomicBoolean logged = new AtomicBoolean(false);

    ServerCall<ReqT, RespT> wrappedCall =
        new ForwardingServerCall.SimpleForwardingServerCall<>(call) {
          @Override
          public void close(Status status, Metadata trailers) {
            logOnce(logged, this, start, status);
            super.close(status, trailers);
          }
        };

    ServerCall.Listener<ReqT> delegate = next.startCall(wrappedCall, headers);
    return new ForwardingServerCallListener.SimpleForwardingServerCallListener<>(delegate) {
      @Override
      public void onCancel() {
        logOnce(logged, wrappedCall, start, Status.CANCELLED);
        try {
          super.onCancel();
        } finally {
          clearRequestMdc();
        }
      }
    };
  }

  private static void logOnce(
      AtomicBoolean logged, ServerCall<?, ?> call, Instant start, Status status) {
    if (!logged.compareAndSet(false, true)) {
      return;
    }
    logGrpcCall(call, start, status);
    clearRequestMdc();
  }

  private static void logGrpcCall(ServerCall<?, ?> call, Instant start, Status status) {
    long durationMs = Duration.between(start, Instant.now()).toMillis();
    MDC.put("event", "grpc_call");
    MDC.put("grpc_method", call.getMethodDescriptor().getFullMethodName());
    MDC.put("grpc_code", status.getCode().name());
    MDC.put("duration_ms", Long.toString(durationMs));
    if (!status.isOk()) {
      String description = status.getDescription();
      if (description != null && !description.isBlank()) {
        MDC.put("error", description);
      }
    }
    log.info("grpc request");
    clearGrpcMdc();
  }

  private static void clearGrpcMdc() {
    for (String key : GRPC_MDC_KEYS) {
      MDC.remove(key);
    }
  }

  private static void clearRequestMdc() {
    clearGrpcMdc();
    MDC.remove(RequestIdFilter.MDC_KEY);
  }
}
