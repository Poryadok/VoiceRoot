package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;

import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.TransactionCallback;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.TokenClaims;

class SdkTerminalFenceTest {
  private final UUID operation = UUID.randomUUID();
  private final NamedParameterJdbcTemplate jdbc = mock(NamedParameterJdbcTemplate.class);
  private final TransactionTemplate transactions = mock(TransactionTemplate.class);

  SdkTerminalFenceTest() {
    when(transactions.execute(any())).thenAnswer(call ->
        ((TransactionCallback<?>) call.getArgument(0)).doInTransaction(mock(TransactionStatus.class)));
    when(jdbc.queryForObject(anyString(), any(Map.class), eq(Boolean.class))).thenReturn(true);
  }

  @Test
  void trustedBindingCompletionCannotReplayErasedReceipt() {
    var service = new AuthGameBindingHandoffService(jdbc, transactions, null, null, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> service.complete(UUID.randomUUID(), operation, "failed", null))
        .hasMessage("operation_erased");
    verify(jdbc, never()).update(anyString(), any(Map.class));
  }

  @Test
  void trustedWorkloadCanReadTerminalResultEvenAfterOriginalAssertionExpires() {
    var issuer = mock(AuthUserPrincipalIssuer.class);
    var binding = new AuthGameBindingHandoffService(jdbc, transactions, issuer, null, null, null, null, Clock.systemUTC());
    var permits = new AuthGameMessageExecutionPermitService(jdbc, transactions, issuer, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> binding.claim("expired", "old-proof", operation, "a".repeat(64)))
        .hasMessage("operation_erased");
    assertThatThrownBy(() -> permits.issue("expired", operation, "a".repeat(64))).hasMessage("operation_erased");
    verifyNoInteractions(issuer);
  }

