package voice.backend.auth.sdkidentity;

import java.util.Map;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@RestControllerAdvice(basePackageClasses = SdkConversionRestController.class)
public class SdkTerminalFenceExceptionHandler {
  @ExceptionHandler(SdkOperationErasedException.class)
  public ResponseEntity<Map<String, String>> erased() {
    return ResponseEntity.status(409).header("Cache-Control", "no-store").body(Map.of("error", "operation_erased"));
  }
}
