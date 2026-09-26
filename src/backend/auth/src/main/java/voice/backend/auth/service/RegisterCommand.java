package voice.backend.auth.service;

import java.util.UUID;

public record RegisterCommand(String email, String phone, String password, boolean guest,
                              String deviceInfoJson, UUID registrationIntentId) {
  public RegisterCommand(String email, String phone, String password, boolean guest, String deviceInfoJson) {
    this(email, phone, password, guest, deviceInfoJson, null);
  }
}
