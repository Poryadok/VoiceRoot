package voice.backend.auth.rest;

import java.net.URI;
import java.util.Map;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.MultiValueMap;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import voice.backend.auth.oauth.OAuth2Service;
import voice.backend.auth.oauth.OAuthAuthorizeRequest;
import voice.backend.auth.oauth.OAuthException;
import voice.backend.auth.oauth.OAuthTokenRequest;
import voice.backend.auth.service.LoginCommand;

@RestController
@RequestMapping("/api/v1/auth")
public class OAuth2RestController {
  private final OAuth2Service oauth2Service;

  public OAuth2RestController(OAuth2Service oauth2Service) {
    this.oauth2Service = oauth2Service;
  }

  @GetMapping(value = "/oauth2/authorize", produces = MediaType.TEXT_HTML_VALUE)
  public ResponseEntity<String> authorizeGet(
      @RequestParam(name = "response_type") String responseType,
      @RequestParam(name = "client_id") String clientId,
      @RequestParam(name = "redirect_uri") String redirectUri,
      @RequestParam(name = "state", required = false) String state,
      @RequestParam(name = "code_challenge") String codeChallenge,
      @RequestParam(name = "code_challenge_method") String codeChallengeMethod) {
    OAuthAuthorizeRequest request =
        new OAuthAuthorizeRequest(
            responseType, clientId, redirectUri, state, codeChallenge, codeChallengeMethod);
    String html = oauth2Service.loginFormHtml(request);
    return ResponseEntity.ok().contentType(MediaType.TEXT_HTML).body(html);
  }

  @PostMapping(
      value = "/oauth2/authorize",
      consumes = MediaType.APPLICATION_FORM_URLENCODED_VALUE)
  public ResponseEntity<Void> authorizePost(@RequestParam MultiValueMap<String, String> form) {
    OAuthAuthorizeRequest request = authorizeFromForm(form);
    LoginCommand login =
        new LoginCommand(
            first(form, "email"),
            first(form, "phone"),
            first(form, "password"),
            first(form, "totp_code"),
            "{}");
    URI redirect = oauth2Service.completeAuthorizeAfterLogin(request, login);
    return ResponseEntity.status(HttpStatus.FOUND).header(HttpHeaders.LOCATION, redirect.toString()).build();
  }

  @PostMapping(
      value = "/oauth2/token",
      consumes = MediaType.APPLICATION_FORM_URLENCODED_VALUE,
      produces = MediaType.APPLICATION_JSON_VALUE)
  public Map<String, Object> token(@RequestParam MultiValueMap<String, String> form) {
    OAuthTokenRequest request =
        new OAuthTokenRequest(
            first(form, "grant_type"),
            first(form, "code"),
            first(form, "redirect_uri"),
            first(form, "client_id"),
            first(form, "code_verifier"),
            first(form, "client_secret"));
    var response = oauth2Service.exchangeAuthorizationCode(request);
    return Map.of(
        "access_token", response.accessToken(),
        "token_type", response.tokenType(),
        "expires_in", response.expiresIn());
  }

  @GetMapping(value = "/.well-known/openid-configuration", produces = MediaType.APPLICATION_JSON_VALUE)
  public Map<String, String> openIdConfiguration() {
    return oauth2Service.openIdConfiguration();
  }

  @ExceptionHandler(OAuthException.class)
  public ResponseEntity<Map<String, String>> oauthError(OAuthException ex) {
    return ResponseEntity.status(ex.httpStatus()).body(Map.of("error", ex.error()));
  }

  private static OAuthAuthorizeRequest authorizeFromForm(MultiValueMap<String, String> form) {
    return new OAuthAuthorizeRequest(
        first(form, "response_type"),
        first(form, "client_id"),
        first(form, "redirect_uri"),
        first(form, "state"),
        first(form, "code_challenge"),
        first(form, "code_challenge_method"));
  }

  private static String first(MultiValueMap<String, String> form, String key) {
    if (form == null) {
      return null;
    }
    String value = form.getFirst(key);
    return value == null || value.isBlank() ? null : value;
  }
}
