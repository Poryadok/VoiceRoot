package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.nio.charset.StandardCharsets;
import java.security.cert.X509Certificate;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.mock.web.MockFilterChain;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

class AuthGameMessageExecutionPermitRestControllerTest {
  private static final String MESSAGING_SAN = "spiffe://voice/service/messaging";

  @Test
  void issueRequiresTheExactJsonAndOneDeviceAuthorityHeaderAndReturnsNoStore() {
    var service = mock(AuthGameMessageExecutionPermitService.class);
    var controller = new AuthGameMessageExecutionPermitRestController(service);
    var operation = java.util.UUID.randomUUID();
    var request = new MockHttpServletRequest();
    request.addHeader("X-Voice-Device-Authority", "device.assertion.signature");
    when(service.issue("device.assertion.signature", operation, "a".repeat(64)))
        .thenReturn(new AuthGameMessageExecutionPermitService.Permit("header.payload.signature"));

    var response = controller.issue(("{\"operation_id\":\"" + operation
        + "\",\"request_sha256\":\"" + "a".repeat(64) + "\"}").getBytes(StandardCharsets.UTF_8), request);

    assertThat(response.getHeaders().getCacheControl()).isEqualTo("no-store");
    assertThat(response.getBody()).containsEntry("permit_jws", "header.payload.signature");
    verify(service).issue("device.assertion.signature", operation, "a".repeat(64));
  }

  @Test
  void issueRejectsDuplicateUnknownMissingTrailingAndNoncanonicalJsonBeforeCallingService() {
    var service = mock(AuthGameMessageExecutionPermitService.class);
    var controller = new AuthGameMessageExecutionPermitRestController(service);
    String operation = java.util.UUID.randomUUID().toString();
    String valid = "{\"operation_id\":\"" + operation + "\",\"request_sha256\":\"" + "a".repeat(64) + "\"}";
    var request = new MockHttpServletRequest();
    request.addHeader("X-Voice-Device-Authority", "device.assertion.signature");

    for (String body : List.of(
        valid.replace("\",\"request_sha256", "\",\"operation_id\":\"" + operation + "\",\"request_sha256"),
        valid.substring(0, valid.length() - 1) + ",\"extra\":true}",
        "{}", valid + " {}", valid.replace(operation, operation.toUpperCase(java.util.Locale.ROOT)),
        valid.replace("a".repeat(64), "A".repeat(64)))) {
      assertThatThrownBy(() -> controller.issue(body.getBytes(StandardCharsets.UTF_8), request))
          .isInstanceOf(SdkIdentityDeniedException.class);
    }
    verify(service, never()).issue(org.mockito.ArgumentMatchers.anyString(), org.mockito.ArgumentMatchers.any(),
        org.mockito.ArgumentMatchers.anyString());
  }

  @Test
  void issueRejectsRepeatedAssertionHeader() {
    var service = mock(AuthGameMessageExecutionPermitService.class);
    var controller = new AuthGameMessageExecutionPermitRestController(service);
    var request = new MockHttpServletRequest();
    request.addHeader("X-Voice-Device-Authority", "first.assertion");
    request.addHeader("X-Voice-Device-Authority", "second.assertion");
    byte[] body = ("{\"operation_id\":\"" + java.util.UUID.randomUUID()
        + "\",\"request_sha256\":\"" + "a".repeat(64) + "\"}").getBytes(StandardCharsets.UTF_8);

    assertThatThrownBy(() -> controller.issue(body, request)).isInstanceOf(SdkIdentityDeniedException.class);
    verify(service, never()).issue(org.mockito.ArgumentMatchers.anyString(), org.mockito.ArgumentMatchers.any(),
        org.mockito.ArgumentMatchers.anyString());
  }

  @Test
  void completionReturnsThePersistedReceiptAndRejectsInvalidOutcome() {
    var service = mock(AuthGameMessageExecutionPermitService.class);
    var controller = new AuthGameMessageExecutionPermitRestController(service);
    var permit = java.util.UUID.randomUUID();
    var operation = java.util.UUID.randomUUID();
    when(service.complete(permit, operation, "committed"))
        .thenReturn(new AuthGameMessageExecutionPermitService.Completion(permit, operation, "committed", "completed"));
    when(service.complete(permit, operation, "renewed")).thenThrow(new SdkIdentityDeniedException());
    byte[] body = ("{\"operation_id\":\"" + operation + "\",\"outcome\":\"committed\"}")
        .getBytes(StandardCharsets.UTF_8);

    var response = controller.complete(permit.toString(), body);

    assertThat(response.getHeaders().getCacheControl()).isEqualTo("no-store");
    assertThat(response.getBody()).containsEntry("permit_id", permit.toString())
        .containsEntry("operation_id", operation.toString()).containsEntry("status", "completed");
    assertThatThrownBy(() -> controller.complete(permit.toString(),
        new String(body, StandardCharsets.UTF_8).replace("committed", "renewed").getBytes(StandardCharsets.UTF_8)))
        .isInstanceOf(SdkIdentityDeniedException.class);
  }

  @Test
  void privatePermitFilterRequiresExactMessagingUriSan() throws Exception {
    var settings = new AuthGameBindingMtlsConfiguration.AuthGameBindingMtlsSettings(true, 9443, null, null,
        null, "spiffe://voice/service/gameintegration", MESSAGING_SAN);
    var filter = new AuthGameMessagePermitClientIdentityFilter(settings);
    var missing = new MockHttpServletRequest("POST", "/api/v1/auth/sdk/game-message/execution-permits");
    var missingResponse = new MockHttpServletResponse();
    filter.doFilterInternal(missing, missingResponse, new MockFilterChain());
    assertThat(missingResponse.getStatus()).isEqualTo(401);

    var wrong = new MockHttpServletRequest("POST", "/api/v1/auth/sdk/game-message/execution-permits");
    X509Certificate wrongCertificate = certificate("spiffe://voice/service/gameintegration");
    wrong.setAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE, new X509Certificate[] {wrongCertificate});
    var wrongResponse = new MockHttpServletResponse();
    filter.doFilterInternal(wrong, wrongResponse, new MockFilterChain());
    assertThat(wrongResponse.getStatus()).isEqualTo(401);

    var accepted = new MockHttpServletRequest("POST", "/api/v1/auth/sdk/game-message/execution-permits");
    accepted.setAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE,
        new X509Certificate[] {certificate(MESSAGING_SAN)});
    var acceptedResponse = new MockHttpServletResponse();
    var acceptedChain = new MockFilterChain();
    filter.doFilterInternal(accepted, acceptedResponse, acceptedChain);
    assertThat(acceptedResponse.getStatus()).isEqualTo(200);
    assertThat(acceptedChain.getRequest()).isSameAs(accepted);
  }

  private static X509Certificate certificate(String uriSan) throws Exception {
    X509Certificate certificate = mock(X509Certificate.class);
    when(certificate.getSubjectAlternativeNames()).thenReturn(List.of(List.of(6, uriSan)));
    return certificate;
  }
}
