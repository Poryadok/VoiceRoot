package voice.backend.auth.grpc;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.Status;
import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;

class RequestIdServerInterceptorTest {
  @Test
  void logsGrpcCallWithStructuredMdc() {
    Logger interceptorLogger =
        (Logger) LoggerFactory.getLogger(RequestIdServerInterceptor.class);
    ListAppender<ILoggingEvent> appender = new ListAppender<>();
    appender.start();
    interceptorLogger.addAppender(appender);
    interceptorLogger.setLevel(Level.INFO);

    try {
      @SuppressWarnings("unchecked")
      ServerCall<byte[], byte[]> call = mock(ServerCall.class);
      when(call.getMethodDescriptor()).thenReturn(pingMethod());

      Metadata headers = new Metadata();
      headers.put(
          Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER),
          "test-req-id");

      RequestIdServerInterceptor interceptor = new RequestIdServerInterceptor();
      AtomicReference<ServerCall<byte[], byte[]>> wrappedCall = new AtomicReference<>();
      ServerCallHandler<byte[], byte[]> next =
          (serverCall, ignoredHeaders) -> {
            wrappedCall.set(serverCall);
            return new ServerCall.Listener<byte[]>() {};
          };
      ServerCall.Listener<byte[]> listener = interceptor.interceptCall(call, headers, next);
      listener.onComplete();
      wrappedCall.get().close(Status.OK, new Metadata());

      assertThat(appender.list).hasSize(1);
      ILoggingEvent event = appender.list.get(0);
      assertThat(event.getFormattedMessage()).isEqualTo("grpc request");
      assertThat(event.getMDCPropertyMap()).containsEntry("event", "grpc_call");
      assertThat(event.getMDCPropertyMap()).containsEntry("grpc_method", "test.TestService/Ping");
      assertThat(event.getMDCPropertyMap()).containsEntry("grpc_code", "OK");
      assertThat(event.getMDCPropertyMap()).containsKey("duration_ms");
      assertThat(event.getMDCPropertyMap()).containsEntry("request_id", "test-req-id");
    } finally {
      interceptorLogger.detachAppender(appender);
    }
  }

  @Test
  void logsFailedRpcWithGrpcCodeAndError() {
    Logger interceptorLogger =
        (Logger) LoggerFactory.getLogger(RequestIdServerInterceptor.class);
    ListAppender<ILoggingEvent> appender = new ListAppender<>();
    appender.start();
    interceptorLogger.addAppender(appender);
    interceptorLogger.setLevel(Level.INFO);

    try {
      @SuppressWarnings("unchecked")
      ServerCall<byte[], byte[]> call = mock(ServerCall.class);
      when(call.getMethodDescriptor()).thenReturn(pingMethod());

      Metadata headers = new Metadata();
      RequestIdServerInterceptor interceptor = new RequestIdServerInterceptor();
      ServerCallHandler<byte[], byte[]> next =
          (serverCall, ignoredHeaders) ->
              new ServerCall.Listener<byte[]>() {
                @Override
                public void onHalfClose() {
                  serverCall.close(
                      Status.UNAUTHENTICATED.withDescription("invalid_credentials"),
                      new Metadata());
                }
              };
      ServerCall.Listener<byte[]> listener = interceptor.interceptCall(call, headers, next);
      listener.onHalfClose();

      assertThat(appender.list).hasSize(1);
      ILoggingEvent event = appender.list.get(0);
      assertThat(event.getMDCPropertyMap()).containsEntry("event", "grpc_call");
      assertThat(event.getMDCPropertyMap()).containsEntry("grpc_code", "UNAUTHENTICATED");
      assertThat(event.getMDCPropertyMap()).containsEntry("error", "invalid_credentials");
    } finally {
      interceptorLogger.detachAppender(appender);
    }
  }

  private static MethodDescriptor<byte[], byte[]> pingMethod() {
    MethodDescriptor.Marshaller<byte[]> marshaller =
        new MethodDescriptor.Marshaller<>() {
          @Override
          public InputStream stream(byte[] value) {
            return new ByteArrayInputStream(value == null ? new byte[0] : value);
          }

          @Override
          public byte[] parse(InputStream stream) {
            return new byte[0];
          }
        };
    return MethodDescriptor.<byte[], byte[]>newBuilder()
        .setType(MethodDescriptor.MethodType.UNARY)
        .setFullMethodName("test.TestService/Ping")
        .setRequestMarshaller(marshaller)
        .setResponseMarshaller(marshaller)
        .build();
  }
}
