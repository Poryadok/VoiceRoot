package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.*;
import io.grpc.Context;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.stub.MetadataUtils;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import voice.backend.auth.principal.AuthPrincipalServerInterceptor;
import voice.backend.auth.sdkidentity.AuthUserPrincipalIssuer;

public final class AuthLifecycleGrpcClient implements AuthLifecyclePublisher.Transport {
  private final Map<String,AccountLifecycleReceiverServiceGrpc.AccountLifecycleReceiverServiceBlockingStub> stubs;
  private final AuthUserPrincipalIssuer issuer;
  public AuthLifecycleGrpcClient(Map<String,AccountLifecycleReceiverServiceGrpc.AccountLifecycleReceiverServiceBlockingStub> stubs,AuthUserPrincipalIssuer issuer) {
    this.stubs=Map.copyOf(stubs);this.issuer=Objects.requireNonNull(issuer);
    if(!this.stubs.keySet().equals(Set.of("messaging","moderation")))throw new IllegalArgumentException("both exact lifecycle recipients required");
  }
  @Override public ApplyLifecycleFactResponse send(AuthLifecycleDeliveryStore.Delivery delivery) {
    if(TransactionSynchronizationManager.isActualTransactionActive() || Thread.holdsLock(this))throw new IllegalStateException("lifecycle owner network call under lock");
    long remaining=TimeUnit.SECONDS.toNanos(2);
    if(Context.current().getDeadline()!=null)remaining=Math.min(remaining,Context.current().getDeadline().timeRemaining(TimeUnit.NANOSECONDS));
    if(remaining<=0 || Context.current().isCancelled())throw Status.DEADLINE_EXCEEDED.asRuntimeException();
    var request=ApplyLifecycleFactRequest.newBuilder().setFact(delivery.fact()).build();String id=UUID.randomUUID().toString();
    var headers=new Metadata();headers.put(Metadata.Key.of("authorization",Metadata.ASCII_STRING_MARSHALLER),
        "Bearer "+issuer.issueAccountLifecycle(delivery.recipient(),id,AuthPrincipalServerInterceptor.requestHash(request)));
    headers.put(Metadata.Key.of("x-request-id",Metadata.ASCII_STRING_MARSHALLER),id);
    var response=stubs.get(delivery.recipient()).withDeadlineAfter(remaining,TimeUnit.NANOSECONDS)
        .withMaxInboundMessageSize(AuthLifecycleCodec.MAX_BYTES).withMaxOutboundMessageSize(2048)
        .withInterceptors(MetadataUtils.newAttachHeadersInterceptor(headers)).applyLifecycleFact(request);
    try {AuthLifecycleCodec.validateReceipt(delivery.fact(),response);}
    catch(IllegalArgumentException invalid){throw new AuthLifecycleDeliveryStore.RestoreDivergence("malformed lifecycle receiver receipt",invalid);}
    return response;
  }
}
