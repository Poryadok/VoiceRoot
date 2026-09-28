package voice.backend.auth.sdkidentity;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.util.Enumeration;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.springframework.http.CacheControl;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import jakarta.servlet.http.HttpServletRequest;

/** T16 issue/completion routes; the exact Messaging mTLS filter protects every method. */
@RestController
@ConditionalOnProperty(prefix = "auth.sdk-authorization", name = "enabled", havingValue = "true")
@RequestMapping("/api/v1/auth/sdk/game-message/execution-permits")
public final class AuthGameMessageExecutionPermitRestController {
  private static final ObjectMapper JSON = new ObjectMapper()
      .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
      .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
  private final AuthGameMessageExecutionPermitService service;

  public AuthGameMessageExecutionPermitRestController(AuthGameMessageExecutionPermitService service) {
    this.service = service;
  }

  @PostMapping(consumes = MediaType.APPLICATION_JSON_VALUE, produces = MediaType.APPLICATION_JSON_VALUE)
  public ResponseEntity<Map<String, String>> issue(@RequestBody byte[] body,
      HttpServletRequest request) {
    JsonNode root = strictObject(body, Set.of("operation_id", "request_sha256"));
    UUID operationId = uuid(root, "operation_id");
    String digest = text(root, "request_sha256");
    if (!digest.matches("[0-9a-f]{64}")) throw deniedError();
    String assertion = uniqueHeader(request, "X-Voice-Device-Authority");
    var permit = service.issue(assertion, operationId, digest);
    return ResponseEntity.ok().cacheControl(CacheControl.noStore()).contentType(MediaType.APPLICATION_JSON)
        .body(Map.of("permit_jws", permit.permitJws()));
  }

  @PostMapping(value = "/{permitJti}/completion", consumes = MediaType.APPLICATION_JSON_VALUE,
      produces = MediaType.APPLICATION_JSON_VALUE)
  public ResponseEntity<Map<String, String>> complete(@PathVariable String permitJti, @RequestBody byte[] body) {
    JsonNode root = strictObject(body, Set.of("operation_id", "outcome"));
    UUID permit = canonicalUuid(permitJti);
    UUID operationId = uuid(root, "operation_id");
    String outcome = text(root, "outcome");
    var completion = service.complete(permit, operationId, outcome);
    return ResponseEntity.ok().cacheControl(CacheControl.noStore()).contentType(MediaType.APPLICATION_JSON)
        .body(Map.of("permit_id", completion.permitId().toString(), "operation_id", completion.operationId().toString(),
            "outcome", completion.outcome(), "status", completion.status()));
  }

  @ExceptionHandler(SdkAuthorizationConflictException.class)
  public ResponseEntity<Void> conflict() { return ResponseEntity.status(HttpStatus.CONFLICT).cacheControl(CacheControl.noStore()).build(); }

  @ExceptionHandler(SdkIdentityDeniedException.class)
  public ResponseEntity<Void> forbidden() { return ResponseEntity.status(HttpStatus.FORBIDDEN).cacheControl(CacheControl.noStore()).build(); }

  private static JsonNode strictObject(byte[] body, Set<String> expected) {
    try {
      if (body == null || body.length == 0 || body.length > 4096) throw deniedError();
      JsonNode root = JSON.readTree(body);
      if (root == null || !root.isObject()) throw deniedError();
      Set<String> names = new HashSet<>();
      root.fieldNames().forEachRemaining(names::add);
      if (!names.equals(expected)) throw deniedError();
      return root;
    } catch (SdkIdentityDeniedException denied) { throw denied; }
    catch (IOException malformed) { throw deniedError(); }
  }

  private static UUID uuid(JsonNode root, String name) { return canonicalUuid(text(root, name)); }

  private static UUID canonicalUuid(String raw) {
    try {
      UUID value = UUID.fromString(raw);
      if (!value.toString().equals(raw) || value.equals(new UUID(0, 0))) throw deniedError();
      return value;
    } catch (RuntimeException malformed) { throw deniedError(); }
  }

  private static String text(JsonNode root, String name) {
    JsonNode value = root.get(name);
    if (value == null || !value.isTextual() || value.textValue().isBlank()) throw deniedError();
    return value.textValue();
  }

  private static String uniqueHeader(HttpServletRequest request, String name) {
    Enumeration<String> values = request.getHeaders(name);
    if (values == null || !values.hasMoreElements()) throw deniedError();
    String value = values.nextElement();
    if (value == null || value.isBlank() || values.hasMoreElements()) throw deniedError();
    return value;
  }

  private static SdkIdentityDeniedException deniedError() { return new SdkIdentityDeniedException(); }
}
