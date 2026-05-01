package voice.backend.auth.service;

public record LoginCommand(String email, String phone, String password, String deviceInfoJson) {}
