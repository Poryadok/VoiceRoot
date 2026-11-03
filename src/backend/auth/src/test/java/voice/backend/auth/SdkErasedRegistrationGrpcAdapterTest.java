package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

import app.voice.auth.v1.RegisterRequest;
import app.voice.auth.v1.RegisterResponse;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import voice.backend.auth.grpc.AuthGrpcService;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;

class SdkErasedRegistrationGrpcAdapterTest {
  @Test void registrationErasureIsTerminalPreconditionWithoutSession() {
    var auth = mock(AuthService.class);
    when(auth.register(any())).thenThrow(new AuthException("operation_erased"));
    @SuppressWarnings("unchecked") var observer = (StreamObserver<RegisterResponse>) mock(StreamObserver.class);
    new AuthGrpcService(auth, null).register(RegisterRequest.newBuilder()
        .setEmail("erased@example.test").setPassword("strong-password").build(), observer);
    var error = ArgumentCaptor.forClass(Throwable.class);
    verify(observer).onError(error.capture());
    assertThat(Status.fromThrowable(error.getValue()).getCode()).isEqualTo(Status.Code.FAILED_PRECONDITION);
    assertThat(Status.fromThrowable(error.getValue()).getDescription()).isEqualTo("operation_erased");
    verify(observer, never()).onNext(any());
    verify(observer, never()).onCompleted();
  }
}
