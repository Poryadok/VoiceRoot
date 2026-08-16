package voice.backend.auth.service;

public record ResetPasswordCommand(String email, String code, String newPassword) {}
