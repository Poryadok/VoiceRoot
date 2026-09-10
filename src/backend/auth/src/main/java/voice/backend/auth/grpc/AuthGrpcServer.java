package voice.backend.auth.grpc;

import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.IOException;
import org.springframework.context.SmartLifecycle;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Component;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.principal.AuthPrincipalServices;
import voice.backend.auth.principal.AuthPrincipalTransport;

@Component
public class AuthGrpcServer implements SmartLifecycle {
  private final AuthGrpcService authGrpcService;
  private final AuthProperties properties;
  private final RequestIdServerInterceptor requestIdServerInterceptor;
  private final AuthorizationServerInterceptor authorizationServerInterceptor;
  private final AuthPrincipalServerInterceptor principalInterceptor;
  private final Environment environment;
  private Server server;
  private Server principalServer;
  private boolean running;

  public AuthGrpcServer(
      AuthGrpcService authGrpcService,
      AuthProperties properties,
      RequestIdServerInterceptor requestIdServerInterceptor,
      AuthorizationServerInterceptor authorizationServerInterceptor,
      AuthPrincipalServerInterceptor principalInterceptor,
      Environment environment) {
    this.authGrpcService = authGrpcService;
    this.properties = properties;
    this.requestIdServerInterceptor = requestIdServerInterceptor;
    this.authorizationServerInterceptor = authorizationServerInterceptor;
    this.principalInterceptor = principalInterceptor;
    this.environment = environment;
  }

  @Override
  public void start() {
    if (properties.getGrpc().getPort() < 0 || running) return;
    try {
      var definition = authGrpcService.bindService();
      int privatePort = principalInterceptor.enabled()
          ? AuthPrincipalTransport.port(environment, properties.getGrpc().getPort()) : 0;
      var privateBuilder = NettyServerBuilder.forPort(privatePort);
      AuthPrincipalTransport.configure(privateBuilder, environment, principalInterceptor.enabled());
      // Construct and validate both surfaces before binding either socket.
      if (principalInterceptor.enabled()) {
        principalServer = privateBuilder.intercept(requestIdServerInterceptor)
            .addService(AuthPrincipalServices.proofService(definition, principalInterceptor)).build();
      }
      server = NettyServerBuilder.forPort(properties.getGrpc().getPort())
          .intercept(requestIdServerInterceptor)
          .intercept(authorizationServerInterceptor)
          .addService(AuthPrincipalServices.legacyService(definition))
          .build();
      server.start();
      if (principalServer != null) principalServer.start();
      running = true;
    } catch (IOException | RuntimeException ex) {
      stop();
      throw new IllegalStateException("start auth grpc servers", ex);
    }
  }

  @Override
  public void stop() {
    if (principalServer != null) principalServer.shutdownNow();
    if (server != null) server.shutdownNow();
    running = false;
  }

  int legacyPort() { return server == null ? -1 : server.getPort(); }
  int principalPort() { return principalServer == null ? -1 : principalServer.getPort(); }

  @Override
  public boolean isRunning() { return running; }
}
