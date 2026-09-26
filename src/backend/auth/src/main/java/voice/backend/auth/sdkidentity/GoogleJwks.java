package voice.backend.auth.sdkidentity;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.jwk.JWK;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.KeyOperation;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.Flow;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;

/** Fixed Google trust source with a bounded cache and a shared refresh budget. */
public final class GoogleJwks implements Function<String, RSAKey> {
  private static final URI URI_GOOGLE = URI.create("https://www.googleapis.com/oauth2/v3/certs");
  private static final Duration TIMEOUT = Duration.ofSeconds(3);
  private static final int MAX_BODY_BYTES = 64 * 1024;
  private final Clock clock;
  private final HttpClient client;
  private Map<String, RSAKey> keys = Map.of();
  private Instant fetchedAt;
  private Instant nextRefresh;

  public GoogleJwks(Clock clock) {
    this(clock, HttpClient.newBuilder().connectTimeout(TIMEOUT)
        .followRedirects(HttpClient.Redirect.NEVER).build());
  }

  GoogleJwks(Clock clock, HttpClient client) {
    this.clock = Objects.requireNonNull(clock);
    this.client = Objects.requireNonNull(client);
  }

  @Override
  public synchronized RSAKey apply(String kid) {
    if (kid == null || kid.isBlank() || kid.length() > 256) return null;
    Instant now = clock.instant();
    RSAKey cached = cached(kid, now);
    if (cached != null) return cached;
    if (nextRefresh != null && now.isBefore(nextRefresh)) return null;
    // Unknown kids and failures all share one budget; no attacker-controlled cache entries.
    nextRefresh = now.plusSeconds(30);
    try {
      HttpRequest request = HttpRequest.newBuilder(URI_GOOGLE).timeout(TIMEOUT)
          .header("Accept", "application/json").GET().build();
      HttpResponse<byte[]> response = client.send(request, info -> new BoundedBody());
      if (response.statusCode() != 200 || response.body() == null
          || response.body().length > MAX_BODY_BYTES) return null;
      Map<String, RSAKey> refreshed = new HashMap<>();
      for (JWK candidate : JWKSet.parse(new String(response.body(), StandardCharsets.UTF_8)).getKeys()) {
        if (!(candidate instanceof RSAKey key) || key.isPrivate() || key.size() < 2048
            || key.getKeyID() == null || key.getKeyID().isBlank() || key.getKeyID().length() > 256
            || (key.getAlgorithm() != null && !JWSAlgorithm.RS256.equals(key.getAlgorithm()))
            || (key.getKeyUse() != null && !KeyUse.SIGNATURE.equals(key.getKeyUse()))
            || (key.getKeyOperations() != null && !key.getKeyOperations().contains(KeyOperation.VERIFY))
            || refreshed.putIfAbsent(key.getKeyID(), key) != null) return null;
        key.toRSAPublicKey();
      }
      if (refreshed.isEmpty()) return null;
      keys = Map.copyOf(refreshed);
      // The TTL starts at request admission, so fetch latency never extends trust.
      fetchedAt = now;
      return cached(kid, clock.instant());
    } catch (InterruptedException ignored) {
      Thread.currentThread().interrupt();
      return null;
    } catch (Exception ignored) {
      // Retain last-good only until its original expiry; never expose transport details.
      return null;
    }
  }

  private RSAKey cached(String kid, Instant now) {
    if (fetchedAt == null || now.isBefore(fetchedAt)
        || !now.isBefore(fetchedAt.plusSeconds(300))) return null;
    return keys.get(kid);
  }

  /** Limits streamed allocation and elapsed body reception, including a stalled body. */
  private static final class BoundedBody implements HttpResponse.BodySubscriber<byte[]> {
    private final ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    private final CompletableFuture<byte[]> body = new CompletableFuture<>();
    private volatile Flow.Subscription subscription;

    BoundedBody() {
      body.orTimeout(TIMEOUT.toMillis(), TimeUnit.MILLISECONDS).whenComplete((result, failure) -> {
        if (failure != null && subscription != null) subscription.cancel();
      });
    }

    @Override public CompletionStage<byte[]> getBody() { return body; }

    @Override public void onSubscribe(Flow.Subscription incoming) {
      if (subscription != null) {
        incoming.cancel();
        return;
      }
      subscription = incoming;
      if (body.isDone()) incoming.cancel();
      else incoming.request(1);
    }

    @Override public void onNext(List<ByteBuffer> buffers) {
      if (body.isDone()) return;
      for (ByteBuffer buffer : buffers) {
        if (buffer.remaining() > MAX_BODY_BYTES - bytes.size()) {
          onError(new IOException("JWKS response too large"));
          return;
        }
        byte[] chunk = new byte[buffer.remaining()];
        buffer.get(chunk);
        bytes.writeBytes(chunk);
      }
      subscription.request(1);
    }

    @Override public void onError(Throwable failure) { body.completeExceptionally(failure); }
    @Override public void onComplete() { body.complete(bytes.toByteArray()); }
  }
}
