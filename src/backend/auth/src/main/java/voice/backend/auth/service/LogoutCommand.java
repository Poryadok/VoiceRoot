package voice.backend.auth.service;

public record LogoutCommand(String accessToken, String refreshToken) {}