  @Test
  void transportRejectsUntrustedWorkloadBeforeAnyFenceLookup() throws Exception {
    var settings = new AuthGameBindingMtlsConfiguration.AuthGameBindingMtlsSettings(true, 9443,
        null, null, null, "spiffe://voice/service/gameintegration", "spiffe://voice/service/messaging");
    var binding = new AuthGameBindingHandoffService(jdbc, transactions, null, null, null, null, null, Clock.systemUTC());
    var permits = new AuthGameMessageExecutionPermitService(jdbc, transactions, null, null, null, null, Clock.systemUTC());
    var bindingFilter = new AuthGameBindingClientIdentityFilter(settings);
    var permitFilter = new AuthGameMessagePermitClientIdentityFilter(settings);
    for (boolean wrongCertificate : new boolean[] {false, true}) {
      var request = new org.springframework.mock.web.MockHttpServletRequest("POST",
          "/internal/v1/auth/game-bindings/handoffs/claim");
      var permitRequest = new org.springframework.mock.web.MockHttpServletRequest("POST",
          "/api/v1/auth/sdk/game-message/execution-permits");
      if (wrongCertificate) {
        var certificate = mock(java.security.cert.X509Certificate.class);
        when(certificate.getSubjectAlternativeNames()).thenReturn(List.of(List.of(6, "spiffe://voice/service/untrusted")));
        var chain = new java.security.cert.X509Certificate[] {certificate};
        request.setAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE, chain);
        permitRequest.setAttribute(AuthGameBindingClientIdentityFilter.CERTIFICATE_ATTRIBUTE, chain);
      }
      var response = new org.springframework.mock.web.MockHttpServletResponse();
      bindingFilter.doFilter(request, response,
          (req, res) -> binding.claim("expired", "proof", operation, "a".repeat(64)));
      org.assertj.core.api.Assertions.assertThat(response.getStatus()).isEqualTo(401);
      var permitResponse = new org.springframework.mock.web.MockHttpServletResponse();
      permitFilter.doFilter(permitRequest, permitResponse,
          (req, res) -> permits.issue("expired", operation, "a".repeat(64)));
      org.assertj.core.api.Assertions.assertThat(permitResponse.getStatus()).isEqualTo(401);
    }
    verifyNoInteractions(jdbc);
  }

  @Test
  void trustedBindingRevocationCannotRecreateErasedReceipt() {
    var service = new AuthGameBindingHandoffService(jdbc, transactions, null, null, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> service.revoke(operation)).hasMessage("operation_erased");
  }

  @Test
  void trustedPermitCompletionDoesNotCallGisAfterErasure() {
    var gis = mock(SdkGameIntegrationExecutionPermitAuthority.class);
    var service = new AuthGameMessageExecutionPermitService(jdbc, transactions, null, null, null, gis, Clock.systemUTC());
    assertThatThrownBy(() -> service.complete(UUID.randomUUID(), operation, "committed"))
        .hasMessage("operation_erased");
    verifyNoInteractions(gis);
  }

  @Test
  void authenticatedConversionAttachGetsTerminalResultWithoutProvisioning() {
    var auth = mock(AuthService.class);
    when(auth.validate("bearer")).thenReturn(new TokenClaims(UUID.randomUUID().toString(), null,
        List.of("user"), "free", Instant.now().plusSeconds(60), "jti", "regular", 1));
    var service = new SdkConversionService(jdbc, transactions, null, null, auth, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> service.attachNewTarget(operation, "bearer")).hasMessage("operation_erased");
  }

  @Test
  void conversionStatusCannotRevealFenceWithoutOriginalProofAuthority() {
    var service = new SdkConversionService(jdbc, transactions, null, null, null, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> service.status(operation, Instant.now().getEpochSecond(), "untrusted"))
        .isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
    verify(jdbc, never()).queryForObject(anyString(), any(Map.class), eq(Boolean.class));
  }

  @Test
  void authenticatedConsentReadsAndApprovalCannotRecoverErasedAuthorization() {
    var auth = mock(AuthService.class);
    when(auth.validate("bearer")).thenReturn(new TokenClaims(UUID.randomUUID().toString(), null,
        List.of("user"), "free", Instant.now().plusSeconds(60), "jti", "regular", 1));
    var service = new SdkAuthorizationService(jdbc, transactions, null, auth, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> service.inspect(operation, "bearer")).hasMessage("operation_erased");
    assertThatThrownBy(() -> service.approve(operation, "bearer", UUID.randomUUID(), 1)).hasMessage("operation_erased");
  }

  @Test
  void missingBearerAndUnverifiedCodeCannotRevealAuthorizationFence() {
    var auth = mock(AuthService.class);
    var service = new SdkAuthorizationService(jdbc, transactions, null, auth, null, null, null, Clock.systemUTC());
    assertThatThrownBy(() -> service.inspect(operation, "invalid"))
        .isInstanceOf(SdkIdentityDeniedException.class);
    verifyNoInteractions(jdbc);
    assertThatThrownBy(() -> service.exchange(operation, "c".repeat(43), "https://example.test", "v".repeat(64), "proof"))
        .isInstanceOf(SdkIdentityDeniedException.class).hasMessage("invalid_sdk_identity");
  }

  @Test
  void registrationIntentCannotBeConsumedAgainAfterErasure() {
    when(jdbc.queryForList(anyString(), any(Map.class))).thenReturn(List.of(Map.of("id", UUID.randomUUID())));
    TransactionSynchronizationManager.setActualTransactionActive(true);
    try {
      assertThatThrownBy(() -> new JdbcSdkRegistrationIntentBinder(jdbc, Clock.systemUTC())
          .bind(operation, UUID.randomUUID())).hasMessage("operation_erased");
      verify(jdbc, never()).update(anyString(), any(Map.class));
    } finally { TransactionSynchronizationManager.setActualTransactionActive(false); }
  }

  @Test
  void terminalHttpResultIsExplicitAndContainsNoReceipt() throws Exception {
    var service = mock(AuthGameBindingHandoffService.class);
    when(service.revoke(operation)).thenThrow(new SdkOperationErasedException());
    var mvc = org.springframework.test.web.servlet.setup.MockMvcBuilders.standaloneSetup(
        new AuthGameBindingHandoffController(service, mock(SdkAuthorizationService.class)))
        .setControllerAdvice(new SdkTerminalFenceExceptionHandler()).build();
    mvc.perform(org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post(
        "/internal/v1/auth/game-bindings/handoffs/revoke")
        .contentType("application/json").content("{\"operation_id\":\"" + operation + "\"}"))
        .andExpect(org.springframework.test.web.servlet.result.MockMvcResultMatchers.status().isConflict())
        .andExpect(org.springframework.test.web.servlet.result.MockMvcResultMatchers.content()
            .json("{\"error\":\"operation_erased\"}", true));
  }
}
