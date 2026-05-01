package voice.backend.auth.grpc;

import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.IOException;
import org.springframework.context.SmartLifecycle;
import org.springframework.stereotype.Component;
import voice.backend.auth.config.AuthProperties;

@Component
public class AuthGrpcServer implements SmartLifecycle {
  private final AuthGrpcService authGrpcService;
  private final AuthProperties properties;
  private Server server;
  private boolean running;

  public AuthGrpcServer(AuthGrpcService authGrpcService, AuthProperties properties) {
    this.authGrpcService = authGrpcService;
    this.properties = properties;
  }

  @Override
  public void start() {
    if (properties.getGrpc().getPort() < 0) {
      return;
    }
    try {
      server = NettyServerBuilder.forPort(properties.getGrpc().getPort())
          .addService(authGrpcService)
          .build()
          .start();
      running = true;
    } catch (IOException ex) {
      throw new IllegalStateException("start auth grpc server", ex);
    }
  }

  @Override
  public void stop() {
    if (server != null) {
      server.shutdownNow();
    }
    running = false;
  }

  @Override
  public boolean isRunning() {
    return running;
  }
}
