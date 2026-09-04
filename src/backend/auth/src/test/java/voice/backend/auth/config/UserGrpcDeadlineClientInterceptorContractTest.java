package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.fail;

import app.voice.user.v1.UserServiceGrpc;
import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.Deadline;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;
import java.lang.reflect.Constructor;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

/** Contract for per-ClientCall deadline creation; a singleton stub deadline would expire. */
class UserGrpcDeadlineClientInterceptorContractTest {
  @Test
  void createsAFreshDeadlineForEveryCallAsTheTickerAdvances() {
    ManualTicker ticker = new ManualTicker();
    ClientInterceptor interceptor = newDeadlineInterceptor(Duration.ofSeconds(5), ticker);
    CapturingChannel channel = new CapturingChannel();

    interceptor.interceptCall(UserServiceGrpc.getEnsurePrimaryProfileMethod(), CallOptions.DEFAULT, channel);
    ticker.advance(Duration.ofSeconds(4));
    interceptor.interceptCall(UserServiceGrpc.getEnsurePrimaryProfileMethod(), CallOptions.DEFAULT, channel);

    assertThat(channel.deadlines).hasSize(2).allSatisfy(deadline -> assertThat(deadline).isNotNull());
    assertThat(channel.deadlines.get(0).timeRemaining(TimeUnit.SECONDS)).isEqualTo(1);
    assertThat(channel.deadlines.get(1).timeRemaining(TimeUnit.SECONDS)).isEqualTo(5);
  }

  private static ClientInterceptor newDeadlineInterceptor(Duration deadline, Deadline.Ticker ticker) {
    try {
      Class<?> type = Class.forName("voice.backend.auth.config.UserGrpcDeadlineClientInterceptor");
      Constructor<?> constructor = type.getDeclaredConstructor(Duration.class, Deadline.Ticker.class);
      constructor.setAccessible(true);
      return (ClientInterceptor) constructor.newInstance(deadline, ticker);
    } catch (ReflectiveOperationException ex) {
      fail("missing per-ClientCall User gRPC deadline interceptor", ex);
      throw new AssertionError("unreachable");
    }
  }

  private static final class ManualTicker extends Deadline.Ticker {
    private long now;

    @Override
    public long nanoTime() {
      return now;
    }

    void advance(Duration duration) {
      now += duration.toNanos();
    }
  }

  private static final class CapturingChannel extends Channel {
    final List<Deadline> deadlines = new ArrayList<>();

    @Override
    public <RequestT, ResponseT> ClientCall<RequestT, ResponseT> newCall(
        MethodDescriptor<RequestT, ResponseT> method, CallOptions callOptions) {
      deadlines.add(callOptions.getDeadline());
      return new NoopClientCall<>();
    }

    @Override
    public String authority() {
      return "test";
    }
  }

  private static final class NoopClientCall<ReqT, RespT> extends ClientCall<ReqT, RespT> {
    @Override public void start(Listener<RespT> responseListener, Metadata headers) {}
    @Override public void request(int numMessages) {}
    @Override public void cancel(String message, Throwable cause) {}
    @Override public void halfClose() {}
    @Override public void sendMessage(ReqT message) {}
  }
}
