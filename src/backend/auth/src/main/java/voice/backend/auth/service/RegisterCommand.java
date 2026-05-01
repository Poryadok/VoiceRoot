package voice.backend.auth.service;

public record RegisterCommand(String email, String phone, String password, boolean guest, String deviceInfoJson) {}
