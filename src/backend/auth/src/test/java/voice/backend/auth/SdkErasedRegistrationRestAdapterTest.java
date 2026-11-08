package voice.backend.auth;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import voice.backend.auth.rest.AuthRestController;
import voice.backend.auth.service.AuthException;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.OtpService;

class SdkErasedRegistrationRestAdapterTest {
  @Test void registrationErasureIsConflictWithoutSessionOrVerificationMail() throws Exception {
    var auth = mock(AuthService.class);
    var otp = mock(OtpService.class);
    when(auth.register(any())).thenThrow(new AuthException("operation_erased"));
    var mvc = MockMvcBuilders.standaloneSetup(new AuthRestController(auth, null, otp)).build();
    mvc.perform(post("/api/v1/auth/register").contentType("application/json").content("""
        {"email":"erased@example.test","password":"strong-password","guest":false,
         "registrationIntentId":"%s"}
        """.formatted(UUID.randomUUID())))
        .andExpect(status().isConflict())
        .andExpect(header().string("Cache-Control", "no-store"))
        .andExpect(content().json("{\"error\":\"operation_erased\"}", true));
    verifyNoInteractions(otp);
  }
}
