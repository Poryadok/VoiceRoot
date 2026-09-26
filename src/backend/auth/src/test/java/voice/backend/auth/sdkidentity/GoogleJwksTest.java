package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;
import static org.junit.jupiter.api.Assertions.assertTimeoutPreemptively;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
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
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.util.List;
import java.util.concurrent.CompletionException;
import java.util.concurrent.Flow;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

class GoogleJwksTest {
  private static RSAKey firstKey;
  private static RSAKey secondKey;
  private final MutableClock clock = new MutableClock();
  private final HttpClient http = mock(HttpClient.class);
  private final Flow.Subscription responseSubscription = mock(Flow.Subscription.class);
  private GoogleJwks jwks;
  private int status = 200;
  private byte[] body;
  private boolean unavailable;
  private boolean stalledBody;

  @BeforeAll
  static void keys() throws Exception {
    firstKey = new RSAKeyGenerator(2048).keyID("google-key-1")
        .algorithm(JWSAlgorithm.RS256).keyUse(KeyUse.SIGNATURE).generate().toPublicJWK();
    secondKey = new RSAKeyGenerator(2048).keyID("google-key-2")
        .algorithm(JWSAlgorithm.RS256).keyUse(KeyUse.SIGNATURE).generate().toPublicJWK();
  }

  @BeforeEach
  void configure() throws Exception {
    body = json(firstKey);
    // Drive the real BodyHandler/BodySubscriber passed to HttpClient. This checks
    // bounded body handling without a socket or Google request.
    doAnswer(invocation -> {
      if (unavailable) throw new IOException("provider unavailable");
      HttpResponse.BodyHandler<?> handler = invocation.getArgument(1);
      HttpResponse.ResponseInfo info = mock(HttpResponse.ResponseInfo.class);
      when(info.statusCode()).thenReturn(status);
      var subscriber = handler.apply(info);
      subscriber.onSubscribe(responseSubscription);
      subscriber.onNext(List.of(ByteBuffer.wrap(body)));
      if (!stalledBody) subscriber.onComplete();
      Object received;
      try {
        received = subscriber.getBody().toCompletableFuture().join();
      } catch (CompletionException failure) {
        throw new IOException("response rejected", failure.getCause());
      }
      @SuppressWarnings("unchecked")
      HttpResponse<Object> response = mock(HttpResponse.class);
      when(response.statusCode()).thenReturn(status);
      when(response.body()).thenReturn(received);
      return response;
    }).when(http).send(any(HttpRequest.class), any(HttpResponse.BodyHandler.class));
    jwks = new GoogleJwks(clock, http);
  }

  private byte[] json(RSAKey key) {
    return new JWKSet(key).toString().getBytes(StandardCharsets.UTF_8);
  }

  private void requests(int count) throws Exception {
    verify(http, times(count)).send(any(HttpRequest.class), any(HttpResponse.BodyHandler.class));
  }

  @Test
  void onlyFetchesFixedGoogleEndpointWithBoundedRequestTimeout() throws Exception {
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    ArgumentCaptor<HttpRequest> request = ArgumentCaptor.forClass(HttpRequest.class);
    verify(http).send(request.capture(), any(HttpResponse.BodyHandler.class));
    assertThat(request.getValue().uri()).isEqualTo(URI.create("https://www.googleapis.com/oauth2/v3/certs"));
    assertThat(request.getValue().method()).isEqualTo("GET");
    assertThat(request.getValue().timeout()).contains(Duration.ofSeconds(3));
  }

  @Test
  void cachedKnownKeyExpiresAtFiveMinutesAndRefreshes() throws Exception {
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    clock.advance(299);
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    requests(1);
    body = json(secondKey);
    clock.advance(1);
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
    assertThat(jwks.apply(secondKey.getKeyID())).isEqualTo(secondKey);
    requests(2);
  }

