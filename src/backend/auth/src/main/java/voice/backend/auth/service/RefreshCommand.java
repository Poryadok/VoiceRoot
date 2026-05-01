package voice.backend.auth.service;

public record RefreshCommand(String refreshToken, String deviceInfoJson) {}
