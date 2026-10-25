package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.*;

import java.lang.reflect.Method;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import voice.backend.auth.events.AuthObservations;

class AuthObservationsServiceTest {
  private AuthService service(AuthObservations observations) throws Exception {
    Method factory = AuthServiceTest.class.getDeclaredMethod("service", Clock.class);
    factory.setAccessible(true);
    AuthService service = (AuthService) factory.invoke(null,
        Clock.fixed(Instant.parse("2026-05-01T10:00:00Z"), ZoneOffset.UTC));
    service.configureObservations(observations);
    return service;
  }

  @Test
  void registerLoginLogoutEmitExactlyTheirCompletedOperations() throws Exception {
    AuthObservations observations = mock(AuthObservations.class);
    AuthService service = service(observations);
    AuthSession registered = service.register(new RegisterCommand("obs@example.com", null,
        "Correct horse battery staple", false, "{\"ip\":\"spoofed\"}"));
    UUID id = UUID.fromString(registered.accountId());
    verify(observations).registered(id, "guest", "email");
    AuthSession login = service.login(new LoginCommand("obs@example.com", null,
        "Correct horse battery staple", null, "{\"ip\":\"spoofed\"}"));
    verify(observations).loggedIn(id, "{\"ip\":\"spoofed\"}");
    service.logout(new LogoutCommand(login.accessToken(), login.refreshToken()));
    verify(observations).loggedOut(id, "{\"ip\":\"spoofed\"}");
    verifyNoMoreInteractions(observations);
  }

  @Test
  void failedLoginAndFailedLogoutDoNotEmitSuccess() throws Exception {
    AuthObservations observations = mock(AuthObservations.class);
    AuthService service = service(observations);
    AuthSession registered = service.register(new RegisterCommand("failed@example.com", null,
        "Correct horse battery staple", false, "{}"));
    clearInvocations(observations);
    assertThatThrownBy(() -> service.login(new LoginCommand("failed@example.com", null,
        "incorrect password", null, "{}"))).isInstanceOf(AuthException.class);
    assertThatThrownBy(() -> service.logout(new LogoutCommand("bad-token", registered.refreshToken())))
        .isInstanceOf(AuthException.class);
    verifyNoInteractions(observations);
  }

  @Test
  void outerRollbackDiscardsObservationAndCommitDefersIt() throws Exception {
    AuthObservations observations = mock(AuthObservations.class);
    AuthService service = service(observations);
    TransactionSynchronizationManager.initSynchronization();
    TransactionSynchronizationManager.setActualTransactionActive(true);
    try {
      service.register(new RegisterCommand(null, null, "Correct horse battery staple", true, "{}"));
      verifyNoInteractions(observations);
      var callbacks = TransactionSynchronizationManager.getSynchronizations();
      assertThat(callbacks).hasSize(1);
      callbacks.forEach(callback -> callback.afterCompletion(1));
      verifyNoInteractions(observations);
    } finally {
      TransactionSynchronizationManager.clear();
    }
    TransactionSynchronizationManager.initSynchronization();
    TransactionSynchronizationManager.setActualTransactionActive(true);
    try {
      AuthSession session = service.register(new RegisterCommand(null, null, "Correct horse battery staple", true, "{}"));
      verifyNoInteractions(observations);
      TransactionSynchronizationManager.getSynchronizations().forEach(callback -> callback.afterCommit());
      verify(observations).registered(UUID.fromString(session.accountId()), "guest", "guest");
    } finally {
      TransactionSynchronizationManager.clear();
    }
  }

  @Test
  void twoFaEmitsOnlyFirstSuccessfulConfirmationAndRefreshEmitsNoLogin() throws Exception {
    AuthObservations observations = mock(AuthObservations.class);
    AuthService service = service(observations);
    AuthSession registered = service.register(new RegisterCommand(null, null,
        "Correct horse battery staple", true, "{}"));
    var enrollment = service.enable2FA(registered.accessToken(), "Correct horse battery staple");
    String secret = enrollment.totpUri().split("secret=")[1].split("&")[0];
    String code = String.format("%06d", new com.warrenstrange.googleauth.GoogleAuthenticator().getTotpPassword(secret));
    clearInvocations(observations);
    assertThatThrownBy(() -> service.verify2FA(registered.accessToken(), "invalid"))
        .isInstanceOf(AuthException.class);
    verifyNoInteractions(observations);
    AuthSession confirmed = service.verify2FA(registered.accessToken(), code);
    verify(observations).twoFaEnabled(UUID.fromString(registered.accountId()));
    service.verify2FA(confirmed.accessToken(), code);
    service.refresh(new RefreshCommand(confirmed.refreshToken(), "{}"));
    verifyNoMoreInteractions(observations);
  }

  @Test
  void observationRejectionDoesNotChangeSuccessfulSession() throws Exception {
    AuthObservations observations = mock(AuthObservations.class);
    doThrow(new IllegalStateException("queue unavailable")).when(observations).registered(any(), any(), any());
    assertThat(service(observations).register(new RegisterCommand(null, null,
        "Correct horse battery staple", true, "{}"))).isNotNull();
  }
}