  @Test
  void differentUnknownKidsShareThirtySecondRefreshThrottle() throws Exception {
    assertThat(jwks.apply("unknown-a")).isNull();
    for (int i = 0; i < 50; i++) assertThat(jwks.apply("unknown-" + i)).isNull();
    clock.advance(29);
    assertThat(jwks.apply("unknown-b")).isNull();
    requests(1);
    clock.advance(1);
    assertThat(jwks.apply("unknown-c")).isNull();
    requests(2);
  }

  @Test
  void unknownKeyRequestDoesNotPoisonKnownKeys() throws Exception {
    assertThat(jwks.apply("unknown")).isNull();
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    requests(1);
  }

  @Test
  void failedRefreshKeepsExistingCacheOnlyUntilOriginalExpiry() throws Exception {
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    clock.advance(30);
    unavailable = true;
    assertThat(jwks.apply("unknown")).isNull();
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    clock.advance(269);
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    requests(2);
    clock.advance(1);
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
    requests(3);
  }

  @Test
  void expiredCacheCannotBeUsedDuringRefreshThrottleAfterFailure() throws Exception {
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    clock.advance(290);
    unavailable = true;
    assertThat(jwks.apply("unknown")).isNull();
    clock.advance(10);
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
    requests(2);
  }

  @Test
  void failedFetchIsThrottledAndCanRecoverAfterThirtySeconds() throws Exception {
    unavailable = true;
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
    assertThat(jwks.apply("other")).isNull();
    unavailable = false;
    clock.advance(29);
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
    requests(1);
    clock.advance(1);
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    requests(2);
  }

  @Test
  void nonSuccessAndRedirectResponsesCannotSupplyKeys() throws Exception {
    for (int responseStatus : new int[] {301, 302, 403, 429, 500}) {
      status = responseStatus;
      assertThat(jwks.apply(firstKey.getKeyID())).isNull();
      clock.advance(30);
    }
    requests(5);
  }

  @Test
  void malformedResponsesFailClosed() throws Exception {
    for (String malformed : List.of("", "not-json", "{}", "{\"keys\":42}", "{\"keys\":[]}")) {
      body = malformed.getBytes(StandardCharsets.UTF_8);
      assertThat(jwks.apply(firstKey.getKeyID())).isNull();
      clock.advance(30);
    }
  }

  @Test
  void acceptsResponseAtSixtyFourKiBLimit() {
    String valid = new String(json(firstKey), StandardCharsets.UTF_8);
    body = (valid + " ".repeat(65536 - valid.length())).getBytes(StandardCharsets.UTF_8);
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
  }

  @Test
  void oversizedResponseIsRejectedAndSubscriptionCancelled() {
    String valid = new String(json(firstKey), StandardCharsets.UTF_8);
    body = (valid + " ".repeat(65537 - valid.length())).getBytes(StandardCharsets.UTF_8);
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
    verify(responseSubscription, atLeastOnce()).cancel();
  }

  @Test
  void oversizedResponseCannotReplaceValidCacheOrExtendItsExpiry() throws Exception {
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    clock.advance(30);
    body = new byte[65537];
    assertThat(jwks.apply("unknown")).isNull();
    assertThat(jwks.apply(firstKey.getKeyID())).isEqualTo(firstKey);
    clock.advance(270);
    assertThat(jwks.apply(firstKey.getKeyID())).isNull();
  }

  @Test
  void stalledResponseBodyTimesOutAndCancelsSubscription() {
    stalledBody = true;
    assertTimeoutPreemptively(Duration.ofSeconds(8),
        () -> assertThat(jwks.apply(firstKey.getKeyID())).isNull());
    verify(responseSubscription, timeout(1000).atLeastOnce()).cancel();
  }

  private static final class MutableClock extends Clock {
    private Instant current = Instant.parse("2026-09-26T12:00:00Z");
    void advance(long seconds) { current = current.plusSeconds(seconds); }
    @Override public ZoneId getZone() { return ZoneOffset.UTC; }
    @Override public Clock withZone(ZoneId zone) { return this; }
    @Override public Instant instant() { return current; }
  }
}
