package voice.backend.auth.roleidentityauthority;

import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContextBuilder;
import java.util.Objects;

/** Required mutual TLS for this dedicated capability, including local/test listeners. */
public final class IdentityAuthorityTransport {
  private IdentityAuthorityTransport() {}

  public static void configure(NettyServerBuilder builder, IdentityAuthorityTransportConfig config) {
    Objects.requireNonNull(builder);
    Objects.requireNonNull(config);
    try {
      builder.sslContext(GrpcSslContexts.configure(SslContextBuilder.forServer(
              config.certificate().toFile(), config.key().toFile()))
          .trustManager(config.clientCa().toFile()).clientAuth(ClientAuth.REQUIRE)
          .protocols("TLSv1.3", "TLSv1.2").build());
      builder.maxInboundMessageSize(1 << 20);
    } catch (Exception ex) {
      throw new IllegalArgumentException("invalid identity authority mutual TLS configuration");
    }
  }
}
