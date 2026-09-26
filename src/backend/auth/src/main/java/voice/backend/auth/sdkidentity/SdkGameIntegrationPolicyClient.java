package voice.backend.auth.sdkidentity;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Authenticated, response-verified read of Game Integration's current SDK policy. */
public final class SdkGameIntegrationPolicyClient implements SdkAuthorizationPolicy {
  private static final int MAX_RESPONSE_BYTES = 65_536;
  private static final ObjectMapper JSON = new ObjectMapper()
      .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
      .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
  private static final Set<String> RESPONSE_FIELDS = Set.of("application_id", "environment_id", "revision",
      "display_name", "redirect_uris", "allowed_origins", "providers", "player_scopes");

  private final String configuredBaseUrl;
  private final String configuredKey;
  private final boolean allowInternalHttp;
  private final Clock clock;
  private final HttpClient http;

  public SdkGameIntegrationPolicyClient(String baseUrl, String keyBase64, boolean allowInternalHttp, Clock clock) {
    this(baseUrl, keyBase64, allowInternalHttp, clock,
        HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2))
            .followRedirects(HttpClient.Redirect.NEVER).build());
  }

  SdkGameIntegrationPolicyClient(String baseUrl, String keyBase64, boolean allowInternalHttp,
      Clock clock, HttpClient http) {
    this.configuredBaseUrl = baseUrl;
    this.configuredKey = keyBase64;
    this.allowInternalHttp = allowInternalHttp;
    this.clock = clock;
    this.http = http;
  }

  @Override
  public Policy resolve(UUID applicationId, UUID environmentId) {
    try {
      if (applicationId == null || environmentId == null || clock == null || http == null) throw denied();
      URI endpoint = endpoint(configuredBaseUrl, environmentId, allowInternalHttp);
      byte[] key = key(configuredKey);
      String path = endpoint.getRawPath();
      String timestamp = Long.toString(clock.instant().getEpochSecond());
      String nonce = UUID.randomUUID().toString();
      HttpRequest request = HttpRequest.newBuilder(endpoint)
          .timeout(Duration.ofSeconds(2))
          .header("X-Voice-Workload", "auth")
          .header("X-Voice-Timestamp", timestamp)
          .header("X-Voice-Nonce", nonce)
          .header("X-Voice-Signature", requestSignature(key, path, timestamp, nonce))
          .GET().build();
      HttpResponse<InputStream> response = http.send(request, HttpResponse.BodyHandlers.ofInputStream());
      try (InputStream body = response.body()) {
        if (response.statusCode() != 200) throw denied();
        requireHeader(response, "Cache-Control", "no-store");
        requireHeader(response, "Content-Type", "application/json");
        requireHeader(response, "X-Voice-Response-Timestamp", timestamp);
        requireHeader(response, "X-Voice-Response-Nonce", nonce);
        String signature = uniqueHeader(response, "X-Voice-Response-Signature");
        byte[] raw = body.readNBytes(MAX_RESPONSE_BYTES + 1);
        if (raw.length > MAX_RESPONSE_BYTES) throw denied();
        verifyResponse(key, path, timestamp, nonce, signature, raw);
        return parsePolicy(raw, applicationId, environmentId);
      }
    } catch (InterruptedException interrupted) {
      Thread.currentThread().interrupt();
      throw denied();
    } catch (SdkIdentityDeniedException denied) {
      throw denied;
    } catch (Exception invalidOrUnavailable) {
      throw denied();
    }
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
      return new Policy(app, env, revision, displayName, Set.copyOf(redirects), Set.copyOf(scopes));
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
