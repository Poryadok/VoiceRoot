package voice.backend.auth.config;

import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.Deadline;
import io.grpc.MethodDescriptor;
import java.time.Duration;
import java.util.Objects;
import java.util.concurrent.TimeUnit;

/** Creates a new relative deadline when each Auth-to-User client call starts. */
final class UserGrpcDeadlineClientInterceptor implements ClientInterceptor {
  private final Duration deadline;
  private final Deadline.Ticker ticker;

  UserGrpcDeadlineClientInterceptor(Duration deadline) {
    this(deadline, Deadline.getSystemTicker());
  }

  UserGrpcDeadlineClientInterceptor(Duration deadline, Deadline.Ticker ticker) {
    this.deadline = requirePositive(deadline);
    this.ticker = Objects.requireNonNull(ticker, "ticker");
  }

  @Override
  public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
      MethodDescriptor<ReqT, RespT> method, CallOptions callOptions, Channel next) {
    Deadline callDeadline =
        Deadline.after(deadline.toNanos(), TimeUnit.NANOSECONDS, ticker);
    return next.newCall(method, callOptions.withDeadline(callDeadline));
  }

  private static Duration requirePositive(Duration value) {
    if (value == null || value.isZero() || value.isNegative()) {
      throw new IllegalArgumentException("auth.user-grpc.deadline must be positive");
    }
    return value;
  }
}
