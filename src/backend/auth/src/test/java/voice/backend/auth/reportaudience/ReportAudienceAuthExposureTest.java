package voice.backend.auth.reportaudience;

import static org.assertj.core.api.Assertions.*;
import app.voice.auth.v1.AuthServiceGrpc;
import org.junit.jupiter.api.Test;
import voice.backend.auth.principal.AuthPrincipalServices;

class ReportAudienceAuthExposureTest {
  @Test void ordinaryAuthDoesNotExposePrivateClassification() {
    var all=new AuthServiceGrpc.AuthServiceImplBase(){}.bindService();
    var legacy=AuthPrincipalServices.legacyService(all);
    var methods=legacy.getMethods().stream().map(method->method.getMethodDescriptor().getFullMethodName()).toList();
    assertThat(methods).contains("voice.auth.v1.AuthService/Login");
    assertThat(methods).doesNotContain("voice.auth.v1.AuthService/ClassifyReportAudienceAccounts");
  }
}
