package voice.backend.auth.grpc;

import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.IOException;
import org.springframework.context.SmartLifecycle;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Component;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.beans.factory.annotation.Autowired;
import voice.backend.auth.moderationauthority.AuthModerationService;
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
  private final AuthModerationService moderationAuthority;
  private Server server;
  private Server principalServer;
  private Server sessionFloorServer;
  private Server moderationAuthorityServer;
  private boolean running;

  public AuthGrpcServer(
      AuthGrpcService authGrpcService,
      AuthProperties properties,
      RequestIdServerInterceptor requestIdServerInterceptor,
      AuthorizationServerInterceptor authorizationServerInterceptor,
      AuthPrincipalServerInterceptor principalInterceptor,
      Environment environment) {
    this(authGrpcService,properties,requestIdServerInterceptor,authorizationServerInterceptor,principalInterceptor,environment,null);
  }

  @Autowired
  public AuthGrpcServer(AuthGrpcService authGrpcService,AuthProperties properties,
      RequestIdServerInterceptor requestIdServerInterceptor,AuthorizationServerInterceptor authorizationServerInterceptor,
      AuthPrincipalServerInterceptor principalInterceptor,Environment environment,ObjectProvider<AuthModerationService> authority) {
    this.authGrpcService = authGrpcService;
    this.properties = properties;
    this.requestIdServerInterceptor = requestIdServerInterceptor;
    this.authorizationServerInterceptor = authorizationServerInterceptor;
    this.principalInterceptor = principalInterceptor;
    this.environment = environment;
    this.moderationAuthority = authority==null?null:authority.getIfAvailable();
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
      if(moderationAuthority!=null && !principalInterceptor.enabled()) {
        throw new IllegalArgumentException("Auth moderation authority requires request-bound principal verification");
      }
      // Construct and validate all surfaces before binding any socket.
      if (principalInterceptor.enabled()) {
        privateBuilder.intercept(requestIdServerInterceptor)
            .addService(AuthPrincipalServices.proofService(definition, principalInterceptor));
        principalServer = privateBuilder.build();
        int floorPort = AuthPrincipalTransport.sessionFloorPort(environment,
            properties.getGrpc().getPort(), privatePort);
        var floorBuilder = NettyServerBuilder.forPort(floorPort);
        AuthPrincipalTransport.configureSessionFloor(floorBuilder, environment, true);
        sessionFloorServer = floorBuilder.intercept(requestIdServerInterceptor)
            .addService(AuthPrincipalServices.sessionFloorService(definition, principalInterceptor)).build();
        if(moderationAuthority!=null) {
          int authorityPort=AuthPrincipalTransport.moderationAuthorityPort(environment,
              properties.getGrpc().getPort(),privatePort,floorPort);
          var authorityBuilder=NettyServerBuilder.forPort(authorityPort);
          AuthPrincipalTransport.configureModerationAuthority(authorityBuilder,environment);
          moderationAuthorityServer=authorityBuilder.intercept(requestIdServerInterceptor)
              .addService(io.grpc.ServerInterceptors.intercept(moderationAuthority,principalInterceptor)).build();
        }
      }
      server = NettyServerBuilder.forPort(properties.getGrpc().getPort())
          .intercept(requestIdServerInterceptor)
          .intercept(authorizationServerInterceptor)
          .addService(AuthPrincipalServices.legacyService(definition))
          .build();
      if (principalServer != null) principalServer.start();
      if (sessionFloorServer != null) sessionFloorServer.start();
      if (moderationAuthorityServer != null) moderationAuthorityServer.start();
      server.start();
      running = true;
    } catch (IOException | RuntimeException ex) {
      stop();
      throw new IllegalStateException("start auth grpc servers", ex);
    }
  }

  @Override
  public void stop() {
    if (principalServer != null) principalServer.shutdownNow();
    if (sessionFloorServer != null) sessionFloorServer.shutdownNow();
    if (moderationAuthorityServer != null) moderationAuthorityServer.shutdownNow();
    if (server != null) server.shutdownNow();
    running = false;
  }

  int legacyPort() { return server == null ? -1 : server.getPort(); }
  int principalPort() { return principalServer == null ? -1 : principalServer.getPort(); }
  int sessionFloorPort() { return sessionFloorServer == null ? -1 : sessionFloorServer.getPort(); }
  int moderationAuthorityPort() { return moderationAuthorityServer == null ? -1 : moderationAuthorityServer.getPort(); }

  @Override
  public boolean isRunning() { return running; }
}
