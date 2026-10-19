package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.AccountLifecycleReceiverServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import java.io.File;
import java.net.URI;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import org.springframework.core.env.Environment;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

/** Exact private endpoints; neither partial configuration nor local plaintext is supported. */
public record AuthLifecycleClientSettings(Map<String,AuthLifecycleClientSettings.Endpoint> endpoints) {
  private static final List<String> FIELDS=List.of("GRPC_ADDR","TLS_SERVER_NAME","TLS_CA_FILE","TLS_CERT_FILE","TLS_KEY_FILE");
  public AuthLifecycleClientSettings {
    endpoints=Map.copyOf(endpoints);
    if(!endpoints.keySet().equals(Set.of("messaging","moderation")))throw new IllegalArgumentException("both lifecycle endpoints required");
  }
  public record Endpoint(String address,String serverName,String caFile,String certFile,String keyFile) {
    public Endpoint {
      for(String value:List.of(address,serverName,caFile,certFile,keyFile))
        if(value.isBlank() || !value.equals(value.trim()))throw new IllegalArgumentException("invalid lifecycle TLS setting");
      try {
        var uri=new URI("tls://"+address);
        if(uri.getHost()==null || uri.getPort()<1 || uri.getPort()>65535 || uri.getUserInfo()!=null
            || !uri.getRawPath().isEmpty() || uri.getRawQuery()!=null || uri.getRawFragment()!=null)throw new IllegalArgumentException();
      }catch(Exception invalid){throw new IllegalArgumentException("exact private lifecycle host and port required",invalid);}
      if(!serverName.matches("[A-Za-z0-9][A-Za-z0-9.:-]*"))throw new IllegalArgumentException("exact lifecycle TLS server name required");
    }
  }
  public static Optional<AuthLifecycleClientSettings> load(Environment env) {
    boolean enabled=env.getProperty("auth.automod.lifecycle.enabled",Boolean.class,false);
    boolean any=false;for(String recipient:List.of("messaging","moderation"))for(String field:FIELDS)
      any|=env.containsProperty(prefix(recipient)+field);
    if(!enabled){if(any)throw new IllegalArgumentException("lifecycle endpoint settings require explicit enablement");return Optional.empty();}
    var endpoints=new LinkedHashMap<String,Endpoint>();
    for(String recipient:List.of("messaging","moderation")) {
      var values=FIELDS.stream().map(field->env.getProperty(prefix(recipient)+field,"")).toList();
      endpoints.put(recipient,new Endpoint(values.get(0),values.get(1),values.get(2),values.get(3),values.get(4)));
    }
    return Optional.of(new AuthLifecycleClientSettings(endpoints));
  }
  private static String prefix(String recipient){return "AUTH_AUTOMOD_LIFECYCLE_"+recipient.toUpperCase(java.util.Locale.ROOT)+"_";}
  public Runtime connect(AuthUserPrincipalIssuer issuer) {
    var channels=new LinkedHashMap<String,ManagedChannel>();
    try {
      for(var entry:endpoints.entrySet()) {
        var endpoint=entry.getValue();var tls=GrpcSslContexts.forClient().trustManager(new File(endpoint.caFile()))
            .keyManager(new File(endpoint.certFile()),new File(endpoint.keyFile())).protocols("TLSv1.3","TLSv1.2").build();
        channels.put(entry.getKey(),NettyChannelBuilder.forTarget(endpoint.address()).sslContext(tls).overrideAuthority(endpoint.serverName()).build());
      }
      return new Runtime(channels,issuer);
    }catch(Exception failure){channels.values().forEach(ManagedChannel::shutdownNow);throw new IllegalArgumentException("invalid lifecycle mutual TLS configuration",failure);}
  }
  public static final class Runtime implements AutoCloseable {
    private final Map<String,ManagedChannel> channels;private final AuthLifecycleGrpcClient client;
    Runtime(Map<String,ManagedChannel> channels,AuthUserPrincipalIssuer issuer) {
      this.channels=Map.copyOf(channels);var stubs=new LinkedHashMap<String,AccountLifecycleReceiverServiceGrpc.AccountLifecycleReceiverServiceBlockingStub>();
      channels.forEach((recipient,channel)->stubs.put(recipient,AccountLifecycleReceiverServiceGrpc.newBlockingStub(channel)));
      this.client=new AuthLifecycleGrpcClient(stubs,issuer);
    }
    public AuthLifecycleGrpcClient client(){return client;}
    @Override public void close(){channels.values().forEach(ManagedChannel::shutdownNow);for(var channel:channels.values())try{channel.awaitTermination(5,TimeUnit.SECONDS);}catch(InterruptedException interrupted){Thread.currentThread().interrupt();return;}}
  }
}
