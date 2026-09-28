package voice.backend.auth.sdkidentity;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.Flow;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Authenticated, response-verified read of Game Integration's current SDK policy. */
public final class SdkGameIntegrationPolicyClient implements SdkAuthorizationPolicy, SdkBindingChallengeAuthority,
    SdkGameIntegrationExecutionPermitAuthority {
  private static final int MAX_RESPONSE_BYTES = 65_536;
  private static final Duration EXECUTION_PERMIT_TIMEOUT = Duration.ofSeconds(1);
  private static final ObjectMapper JSON = new ObjectMapper()
      .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
      .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
  private static final Set<String> RESPONSE_FIELDS = Set.of("application_id", "environment_id", "revision",
      "display_name", "redirect_uris", "allowed_origins", "providers", "player_scopes");
  private static final Set<String> BINDING_CHALLENGE_FIELDS = Set.of("challenge_id", "nonce", "application_id",
      "environment_id", "provider", "redirect_uri_sha256", "pkce_challenge", "device_key_id",
      "device_key_thumbprint", "operation_id", "expires_at", "status", "source_account_id", "source_actor_id",
      "source_device_id", "source_generation", "target_account_id", "target_profile_id", "profile_revision",
      "consent_revision", "policy_revision", "scopes");
  private static final Set<String> BINDING_CHALLENGE_CREATE_RESPONSE_FIELDS =
      Set.of("challenge_id", "nonce", "expires_at");
  private static final Set<String> EXECUTION_PERMIT_FIELDS = Set.of("permit_id", "binding_id", "application_id",
      "environment_id", "binding_revision", "assertion_jti", "operation_id", "expires_at");
  private static final Set<String> EXECUTION_COMPLETION_FIELDS = Set.of("permit_id", "operation_id", "outcome", "status");


  private final String configuredBaseUrl;
  private final String configuredKey;
  private final boolean allowInternalHttp;
  private final Clock clock;
  private final HttpClient http;
  private final Duration requestTimeout;

  public SdkGameIntegrationPolicyClient(String baseUrl, String keyBase64, boolean allowInternalHttp, Clock clock) {
    this(baseUrl, keyBase64, allowInternalHttp, clock,
        HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2))
            .followRedirects(HttpClient.Redirect.NEVER).build(), Duration.ofSeconds(2));
  }

  SdkGameIntegrationPolicyClient(String baseUrl, String keyBase64, boolean allowInternalHttp,
      Clock clock, HttpClient http, Duration requestTimeout) {
    this.configuredBaseUrl = baseUrl;
    this.configuredKey = keyBase64;
    this.allowInternalHttp = allowInternalHttp;
    this.clock = clock;
    this.http = http;
    this.requestTimeout = requestTimeout;
  }

  @Override
  public Policy resolve(UUID applicationId, UUID environmentId) {
    return resolve(applicationId, environmentId, requestTimeout);
  }

  @Override
  public Policy resolveForExecutionPermit(UUID applicationId, UUID environmentId) {
    return resolve(applicationId, environmentId, Duration.ofMillis(250));
  }

  private Policy resolve(UUID applicationId, UUID environmentId, Duration callTimeout) {
    try {
      if (applicationId == null || environmentId == null || clock == null || http == null
          || requestTimeout == null || requestTimeout.isZero() || requestTimeout.isNegative()) throw denied();
      URI endpoint = endpoint(configuredBaseUrl, environmentId, allowInternalHttp);
      byte[] key = key(configuredKey);
      String path = endpoint.getRawPath();
      String timestamp = Long.toString(clock.instant().getEpochSecond());
      String nonce = UUID.randomUUID().toString();
      HttpRequest request = HttpRequest.newBuilder(endpoint)
          .timeout(callTimeout)
          .header("X-Voice-Workload", "auth")
          .header("X-Voice-Timestamp", timestamp)
          .header("X-Voice-Nonce", nonce)
          .header("X-Voice-Signature", requestSignature(key, path, timestamp, nonce))
          .GET().build();
      HttpResponse<byte[]> response = http.send(request,
          responseInfo -> new BoundedBodySubscriber(MAX_RESPONSE_BYTES));
      if (response.statusCode() != 200) throw denied();
      requireHeader(response, "Cache-Control", "no-store");
      requireHeader(response, "Content-Type", "application/json");
      requireHeader(response, "X-Voice-Response-Timestamp", timestamp);
      requireHeader(response, "X-Voice-Response-Nonce", nonce);
      String signature = uniqueHeader(response, "X-Voice-Response-Signature");
      byte[] raw = response.body();
      verifyResponse(key, path, timestamp, nonce, signature, raw);
      return parsePolicy(raw, applicationId, environmentId);
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw denied();
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception invalidOrUnavailable) {
      throw denied();
    }
  }

  /** Authenticated execution-time read of a GIS-owned immutable binding challenge. */
  public SdkBindingChallengeAuthority.Challenge resolveBindingChallenge(UUID challengeId) {
    try {
      if (challengeId == null || challengeId.equals(new UUID(0, 0)) || clock == null || http == null
          || requestTimeout == null || requestTimeout.isZero() || requestTimeout.isNegative()) throw denied();
      String rawPath = "/internal/v1/bindings/challenges/" + challengeId;
      URI endpoint = endpointPath(configuredBaseUrl, rawPath, allowInternalHttp);
      byte[] key = key(configuredKey);
      String path = endpoint.getRawPath();
      String timestamp = Long.toString(clock.instant().getEpochSecond());
      String nonce = UUID.randomUUID().toString();
      HttpRequest request = HttpRequest.newBuilder(endpoint).timeout(requestTimeout)
          .header("X-Voice-Workload", "auth").header("X-Voice-Timestamp", timestamp)
          .header("X-Voice-Nonce", nonce).header("X-Voice-Signature", requestSignature(key, path, timestamp, nonce))
          .GET().build();
      HttpResponse<byte[]> response = http.send(request,
          responseInfo -> new BoundedBodySubscriber(MAX_RESPONSE_BYTES));
      if (response.statusCode() != 200) throw denied();
      requireHeader(response, "Cache-Control", "no-store");
      requireHeader(response, "Content-Type", "application/json");
      requireHeader(response, "X-Voice-Response-Timestamp", timestamp);
      requireHeader(response, "X-Voice-Response-Nonce", nonce);
      verifyResponse(key, path, timestamp, nonce, uniqueHeader(response, "X-Voice-Response-Signature"), response.body());
      return parseBindingChallenge(response.body(), challengeId);
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw denied();
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception invalidOrUnavailable) {
      throw denied();
    }
  }

  @Override
  public SdkBindingChallengeAuthority.Challenge createBindingChallenge(SdkBindingChallengeAuthority.CreateRequest value) {
    try {
      requireCreateRequest(value);
      URI endpoint = endpointPath(configuredBaseUrl, "/internal/v1/bindings/challenges", allowInternalHttp);
      byte[] key = key(configuredKey);
      var node = JSON.createObjectNode();
      node.put("application_id", value.applicationId().toString());
      node.put("environment_id", value.environmentId().toString());
      node.put("provider", value.provider());
      node.put("redirect_uri_sha256", value.redirectUriSha256());
      node.put("pkce_challenge", value.pkceChallenge());
      node.put("device_key_id", value.deviceKeyId().toString());
      node.put("device_key_thumbprint", value.deviceKeyThumbprint());
      node.put("operation_id", value.operationId().toString());
      node.put("expires_at", java.time.format.DateTimeFormatter.ISO_INSTANT.format(value.expiresAt()));
      node.put("source_account_id", value.sourceAccountId().toString());
      node.put("source_actor_id", value.sourceActorId().toString());
      node.put("source_device_id", value.sourceDeviceId().toString());
      node.put("source_generation", value.sourceGeneration());
      node.put("target_account_id", value.targetAccountId().toString());
      node.put("target_profile_id", value.targetProfileId().toString());
      node.put("profile_revision", value.profileRevision());
      node.put("consent_revision", value.consentRevision());
      node.put("policy_revision", value.policyRevision());
      var scopeNode = node.putArray("scopes");
      value.scopes().forEach(scopeNode::add);
      byte[] body = JSON.writeValueAsBytes(node);
      String path = endpoint.getRawPath();
      String timestamp = Long.toString(clock.instant().getEpochSecond());
      String nonce = UUID.randomUUID().toString();
      HttpRequest request = HttpRequest.newBuilder(endpoint).timeout(requestTimeout)
          .header("X-Voice-Workload", "auth").header("Content-Type", "application/json")
          .header("X-Voice-Timestamp", timestamp).header("X-Voice-Nonce", nonce)
          .header("X-Voice-Signature", bodyRequestSignature(key, "POST", path, timestamp, nonce, body))
          .POST(HttpRequest.BodyPublishers.ofByteArray(body)).build();
      HttpResponse<byte[]> response = http.send(request, info -> new BoundedBodySubscriber(MAX_RESPONSE_BYTES));
      if (response.statusCode() != 200) throw denied();
      requireHeader(response, "Cache-Control", "no-store");
      requireHeader(response, "Content-Type", "application/json");
      requireHeader(response, "X-Voice-Response-Timestamp", timestamp);
      requireHeader(response, "X-Voice-Response-Nonce", nonce);
      verifyResponse(key, path, timestamp, nonce, uniqueHeader(response, "X-Voice-Response-Signature"), response.body());
      return parseCreatedBindingChallenge(response.body(), value);
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw denied();
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception invalidOrUnavailable) {
      throw denied();
    }
  }

  @Override
  public SdkGameIntegrationExecutionPermitAuthority.Permit issue(UUID bindingId, UUID operationId,
      String deviceAuthorityAssertion) {
    try {
      if (bindingId == null || operationId == null || deviceAuthorityAssertion == null
          || deviceAuthorityAssertion.isBlank() || deviceAuthorityAssertion.length() > 16_384 || clock == null
          || http == null || requestTimeout == null || requestTimeout.isZero() || requestTimeout.isNegative()) throw denied();
      String rawPath = "/internal/v1/game-integrations/bindings/" + bindingId + "/execution-permits";
      URI endpoint = endpointPath(configuredBaseUrl, rawPath, allowInternalHttp);
      byte[] key = key(configuredKey);
      var node = JSON.createObjectNode().put("operation_id", operationId.toString());
      byte[] body = JSON.writeValueAsBytes(node);
      String path = endpoint.getRawPath();
      String timestamp = Long.toString(clock.instant().getEpochSecond());
      String nonce = UUID.randomUUID().toString();
      String signature = executionPermitRequestSignature(key, path, timestamp, nonce, body, deviceAuthorityAssertion);
      HttpRequest request = HttpRequest.newBuilder(endpoint).timeout(EXECUTION_PERMIT_TIMEOUT)
          .header("X-Voice-Workload", "auth").header("X-Voice-Workload-Version", "2")
          .header("X-Voice-Timestamp", timestamp).header("X-Voice-Nonce", nonce)
          .header("X-Voice-Signature", signature).header("Content-Type", "application/json")
          .header("X-Voice-Device-Authority", deviceAuthorityAssertion)
          .POST(HttpRequest.BodyPublishers.ofByteArray(body)).build();
      HttpResponse<byte[]> response = http.send(request, info -> new BoundedBodySubscriber(MAX_RESPONSE_BYTES));
      if (response.statusCode() != 200) throw denied();
      verifySignedResponse(response, path, timestamp, nonce, key);
      return parseExecutionPermit(response.body(), bindingId, operationId, deviceAuthorityAssertion, clock.instant());
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw denied();
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception invalidOrUnavailable) {
      throw denied();
    }
  }

  @Override
  public SdkGameIntegrationExecutionPermitAuthority.Completion complete(UUID permitId, UUID operationId, String outcome) {
    try {
      if (permitId == null || operationId == null || !Set.of("committed", "aborted").contains(outcome)
          || clock == null || http == null || requestTimeout == null || requestTimeout.isZero()
          || requestTimeout.isNegative()) throw denied();
      String rawPath = "/internal/v1/game-integrations/execution-permits/" + permitId + "/completion";
      URI endpoint = endpointPath(configuredBaseUrl, rawPath, allowInternalHttp);
      byte[] key = key(configuredKey);
      var node = JSON.createObjectNode().put("operation_id", operationId.toString()).put("outcome", outcome);
      byte[] body = JSON.writeValueAsBytes(node);
      String path = endpoint.getRawPath();
      String timestamp = Long.toString(clock.instant().getEpochSecond());
      String nonce = UUID.randomUUID().toString();
      HttpRequest request = HttpRequest.newBuilder(endpoint).timeout(EXECUTION_PERMIT_TIMEOUT)
          .header("X-Voice-Workload", "auth").header("X-Voice-Timestamp", timestamp)
          .header("X-Voice-Nonce", nonce)
          .header("X-Voice-Signature", bodyRequestSignature(key, "POST", path, timestamp, nonce, body))
          .header("Content-Type", "application/json").POST(HttpRequest.BodyPublishers.ofByteArray(body)).build();
      HttpResponse<byte[]> response = http.send(request, info -> new BoundedBodySubscriber(MAX_RESPONSE_BYTES));
      if (response.statusCode() != 200) throw denied();
      verifySignedResponse(response, path, timestamp, nonce, key);
      return parseExecutionCompletion(response.body(), permitId, operationId, outcome);
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw denied();
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception invalidOrUnavailable) {
      throw denied();
    }
  }

  private void verifySignedResponse(HttpResponse<byte[]> response, String path, String timestamp, String nonce, byte[] key) {
    requireHeader(response, "Cache-Control", "no-store");
    requireHeader(response, "Content-Type", "application/json");
    requireHeader(response, "X-Voice-Response-Timestamp", timestamp);
    requireHeader(response, "X-Voice-Response-Nonce", nonce);
    verifyResponse(key, path, timestamp, nonce, uniqueHeader(response, "X-Voice-Response-Signature"), response.body());
  }

  private static SdkGameIntegrationExecutionPermitAuthority.Permit parseExecutionPermit(byte[] raw, UUID bindingId,
      UUID operationId, String assertion, Instant now) {
    try {
      JsonNode root = JSON.readTree(raw);
      if (root == null || !root.isObject()) throw denied();
      Set<String> fields = new HashSet<>();
      root.fieldNames().forEachRemaining(fields::add);
      if (!fields.equals(EXECUTION_PERMIT_FIELDS)) throw denied();
      UUID permit = canonicalUuid(text(root, "permit_id"));
      UUID binding = canonicalUuid(text(root, "binding_id"));
      UUID app = canonicalUuid(text(root, "application_id"));
      UUID env = canonicalUuid(text(root, "environment_id"));
      long revision = positiveLong(root, "binding_revision");
      UUID assertionJti = assertionJti(assertion);
      if (!binding.equals(bindingId) || !operationId.equals(canonicalUuid(text(root, "operation_id")))
          || !assertionJti.equals(canonicalUuid(text(root, "assertion_jti")))) throw denied();
      Instant expires = Instant.parse(text(root, "expires_at"));
      if (!expires.isAfter(now.plusMillis(500)) || expires.isAfter(now.plusMillis(3750))) throw denied();
      return new SdkGameIntegrationExecutionPermitAuthority.Permit(permit, binding, app, env, revision,
          assertionJti, operationId, expires);
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception malformed) { throw denied(); }
  }

  private static SdkGameIntegrationExecutionPermitAuthority.Completion parseExecutionCompletion(byte[] raw,
      UUID permitId, UUID operationId, String outcome) {
    try {
      JsonNode root = JSON.readTree(raw);
      if (root == null || !root.isObject()) throw denied();
      Set<String> fields = new HashSet<>();
      root.fieldNames().forEachRemaining(fields::add);
      if (!fields.equals(EXECUTION_COMPLETION_FIELDS)) throw denied();
      UUID permit = canonicalUuid(text(root, "permit_id"));
      UUID operation = canonicalUuid(text(root, "operation_id"));
      String resultOutcome = text(root, "outcome");
      String status = text(root, "status");
      if (!permitId.equals(permit) || !operationId.equals(operation) || !outcome.equals(resultOutcome)
          || !"completed".equals(status)) throw denied();
      return new SdkGameIntegrationExecutionPermitAuthority.Completion(permit, operation, resultOutcome, status);
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception malformed) { throw denied(); }
  }

  private static UUID assertionJti(String compact) {
    try {
      JsonNode claims = JSON.readTree(com.nimbusds.jose.JWSObject.parse(compact).getPayload().toBytes());
      return canonicalUuid(text(claims, "jti"));
    } catch (Exception invalid) { throw denied(); }
  }

  private static String executionPermitRequestSignature(byte[] key, String path, String timestamp, String nonce,
      byte[] body, String assertion) {
    String message = "v2\nPOST\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex(sha256(body))
        + "\n" + hex(sha256(assertion.getBytes(StandardCharsets.UTF_8)));
    return Base64.getUrlEncoder().withoutPadding().encodeToString(hmac(key, message.getBytes(StandardCharsets.UTF_8)));
  }

  private static URI endpointPath(String rawBaseUrl, String rawPath, boolean allowInternalHttp) {
    if (rawPath == null || !(rawPath.equals("/internal/v1/bindings/challenges")
        || rawPath.startsWith("/internal/v1/bindings/challenges/")
        || rawPath.matches("/internal/v1/game-integrations/bindings/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/execution-permits")
        || rawPath.matches("/internal/v1/game-integrations/execution-permits/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/completion"))
        || rawPath.contains("?") || rawPath.contains("#")) throw denied();
    URI base;
    try { base = URI.create(rawBaseUrl.trim()); }
    catch (RuntimeException invalid) { throw denied(); }
    String scheme = base.getScheme();
    if (base.getHost() == null || base.getRawUserInfo() != null || base.getRawQuery() != null
        || base.getRawFragment() != null || (base.getRawPath() != null && !base.getRawPath().isEmpty()
            && !"/".equals(base.getRawPath()))
        || !("https".equalsIgnoreCase(scheme)
            || (allowInternalHttp && "http".equalsIgnoreCase(scheme) && isInternalHost(base.getHost())))) throw denied();
    try { return new URI(scheme.toLowerCase(java.util.Locale.ROOT), null, base.getHost(), base.getPort(), rawPath, null, null); }
    catch (Exception invalid) { throw denied(); }
  }

  private static SdkBindingChallengeAuthority.Challenge parseBindingChallenge(byte[] raw, UUID expectedChallenge) {
    try {
      JsonNode root = JSON.readTree(raw);
      if (root == null || !root.isObject()) throw denied();
      Set<String> fields = new HashSet<>();
      root.fieldNames().forEachRemaining(fields::add);
      if (!fields.equals(BINDING_CHALLENGE_FIELDS)) throw denied();
      UUID challenge = canonicalUuid(text(root, "challenge_id"));
      UUID app = canonicalUuid(text(root, "application_id"));
      UUID env = canonicalUuid(text(root, "environment_id"));
      UUID device = canonicalUuid(text(root, "device_key_id"));
      UUID operation = canonicalUuid(text(root, "operation_id"));
      String nonce = text(root, "nonce");
      String provider = text(root, "provider");
      String redirectHash = text(root, "redirect_uri_sha256");
      String pkce = text(root, "pkce_challenge");
      String thumbprint = text(root, "device_key_thumbprint");
      Instant expires = Instant.parse(text(root, "expires_at"));
      String status = text(root, "status");
      if ((expectedChallenge != null && !expectedChallenge.equals(challenge)) || !nonce.matches("[A-Za-z0-9_-]{43}")
          || !provider.matches("[a-z][a-z0-9_-]{0,31}") || !redirectHash.matches("[0-9a-f]{64}")
          || !pkce.matches("[A-Za-z0-9_-]{43}") || !thumbprint.matches("[A-Za-z0-9_-]{43}")
          || !"pending".equals(status)) throw denied();
      UUID sourceAccount = canonicalUuid(text(root, "source_account_id"));
      UUID sourceActor = canonicalUuid(text(root, "source_actor_id"));
      UUID sourceDevice = canonicalUuid(text(root, "source_device_id"));
      long sourceGeneration = positiveLong(root, "source_generation");
      UUID targetAccount = canonicalUuid(text(root, "target_account_id"));
      UUID targetProfile = canonicalUuid(text(root, "target_profile_id"));
      long profileRevision = positiveLong(root, "profile_revision");
      long consentRevision = positiveLong(root, "consent_revision");
      long policyRevision = positiveLong(root, "policy_revision");
      List<String> scopes = strings(root, "scopes", false);
      if (!device.equals(sourceDevice) || scopes.stream().sorted().toList().equals(scopes) == false) throw denied();
      return new SdkBindingChallengeAuthority.Challenge(challenge, nonce, app, env, provider, redirectHash, pkce, device, thumbprint,
          operation, expires, status, sourceAccount, sourceActor, sourceDevice, sourceGeneration, targetAccount,
          targetProfile, profileRevision, consentRevision, policyRevision, scopes);
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception malformed) { throw denied(); }
  }

  private static SdkBindingChallengeAuthority.Challenge parseCreatedBindingChallenge(byte[] raw,
      SdkBindingChallengeAuthority.CreateRequest expected) {
    try {
      JsonNode root = JSON.readTree(raw);
      if (root == null || !root.isObject()) throw denied();
      Set<String> fields = new HashSet<>();
      root.fieldNames().forEachRemaining(fields::add);
      if (!fields.equals(BINDING_CHALLENGE_CREATE_RESPONSE_FIELDS)) throw denied();
      UUID challengeId = canonicalUuid(text(root, "challenge_id"));
      String nonce = text(root, "nonce");
      Instant expiresAt = Instant.parse(text(root, "expires_at"));
      if (!nonce.matches("[A-Za-z0-9_-]{43}") || !expiresAt.equals(expected.expiresAt())) throw denied();
      return new SdkBindingChallengeAuthority.Challenge(challengeId, nonce, expected.applicationId(),
          expected.environmentId(), expected.provider(), expected.redirectUriSha256(), expected.pkceChallenge(),
          expected.deviceKeyId(), expected.deviceKeyThumbprint(), expected.operationId(), expiresAt, "pending",
          expected.sourceAccountId(), expected.sourceActorId(), expected.sourceDeviceId(), expected.sourceGeneration(),
          expected.targetAccountId(), expected.targetProfileId(), expected.profileRevision(), expected.consentRevision(),
          expected.policyRevision(), expected.scopes());
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (Exception malformed) { throw denied(); }
  }

  private static URI endpoint(String rawBaseUrl, UUID environmentId, boolean allowInternalHttp) {
    if (rawBaseUrl == null || rawBaseUrl.isBlank() || environmentId == null) throw denied();
    URI base;
    try { base = URI.create(rawBaseUrl.trim()); }
    catch (RuntimeException invalid) { throw denied(); }
    String scheme = base.getScheme();
    if (base.getHost() == null || base.getRawUserInfo() != null || base.getRawQuery() != null
        || base.getRawFragment() != null || (base.getRawPath() != null && !base.getRawPath().isEmpty()
            && !"/".equals(base.getRawPath()))
        || !("https".equalsIgnoreCase(scheme)
            || (allowInternalHttp && "http".equalsIgnoreCase(scheme) && isInternalHost(base.getHost())))) {
      throw denied();
    }
    try {
      return new URI(scheme.toLowerCase(java.util.Locale.ROOT), null, base.getHost(), base.getPort(),
          "/internal/v1/authorizations/environments/" + environmentId, null, null);
    } catch (Exception invalid) {
      throw denied();
    }
  }

  private static boolean isInternalHost(String host) {
    return "gameintegration".equalsIgnoreCase(host) || "localhost".equalsIgnoreCase(host)
        || "127.0.0.1".equals(host) || "::1".equals(host);
  }

  private static final class BoundedBodySubscriber implements HttpResponse.BodySubscriber<byte[]> {
    private final int maxBytes;
    private final CompletableFuture<byte[]> body = new CompletableFuture<>();
    private final ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    private Flow.Subscription subscription;

    private BoundedBodySubscriber(int maxBytes) { this.maxBytes = maxBytes; }

    @Override
    public CompletionStage<byte[]> getBody() { return body; }

    @Override
    public void onSubscribe(Flow.Subscription received) {
      subscription = received;
      received.request(Long.MAX_VALUE);
    }

    @Override
    public void onNext(List<ByteBuffer> chunks) {
      try {
        for (ByteBuffer chunk : chunks) {
          int length = chunk.remaining();
          if (bytes.size() + length > maxBytes) {
            subscription.cancel();
            body.completeExceptionally(new IOException("response body exceeds limit"));
            return;
          }
          byte[] buffer = new byte[length];
          chunk.get(buffer);
          bytes.writeBytes(buffer);
        }
      } catch (RuntimeException failure) {
        subscription.cancel();
        body.completeExceptionally(failure);
      }
    }

    @Override
    public void onError(Throwable failure) { body.completeExceptionally(failure); }

    @Override
    public void onComplete() { body.complete(bytes.toByteArray()); }
  }

  private static byte[] key(String raw) {
    try {
      byte[] decoded = Base64.getDecoder().decode(raw);
      if (decoded.length != 32 || !Base64.getEncoder().encodeToString(decoded).equals(raw)) throw denied();
      return decoded;
    } catch (RuntimeException invalid) {
      throw denied();
    }
  }

  private static String requestSignature(byte[] key, String path, String timestamp, String nonce) {
    byte[] emptyDigest = sha256(new byte[0]);
    String message = "v1\nGET\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex(emptyDigest);
    return Base64.getUrlEncoder().withoutPadding().encodeToString(hmac(key, message.getBytes(StandardCharsets.UTF_8)));
  }

  private static String bodyRequestSignature(byte[] key, String method, String path, String timestamp, String nonce, byte[] body) {
    String message = "v1\n" + method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex(sha256(body));
    return Base64.getUrlEncoder().withoutPadding().encodeToString(hmac(key, message.getBytes(StandardCharsets.UTF_8)));
  }

  private void requireCreateRequest(SdkBindingChallengeAuthority.CreateRequest request) {
    if (request == null || request.applicationId() == null || request.environmentId() == null
        || request.deviceKeyId() == null || request.operationId() == null || request.sourceAccountId() == null
        || request.sourceActorId() == null || request.sourceDeviceId() == null || request.targetAccountId() == null
        || request.targetProfileId() == null || request.sourceGeneration() <= 0 || request.profileRevision() <= 0
        || request.consentRevision() <= 0 || request.policyRevision() <= 0 || request.expiresAt() == null
        || !request.expiresAt().isAfter(clock.instant()) || request.expiresAt().isAfter(clock.instant().plusSeconds(300)) || request.provider() == null
        || !request.provider().matches("[a-z][a-z0-9_-]{0,31}") || request.redirectUriSha256() == null
        || !request.redirectUriSha256().matches("[0-9a-f]{64}") || request.pkceChallenge() == null
        || !request.pkceChallenge().matches("[A-Za-z0-9_-]{43}") || request.deviceKeyThumbprint() == null
        || !request.deviceKeyThumbprint().matches("[A-Za-z0-9_-]{43}") || !request.deviceKeyId().equals(request.sourceDeviceId())
        || request.scopes() == null || request.scopes().isEmpty() || request.scopes().size() > 16
        || !request.scopes().equals(request.scopes().stream().sorted().distinct().toList())) throw denied();
  }

  private static void verifyResponse(byte[] key, String path, String timestamp, String nonce,
      String supplied, byte[] body) {
    if (supplied == null || !supplied.matches("[A-Za-z0-9_-]{43}")) throw denied();
    byte[] provided;
    try { provided = Base64.getUrlDecoder().decode(supplied); }
    catch (RuntimeException invalid) { throw denied(); }
    if (provided.length != 32 || !Base64.getUrlEncoder().withoutPadding().encodeToString(provided).equals(supplied)) {
      throw denied();
    }
    byte[] digest = sha256(body);
    String message = "v1\n200\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex(digest);
    if (!MessageDigest.isEqual(provided, hmac(key, message.getBytes(StandardCharsets.UTF_8)))) throw denied();
  }

  private static Policy parsePolicy(byte[] raw, UUID expectedApp, UUID expectedEnv) {
    try {
      JsonNode root = JSON.readTree(raw);
      if (root == null || !root.isObject()) throw denied();
      Set<String> fields = new HashSet<>();
      root.fieldNames().forEachRemaining(fields::add);
      if (!fields.equals(RESPONSE_FIELDS)) throw denied();
      UUID app = canonicalUuid(text(root, "application_id"));
      UUID env = canonicalUuid(text(root, "environment_id"));
      long revision = positiveLong(root, "revision");
      String displayName = text(root, "display_name");
      if (displayName.isBlank()) throw denied();
      List<String> redirects = strings(root, "redirect_uris", false);
      List<String> origins = strings(root, "allowed_origins", true);
      List<String> providers = strings(root, "providers", false);
      List<String> scopes = strings(root, "player_scopes", false);
      if (!expectedApp.equals(app) || !expectedEnv.equals(env) || !providers.equals(List.of("google"))) throw denied();
      for (String origin : origins) requireHttpsOrigin(origin);
      return new Policy(app, env, revision, displayName, Set.copyOf(redirects), Set.copyOf(scopes),
          Set.copyOf(providers));
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception malformed) {
      throw denied();
    }
  }

  private static List<String> strings(JsonNode root, String field, boolean mayBeEmpty) {
    JsonNode values = root.get(field);
    if (values == null || !values.isArray() || (!mayBeEmpty && values.isEmpty())) throw denied();
    List<String> result = new ArrayList<>();
    Set<String> unique = new HashSet<>();
    for (JsonNode value : values) {
      if (!value.isTextual() || value.textValue().isBlank() || !unique.add(value.textValue())) throw denied();
      result.add(value.textValue());
    }
    return List.copyOf(result);
  }

  private static void requireHttpsOrigin(String value) {
    try {
      URI origin = URI.create(value);
      if (!"https".equals(origin.getScheme()) || origin.getHost() == null || origin.getRawUserInfo() != null
          || origin.getRawPath() != null && !origin.getRawPath().isEmpty()
          || origin.getRawQuery() != null || origin.getRawFragment() != null) throw denied();
    } catch (RuntimeException invalid) { throw denied(); }
  }

  private static UUID canonicalUuid(String value) {
    try {
      UUID id = UUID.fromString(value);
      if (id.equals(new UUID(0, 0)) || !id.toString().equals(value)) throw denied();
      return id;
    } catch (RuntimeException invalid) { throw denied(); }
  }

  private static String text(JsonNode root, String field) {
    JsonNode value = root.get(field);
    if (value == null || !value.isTextual() || value.textValue().isBlank()) throw denied();
    return value.textValue();
  }

  private static long positiveLong(JsonNode root, String field) {
    JsonNode value = root.get(field);
    if (value == null || !value.isIntegralNumber() || !value.canConvertToLong() || value.longValue() <= 0) throw denied();
    return value.longValue();
  }

  private static String uniqueHeader(HttpResponse<?> response, String name) {
    List<String> values = response.headers().allValues(name);
    if (values.size() != 1 || values.getFirst().isBlank()) throw denied();
    return values.getFirst();
  }

  private static void requireHeader(HttpResponse<?> response, String name, String expected) {
    if (!expected.equalsIgnoreCase(uniqueHeader(response, name))) throw denied();
  }

  private static byte[] hmac(byte[] key, byte[] content) {
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(key, "HmacSHA256"));
      return mac.doFinal(content);
    } catch (Exception impossible) {
      throw new IllegalStateException("HMAC-SHA256 unavailable", impossible);
    }
  }

  private static byte[] sha256(byte[] content) {
    try { return java.security.MessageDigest.getInstance("SHA-256").digest(content); }
    catch (Exception impossible) { throw new IllegalStateException("SHA-256 unavailable", impossible); }
  }

  private static String hex(byte[] bytes) { return java.util.HexFormat.of().formatHex(bytes); }
  private static SdkIdentityDeniedException denied() { return new SdkIdentityDeniedException(); }
}
