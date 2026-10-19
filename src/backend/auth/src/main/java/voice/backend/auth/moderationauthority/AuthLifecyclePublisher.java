package voice.backend.auth.moderationauthority;

import app.voice.automod.v1.ApplyLifecycleFactResponse;
import io.grpc.StatusRuntimeException;
import java.util.Objects;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/** One finite pass with a separate budget for each recipient; counts are not a convergence seal. */
public final class AuthLifecyclePublisher {
  @FunctionalInterface public interface Transport { ApplyLifecycleFactResponse send(AuthLifecycleDeliveryStore.Delivery delivery); }
  public record Batch(int attempted,int acknowledged,int superseded,int retryable) {}
  private final AuthLifecycleDeliveryStore store;private final Transport transport;private final Runnable authorityCheck;
  public AuthLifecyclePublisher(AuthLifecycleDeliveryStore store,Transport transport,Runnable authorityCheck) {
    this.store=Objects.requireNonNull(store);this.transport=Objects.requireNonNull(transport);this.authorityCheck=Objects.requireNonNull(authorityCheck);
  }
  public Batch runOnce(int perRecipientLimit) {
    if(TransactionSynchronizationManager.isActualTransactionActive())throw new IllegalStateException("lifecycle network call under transaction");
    authorityCheck.run();
    var pending=new java.util.ArrayList<AuthLifecycleDeliveryStore.Delivery>();
    // One unavailable peer cannot fill the healthy peer's selection budget.
    pending.addAll(store.pendingForRecipient("messaging",perRecipientLimit));
    pending.addAll(store.pendingForRecipient("moderation",perRecipientLimit));
    int acknowledged=0,superseded=0,retryable=0;
    for(var delivery:pending) {
      authorityCheck.run();ApplyLifecycleFactResponse response;
      try {response=transport.send(delivery);}
      catch(StatusRuntimeException unavailable){retryable++;continue;}
      authorityCheck.run();
      if(store.acknowledge(delivery,response,authorityCheck)==AuthLifecycleDeliveryStore.Outcome.ACKED)acknowledged++;else superseded++;
    }
    return new Batch(pending.size(),acknowledged,superseded,retryable);
  }
}
