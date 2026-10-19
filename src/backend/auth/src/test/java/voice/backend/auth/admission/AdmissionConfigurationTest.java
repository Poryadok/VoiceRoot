package voice.backend.auth.admission;

import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.Test;
import org.springframework.mock.env.MockEnvironment;

class AdmissionConfigurationTest {
  @Test void disabledWithoutOwnedSettingsIsValidAndPartialConfigurationFailsClosed() {
    var config=new AdmissionConfiguration();
    assertFalse(config.admissionSettings(new MockEnvironment()).enabled());
    assertThrows(IllegalStateException.class,()->config.admissionSettings(new MockEnvironment().withProperty("AUTH_SPACE_ADMISSION_USER_GRPC_ADDR","user:9097")));
    assertThrows(IllegalArgumentException.class,()->config.admissionSettings(new MockEnvironment().withProperty("auth.space-admission.enabled","true")));
    var env=new MockEnvironment().withProperty("auth.space-admission.enabled","true");
    for(String suffix:java.util.List.of("GRPC_ADDR","TLS_CA_FILE","TLS_CERT_FILE","TLS_KEY_FILE","TLS_SERVER_NAME")) env.withProperty("AUTH_SPACE_ADMISSION_USER_"+suffix,"value");
    assertTrue(config.admissionSettings(env).enabled());
  }
}
